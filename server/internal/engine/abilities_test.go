package engine

import (
	"testing"

	"cardwars/internal/card"
)

func floop(lane int, targets ...[]TargetRef) Action {
	return Action{Kind: ActFloop, Lane: lane, Targets: targets}
}

func activate(lane, idx int, targets ...[]TargetRef) Action {
	return Action{Kind: ActActivate, Lane: lane, Index: idx, Targets: targets}
}

func TestFloopRules(t *testing.T) {
	g := realGame(t)
	pig := g.put(1, 0, "the-pig")
	target := []TargetRef{lsRef(0, 0)} // Jake's Cornfield in this Lane

	g.must(t, 1, floop(0, target))
	if pig.Status != Flooped {
		t.Errorf("status = %s", pig.Status)
	}
	if g.State.ActionsLeft != 2 {
		t.Error("Floop costs no Actions")
	}
	g.mustFail(t, 1, floop(0, []TargetRef{lsRef(0, 0)}), ErrNotReady) // already exhausted
	// Floop is a main-phase thing.
	g.put(1, 1, "the-pig")
	g.put(1, 2, "evil-eye")
	g.must(t, 1, endMain) // the Eye is Ready, so the Fight phase begins
	if g.State.Phase != PhaseFight {
		t.Fatal("setup: expected the Fight phase")
	}
	g.mustFail(t, 1, floop(1, []TargetRef{lsRef(0, 1)}), ErrWrongPhase)
}

func TestFloopOnlyOnYourOwnTurn(t *testing.T) {
	g := realGame(t)
	g.put(0, 0, "the-pig")
	g.mustFail(t, 0, floop(0, []TargetRef{lsRef(1, 0)}), ErrNotYourTurn)
}

func TestFirstPlayerCannotFloopOnTheFirstTurn(t *testing.T) {
	g := newTestGame(t, 0, nil, nil)
	g.start(t)
	cat := realGame(t).cat
	g.cat = cat
	g.State.Players[0].Lanes[0].Creature = &Creature{Instance: g.newInstance("the-pig"), Status: Ready}
	g.mustFail(t, 0, floop(0, []TargetRef{lsRef(0, 0)}), ErrFirstTurn)
}

func TestFloopedCreatureDoesNotFightAtEndOfTurn(t *testing.T) {
	g := realGame(t)
	g.put(1, 0, "the-pig")
	g.must(t, 1, floop(0, []TargetRef{lsRef(0, 0)}))
	g.must(t, 1, endMain)
	if g.State.Active != 0 || g.State.Players[0].HP != StartingHP {
		t.Error("a Flooped Creature does not Fight: the turn ends at once")
	}
}

// The rulebook's example: Jake's Husker Knight faces Finn's The Pig. Finn
// Floops The Pig to flip the Cornfield in his Lane, shrinking the Knight.
func TestRulebookPigFloopExample(t *testing.T) {
	g := realGame(t)
	g.put(0, 0, "husker-knight")
	pig := g.put(1, 0, "the-pig")
	soldier := g.put(1, 1, "x-large-spirit-soldier")
	if g.atkOf(0, 0) != 4 {
		t.Fatalf("setup: Knight ATK %d", g.atkOf(0, 0))
	}
	g.must(t, 1, floop(0, []TargetRef{lsRef(0, 0)}))
	if !g.State.Players[0].Lanes[0].FaceDown {
		t.Fatal("Cornfield in The Pig's Lane should be face down")
	}
	if g.atkOf(0, 0) != 3 || g.defOf(0, 0) != 6 {
		t.Errorf("Knight is now %d/%d, want 3/6", g.atkOf(0, 0), g.defOf(0, 0))
	}
	g.must(t, 1, endMain)
	g.must(t, 1, attack(1)) // the Soldier has no Floop: it Fights an empty Lane
	if g.State.Players[0].HP != StartingHP-1 {
		t.Errorf("HP = %d, want %d", g.State.Players[0].HP, StartingHP-1)
	}
	if pig.Damage != 0 || soldier.Status != Fought {
		t.Errorf("pig damage %d, soldier %s", pig.Damage, soldier.Status)
	}
	// "The Pig won't be able to Floop again as the Cornfield in his lane is
	// already face down": there is no legal target.
	g.must(t, 0, endMain) // player 0 turn: Knight is ready, Pig flooped
	g.must(t, 0, attack(0))
	if g.State.Active != 1 {
		t.Fatal("setup")
	}
	g.mustFail(t, 1, floop(0, []TargetRef{lsRef(0, 0)}), ErrNoTarget) // a face-down Landscape is not a Cornfield
	g.mustFail(t, 1, floop(0), ErrNoTarget)
}

func TestPigOnlyTargetsItsOwnLane(t *testing.T) {
	g := realGame(t)
	g.put(1, 0, "the-pig")
	g.mustFail(t, 1, floop(0, []TargetRef{lsRef(0, 1)}), ErrBadTarget)
	g.mustFail(t, 1, floop(0, []TargetRef{lsRef(1, 0)}), ErrBadTarget) // own side is Blue Plains
}

func TestDragonClawMovesAnyOfYourCreatures(t *testing.T) {
	g := realGame(t)
	g.put(1, 0, "dragon-claw")
	other := g.put(1, 1, "evil-eye")
	g.must(t, 1, floop(0, []TargetRef{creatureRef(1, 1)}, []TargetRef{laneRef(1, 3)}))
	if g.State.Players[1].Lanes[3].Creature != other || !other.ChangedLanes {
		t.Error("Creature should have moved to lane 3")
	}
}

func TestEmbarrassingBardCountsItself(t *testing.T) {
	g := realGame(t)
	g.put(1, 0, "embarrassing-bard")
	g.put(1, 1, "the-pig").Status = Flooped
	before := len(g.State.Players[1].Hand)
	g.must(t, 1, floop(0))
	if got := len(g.State.Players[1].Hand); got != before+2 {
		t.Errorf("drew %d, want 2 (this Bard and the Pig)", got-before)
	}
}

func TestHeavenlyGazerNeedsASpellInDiscard(t *testing.T) {
	g := realGame(t)
	g.put(1, 0, "heavenly-gazer")
	g.mustFail(t, 1, floop(0), ErrNoTarget)
	g.State.Players[1].Discard = []Instance{g.newInstance("evil-eye")}
	g.mustFail(t, 1, floop(0), ErrNoTarget) // a Creature is not a Spell
	spell := g.newInstance("gnome-snot")
	g.State.Players[1].Discard = append(g.State.Players[1].Discard, spell)
	g.must(t, 1, floop(0, []TargetRef{{Kind: "discard_card", Player: 1, Card: spell.UID}}))
	if top := g.State.Players[1].Deck[0]; top.UID != spell.UID {
		t.Errorf("top of deck = %+v", top)
	}
	if len(g.State.Players[1].Discard) != 1 {
		t.Error("the Spell should have left the discard pile")
	}
}

func TestAncientScholarReturnsARandomRainbowCard(t *testing.T) {
	g := realGame(t)
	g.put(1, 0, "ancient-scholar")
	rainbow := g.newInstance("teleport")
	g.State.Players[1].Discard = []Instance{g.newInstance("cool-dog"), rainbow} // Cool Dog is Blue Plains
	hand := len(g.State.Players[1].Hand)
	g.must(t, 1, floop(0))
	h := g.State.Players[1].Hand
	if len(h) != hand+1 || h[len(h)-1].UID != rainbow.UID {
		t.Errorf("hand = %+v", h)
	}
	if g.State.ActionsLeft != 2 {
		t.Error("no Building in this Lane: no extra Action")
	}
}

func TestAncientScholarWithBuildingGainsAnAction(t *testing.T) {
	g := realGame(t)
	g.put(1, 0, "ancient-scholar")
	g.State.Players[1].Lanes[0].Building = &Building{Instance: g.newInstance("blood-fortress")}
	g.must(t, 1, floop(0)) // empty discard pile: nothing to return, still gains the Action
	if g.State.ActionsLeft != 3 {
		t.Errorf("actions = %d, want 3", g.State.ActionsLeft)
	}
}

func TestArcherDanDestroysABuildingInHisLane(t *testing.T) {
	g := realGame(t)
	g.State.Active = 0
	g.put(0, 0, "archer-dan")
	g.mustFail(t, 0, floop(0), ErrNoTarget)
	g.State.Players[1].Lanes[0].Building = &Building{Instance: g.newInstance("celestial-castle")}
	g.State.Players[1].Lanes[1].Building = &Building{Instance: g.newInstance("blood-fortress")}
	g.mustFail(t, 0, floop(0, []TargetRef{{Kind: "building", Player: 1, Lane: 1}}), ErrBadTarget) // other Lane
	g.must(t, 0, floop(0, []TargetRef{{Kind: "building", Player: 1, Lane: 0}}))
	if g.State.Players[1].Lanes[0].Building != nil {
		t.Error("Building should be destroyed")
	}
	if d := g.State.Players[1].Discard; len(d) != 1 || d[0].CardID != "celestial-castle" {
		t.Errorf("a destroyed Building goes to its owner's discard: %+v", d)
	}
}

func TestBigFootFlipsALandscapeFaceUp(t *testing.T) {
	g := realGame(t)
	g.State.Active = 0
	g.put(0, 0, "big-foot")
	g.mustFail(t, 0, floop(0), ErrNoTarget) // nothing face down
	g.State.Players[0].Lanes[2].FaceDown = true
	g.must(t, 0, floop(0, []TargetRef{lsRef(0, 2)}))
	if g.State.Players[0].Lanes[2].FaceDown {
		t.Error("should be face up")
	}
}

func TestPatchyHitsEachCreatureOnce(t *testing.T) {
	g := realGame(t)
	g.State.Active = 0
	g.put(0, 0, "patchy-the-pumpkin")
	a := g.put(1, 0, "cool-dog")
	b := g.put(1, 1, "cool-dog")
	c := g.put(1, 2, "cool-dog")
	// 4 Cornfields: up to 4 distinct targets; with 4 Creatures in play she must hit all.
	g.mustFail(t, 0, floop(0, []TargetRef{creatureRef(1, 0), creatureRef(1, 0)}), ErrBadTarget) // not twice
	g.mustFail(t, 0, floop(0, []TargetRef{creatureRef(1, 0)}), ErrBadTarget)                    // must use all she can
	g.must(t, 0, floop(0, []TargetRef{creatureRef(1, 0), creatureRef(1, 1), creatureRef(1, 2), creatureRef(0, 0)}))
	if a.Damage != 1 || b.Damage != 1 || c.Damage != 1 {
		t.Errorf("damage %d %d %d", a.Damage, b.Damage, c.Damage)
	}
}

func TestPatchyWithFewerCornfields(t *testing.T) {
	g := realGame(t)
	g.State.Active = 0
	g.put(0, 0, "patchy-the-pumpkin")
	g.put(1, 0, "cool-dog")
	g.put(1, 1, "cool-dog")
	g.State.Players[0].Lanes[2].FaceDown = true
	g.State.Players[0].Lanes[3].FaceDown = true // 2 Cornfields: hits exactly 2
	g.mustFail(t, 0, floop(0, []TargetRef{creatureRef(1, 0), creatureRef(1, 1), creatureRef(0, 0)}), ErrBadTarget)
	g.must(t, 0, floop(0, []TargetRef{creatureRef(1, 0), creatureRef(1, 1)}))
}

func TestUniKnightActivationCostsAnAction(t *testing.T) {
	g := realGame(t)
	g.put(1, 0, "uni-knight") // 3/4
	foe := g.put(0, 0, "husker-knight")
	g.must(t, 1, activate(0, 0, []TargetRef{creatureRef(0, 0)}))
	if g.State.ActionsLeft != 1 {
		t.Errorf("actions = %d", g.State.ActionsLeft)
	}
	if got := g.atkOf(0, 0); got != 0 {
		t.Errorf("foe ATK = %d, want 0 (clamped)", got)
	}
	g.must(t, 1, activate(0, 0, []TargetRef{creatureRef(1, 0)})) // may target your own, any number of times
	g.mustFail(t, 1, activate(0, 0, []TargetRef{creatureRef(0, 0)}), ErrNoActions)
	if foe.TempATK != -10 {
		t.Errorf("TempATK = %d", foe.TempATK)
	}
	g.must(t, 1, endMain)
	g.must(t, 1, attack(0)) // Uni-Knight now has -10 ATK: the fight deals no damage to the Knight
	if foe.Damage != 0 {
		t.Errorf("foe damage = %d", foe.Damage)
	}
}

func TestUniKnightOnlyTargetsItsLane(t *testing.T) {
	g := realGame(t)
	g.put(1, 0, "uni-knight")
	g.put(0, 1, "evil-eye")
	g.mustFail(t, 1, activate(0, 0, []TargetRef{creatureRef(0, 1)}), ErrBadTarget)
}

func TestNoSuchAbility(t *testing.T) {
	g := realGame(t)
	g.put(1, 0, "evil-eye")
	g.mustFail(t, 1, floop(0), ErrNoAbility)
	g.mustFail(t, 1, activate(0, 0), ErrNoAbility)
	g.mustFail(t, 1, floop(1), ErrNoAbility) // empty lane
	g.mustFail(t, 1, floop(9), ErrBadLane)
	g.mustFail(t, 1, Action{Kind: ActFloop, Lane: 0, Source: "building"}, ErrNoAbility)
}

func TestBuildingsCanFloop(t *testing.T) {
	cat := testCatalog()
	cat["Well"] = &card.Card{ID: "Well", Name: "Well", Type: card.Building, Landscape: card.Rainbow, Cost: ip(0), Modeled: true,
		Effects: []card.Effect{{Kind: card.KindFloop, Do: []card.Op{{Do: "draw", Who: "you", Amount: &card.Value{N: 1}}}}}}
	d := DeckSpec{cornfields, repeat("Grunt", 12)}
	g, err := NewGame(cat, Config{Seed: 1, SkipDeckRules: true}, [2]DeckSpec{d, d})
	if err != nil {
		t.Fatal(err)
	}
	g.start(t)
	g.must(t, 0, endMain)
	g.State.Players[1].Lanes[0].Building = &Building{Instance: g.newInstance("Well"), Status: Ready}
	hand := len(g.State.Players[1].Hand)
	a := Action{Kind: ActFloop, Lane: 0, Source: "building"}
	g.must(t, 1, a)
	if len(g.State.Players[1].Hand) != hand+1 || g.State.Players[1].Lanes[0].Building.Status != Flooped {
		t.Error("the Building should exhaust and draw")
	}
	g.mustFail(t, 1, a, ErrNotReady)
}

func TestLegalActionsIncludeAbilities(t *testing.T) {
	g := realGame(t)
	g.put(1, 0, "the-pig")
	g.put(1, 1, "uni-knight")
	g.put(1, 2, "dragon-claw")
	g.put(0, 0, "evil-eye")
	counts := map[ActionKind]int{}
	for _, a := range g.LegalActions(1) {
		counts[a.Kind]++
		if a.Kind == ActFloop || a.Kind == ActActivate {
			if _, err := g.Clone().Apply(1, a); err != nil {
				t.Errorf("%s: %v", a, err)
			}
		}
	}
	// Pig: one Cornfield (the Jake side, lane 0). Dragon Claw: 3 Creatures x 1 empty lane.
	// Uni-Knight: one Creature in its own lane.
	if counts[ActFloop] != 1+3 || counts[ActActivate] != 1 {
		t.Errorf("floop %d, activate %d", counts[ActFloop], counts[ActActivate])
	}
}

func TestWoadMobileHomeMovesAnAdjacentCreatureHere(t *testing.T) {
	g := realGame(t)
	home := &Building{Instance: g.newInstance("woad-mobile-home"), Status: Ready}
	g.State.Players[1].Lanes[1].Building = home
	mover := g.put(1, 0, "the-pig")
	far := g.put(1, 3, "evil-eye")
	bf := Action{Kind: ActFloop, Lane: 1, Source: "building"}

	with := func(c, l TargetRef) Action {
		a := bf
		a.Targets = [][]TargetRef{{c}, {l}}
		return a
	}
	g.mustFail(t, 1, with(creatureRef(1, 3), laneRef(1, 1)), ErrBadTarget) // not adjacent
	g.mustFail(t, 1, with(creatureRef(0, 0), laneRef(1, 1)), ErrBadTarget) // not yours
	g.must(t, 1, with(creatureRef(1, 0), laneRef(1, 1)))
	if g.State.Players[1].Lanes[1].Creature != mover || g.State.Players[1].Lanes[0].Creature != nil || !mover.ChangedLanes {
		t.Error("the adjacent Creature should have moved into this Lane")
	}
	if home.Status != Flooped || far.Status != Ready {
		t.Error("only the Building exhausts")
	}
}

func TestWoadMobileHomeNeedsAnEmptyLaneAndAnAdjacentCreature(t *testing.T) {
	g := realGame(t)
	g.State.Players[1].Lanes[1].Building = &Building{Instance: g.newInstance("woad-mobile-home"), Status: Ready}
	bf := Action{Kind: ActFloop, Lane: 1, Source: "building"}
	g.mustFail(t, 1, bf, ErrNoTarget) // nobody adjacent
	g.put(1, 0, "the-pig")
	g.put(1, 1, "the-pig") // "if empty": this Lane is occupied
	g.mustFail(t, 1, bf, ErrNoTarget)
}
