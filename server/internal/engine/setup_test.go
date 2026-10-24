package engine

import (
	"fmt"
	"testing"

	"cardwars/internal/card"
)

func TestNewGameDealsHands(t *testing.T) {
	g := newTestGame(t, 0, nil, nil)
	for p, pl := range g.State.Players {
		if len(pl.Hand) != StartingHand || len(pl.Deck) != 20-StartingHand {
			t.Errorf("player %d: hand %d deck %d", p, len(pl.Hand), len(pl.Deck))
		}
		if pl.HP != StartingHP {
			t.Errorf("player %d HP = %d", p, pl.HP)
		}
	}
	if g.State.Phase != PhaseMulligan {
		t.Errorf("phase = %s", g.State.Phase)
	}
}

func TestSeedIsDeterministic(t *testing.T) {
	a := newTestGame(t, 0, nil, nil)
	b := newTestGame(t, 0, nil, nil)
	if a.String() != b.String() {
		t.Error("same seed produced different games")
	}
}

func TestMulliganOnceThenTurnStarts(t *testing.T) {
	g := newTestGame(t, 0, nil, nil)
	g.must(t, 0, Action{Kind: ActMulligan})
	pl := g.State.Players[0]
	if len(pl.Hand) != StartingHand || len(pl.Deck) != 20-StartingHand {
		t.Errorf("after mulligan: hand %d deck %d", len(pl.Hand), len(pl.Deck))
	}
	g.mustFail(t, 0, Action{Kind: ActMulligan}, ErrAlreadyDecided)
	g.mustFail(t, 0, Action{Kind: ActKeepHand}, ErrAlreadyDecided)
	if g.State.Phase != PhaseMulligan {
		t.Fatal("game started before player 1 decided")
	}
	g.must(t, 1, Action{Kind: ActKeepHand})
	st := g.State
	if st.Phase != PhaseMain || st.Active != 0 || st.Turn != 1 || st.ActionsLeft != 2 {
		t.Errorf("phase=%s active=%d turn=%d actions=%d", st.Phase, st.Active, st.Turn, st.ActionsLeft)
	}
	if len(st.Players[0].Hand) != StartingHand+1 {
		t.Errorf("first player draws on turn 1: hand = %d", len(st.Players[0].Hand))
	}
	if len(st.Players[1].Hand) != StartingHand {
		t.Errorf("second player hand = %d", len(st.Players[1].Hand))
	}
}

func TestMulliganReshufflesWholeDeck(t *testing.T) {
	// A mulligan puts the hand back and reshuffles all cards together, so no
	// card is lost or duplicated.
	g := newTestGame(t, 0, nil, nil)
	before := len(g.State.Players[0].Hand) + len(g.State.Players[0].Deck)
	g.must(t, 0, Action{Kind: ActMulligan})
	after := len(g.State.Players[0].Hand) + len(g.State.Players[0].Deck)
	if before != after {
		t.Errorf("cards %d -> %d", before, after)
	}
}

func TestSecondPlayerCanGoFirst(t *testing.T) {
	g := newTestGame(t, 1, nil, nil)
	g.start(t)
	if g.State.Active != 1 || g.State.First != 1 {
		t.Errorf("active=%d first=%d, want 1", g.State.Active, g.State.First)
	}
}

func TestDeckRules(t *testing.T) {
	cat := testCatalog()
	rainbow := [NumLanes]card.Landscape{card.Rainbow, card.Rainbow, card.Rainbow, card.Rainbow}
	relaxed := Config{SkipDeckRules: true}
	bad := map[string]struct {
		cfg  Config
		deck DeckSpec
	}{
		"too small":         {Config{}, DeckSpec{cornfields, repeat("Grunt", 3)}},
		"four copies":       {Config{}, DeckSpec{cornfields, append(repeat("Grunt", 4), fillers(36)...)}},
		"unknown card":      {relaxed, DeckSpec{cornfields, []string{"Nope"}}},
		"hero in deck":      {relaxed, DeckSpec{cornfields, []string{"Hero Guy"}}},
		"unimplemented":     {relaxed, DeckSpec{cornfields, []string{"Magic Text"}}},
		"rainbow landscape": {relaxed, DeckSpec{rainbow, []string{"Grunt"}}},
	}
	for name, tc := range bad {
		if _, err := NewGame(cat, tc.cfg, [2]DeckSpec{tc.deck, tc.deck}); err == nil {
			t.Errorf("%s: expected an error", name)
		}
	}

	allow := Config{SkipDeckRules: true, AllowUnimplemented: true}
	text := DeckSpec{cornfields, []string{"Magic Text"}}
	if _, err := NewGame(cat, allow, [2]DeckSpec{text, text}); err != nil {
		t.Errorf("AllowUnimplemented: %v", err)
	}
	legal := DeckSpec{cornfields, append(repeat("Grunt", 3), fillers(37)...)}
	if _, err := NewGame(cat, Config{}, [2]DeckSpec{legal, legal}); err != nil {
		t.Errorf("legal 40-card deck rejected: %v", err)
	}
}

// fillers returns n distinct-enough filler cards (at most 2 copies each).
func fillers(n int) []string {
	var out []string
	for i := 0; len(out) < n; i++ {
		out = append(out, fmt.Sprintf("Filler %d", i%20))
	}
	return out
}

func TestCloneIsIndependent(t *testing.T) {
	g := newTestGame(t, 0, nil, nil)
	c := g.Clone()
	g.must(t, 0, Action{Kind: ActKeepHand})
	if c.State.Players[0].Kept {
		t.Error("clone shares state with original")
	}
	c.State.Players[0].Hand[0].CardID = "x"
	if g.State.Players[0].Hand[0].CardID == "x" {
		t.Error("clone shares hand slice")
	}
}
