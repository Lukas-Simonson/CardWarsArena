package engine

import (
	"math/rand/v2"
	"testing"
)

// schoolGame has a Schoolhouse in player 1's lane 0 with a creature under it.
func schoolGame(t *testing.T, under string, discard ...string) (*Game, *Creature, Action) {
	t.Helper()
	g := realGame(t)
	g.State.Players[1].Lanes[0].Building = &Building{Instance: g.newInstance("schoolhouse", 1), Status: Ready}
	cr := g.put(1, 0, under)
	for _, id := range discard {
		g.State.Players[1].Discard = append(g.State.Players[1].Discard, g.newInstance(id, 1))
	}
	return g, cr, Action{Kind: ActFloop, Lane: 0, Source: "building"}
}

func TestSchoolhouseGivesTheFloopOfADiscardedCreature(t *testing.T) {
	g, cr, sh := schoolGame(t, "evil-eye", "dragon-claw")
	g.must(t, 1, sh)
	if !cr.Silenced || cr.Lent != "dragon-claw" {
		t.Fatalf("creature = %+v", cr)
	}
	// The vanilla Evil Eye can now use Dragon Claw's Floop this turn.
	g.put(1, 2, "evil-eye")
	g.must(t, 1, floop(0, []TargetRef{creatureRef(1, 2)}, []TargetRef{laneRef(1, 3)}))
	if g.State.Players[1].Lanes[3].Creature == nil {
		t.Error("the borrowed Floop should have moved a Creature")
	}
	if cr.Status != Flooped {
		t.Error("using the borrowed Floop exhausts the Creature")
	}
}

func TestSchoolhouseRemovesTheCreaturesOwnAbilities(t *testing.T) {
	g, cr, sh := schoolGame(t, "the-pig", "dragon-claw")
	g.must(t, 1, sh)
	// The Pig's own Floop (flip a Cornfield) is gone: that target is not valid for Dragon Claw's.
	g.mustFail(t, 1, floop(0, []TargetRef{lsRef(0, 0)}), ErrBadTarget)
	if !cr.Silenced {
		t.Fatal("setup")
	}
}

func TestSchoolhouseSilencesStaticAbilities(t *testing.T) {
	g, _, sh := schoolGame(t, "corn-dog", "dragon-claw")
	g.State.Players[1].Lanes[1].Landscape = "cornfield"
	g.State.Players[1].Lanes[2].Landscape = "cornfield" // Corn Dog: 0/12, +1 DEF per Cornfield, +1 ATK with 3 or fewer
	if g.atkOf(1, 0) != 1 || g.defOf(1, 0) != 14 {
		t.Fatalf("setup: Corn Dog should be 1/14, is %d/%d", g.atkOf(1, 0), g.defOf(1, 0))
	}
	g.must(t, 1, sh)
	if g.atkOf(1, 0) != 0 || g.defOf(1, 0) != 12 {
		t.Errorf("a silenced Corn Dog is just its printed 0/12: %d/%d", g.atkOf(1, 0), g.defOf(1, 0))
	}
}

func TestSilencingAStatDependentCreatureCanKillIt(t *testing.T) {
	// Husker Knight has no printed ATK/DEF: without its ability it is 0/0.
	g, _, sh := schoolGame(t, "husker-knight", "dragon-claw")
	g.State.Players[1].Lanes[1].Landscape = "cornfield"
	g.must(t, 1, sh)
	if g.State.Players[1].Lanes[0].Creature != nil {
		t.Error("a 0/0 Creature dies as soon as it is silenced")
	}
}

func TestSilencedCreatureDoesNotTriggerOnLeavingPlay(t *testing.T) {
	g, farmer, sh := schoolGame(t, "travelin-farmer", "dragon-claw")
	g.must(t, 1, sh)
	farmer.Damage = 100
	g.State.Players[1].Lanes[0].Creature.Damage = 100
	hp := g.State.Players[0].HP
	g.settle()
	if g.State.Players[0].HP != hp {
		t.Error("a Creature with no abilities has no leave-play trigger")
	}
	if g.State.Players[1].Lanes[0].Creature != nil {
		t.Fatal("setup: the Farmer should have died")
	}
}

func TestSchoolhouseEffectEndsWithTheTurn(t *testing.T) {
	g, cr, sh := schoolGame(t, "the-pig", "dragon-claw")
	g.must(t, 1, sh)
	g.must(t, 1, endMain)
	if g.State.Active != 0 {
		g.must(t, 1, attack(0))
	}
	if cr.Silenced || cr.Lent != "" {
		t.Errorf("abilities should be restored: %+v", cr)
	}
}

func TestSchoolhouseNeedsADonorWithAFloop(t *testing.T) {
	// Spells, Floop-less Creatures and an empty pile all do nothing.
	for _, discard := range [][]string{nil, {"gnome-snot"}, {"cool-dog"}, {"blood-fortress"}} {
		g, cr, sh := schoolGame(t, "the-pig", discard...)
		g.must(t, 1, sh)
		if cr.Silenced {
			t.Errorf("discard %v: nothing to borrow, the Creature keeps its abilities", discard)
		}
		if g.State.Players[1].Lanes[0].Building.Status != Flooped {
			t.Errorf("discard %v: the Schoolhouse Floop is still spent", discard)
		}
	}
}

func TestSchoolhouseOnlyLooksAtYourOwnDiscardPile(t *testing.T) {
	g, cr, sh := schoolGame(t, "evil-eye")
	g.State.Players[0].Discard = append(g.State.Players[0].Discard, g.newInstance("dragon-claw", 0))
	g.must(t, 1, sh)
	if cr.Silenced {
		t.Error("the opponent's discard pile is not a donor")
	}
}

func TestSchoolhouseChoosesRandomlyAmongDonors(t *testing.T) {
	seen := map[string]bool{}
	for seed := uint64(1); seed <= 40; seed++ {
		g, cr, sh := schoolGame(t, "evil-eye", "dragon-claw", "embarrassing-bard")
		g.rng = *rand.NewPCG(seed, seed)
		g.must(t, 1, sh)
		seen[cr.Lent] = true
	}
	if !seen["dragon-claw"] || !seen["embarrassing-bard"] || len(seen) != 2 {
		t.Errorf("donors chosen across seeds: %v", seen)
	}
}

func TestSchoolhouseWithNoCreatureInItsLane(t *testing.T) {
	g, _, sh := schoolGame(t, "evil-eye", "dragon-claw")
	g.State.Players[1].Lanes[0].Creature = nil
	g.must(t, 1, sh) // nothing to affect; not an error
}

func TestSilencedFloopIsListedByLegalActions(t *testing.T) {
	g, _, sh := schoolGame(t, "evil-eye", "embarrassing-bard")
	g.must(t, 1, sh)
	found := false
	for _, a := range g.LegalActions(1) {
		if a.Kind == ActFloop && a.Source != "building" && a.Lane == 0 {
			found = true
			if _, err := g.Clone().Apply(1, a); err != nil {
				t.Error(err)
			}
		}
	}
	if !found {
		t.Error("the borrowed Floop should be offered")
	}
}
