package engine

import (
	"testing"

	"cardwars/internal/card"
)

func TestPlayCreatureSpendsActions(t *testing.T) {
	g := newTestGame(t, 0, []string{"Brute", "Grunt"}, nil)
	g.start(t)
	g.play(t, 0, "Brute", 2)
	if g.State.ActionsLeft != 0 {
		t.Errorf("actions = %d, want 0", g.State.ActionsLeft)
	}
	cr := g.State.Players[0].Lanes[2].Creature
	if cr == nil || cr.CardID != "Brute" || cr.Status != Ready || cr.Damage != 0 {
		t.Fatalf("lane 2 creature = %+v", cr)
	}
	g.mustFail(t, 0, Action{Kind: ActPlayCard, Card: g.uid(t, 0, "Grunt"), Lane: 0}, ErrNoActions)
}

func TestOnlyActivePlayerActs(t *testing.T) {
	g := newTestGame(t, 0, nil, []string{"Grunt"})
	g.start(t)
	g.mustFail(t, 1, Action{Kind: ActPlayCard, Card: g.uid(t, 1, "Grunt"), Lane: 0}, ErrNotYourTurn)
	g.mustFail(t, 1, Action{Kind: ActDraw}, ErrNotYourTurn)
}

func TestCannotPlayBeforeGameStarts(t *testing.T) {
	g := newTestGame(t, 0, []string{"Grunt"}, nil)
	g.mustFail(t, 0, Action{Kind: ActPlayCard, Card: g.uid(t, 0, "Grunt")}, ErrWrongPhase)
}

func TestCostIsLandscapeRequirement(t *testing.T) {
	// Shark costs 2 and is a SandyLands card: needs two face-up SandyLands.
	cat := testCatalog()
	oneSandy := [NumLanes]card.Landscape{card.SandyLands, card.Cornfield, card.Cornfield, card.Cornfield}
	deck := func(ls [NumLanes]card.Landscape) DeckSpec {
		return DeckSpec{ls, append([]string{"Shark", "Pig"}, repeat("Grunt", 10)...)}
	}
	build := func(ls [NumLanes]card.Landscape) *Game {
		g, err := NewGame(cat, Config{Seed: 3, SkipDeckRules: true}, [2]DeckSpec{deck(ls), deck(ls)})
		if err != nil {
			t.Fatal(err)
		}
		g.stackHand(0, []string{"Shark", "Pig"})
		g.start(t)
		return g
	}

	g := build(oneSandy)
	g.mustFail(t, 0, Action{Kind: ActPlayCard, Card: g.uid(t, 0, "Shark"), Lane: 0}, ErrLandscapes)

	g = build(sandy)
	g.play(t, 0, "Shark", 0)

	// Face-down Landscapes provide no type.
	g = build(sandy)
	for i := 0; i < 3; i++ {
		g.State.Players[0].Lanes[i].FaceDown = true
	}
	g.mustFail(t, 0, Action{Kind: ActPlayCard, Card: g.uid(t, 0, "Shark"), Lane: 3}, ErrLandscapes)
}

func TestTwoOneCostCardsNeedOnlyOneLandscape(t *testing.T) {
	// "You can still play two Cornfield Creatures with a 1 Action cost in the
	// same turn, even if you only have 1 Cornfield in play."
	cat := testCatalog()
	one := [NumLanes]card.Landscape{card.Cornfield, card.BluePlains, card.BluePlains, card.BluePlains}
	d := DeckSpec{one, append([]string{"Grunt", "Wall"}, repeat("Filler 0", 2)...)}
	g, err := NewGame(cat, Config{Seed: 1, SkipDeckRules: true}, [2]DeckSpec{d, d})
	if err != nil {
		t.Fatal(err)
	}
	g.stackHand(0, []string{"Grunt", "Wall"})
	g.start(t)
	g.play(t, 0, "Grunt", 0)
	g.play(t, 0, "Wall", 1)
}

func TestRainbowIgnoresLandscapes(t *testing.T) {
	g := newTestGame(t, 0, []string{"Rainbow Giant"}, nil)
	g.start(t)
	for i := range g.State.Players[0].Lanes {
		g.State.Players[0].Lanes[i].FaceDown = true
	}
	g.play(t, 0, "Rainbow Giant", 3) // cost 2, all Landscapes face down, still fine
	if g.State.ActionsLeft != 0 {
		t.Errorf("actions = %d", g.State.ActionsLeft)
	}
}

func TestZeroCostPlayableAfterActionsSpent(t *testing.T) {
	g := newTestGame(t, 0, []string{"Brute", "Free Rat"}, nil)
	g.start(t)
	g.play(t, 0, "Brute", 0)
	if g.State.ActionsLeft != 0 {
		t.Fatal("setup: actions should be spent")
	}
	g.play(t, 0, "Free Rat", 1)
	if g.State.ActionsLeft != 0 {
		t.Errorf("actions = %d", g.State.ActionsLeft)
	}
}

func TestReplaceCreature(t *testing.T) {
	g := newTestGame(t, 0, []string{"Grunt", "Wall"}, nil)
	g.start(t)
	g.play(t, 0, "Grunt", 1)
	g.State.Players[0].Lanes[1].Creature.Damage = 2
	g.play(t, 0, "Wall", 1)

	pl := g.State.Players[0]
	if cr := pl.Lanes[1].Creature; cr.CardID != "Wall" || cr.Damage != 0 {
		t.Errorf("replacement = %+v", cr)
	}
	if len(pl.Discard) != 1 || pl.Discard[0].CardID != "Grunt" {
		t.Errorf("discard = %+v", pl.Discard)
	}
}

func TestCannotReplaceFloopedCreature(t *testing.T) {
	g := newTestGame(t, 0, []string{"Grunt", "Wall"}, nil)
	g.start(t)
	g.play(t, 0, "Grunt", 1)
	g.State.Players[0].Lanes[1].Creature.Status = Flooped
	g.mustFail(t, 0, Action{Kind: ActPlayCard, Card: g.uid(t, 0, "Wall"), Lane: 1}, ErrFlooped)
}

func TestBuildingsShareLaneAndAreReplaceable(t *testing.T) {
	g := newTestGame(t, 0, []string{"Grunt", "Hut", "Hut"}, nil)
	g.start(t)
	g.play(t, 0, "Grunt", 0)
	g.play(t, 0, "Hut", 0) // Buildings sit in the same Lane as a Creature
	lane := g.State.Players[0].Lanes[0]
	if lane.Creature == nil || lane.Building == nil {
		t.Fatalf("lane = %+v", lane)
	}
	g.State.ActionsLeft = 1
	g.play(t, 0, "Hut", 0) // replacing a Building is always allowed
	if got := len(g.State.Players[0].Discard); got != 1 {
		t.Errorf("discard = %d, want old Hut", got)
	}
}

func TestSpellGoesToDiscard(t *testing.T) {
	g := newTestGame(t, 0, []string{"Fizzle"}, nil)
	g.start(t)
	g.play(t, 0, "Fizzle", 0)
	pl := g.State.Players[0]
	if len(pl.Discard) != 1 || pl.Discard[0].CardID != "Fizzle" {
		t.Errorf("discard = %+v", pl.Discard)
	}
	for _, l := range pl.Lanes {
		if l.Creature != nil || l.Building != nil {
			t.Error("a Spell entered play")
		}
	}
}

func TestBadLaneAndUnknownCard(t *testing.T) {
	g := newTestGame(t, 0, []string{"Grunt"}, nil)
	g.start(t)
	g.mustFail(t, 0, Action{Kind: ActPlayCard, Card: g.uid(t, 0, "Grunt"), Lane: 4}, ErrBadLane)
	g.mustFail(t, 0, Action{Kind: ActPlayCard, Card: g.uid(t, 0, "Grunt"), Lane: -1}, ErrBadLane)
	g.mustFail(t, 0, Action{Kind: ActPlayCard, Card: 9999, Lane: 0}, ErrNoSuchCard)
}

func TestDrawSpendsAnAction(t *testing.T) {
	g := newTestGame(t, 0, nil, nil)
	g.start(t)
	hand := len(g.State.Players[0].Hand)
	g.must(t, 0, Action{Kind: ActDraw})
	g.must(t, 0, Action{Kind: ActDraw})
	if got := len(g.State.Players[0].Hand); got != hand+2 || g.State.ActionsLeft != 0 {
		t.Errorf("hand %d (was %d), actions %d", got, hand, g.State.ActionsLeft)
	}
	g.mustFail(t, 0, Action{Kind: ActDraw}, ErrNoActions)
}

func TestDrawFromEmptyDeck(t *testing.T) {
	g := newTestGame(t, 0, nil, nil)
	g.start(t)
	g.State.Players[0].Deck = nil
	g.mustFail(t, 0, Action{Kind: ActDraw}, ErrDeckEmpty)
	if g.State.Phase == PhaseOver {
		t.Error("running out of cards must not end the game")
	}
}
