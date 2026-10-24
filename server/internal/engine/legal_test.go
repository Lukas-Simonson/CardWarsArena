package engine

import (
	"reflect"
	"testing"
)

func kinds(as []Action) map[ActionKind]int {
	m := map[ActionKind]int{}
	for _, a := range as {
		m[a.Kind]++
	}
	return m
}

func TestLegalActionsMulligan(t *testing.T) {
	g := newTestGame(t, 0, nil, nil)
	if got := kinds(g.LegalActions(0)); got[ActKeepHand] != 1 || got[ActMulligan] != 1 || len(got) != 2 {
		t.Errorf("fresh: %v", got)
	}
	g.must(t, 0, Action{Kind: ActMulligan})
	if n := len(g.LegalActions(0)); n != 0 {
		t.Errorf("decided player has %d actions", n)
	}
	if n := len(g.LegalActions(1)); n != 2 {
		t.Errorf("undecided player has %d actions", n)
	}
}

func TestLegalActionsMainPhase(t *testing.T) {
	g := newTestGame(t, 0, []string{"Brute", "Fizzle", "Free Rat"}, nil)
	g.start(t)
	got := kinds(g.LegalActions(0))
	// Brute and the Rat go in any of 4 Lanes; Fizzle needs no Lane. The
	// remaining Grunts also fit in 4 Lanes each.
	if got[ActDraw] != 1 || got[ActEndMain] != 1 || got[ActPlayCard] < 4+4+1 {
		t.Errorf("main: %v", got)
	}
	if n := len(g.LegalActions(1)); n != 0 {
		t.Errorf("inactive player has %d actions", n)
	}
	g.State.ActionsLeft = 0
	for _, a := range g.LegalActions(0) {
		if a.Kind == ActPlayCard && g.Card(g.State.Players[0].Hand[g.handIndex(0, a.Card)]).Name != "Free Rat" {
			t.Errorf("with no Actions only the 0-cost card is playable, got %s", a)
		}
	}
}

func TestLegalActionsFightPhase(t *testing.T) {
	g := midGame(t)
	g.put(1, 0, "Grunt")
	g.put(1, 3, "Grunt")
	g.must(t, 1, endMain)
	acts := g.LegalActions(1)
	if len(acts) != 2 || !reflect.DeepEqual(acts[0], attack(0)) || !reflect.DeepEqual(acts[1], attack(3)) {
		t.Errorf("fight: %v", acts)
	}
}

func TestLegalActionsAreAlwaysApplicable(t *testing.T) {
	g := newTestGame(t, 0, []string{"Brute", "Fizzle", "Free Rat", "Hut"}, nil)
	g.start(t)
	for _, a := range g.LegalActions(0) {
		if _, err := g.Clone().Apply(0, a); err != nil {
			t.Errorf("%s listed as legal but failed: %v", a, err)
		}
	}
}

func TestLegalActionsGameOver(t *testing.T) {
	g := midGame(t)
	g.State.Phase = PhaseOver
	if n := len(g.LegalActions(0)) + len(g.LegalActions(1)); n != 0 {
		t.Errorf("%d actions after game over", n)
	}
}

func TestAwaiting(t *testing.T) {
	g := newTestGame(t, 0, nil, nil)
	if got := g.Awaiting(); len(got) != 2 {
		t.Errorf("mulligan: %v", got)
	}
	g.must(t, 0, Action{Kind: ActKeepHand})
	if got := g.Awaiting(); len(got) != 1 || got[0] != 1 {
		t.Errorf("after player 0 decides: %v", got)
	}
	g.must(t, 1, Action{Kind: ActKeepHand})
	if got := g.Awaiting(); len(got) != 1 || got[0] != 0 {
		t.Errorf("main phase: %v", got)
	}
	g.State.Phase = PhaseOver
	if got := g.Awaiting(); len(got) != 0 {
		t.Errorf("over: %v", got)
	}
}
