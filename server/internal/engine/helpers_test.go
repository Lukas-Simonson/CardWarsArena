package engine

import (
	"encoding/json"
	"fmt"
	"testing"

	"cardwars/internal/card"
)

func ip(n int) *int { return &n }

// testCatalog is a small catalog of vanilla cards (no rules text).
func testCatalog() card.Catalog {
	cat := card.Catalog{}
	add := func(c card.Card) { c.ID = c.Name; cat[c.ID] = &c }
	add(card.Card{Name: "Grunt", Type: card.Creature, Landscape: card.Cornfield, Cost: ip(1), Attack: ip(2), Defense: ip(3)})
	add(card.Card{Name: "Brute", Type: card.Creature, Landscape: card.Cornfield, Cost: ip(2), Attack: ip(4), Defense: ip(5)})
	add(card.Card{Name: "Wall", Type: card.Creature, Landscape: card.Cornfield, Cost: ip(1), Attack: ip(0), Defense: ip(6)})
	add(card.Card{Name: "Shark", Type: card.Creature, Landscape: card.SandyLands, Cost: ip(2), Attack: ip(1), Defense: ip(10)})
	add(card.Card{Name: "Pig", Type: card.Creature, Landscape: card.Rainbow, Cost: ip(1), Attack: ip(1), Defense: ip(4)})
	add(card.Card{Name: "Rainbow Giant", Type: card.Creature, Landscape: card.Rainbow, Cost: ip(2), Attack: ip(3), Defense: ip(6)})
	add(card.Card{Name: "Free Rat", Type: card.Creature, Landscape: card.Rainbow, Cost: ip(0), Attack: ip(1), Defense: ip(1)})
	add(card.Card{Name: "Hut", Type: card.Building, Landscape: card.Cornfield, Cost: ip(1)})
	add(card.Card{Name: "Fizzle", Type: card.Spell, Landscape: card.Rainbow, Cost: ip(1)})
	add(card.Card{Name: "Magic Text", Type: card.Creature, Landscape: card.Cornfield, Cost: ip(1), Attack: ip(1), Defense: ip(1), Text: "Does something clever."})
	add(card.Card{Name: "Hero Guy", Type: card.Hero})
	for i := 0; i < 20; i++ {
		add(card.Card{Name: fmt.Sprintf("Filler %d", i), Type: card.Creature, Landscape: card.Cornfield, Cost: ip(1), Attack: ip(1), Defense: ip(2)})
	}
	return cat
}

var cornfields = [NumLanes]card.Landscape{card.Cornfield, card.Cornfield, card.Cornfield, card.Cornfield}
var sandy = [NumLanes]card.Landscape{card.SandyLands, card.SandyLands, card.SandyLands, card.SandyLands}

// repeat builds a card list of n copies of id.
func repeat(id string, n int) []string {
	out := make([]string, n)
	for i := range out {
		out[i] = id
	}
	return out
}

// newTestGame makes a game with relaxed deck rules, both players kept, and
// player 0 on turn 1 (main phase). Each deck is 20 copies of a Grunt unless
// overridden by the player's hand-picked top cards.
func newTestGame(t *testing.T, first PlayerID, top0, top1 []string) *Game {
	t.Helper()
	build := func(top []string) []string {
		cards := append([]string(nil), top...)
		return append(cards, repeat("Grunt", 20)...)
	}
	g, err := NewGame(testCatalog(), Config{Seed: 1, First: first, SkipDeckRules: true},
		[2]DeckSpec{{Landscapes: cornfields, Cards: build(top0)}, {Landscapes: cornfields, Cards: build(top1)}})
	if err != nil {
		t.Fatal(err)
	}
	// Make the "top" lists deterministic: stack them as the opening hand.
	g.stackHand(0, top0)
	g.stackHand(1, top1)
	return g
}

// stackHand puts the named cards into player p's hand (taken from their hand
// or deck), topping the hand up to at least StartingHand cards (test helper).
func (g *Game) stackHand(p PlayerID, ids []string) {
	pl := &g.State.Players[p]
	pool := append(append([]Instance(nil), pl.Hand...), pl.Deck...)
	var hand []Instance
	for _, id := range ids {
		for i, in := range pool {
			if in.CardID == id {
				hand = append(hand, in)
				pool = append(pool[:i], pool[i+1:]...)
				break
			}
		}
	}
	for len(hand) < StartingHand && len(pool) > 0 {
		hand = append(hand, pool[0])
		pool = pool[1:]
	}
	pl.Hand, pl.Deck = hand, pool
}

func (g *Game) must(t *testing.T, p PlayerID, a Action) []Event {
	t.Helper()
	ev, err := g.Apply(p, a)
	if err != nil {
		t.Fatalf("Apply(%d, %s): %v", p, a, err)
	}
	return ev
}

func (g *Game) mustFail(t *testing.T, p PlayerID, a Action, want error) {
	t.Helper()
	if _, err := g.Apply(p, a); err != want {
		t.Fatalf("Apply(%d, %s) error = %v, want %v", p, a, err, want)
	}
}

// start keeps both hands, entering turn 1.
func (g *Game) start(t *testing.T) {
	t.Helper()
	g.must(t, 0, Action{Kind: ActKeepHand})
	g.must(t, 1, Action{Kind: ActKeepHand})
}

// uid returns the UID of the first card named id in p's hand.
func (g *Game) uid(t *testing.T, p PlayerID, id string) int {
	t.Helper()
	for _, in := range g.State.Players[p].Hand {
		if in.CardID == id {
			return in.UID
		}
	}
	t.Fatalf("player %d has no %s in hand", p, id)
	return 0
}

func (g *Game) play(t *testing.T, p PlayerID, id string, lane int) {
	t.Helper()
	g.must(t, p, Action{Kind: ActPlayCard, Card: g.uid(t, p, id), Lane: lane})
}

// String is a canonical dump of the state, for equality checks.
func (g *Game) String() string {
	b, err := json.Marshal(g.State)
	if err != nil {
		return err.Error()
	}
	return string(b)
}

// ownedCount is every card player p owns, wherever it is.
func (g *Game) ownedCount(p PlayerID) int {
	n := 0
	tally := func(in Instance) {
		if in.Owner == p {
			n++
		}
	}
	for q := range g.State.Players {
		pl := g.State.Players[q]
		for _, list := range [][]Instance{pl.Deck, pl.Hand, pl.Discard} {
			for _, in := range list {
				tally(in)
			}
		}
		for _, l := range pl.Lanes {
			if l.Creature != nil {
				tally(l.Creature.Instance)
				for _, in := range l.Creature.Under {
					tally(in)
				}
			}
			for _, b := range l.buildings() {
				tally(b.Instance)
			}
		}
	}
	if pe := g.State.Pending; pe != nil {
		tally(pe.Card)
	}
	for _, ch := range append(append([]Choice{}, g.State.Queue...), ptrVal(g.State.Choice)...) {
		if ch.Kind != ChoiceReveal && ch.Kind != ChoiceCamel {
			continue // other choices list cards that sit in a deck or on the board
		}
		for _, in := range ch.Revealed {
			tally(in)
		}
	}
	return n
}

// give puts a new copy of a card into p's hand (test helper).
func (g *Game) give(p PlayerID, id string) int {
	in := g.newInstance(id, p)
	g.State.Players[p].Hand = append(g.State.Players[p].Hand, in)
	return in.UID
}
