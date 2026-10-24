package bot

import (
	"math/rand/v2"
	"testing"

	"cardwars/internal/card"
	"cardwars/internal/engine"
)

func starterGame(t *testing.T, seed uint64) *engine.Game {
	t.Helper()
	cat, err := card.Load("../../data")
	if err != nil {
		t.Fatal(err)
	}
	finn := engine.DeckSpec{Landscapes: [4]card.Landscape{card.BluePlains, card.BluePlains, card.BluePlains, card.BluePlains}, Cards: cat.Deck("CP1", "Finn's Blue Plains Deck")}
	jake := engine.DeckSpec{Landscapes: [4]card.Landscape{card.Cornfield, card.Cornfield, card.Cornfield, card.Cornfield}, Cards: cat.Deck("CP1", "Jake's Cornfield Deck")}
	g, err := engine.NewGame(cat, engine.Config{Seed: seed, First: engine.PlayerID(seed % 2)}, [2]engine.DeckSpec{finn, jake})
	if err != nil {
		t.Fatal(err)
	}
	return g
}

// play runs bot vs bot to the end and returns the winner and number of actions.
func play(t *testing.T, g *engine.Game, rng *rand.Rand) (engine.PlayerID, int) {
	t.Helper()
	for n := 0; n < 20000; n++ {
		if g.State.Phase == engine.PhaseOver {
			return g.State.Winner, n
		}
		acted := false
		for _, p := range []engine.PlayerID{g.State.Active, g.State.Active.Opp()} {
			a, ok := Choose(g, p, rng)
			if !ok {
				continue
			}
			if _, err := g.Apply(p, a); err != nil {
				t.Fatalf("bot chose illegal action %s: %v", a, err)
			}
			acted = true
			break
		}
		if !acted {
			t.Fatalf("nobody can act in phase %s", g.State.Phase)
		}
	}
	t.Fatal("game did not finish")
	return 0, 0
}

func TestBotPlaysCompleteGames(t *testing.T) {
	wins := [2]int{}
	for seed := uint64(1); seed <= 60; seed++ {
		w, n := play(t, starterGame(t, seed), rand.New(rand.NewPCG(seed, 3)))
		wins[w]++
		if n < 10 {
			t.Errorf("seed %d: game over after only %d actions", seed, n)
		}
	}
	if wins[0] == 0 || wins[1] == 0 {
		t.Errorf("one side never wins: %v", wins)
	}
}

func TestBotIsDeterministic(t *testing.T) {
	run := func() (engine.PlayerID, int) {
		return play(t, starterGame(t, 9), rand.New(rand.NewPCG(9, 3)))
	}
	w1, n1 := run()
	w2, n2 := run()
	if w1 != w2 || n1 != n2 {
		t.Errorf("same seeds differ: (%d,%d) vs (%d,%d)", w1, n1, w2, n2)
	}
}

func TestBotMulligansWeakHands(t *testing.T) {
	g := starterGame(t, 1)
	p := engine.PlayerID(0)
	counts := map[engine.ActionKind]int{}
	for seed := uint64(1); seed <= 200; seed++ {
		g = starterGame(t, seed)
		creatures := 0
		for _, in := range g.State.Players[p].Hand {
			if g.Card(in).Type == card.Creature {
				creatures++
			}
		}
		a, _ := Choose(g, p, rand.New(rand.NewPCG(1, 1)))
		counts[a.Kind]++
		if (creatures < 2) != (a.Kind == engine.ActMulligan) {
			t.Fatalf("seed %d: %d Creatures but chose %s", seed, creatures, a.Kind)
		}
	}
	if counts[engine.ActMulligan] == 0 || counts[engine.ActKeepHand] == 0 {
		t.Errorf("expected both choices across seeds: %v", counts)
	}
}

func TestBotReturnsFalseWhenNothingToDo(t *testing.T) {
	g := starterGame(t, 1)
	g.State.Phase = engine.PhaseOver
	if _, ok := Choose(g, 0, rand.New(rand.NewPCG(1, 1))); ok {
		t.Error("no action expected after game over")
	}
}
