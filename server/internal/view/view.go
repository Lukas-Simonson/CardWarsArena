// Package view builds what one player is allowed to see of a game. A client
// is only ever sent a View, never the engine state: the opponent's hand and
// both decks are absent from it, not merely hidden. The one exception is the
// Preload list of card ids, which lets a client fetch card art early and so
// names the opponent's hand and the viewer's next draws, unlabelled.
package view

import (
	"sort"

	"cardwars/internal/card"
	"cardwars/internal/engine"
)

// Creature is a Creature in play with its current, computed stats.
type Creature struct {
	engine.Instance
	Damage       int    `json:"damage"`
	Status       string `json:"status"` // ready, flooped, fought
	Attack       int    `json:"attack"`
	Defense      int    `json:"defense"`
	Protected    bool   `json:"protected,omitempty"` // cannot be attacked right now
	Stolen       bool   `json:"stolen,omitempty"`    // controlled by someone other than its owner
	Silenced     bool   `json:"silenced,omitempty"`
	Lent         string `json:"lent,omitempty"`
	ChangedLanes bool   `json:"changed_lanes,omitempty"`
	// Types, when set, are the Landscape types it has now instead of its card's own.
	Types []card.Landscape `json:"types,omitempty"`
}

// Building is a Building in play.
type Building struct {
	engine.Instance
	Status string `json:"status"`
}

// Lane is one side of a Lane.
type Lane struct {
	Landscape card.Landscape `json:"landscape"`
	FaceDown  bool           `json:"face_down"`
	Creature  *Creature      `json:"creature,omitempty"`
	Building  *Building      `json:"building,omitempty"`
	// Extra are the Buildings here besides Building (Burly Lumberjack).
	Extra []Building `json:"extra,omitempty"`
}

// Player is one seat. Hand is only filled in for the viewer's own seat.
type Player struct {
	HP int `json:"hp"`
	// Hero is the Hero card that came with the deck; its abilities appear in Legal with source "hero".
	Hero      *engine.Instance  `json:"hero,omitempty"`
	DeckCount int               `json:"deck_count"`
	HandCount int               `json:"hand_count"`
	Hand      []engine.Instance `json:"hand,omitempty"`
	// RainbowHand lists the UIDs of the cards in Hand that are Rainbow cards this turn.
	RainbowHand []int                 `json:"rainbow_hand,omitempty"`
	Discard     []engine.Instance     `json:"discard"` // public
	Lanes       [engine.NumLanes]Lane `json:"lanes"`
	Kept        bool                  `json:"kept"`
	Mulliganed  bool                  `json:"mulliganed"`
}

// Choice is a decision some player must make before the game goes on. Both
// seats see it (an opponent's discard is public), and Revealed cards are shown
// to both.
type Choice struct {
	Player   int               `json:"player"` // who decides
	Kind     string            `json:"kind"`   // discard, reveal or may
	Card     string            `json:"card"`   // the card that asks
	Count    int               `json:"count,omitempty"`
	Revealed []engine.Instance `json:"revealed,omitempty"`
}

// View is a complete picture of the game for one viewer.
type View struct {
	You            int              `json:"you"`
	Phase          engine.Phase     `json:"phase"`
	Turn           int              `json:"turn"`
	Active         int              `json:"active"`
	First          int              `json:"first"`
	ActionsLeft    int              `json:"actions_left"`
	SpellsThisTurn int              `json:"spells_this_turn"`
	Over           bool             `json:"over"`
	Winner         *int             `json:"winner,omitempty"`
	Awaiting       []int            `json:"awaiting"`
	Players        [2]Player        `json:"players"` // indexed by seat
	Pending        *engine.Instance `json:"pending,omitempty"`
	Choice         *Choice          `json:"choice,omitempty"`
	// Preload names cards whose art the client may fetch ahead of time: the
	// opponent's hand and the viewer's next draws, mixed together and sorted so
	// nothing says which is which. See docs/protocol.md for what this gives away.
	Preload []string `json:"preload"`
	// Legal lists what the viewer may do right now (empty when it is not
	// their decision). Long target lists are capped; see docs/protocol.md.
	Legal []engine.Action `json:"legal"`
}

// Build returns the game as seen by viewer.
func Build(g *engine.Game, viewer engine.PlayerID) View {
	st := &g.State
	v := View{
		You:            int(viewer),
		Phase:          st.Phase,
		Turn:           st.Turn,
		Active:         int(st.Active),
		First:          int(st.First),
		ActionsLeft:    st.ActionsLeft,
		SpellsThisTurn: st.SpellsThisTurn,
		Over:           st.Phase == engine.PhaseOver,
		Awaiting:       []int{},
		Legal:          []engine.Action{},
		Preload:        preloadHints(st, viewer),
	}
	if v.Over {
		w := int(st.Winner)
		v.Winner = &w
	}
	for _, p := range g.Awaiting() {
		v.Awaiting = append(v.Awaiting, int(p))
		if p == viewer {
			for _, a := range g.LegalActions(viewer) {
				v.Legal = append(v.Legal, noNullSlots(a))
			}
		}
	}
	if st.Pending != nil {
		in := st.Pending.Card
		v.Pending = &in
	}
	if ch := st.Choice; ch != nil {
		v.Choice = &Choice{Player: int(ch.Player), Kind: ch.Kind, Card: ch.Card, Count: ch.Count, Revealed: append([]engine.Instance(nil), ch.Revealed...)}
		if ch.Kind == engine.ChoiceSearch && ch.Player != viewer {
			v.Choice.Revealed = nil // what is in a deck is not for the other player to see
		}
	}
	for p := engine.PlayerID(0); p < 2; p++ {
		src := &st.Players[p]
		pv := Player{
			HP:         src.HP,
			DeckCount:  len(src.Deck),
			HandCount:  len(src.Hand),
			Discard:    append([]engine.Instance{}, src.Discard...),
			Kept:       src.Kept,
			Mulliganed: src.Mulliganed,
		}
		if src.Hero.CardID != "" {
			h := src.Hero
			pv.Hero = &h
		}
		if p == viewer {
			pv.Hand = append([]engine.Instance{}, src.Hand...)
			for _, in := range src.Hand {
				if g.IsRainbowInHand(in.UID) {
					pv.RainbowHand = append(pv.RainbowHand, in.UID)
				}
			}
		}
		for i, l := range src.Lanes {
			lv := Lane{Landscape: l.Landscape, FaceDown: l.FaceDown}
			if c := l.Creature; c != nil {
				lv.Creature = &Creature{
					Instance:     c.Instance,
					Damage:       c.Damage,
					Status:       string(c.Status),
					Attack:       g.Attack(p, i),
					Defense:      g.Defense(p, i),
					Protected:    !g.CanBeAttacked(p, i),
					Stolen:       c.Owner != p,
					Silenced:     c.Silenced,
					Lent:         c.Lent,
					ChangedLanes: c.ChangedLanes,
				}
				if types := g.CreatureTypes(p, i); len(types) != 1 || types[0] != g.Card(c.Instance).Landscape {
					lv.Creature.Types = types
				}
			}
			if b := l.Building; b != nil {
				lv.Building = &Building{Instance: b.Instance, Status: string(b.Status)}
			}
			for _, b := range l.Extra {
				lv.Extra = append(lv.Extra, Building{Instance: b.Instance, Status: string(b.Status)})
			}
			pv.Lanes[i] = lv
		}
		v.Players[p] = pv
	}
	return v
}

// noNullSlots makes every target slot a real (possibly empty) list. Go encodes
// a nil slice as null, which clients should never have to special-case.
func noNullSlots(a engine.Action) engine.Action {
	fix := func(slots [][]engine.TargetRef) [][]engine.TargetRef {
		if slots == nil {
			return nil
		}
		out := make([][]engine.TargetRef, len(slots))
		for i, s := range slots {
			if s == nil {
				s = []engine.TargetRef{}
			}
			out[i] = s
		}
		return out
	}
	a.Targets = fix(a.Targets)
	a.CostTargets = fix(a.CostTargets)
	return a
}

// Event is an engine event as sent to a client. CardID names the card, so a
// client can describe cards it never sees in a view (a Creature that entered
// play and went straight back to a hand).
type Event struct {
	engine.Event
	CardID string `json:"card_id,omitempty"`
}

// FilterEvents returns the events a viewer may see. An opponent's draws say
// nothing about which card was drawn; every other card an event mentions has
// been shown to both players, so it is named.
func FilterEvents(g *engine.Game, viewer engine.PlayerID, events []engine.Event) []Event {
	names := g.Instances()
	out := make([]Event, len(events))
	for i, e := range events {
		if e.Kind == engine.EvDraw && e.Player != viewer {
			e.Card = 0
		}
		out[i] = Event{Event: e}
		if e.Card != 0 {
			out[i].CardID = names[e.Card]
		}
	}
	return out
}

// preloadTop is how many of the viewer's next draws are hinted.
const preloadTop = 3

// preloadHints is the set of card ids behind the opponent's hand and the
// viewer's top cards of deck, sorted and without repeats.
func preloadHints(st *engine.State, viewer engine.PlayerID) []string {
	seen := map[string]bool{}
	for _, in := range st.Players[viewer.Opp()].Hand {
		seen[in.CardID] = true
	}
	deck := st.Players[viewer].Deck
	for i := 0; i < preloadTop && i < len(deck); i++ {
		seen[deck[i].CardID] = true
	}
	out := make([]string, 0, len(seen))
	for id := range seen {
		out = append(out, id)
	}
	sort.Strings(out)
	return out
}
