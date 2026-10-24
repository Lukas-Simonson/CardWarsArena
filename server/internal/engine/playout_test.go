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
	cat, err := card.Load("../../data")
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

// strictStarterGame is the same match using only cards whose rules are fully
// modelled; the few still-unimplemented cards are left out of the decks.
func strictStarterGame(t testing.TB, seed uint64) *Game {
	t.Helper()
	cat, err := card.Load("../../data")
	if err != nil {
		t.Fatal(err)
	}
	keep := func(ids []string) []string {
		var out []string
		for _, id := range ids {
			if cat[id].Implemented() {
				out = append(out, id)
			}
		}
		return out
	}
	finn := DeckSpec{[NumLanes]card.Landscape{card.BluePlains, card.BluePlains, card.BluePlains, card.BluePlains}, keep(cat.Deck("CP1", "Finn's Blue Plains Deck"))}
	jake := DeckSpec{cornfields, keep(cat.Deck("CP1", "Jake's Cornfield Deck"))}
	g, err := NewGame(cat, Config{Seed: seed, First: PlayerID(seed % 2), SkipDeckRules: true}, [2]DeckSpec{finn, jake})
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
				if l.Creature.Damage >= g.def(pos{PlayerID(p), i}) {
					return fmt.Errorf("player %d lane %d: creature has damage %d >= DEF %d and is still alive", p, i, l.Creature.Damage, g.def(pos{PlayerID(p), i}))
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
	if st.ActionsLeft < 0 { // effects such as Ancient Scholar can grant extra Actions
		return fmt.Errorf("actions left %d", st.ActionsLeft)
	}
	if st.Phase == PhaseFight && !g.hasAttacker(st.Active) {
		return fmt.Errorf("fight phase with nothing Ready: the turn should have ended")
	}
	return nil
}

// playRandom plays a game choosing uniformly among legal actions. It returns
// the number of actions taken and whether the game finished.
func playRandom(t testing.TB, g *Game, rng *rand.Rand, maxActions int) (int, bool) {
	return playRandomSeen(t, g, rng, maxActions, nil)
}

// playRandomSeen is playRandom, also tallying the kinds of events produced.
func playRandomSeen(t testing.TB, g *Game, rng *rand.Rand, maxActions int, seen map[EventKind]int) (int, bool) {
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
		evs, err := g.Apply(p, a)
		if err != nil {
			t.Fatalf("legal action %s failed: %v", a, err)
		}
		for _, e := range evs {
			if seen != nil {
				seen[e.Kind]++
			}
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

func TestStrictRandomPlayoutsExerciseCardEffects(t *testing.T) {
	games := 300
	if testing.Short() {
		games = 30
	}
	seen := map[EventKind]int{}
	finished := 0
	for seed := uint64(1); seed <= uint64(games); seed++ {
		g := strictStarterGame(t, seed)
		if _, done := playRandomSeen(t, g, rand.New(rand.NewPCG(seed, 7)), 20000, seen); done {
			finished++
		}
	}
	if finished < games*95/100 {
		t.Errorf("only %d of %d games finished", finished, games)
	}
	t.Logf("events across %d games: %v", games, seen)
	if !testing.Short() {
		for _, k := range []EventKind{EvFloop, EvActivate, EvMove, EvFlip, EvBounce, EvReady, EvReturn, EvDeckTop, EvDestroy, EvActions} {
			if seen[k] == 0 {
				t.Errorf("no %s events in %d random games: that effect path is untested", k, games)
			}
		}
	}
}

// The starter decks are fully modelled except for four cards whose rules need
// features the engine does not have yet (see docs/effects.md). Fails when one
// of them gets modelled, as a reminder to shrink this list and the docs.
func TestOnlyKnownStarterCardsAreUnmodeled(t *testing.T) {
	cat, err := card.Load("../../data")
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]bool{"schoolhouse": true, "woad-mobile-home": true, "field-reaper": true, "silo-of-truth": true}
	for _, deck := range [][]string{cat.Deck("CP1", "Finn's Blue Plains Deck"), cat.Deck("CP1", "Jake's Cornfield Deck")} {
		for _, id := range deck {
			if cat[id].Implemented() == want[id] {
				t.Errorf("%s: implemented=%v, expected %v", id, cat[id].Implemented(), !want[id])
			}
		}
	}
}
