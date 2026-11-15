package bot

import (
	"math/rand/v2"
	"testing"

	"cardwars/internal/card"
	"cardwars/internal/engine"
)

// frozenPosition is a game in the active player's main phase where all four of their Landscapes are frozen,
// their hand is `hand` and the opponent has nothing on the board.
func frozenPosition(t *testing.T, hand ...string) (*engine.Game, engine.PlayerID) {
	t.Helper()
	g := starterGame(t, 2)
	for g.State.Phase == engine.PhaseMulligan {
		for _, p := range []engine.PlayerID{g.State.Active, g.State.Active.Opp()} {
			for _, a := range g.LegalActions(p) {
				if a.Kind == engine.ActKeepHand {
					if _, err := g.Apply(p, a); err != nil {
						t.Fatal(err)
					}
					break
				}
			}
		}
	}
	p := g.State.Active
	for g.State.Phase != engine.PhaseMain {
		t.Fatalf("not in the main phase: %s", g.State.Phase)
	}
	g.State.Players[p].Hand = nil
	for _, id := range hand {
		if _, err := g.Give(p, id); err != nil {
			t.Fatal(err)
		}
	}
	for i := range g.State.Players[p].Lanes {
		g.State.Players[p].Lanes[i].Frozen = true
	}
	return g, p
}

// A frozen Landscape cannot take a Creature, so a bot that holds Creatures must pay a card to thaw one and play it.
func TestHeuristicThawsWhenAllLanesAreFrozenAndItHoldsCreatures(t *testing.T) {
	g, p := frozenPosition(t, "uni-knight", "punk-cat", "gnome-snot")
	a, ok := Heuristic{}.Choose(g, p, rand.New(rand.NewPCG(1, 1)))
	if !ok || a.Kind != engine.ActThaw {
		t.Fatalf("with every lane frozen and Creatures in hand the bot chose %s, want a thaw", a)
	}
}

// It pays with the card it needs least (a spell, not the Creature it is thawing for), then plays the Creature.
func TestHeuristicThawsWithTheSpareCardAndThenPlays(t *testing.T) {
	g, p := frozenPosition(t, "uni-knight", "gnome-snot")
	rng := rand.New(rand.NewPCG(3, 3))
	a, _ := Heuristic{}.Choose(g, p, rng)
	if a.Kind != engine.ActThaw {
		t.Fatalf("chose %s, want a thaw", a)
	}
	if g.Card(g.State.Players[p].Hand[indexOf(g, p, a.Card)]).Type == "creature" {
		t.Errorf("discarded the Creature it wanted to play")
	}
	if _, err := g.Apply(p, a); err != nil {
		t.Fatal(err)
	}
	b, _ := Heuristic{}.Choose(g, p, rng)
	if b.Kind != engine.ActPlayCard {
		t.Errorf("after thawing it chose %s, want to play the Creature", b)
	}
}

func indexOf(g *engine.Game, p engine.PlayerID, uid int) int {
	for i, in := range g.State.Players[p].Hand {
		if in.UID == uid {
			return i
		}
	}
	return -1
}

// Nothing to play, nothing to thaw for: it keeps its cards.
func TestHeuristicDoesNotThawForNothing(t *testing.T) {
	for _, hand := range [][]string{nil, {"gnome-snot"}, {"gnome-snot", "pants-of-awesome"}} {
		g, p := frozenPosition(t, hand...)
		for seed := uint64(1); seed <= 20; seed++ {
			if a, _ := (Heuristic{}).Choose(g, p, rand.New(rand.NewPCG(seed, 7))); a.Kind == engine.ActThaw {
				t.Fatalf("hand %v: thawed with no Creature to play", hand)
			}
		}
	}
}

// Freezing the opponent's Landscapes is worth something, so a freeze is not a wasted move.
func TestFrozenOpponentLandscapesScoreForTheBot(t *testing.T) {
	g, p := frozenPosition(t)
	for i := range g.State.Players[p].Lanes {
		g.State.Players[p].Lanes[i].Frozen = false
	}
	before := evaluate(g, p)
	g.State.Players[p.Opp()].Lanes[0].Frozen = true
	if evaluate(g, p) <= before {
		t.Error("an opposing Landscape frozen did not raise the score")
	}
}

// Whole games with Gunter's Deck, which freezes Landscapes, finish and the bots do thaw now and then.
func TestHeuristicPlaysGuntersDeck(t *testing.T) {
	cat := starterGame(t, 1).Catalog()
	thaws, games := 0, 12
	if testing.Short() {
		games = 6
	}
	for seed := uint64(1); seed <= uint64(games); seed++ {
		ls := [4]card.Landscape{card.IcyLands, card.IcyLands, card.BluePlains, card.BluePlains}
		g, err := engine.NewGame(cat, engine.Config{Seed: seed, First: engine.PlayerID(seed % 2)}, [2]engine.DeckSpec{
			{Landscapes: ls, Cards: cat.Deck("Lemongrab Vs Gunter", "Gunter's Deck"), Hero: "gunter"},
			{Landscapes: ls, Cards: cat.Deck("Lemongrab Vs Gunter", "Gunter's Deck"), Hero: "gunter"},
		})
		if err != nil {
			t.Fatal(err)
		}
		rng := rand.New(rand.NewPCG(seed, 9))
		for n := 0; g.State.Phase != engine.PhaseOver; n++ {
			if n > 20000 {
				t.Fatalf("seed %d: the game did not finish", seed)
			}
			acted := false
			for _, p := range []engine.PlayerID{g.State.Active, g.State.Active.Opp()} {
				a, ok := Heuristic{}.Choose(g, p, rng)
				if !ok {
					continue
				}
				if a.Kind == engine.ActThaw {
					thaws++
				}
				if _, err := g.Apply(p, a); err != nil {
					t.Fatalf("illegal action %s: %v", a, err)
				}
				acted = true
				break
			}
			if !acted {
				t.Fatalf("seed %d: nobody can act", seed)
			}
		}
	}
	t.Logf("%d thaws in %d games", thaws, games)
	if thaws == 0 {
		t.Error("the bots never thawed a Landscape")
	}
}

// Whole games with Lemongrab's deck, which flips and retypes Landscapes, moves Creatures into the opponent's
// side and stacks them under a Camel, finish with the bots answering every decision it asks.
func TestHeuristicPlaysLemongrabsDeck(t *testing.T) {
	cat := starterGame(t, 1).Catalog()
	games := 12
	if testing.Short() {
		games = 6
	}
	for seed := uint64(1); seed <= uint64(games); seed++ {
		ls := [4]card.Landscape{card.Cornfield, card.Cornfield, card.SandyLands, card.SandyLands}
		g, err := engine.NewGame(cat, engine.Config{Seed: seed, First: engine.PlayerID(seed % 2)}, [2]engine.DeckSpec{
			{Landscapes: ls, Cards: cat.Deck("Lemongrab Vs Gunter", "Lemongrab's Deck"), Hero: "lemongrab"},
			{Landscapes: ls, Cards: cat.Deck("Lemongrab Vs Gunter", "Lemongrab's Deck"), Hero: "lemongrab"},
		})
		if err != nil {
			t.Fatal(err)
		}
		rng := rand.New(rand.NewPCG(seed, 9))
		for n := 0; g.State.Phase != engine.PhaseOver; n++ {
			if n > 20000 {
				t.Fatalf("seed %d: the game did not finish", seed)
			}
			acted := false
			for _, p := range []engine.PlayerID{g.State.Active, g.State.Active.Opp()} {
				a, ok := Heuristic{}.Choose(g, p, rng)
				if !ok {
					continue
				}
				if _, err := g.Apply(p, a); err != nil {
					t.Fatalf("illegal action %s: %v", a, err)
				}
				acted = true
				break
			}
			if !acted {
				t.Fatalf("seed %d: nobody can act", seed)
			}
		}
	}
}
