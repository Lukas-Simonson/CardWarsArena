package engine

import (
	"fmt"
	"math/rand/v2"

	"cardwars/internal/card"
)

const (
	MinDeckSize = 40
	MaxCopies   = 3
)

// DeckSpec is one player's deck: four Landscapes plus the draw deck.
type DeckSpec struct {
	Landscapes [NumLanes]card.Landscape
	Cards      []string // card ids, one entry per copy
	Hero       string   // the Hero's card id; empty for none (tests)
}

// Config describes a new game.
type Config struct {
	Seed  uint64
	First PlayerID // who takes the first turn
	// AllowUnimplemented lets decks include cards whose rules text the engine
	// cannot play yet. Their text is ignored. For simulation and tests only.
	AllowUnimplemented bool
	// SkipDeckRules disables the 40-card minimum and 3-copy limit (tests).
	SkipDeckRules bool
}

// Game is a game in progress. Mutate it only through Apply.
type Game struct {
	State State

	cat card.Catalog
	rng rand.PCG
	cfg Config

	milledCost   int // the cost of the card last milled, for Green Merman
	milledSpells int // how many Spells the last mill discarded (Skeletal Hand)
	heroDamage   int // damage a Creature just dealt to a player (Snakemint)
	destroyed    int // how many Creatures the last destroy_creature op destroyed (Cute Overload)
}

// NewGame validates the decks, shuffles, deals opening hands and returns a
// Game in the mulligan phase.
func NewGame(cat card.Catalog, cfg Config, decks [2]DeckSpec) (*Game, error) {
	g := &Game{cat: cat, cfg: cfg, rng: *rand.NewPCG(cfg.Seed, cfg.Seed^0x9e3779b97f4a7c15)}
	for p := range decks {
		if err := g.checkDeck(decks[p]); err != nil {
			return nil, fmt.Errorf("player %d deck: %w", p, err)
		}
	}
	g.State = State{First: cfg.First, Active: cfg.First, Phase: PhaseMulligan}
	for p := range decks {
		pl := &g.State.Players[p]
		pl.HP = StartingHP
		if id := decks[p].Hero; id != "" {
			pl.Hero = g.newInstance(id, PlayerID(p))
		}
		for i, ls := range decks[p].Landscapes {
			pl.Lanes[i].Landscape = ls
		}
		for _, id := range decks[p].Cards {
			pl.Deck = append(pl.Deck, g.newInstance(id, PlayerID(p)))
		}
		g.shuffle(pl.Deck)
		g.draw(PlayerID(p), StartingHand)
	}
	return g, nil
}

func (g *Game) checkDeck(d DeckSpec) error {
	if d.Hero != "" {
		h, ok := g.cat[d.Hero]
		switch {
		case !ok:
			return fmt.Errorf("unknown Hero %q", d.Hero)
		case h.Type != card.Hero:
			return fmt.Errorf("%s is not a Hero", h.Name)
		case !g.cfg.AllowUnimplemented && !h.Implemented():
			return fmt.Errorf("%s has rules text the engine does not implement yet", h.Name)
		}
	}
	for _, ls := range d.Landscapes {
		if ls == "" || ls == card.Rainbow {
			return fmt.Errorf("invalid Landscape %q", ls)
		}
	}
	if !g.cfg.SkipDeckRules && len(d.Cards) < MinDeckSize {
		return fmt.Errorf("%d cards, minimum is %d", len(d.Cards), MinDeckSize)
	}
	copies := map[string]int{}
	for _, id := range d.Cards {
		c, ok := g.cat[id]
		if !ok {
			return fmt.Errorf("unknown card %q", id)
		}
		if c.Type == card.Hero {
			return fmt.Errorf("%s is a Hero and cannot be in the deck", c.Name)
		}
		if !g.cfg.AllowUnimplemented && !c.Implemented() {
			return fmt.Errorf("%s has rules text the engine does not implement yet", c.Name)
		}
		copies[id]++
		if !g.cfg.SkipDeckRules && copies[id] > MaxCopies {
			return fmt.Errorf("more than %d copies of %s", MaxCopies, c.Name)
		}
	}
	return nil
}

// Clone returns an independent copy, for AI search and what-if checks.
func (g *Game) Clone() *Game {
	c := *g
	for p := range c.State.Players {
		src := &g.State.Players[p]
		dst := &c.State.Players[p]
		dst.HeroFired = append([]int(nil), src.HeroFired...)
		dst.Deck = append([]Instance(nil), src.Deck...)
		dst.Hand = append([]Instance(nil), src.Hand...)
		dst.Discard = append([]Instance(nil), src.Discard...)
		for i := range src.Lanes {
			if cr := src.Lanes[i].Creature; cr != nil {
				v := *cr
				v.Fired = append([]int(nil), cr.Fired...)
				v.Offered = append([]Offer(nil), cr.Offered...)
				dst.Lanes[i].Creature = &v
			}
			if b := src.Lanes[i].Building; b != nil {
				v := *b
				dst.Lanes[i].Building = &v
			}
			dst.Lanes[i].Extra = nil
			for _, b := range src.Lanes[i].Extra {
				v := *b
				dst.Lanes[i].Extra = append(dst.Lanes[i].Extra, &v)
			}
		}
	}
	if g.State.Choice != nil {
		ch := *g.State.Choice
		ch.Revealed = append([]Instance(nil), ch.Revealed...)
		ch.Lanes = append([]int(nil), ch.Lanes...)
		c.State.Choice = &ch
	}
	c.State.Queue = append([]Choice(nil), g.State.Queue...)
	for i := range c.State.Queue {
		c.State.Queue[i].Revealed = append([]Instance(nil), c.State.Queue[i].Revealed...)
		c.State.Queue[i].Lanes = append([]int(nil), c.State.Queue[i].Lanes...)
	}
	c.State.Played = append([]PlayedCard(nil), g.State.Played...)
	c.State.RainbowHand = append([]int(nil), g.State.RainbowHand...)
	return &c
}

// Card returns the catalog entry for an instance.
func (g *Game) Card(i Instance) *card.Card { return g.cat[i.CardID] }

func (g *Game) newInstance(id string, owner PlayerID) Instance {
	g.State.NextUID++
	return Instance{UID: g.State.NextUID, CardID: id, Owner: owner}
}

func (g *Game) shuffle(s []Instance) {
	rand.New(&g.rng).Shuffle(len(s), func(i, j int) { s[i], s[j] = s[j], s[i] })
}

// draw moves up to n cards from the top of p's deck to their hand and returns
// how many were drawn. An empty deck is not a loss.
func (g *Game) draw(p PlayerID, n int) []Instance {
	pl := &g.State.Players[p]
	if n > len(pl.Deck) {
		n = len(pl.Deck)
	}
	drawn := append([]Instance(nil), pl.Deck[:n]...)
	pl.Deck = pl.Deck[n:]
	pl.Hand = append(pl.Hand, drawn...)
	g.State.Drawn[p] += n
	return drawn
}

// Apply performs a player's action and returns what happened. The action is
// fully validated first, so an error leaves the game untouched.
func (g *Game) Apply(p PlayerID, a Action) ([]Event, error) {
	if err := g.Validate(p, a); err != nil {
		return nil, err
	}
	if !g.usesOffer(p, a) {
		g.clearOffers() // anything else the player does lets the offers pass
	}
	var ev []Event
	switch a.Kind {
	case ActKeepHand:
		g.State.Players[p].Kept = true
		ev = append(ev, Event{Kind: EvKeepHand, Player: p})
	case ActMulligan:
		ev = append(ev, g.mulligan(p)...)
	case ActPlayCard:
		ev = append(ev, g.doPlay(p, a)...)
	case ActDraw:
		ev = append(ev, g.doDraw(p)...)
	case ActEndMain:
		ev = append(ev, g.doEndMain(p)...)
	case ActAttack:
		ev = append(ev, g.doAttack(p, a)...)
	case ActFloop, ActActivate:
		ev = append(ev, g.doAbility(p, a)...)
	case ActPlayPending:
		ev = append(ev, g.doPending(p, a)...)
	case ActChoose:
		ev = append(ev, g.doChoice(p, a)...)
	case ActThaw:
		ev = append(ev, g.doThaw(p, a)...)
	}
	// State-based rules hold after every action, not just after combat.
	ev = append(ev, g.settle()...)
	if g.State.Phase == PhaseMulligan && g.State.Players[0].Kept && g.State.Players[1].Kept {
		ev = append(ev, g.startTurn(g.State.First)...)
	}
	return ev, nil
}

// Validate reports why p may not perform a, or nil if it is legal right now.
func (g *Game) Validate(p PlayerID, a Action) error {
	st := &g.State
	if st.Phase == PhaseOver {
		return ErrGameOver
	}
	if p != 0 && p != 1 {
		return ErrNotYourTurn
	}
	if st.Choice != nil {
		if a.Kind != ActChoose {
			return ErrChoice
		}
		return g.validateChoice(p, a)
	}
	if a.Kind == ActChoose {
		return ErrNoChoice
	}
	if st.Pending != nil && a.Kind != ActPlayPending {
		return ErrPending
	}
	switch a.Kind {
	case ActKeepHand, ActMulligan:
		if st.Phase != PhaseMulligan {
			return ErrWrongPhase
		}
		pl := &st.Players[p]
		if pl.Kept {
			return ErrAlreadyDecided
		}
		if a.Kind == ActMulligan && pl.Mulliganed {
			return ErrNoMulligan
		}
		return nil
	case ActPlayCard:
		return g.validatePlay(p, a)
	case ActDraw:
		return g.validateDraw(p)
	case ActEndMain:
		return g.validateEndMain(p)
	case ActAttack:
		return g.validateAttack(p, a)
	case ActFloop, ActActivate:
		return g.validateAbility(p, a)
	case ActPlayPending:
		return g.validatePending(p, a)
	case ActThaw:
		return g.validateThaw(p, a)
	}
	return ErrUnknownAction
}

// mulligan returns p's hand to the deck, reshuffles all cards, and draws a new
// hand of five. It can be done once, and ends the player's decision.
func (g *Game) mulligan(p PlayerID) []Event {
	pl := &g.State.Players[p]
	pl.Deck = append(pl.Deck, pl.Hand...)
	pl.Hand = nil
	g.shuffle(pl.Deck)
	g.draw(p, StartingHand)
	pl.Mulliganed = true
	pl.Kept = true
	return []Event{{Kind: EvMulligan, Player: p}}
}

// startTurn begins p's turn: Ready all cards, draw a card, receive 2 Actions.
func (g *Game) startTurn(p PlayerID) []Event {
	st := &g.State
	g.clearOffers()
	st.Drawn = [2]int{}
	st.Turn++
	st.Active = p
	st.Phase = PhaseMain
	st.ActionsLeft = ActionsPerTurn
	pl := &st.Players[p]
	for i := range pl.Lanes {
		if c := pl.Lanes[i].Creature; c != nil {
			if !hasRestrict(g.effectsOf(c), card.RestrictNoReady) {
				c.Status = Ready // Fatigued Librarian stays exhausted until it changes Lanes
			}
			c.Shielded = false // "until the start of your next turn"
		}
		if b := pl.Lanes[i].Building; b != nil {
			b.Status = Ready
		}
	}
	ev := []Event{{Kind: EvTurnStart, Player: p}}
	for _, c := range g.draw(p, 1) {
		ev = append(ev, Event{Kind: EvDraw, Player: p, Card: c.UID})
	}
	return append(ev, g.fireTurnStart(p)...)
}

// Awaiting lists the players who must make a decision now: both undecided
// players during the mulligan phase, otherwise the active player. It is empty
// when the game is over.
func (g *Game) Awaiting() []PlayerID {
	st := &g.State
	switch {
	case st.Phase == PhaseOver:
		return nil
	case st.Choice != nil:
		return []PlayerID{st.Choice.Player}
	case st.Phase == PhaseMulligan:
		var out []PlayerID
		for p := PlayerID(0); p < 2; p++ {
			if !st.Players[p].Kept {
				out = append(out, p)
			}
		}
		return out
	}
	return []PlayerID{st.Active}
}
