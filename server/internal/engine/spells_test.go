package engine

import "testing"

func creatureRef(p PlayerID, lane int) TargetRef {
	return TargetRef{Kind: "creature", Player: p, Lane: lane}
}
func laneRef(p PlayerID, lane int) TargetRef { return TargetRef{Kind: "lane", Player: p, Lane: lane} }

// cast plays the spell id from p's hand with the given target slots.
func (g *Game) cast(t *testing.T, p PlayerID, id string, targets ...[]TargetRef) []Event {
	t.Helper()
	uid := g.give(p, id)
	return g.must(t, p, Action{Kind: ActPlayCard, Card: uid, Targets: targets})
}

func (g *Game) castFails(t *testing.T, p PlayerID, id string, want error, targets ...[]TargetRef) {
	t.Helper()
	uid := g.give(p, id)
	g.mustFail(t, p, Action{Kind: ActPlayCard, Card: uid, Targets: targets}, want)
}

func TestCerebralBloodstormHitsOnlyOpposingCreatures(t *testing.T) {
	g := realGame(t)
	mine := g.put(1, 0, "the-pig")
	a := g.put(0, 0, "evil-eye")
	b := g.put(0, 3, "evil-eye")
	g.cast(t, 1, "cerebral-bloodstorm")
	if a.Damage != 1 || b.Damage != 1 || mine.Damage != 0 {
		t.Errorf("damage: opposing %d %d, mine %d", a.Damage, b.Damage, mine.Damage)
	}
	if g.State.SpellsThisTurn != 1 {
		t.Errorf("spells this turn = %d", g.State.SpellsThisTurn)
	}
	d := g.State.Players[1].Discard
	if len(d) != 1 || d[0].CardID != "cerebral-bloodstorm" {
		t.Errorf("spell should be discarded after resolving: %+v", d)
	}
}

func TestBloodstormKillsAreSettled(t *testing.T) {
	g := realGame(t)
	c := g.put(0, 0, "evil-eye")
	c.Damage = 4 // DEF 5
	g.cast(t, 1, "cerebral-bloodstorm")
	if g.State.Players[0].Lanes[0].Creature != nil {
		t.Error("a Creature reaching its DEF dies")
	}
	if len(g.State.Players[0].Discard) != 1 {
		t.Error("it goes to its owner's discard")
	}
}

func TestGnomeSnotDrawsThree(t *testing.T) {
	g := realGame(t)
	before := len(g.State.Players[1].Hand)
	g.cast(t, 1, "gnome-snot")
	if got := len(g.State.Players[1].Hand); got != before+3 {
		t.Errorf("hand %d -> %d, want +3 (spell leaves hand, +3 drawn)", before, got)
	}
}

func TestTeleportMovesToEmptyLane(t *testing.T) {
	g := realGame(t)
	pig := g.put(1, 0, "the-pig")
	pig.Damage = 2
	g.cast(t, 1, "teleport", []TargetRef{creatureRef(1, 0)}, []TargetRef{laneRef(1, 2)})
	pl := g.State.Players[1]
	if pl.Lanes[0].Creature != nil || pl.Lanes[2].Creature != pig {
		t.Fatal("Creature did not move")
	}
	if !pig.ChangedLanes || pig.Damage != 2 {
		t.Errorf("moved Creature: changed=%v damage=%d", pig.ChangedLanes, pig.Damage)
	}
	if g.State.ActionsLeft != 2 {
		t.Error("Teleport costs 0 Actions")
	}
}

func TestTeleportTargetRules(t *testing.T) {
	g := realGame(t)
	g.put(1, 0, "the-pig")
	g.put(1, 1, "the-pig")
	// Destination must be an empty lane, and the Creature must be yours.
	g.castFails(t, 1, "teleport", ErrBadTarget, []TargetRef{creatureRef(1, 0)}, []TargetRef{laneRef(1, 1)})
	g.put(0, 0, "evil-eye")
	g.castFails(t, 1, "teleport", ErrBadTarget, []TargetRef{creatureRef(0, 0)}, []TargetRef{laneRef(1, 2)})
	g.castFails(t, 1, "teleport", ErrBadTarget, []TargetRef{creatureRef(1, 0)}, []TargetRef{laneRef(0, 2)})
	g.castFails(t, 1, "teleport", ErrBadTarget) // targets are mandatory
	g.castFails(t, 1, "teleport", ErrBadTarget, []TargetRef{creatureRef(1, 0), creatureRef(1, 1)}, []TargetRef{laneRef(1, 2)})
}

func TestSpellWithNoLegalTargetCannotBePlayed(t *testing.T) {
	g := realGame(t)
	// Teleport with no Creatures and then with no empty lanes.
	g.castFails(t, 1, "teleport", ErrNoTarget)
	for i := 0; i < NumLanes; i++ {
		g.put(1, i, "the-pig")
	}
	g.castFails(t, 1, "teleport", ErrNoTarget)
}

func TestWoadBloodAndPunkCat(t *testing.T) {
	g := realGame(t)
	moved := g.put(1, 0, "the-pig")
	still := g.put(1, 1, "the-pig")
	moved.ChangedLanes = true
	g.cast(t, 1, "woad-blood")
	if moved.TempATK != 2 || still.TempATK != 0 {
		t.Errorf("Woad Blood: moved %d, still %d", moved.TempATK, still.TempATK)
	}
	if g.atkOf(1, 0) != 3 {
		t.Errorf("ATK = %d, want 1+2", g.atkOf(1, 0))
	}
}

func TestWoadTalismanNeedsABluePlainsCreatureYouControl(t *testing.T) {
	g := realGame(t)
	g.put(1, 0, "cool-dog")    // Blue Plains
	g.put(1, 1, "the-pig")     // Rainbow
	g.put(0, 0, "dragon-claw") // Blue Plains, but the opponent's
	g.castFails(t, 1, "woad-talisman", ErrBadTarget, []TargetRef{creatureRef(1, 1)})
	g.castFails(t, 1, "woad-talisman", ErrBadTarget, []TargetRef{creatureRef(0, 0)})
	g.cast(t, 1, "woad-talisman", []TargetRef{creatureRef(1, 0)})
	if g.atkOf(1, 0) != 4 {
		t.Errorf("Cool Dog ATK = %d, want 2+2", g.atkOf(1, 0))
	}
}

func TestCrossPollinationCountsDifferentLandscapeTypes(t *testing.T) {
	g := realGame(t)
	g.State.Active = 0              // Jake's spell; make player 0 the active player
	a := g.put(0, 0, "husker-worm") // Cornfield Creature
	b := g.put(0, 1, "evil-eye")    // Rainbow: not a Cornfield Creature
	g.cast(t, 0, "cross-pollination")
	if a.TempATK != 1 || b.TempATK != 0 {
		t.Errorf("4 Cornfields is one type: %d / %d, want 1 / 0", a.TempATK, b.TempATK)
	}
	a.TempATK = 0
	g.State.ActionsLeft = ActionsPerTurn
	g.State.Players[0].Lanes[1].Landscape = "blue_plains"
	g.State.Players[0].Lanes[2].Landscape = "sandy_lands"
	g.State.Players[0].Lanes[3].FaceDown = true // face-down Landscapes have no type
	g.cast(t, 0, "cross-pollination")
	if a.TempATK != 3 {
		t.Errorf("three types in play: bonus %d, want 3", a.TempATK)
	}
}

func TestCornScepterScalesWithCornfields(t *testing.T) {
	g := realGame(t)
	g.State.Active = 0
	target := g.put(1, 0, "cool-dog") // DEF 7
	g.cast(t, 0, "corn-scepter", []TargetRef{creatureRef(1, 0)})
	if target.Damage != 4 {
		t.Errorf("damage = %d, want 4 (one per Cornfield)", target.Damage)
	}
	g.State.Players[0].Lanes[0].FaceDown = true
	g.cast(t, 0, "corn-scepter", []TargetRef{creatureRef(1, 0)})
	if g.State.Players[1].Lanes[0].Creature != nil {
		t.Error("4 + 3 damage kills a DEF 7 Creature")
	}
}

func TestCornScepterMayTargetYourOwnCreature(t *testing.T) {
	// "If your card has a target, you must target something, even your own card."
	g := realGame(t)
	g.State.Active = 0
	own := g.put(0, 0, "husker-worm")
	g.cast(t, 0, "corn-scepter", []TargetRef{creatureRef(0, 0)})
	if own.Damage != 4 {
		t.Errorf("damage = %d", own.Damage)
	}
}

func TestFieldOfNightmaresCountsOpponentsHand(t *testing.T) {
	g := realGame(t)
	g.State.Active = 0
	hand := len(g.State.Players[1].Hand)
	g.cast(t, 0, "field-of-nightmares")
	if got := g.State.Players[1].HP; got != StartingHP-hand {
		t.Errorf("HP = %d, want %d", got, StartingHP-hand)
	}
}

func TestFieldOfNightmaresCanWin(t *testing.T) {
	g := realGame(t)
	g.State.Active = 0
	g.State.Players[1].HP = 3
	g.cast(t, 0, "field-of-nightmares")
	if g.State.Phase != PhaseOver || g.State.Winner != 0 {
		t.Errorf("phase=%s winner=%d", g.State.Phase, g.State.Winner)
	}
}

func TestReclaimLandscapeFlipsAndMovesBuilding(t *testing.T) {
	g := realGame(t)
	g.State.Active = 0
	g.State.Players[0].Lanes[1].FaceDown = true
	g.State.Players[0].Lanes[0].Building = &Building{Instance: Instance{UID: 800, CardID: "blood-fortress"}}
	g.cast(t, 0, "reclaim-landscape",
		[]TargetRef{{Kind: "landscape", Player: 0, Lane: 1}},
		[]TargetRef{{Kind: "building", Player: 0, Lane: 0}},
		[]TargetRef{laneRef(0, 2)})
	pl := g.State.Players[0]
	if pl.Lanes[1].FaceDown {
		t.Error("Landscape should be face up")
	}
	if pl.Lanes[0].Building != nil || pl.Lanes[2].Building == nil {
		t.Error("Building should have moved to lane 2")
	}
}

func TestReclaimLandscapeEverythingIsOptional(t *testing.T) {
	g := realGame(t)
	g.State.Active = 0
	g.cast(t, 0, "reclaim-landscape") // "you may": nothing to do is fine
	g.State.Players[0].Lanes[2].FaceDown = true
	g.cast(t, 0, "reclaim-landscape", []TargetRef{{Kind: "landscape", Player: 0, Lane: 2}})
	if g.State.Players[0].Lanes[2].FaceDown {
		t.Error("flip alone should work")
	}
	// A face-up Landscape is not a legal target.
	g.castFails(t, 0, "reclaim-landscape", ErrBadTarget, []TargetRef{{Kind: "landscape", Player: 0, Lane: 2}})
}

func TestWoadicChiefGrowsWithSpells(t *testing.T) {
	g := realGame(t)
	g.put(1, 0, "woadic-chief")
	g.cast(t, 1, "gnome-snot")
	g.cast(t, 1, "gnome-snot")
	if g.atkOf(1, 0) != 2+4 {
		t.Errorf("ATK = %d, want 6", g.atkOf(1, 0))
	}
}

func TestLegalActionsListSpellTargets(t *testing.T) {
	g := realGame(t)
	g.put(1, 0, "the-pig")
	g.put(1, 1, "the-pig")
	uid := g.give(1, "teleport")
	n := 0
	for _, a := range g.LegalActions(1) {
		if a.Kind == ActPlayCard && a.Card == uid {
			n++
			if _, err := g.Clone().Apply(1, a); err != nil {
				t.Errorf("%s: %v", a, err)
			}
		}
	}
	if n != 2*2 { // two Creatures x two empty lanes
		t.Errorf("Teleport plays listed = %d, want 4", n)
	}
}
