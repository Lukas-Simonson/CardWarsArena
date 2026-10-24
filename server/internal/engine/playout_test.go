package engine

import (
	"fmt"
	"math/rand/v2"
	"testing"

	"cardwars/internal/card"
)

// starterGame builds Finn (Blue Plains) vs Jake (Cornfield) from the real
// catalog. Their rules text is not modelled yet, so it is ignored.
func starterGame(t testing.TB, seed uint64) *Game {
	t.Helper()
	cat, err := card.LoadDir("../../data/cards")
	if err != nil {
		t.Fatal(err)
	}
	finn := DeckSpec{[NumLanes]card.Landscape{card.BluePlains, card.BluePlains, card.BluePlains, card.BluePlains}, cat.Deck("CP1", "Finn's Blue Plains Deck")}
	jake := DeckSpec{cornfields, cat.Deck("CP1", "Jake's Cornfield Deck")}
	g, err := NewGame(cat, Config{Seed: seed, First: PlayerID(seed % 2), AllowUnimplemented: true}, [2]DeckSpec{finn, jake})
	if err != nil {
		t.Fatal(err)
	}
	return g
}

// checkInvariants returns a description of the first broken invariant.
func (g *Game) checkInvariants(total [2]int) error {
	st := &g.State
	seen := map[int]bool{}
	for p := range st.Players {
		pl := &st.Players[p]
		n := 0
		count := func(in Instance) error {
			n++
			if seen[in.UID] {
				return fmt.Errorf("uid %d appears twice", in.UID)
			}
			seen[in.UID] = true
			return nil
		}
		for _, list := range [][]Instance{pl.Deck, pl.Hand, pl.Discard} {
			for _, in := range list {
				if err := count(in); err != nil {
					return err
				}
			}
		}
		for i, l := range pl.Lanes {
			if l.Creature != nil {
				if err := count(l.Creature.Instance); err != nil {
					return err
				}
				if l.Creature.Damage >= g.defense(l.Creature) {
					return fmt.Errorf("player %d lane %d: creature has damage %d >= DEF %d and is still alive", p, i, l.Creature.Damage, g.defense(l.Creature))
				}
				if l.Creature.Damage < 0 {
					return fmt.Errorf("negative damage")
				}
			}
			if l.Building != nil {
				if err := count(l.Building.Instance); err != nil {
					return err
				}
			}
		}
		if n != total[p] {
			return fmt.Errorf("player %d has %d cards, started with %d", p, n, total[p])
		}
		if pl.HP > StartingHP {
			return fmt.Errorf("player %d HP %d above start", p, pl.HP)
		}
		if st.Phase != PhaseOver && pl.HP <= 0 {
			return fmt.Errorf("player %d at %d HP but game not over", p, pl.HP)
		}
	}
	if st.ActionsLeft < 0 || st.ActionsLeft > ActionsPerTurn {
		return fmt.Errorf("actions left %d", st.ActionsLeft)
	}
	if st.Phase == PhaseFight && !g.hasReadyCreature(st.Active) {
		return fmt.Errorf("fight phase with nothing Ready: the turn should have ended")
	}
	return nil
}

// playRandom plays a game choosing uniformly among legal actions. It returns
// the number of actions taken and whether the game finished.
func playRandom(t testing.TB, g *Game, rng *rand.Rand, maxActions int) (int, bool) {
	t.Helper()
	var total [2]int
	for p := range total {
		total[p] = g.cardCount(PlayerID(p))
	}
	for n := 0; n < maxActions; n++ {
		if g.State.Phase == PhaseOver {
			return n, true
		}
		p := g.State.Active
		if g.State.Phase == PhaseMulligan {
			p = PlayerID(rng.IntN(2))
		}
		legal := g.LegalActions(p)
		if len(legal) == 0 {
			// The other player may be the one who must decide (mulligan).
			legal = g.LegalActions(p.Opp())
			p = p.Opp()
		}
		if len(legal) == 0 {
			t.Fatalf("no legal actions for anyone in phase %s", g.State.Phase)
		}
		a := legal[rng.IntN(len(legal))]
		if _, err := g.Apply(p, a); err != nil {
			t.Fatalf("legal action %s failed: %v", a, err)
		}
		if err := g.checkInvariants(total); err != nil {
			t.Fatalf("after %s by %d (turn %d): %v", a, p, g.State.Turn, err)
		}
	}
	return maxActions, g.State.Phase == PhaseOver
}

func TestRandomPlayoutsKeepInvariants(t *testing.T) {
	games := 300
	if testing.Short() {
		games = 30
	}
	finished := 0
	for seed := uint64(1); seed <= uint64(games); seed++ {
		g := starterGame(t, seed)
		_, done := playRandom(t, g, rand.New(rand.NewPCG(seed, 99)), 20000)
		if done {
			finished++
		}
	}
	if finished < games*95/100 {
		t.Errorf("only %d of %d random games finished", finished, games)
	}
}

func TestPlayoutIsDeterministic(t *testing.T) {
	run := func() string {
		g := starterGame(t, 7)
		playRandom(t, g, rand.New(rand.NewPCG(7, 99)), 20000)
		return g.String()
	}
	if run() != run() {
		t.Error("same seeds gave different games")
	}
}

func TestCloneDiverges(t *testing.T) {
	g := starterGame(t, 3)
	rng := rand.New(rand.NewPCG(3, 1))
	for i := 0; i < 40 && g.State.Phase != PhaseOver; i++ {
		p := g.State.Active
		if g.State.Phase == PhaseMulligan {
			p = 0
		}
		legal := g.LegalActions(p)
		if len(legal) == 0 {
			p = p.Opp()
			legal = g.LegalActions(p)
		}
		g.must(t, p, legal[rng.IntN(len(legal))])
	}
	snap := g.String()
	c := g.Clone()
	playRandom(t, c, rand.New(rand.NewPCG(5, 5)), 500)
	if g.String() != snap {
		t.Error("playing on a clone changed the original")
	}
}
