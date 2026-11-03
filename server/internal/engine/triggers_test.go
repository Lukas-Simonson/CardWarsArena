package engine

import "testing"

// enter plays creature/building id from p's hand into lane with target slots.
func (g *Game) enter(t *testing.T, p PlayerID, id string, lane int, targets ...[]TargetRef) []Event {
	t.Helper()
	uid := g.give(p, id)
	g.State.ActionsLeft = ActionsPerTurn
	return g.must(t, p, Action{Kind: ActPlayCard, Card: uid, Lane: lane, Targets: targets})
}

func (g *Game) enterFails(t *testing.T, p PlayerID, id string, lane int, want error, targets ...[]TargetRef) {
	t.Helper()
	uid := g.give(p, id)
	g.State.ActionsLeft = ActionsPerTurn
	g.mustFail(t, p, Action{Kind: ActPlayCard, Card: uid, Lane: lane, Targets: targets}, want)
}

func lsRef(p PlayerID, lane int) TargetRef {
	return TargetRef{Kind: "landscape", Player: p, Lane: lane}
}

func TestCornataurHurtsOpponentPerCornfield(t *testing.T) {
	g := realGame(t)
	g.State.Active = 0
	g.State.Players[0].Lanes[3].FaceDown = true
	g.enter(t, 0, "cornataur", 0)
	if hp := g.State.Players[1].HP; hp != StartingHP-3 {
		t.Errorf("HP = %d, want %d", hp, StartingHP-3)
	}
}

func TestHuskerWormFlipsAChosenCornfield(t *testing.T) {
	g := realGame(t)
	g.State.Active = 0
	g.enter(t, 0, "husker-worm", 0, []TargetRef{lsRef(0, 2)})
	if !g.State.Players[0].Lanes[2].FaceDown {
		t.Error("chosen Cornfield should be face down")
	}
	if g.State.Players[0].Lanes[1].FaceDown {
		t.Error("only the chosen Landscape flips")
	}
	// The flip is a cost of playing it: a Husker Knight now has 3 Cornfields.
	g.put(0, 1, "husker-knight")
	if got := g.atkOf(0, 1); got != 3 {
		t.Errorf("Knight ATK = %d, want 3", got)
	}
}

func TestHuskerWormTargetRules(t *testing.T) {
	g := realGame(t)
	g.State.Active = 0
	g.enterFails(t, 0, "husker-worm", 0, ErrBadTarget)                           // must choose
	g.enterFails(t, 0, "husker-worm", 0, ErrBadTarget, []TargetRef{lsRef(1, 0)}) // opponent's Landscape
	g.enterFails(t, 0, "husker-worm", 0, ErrBadTarget, []TargetRef{lsRef(0, 1), lsRef(0, 2)})
	g.State.Players[0].Lanes[1].FaceDown = true
	g.enterFails(t, 0, "husker-worm", 0, ErrBadTarget, []TargetRef{lsRef(0, 1)}) // a face-down Landscape is not a Cornfield
}

func TestLegionOfEarlingsBouncesCreatureInLane(t *testing.T) {
	g := realGame(t)
	g.State.Active = 0
	foe := g.put(1, 1, "cool-dog")
	foe.Damage = 3
	handBefore := len(g.State.Players[1].Hand)
	g.enter(t, 0, "legion-of-earlings", 1, []TargetRef{creatureRef(1, 1)})
	if g.State.Players[1].Lanes[1].Creature != nil {
		t.Error("opposing Creature should be gone")
	}
	hand := g.State.Players[1].Hand
	if len(hand) != handBefore+1 || hand[len(hand)-1].CardID != "cool-dog" {
		t.Error("it returns to its owner's hand")
	}
	if len(g.State.Players[1].Discard) != 0 {
		t.Error("returning to hand is not a discard")
	}
}

func TestLegionOfEarlingsIsOptionalAndLaneBound(t *testing.T) {
	g := realGame(t)
	g.State.Active = 0
	g.put(1, 1, "cool-dog")
	g.put(1, 2, "cool-dog")
	g.enterFails(t, 0, "legion-of-earlings", 1, ErrBadTarget, []TargetRef{creatureRef(1, 2)}) // other Lane
	g.enter(t, 0, "legion-of-earlings", 1)                                                    // "you may": declining is fine
	if g.State.Players[1].Lanes[1].Creature == nil {
		t.Error("declined: Creature stays")
	}
}

func TestPsionicArchitectReadiesAFloopedCreature(t *testing.T) {
	g := realGame(t)
	flooped := g.put(1, 0, "the-pig")
	flooped.Status = Flooped
	ready := g.put(1, 1, "the-pig")
	g.enterFails(t, 1, "psionic-architect", 2, ErrBadTarget, []TargetRef{creatureRef(1, 1)}) // already Ready
	g.enter(t, 1, "psionic-architect", 2, []TargetRef{creatureRef(1, 0)})
	if flooped.Status != Ready || ready.Status != Ready {
		t.Errorf("status %s / %s", flooped.Status, ready.Status)
	}
}

func TestTravelinFarmerPunishesOnEveryWayOut(t *testing.T) {
	// dies in combat
	g := realGame(t)
	g.put(1, 0, "travelin-farmer") // 2/12... make it die by damage
	g.State.Players[1].Lanes[0].Creature.Damage = 11
	g.put(0, 0, "evil-eye") // ATK 1 kills it
	hand0 := len(g.State.Players[0].Hand)
	g.must(t, 1, endMain)
	g.must(t, 1, attack(0))
	if hp := g.State.Players[0].HP; hp != StartingHP-hand0 {
		t.Errorf("death: HP = %d, want %d", hp, StartingHP-hand0)
	}

	// replaced
	g = realGame(t)
	g.put(1, 0, "travelin-farmer")
	hand0 = len(g.State.Players[0].Hand)
	g.enter(t, 1, "evil-eye", 0)
	if hp := g.State.Players[0].HP; hp != StartingHP-hand0 {
		t.Errorf("replace: HP = %d, want %d", hp, StartingHP-hand0)
	}

	// returned to hand: Farmer's controller is player 1, so player 0's hand is counted
	g = realGame(t)
	g.State.Active = 0
	g.put(1, 0, "travelin-farmer")
	hand0 = len(g.State.Players[0].Hand)
	g.enter(t, 0, "legion-of-earlings", 0, []TargetRef{creatureRef(1, 0)})
	if hp := g.State.Players[1].HP; hp != StartingHP {
		t.Errorf("bounce: Farmer's controller took %d", StartingHP-hp)
	}
	if hp := g.State.Players[0].HP; hp != StartingHP-hand0 {
		t.Errorf("bounce: HP = %d, want %d", hp, StartingHP-hand0)
	}
}

func TestLeavePlayDamageCanEndTheGame(t *testing.T) {
	g := realGame(t)
	g.put(1, 0, "travelin-farmer")
	g.State.Players[1].Lanes[0].Creature.Damage = 11
	g.State.Players[0].HP = 1
	g.put(0, 0, "evil-eye")
	g.must(t, 1, endMain)
	g.must(t, 1, attack(0))
	if g.State.Phase != PhaseOver || g.State.Winner != 1 {
		t.Errorf("phase=%s winner=%d", g.State.Phase, g.State.Winner)
	}
}

func TestFieldStalkerDrawsForBothAtStartOfItsControllersTurn(t *testing.T) {
	g := realGame(t)
	g.put(0, 0, "field-stalker")
	h0, h1 := len(g.State.Players[0].Hand), len(g.State.Players[1].Hand)
	g.must(t, 1, endMain) // nothing to Fight: player 0's turn starts
	if g.State.Active != 0 {
		t.Fatal("setup")
	}
	if got := len(g.State.Players[0].Hand); got != h0+2 { // turn draw + Field Stalker
		t.Errorf("controller hand %d -> %d, want +2", h0, got)
	}
	if got := len(g.State.Players[1].Hand); got != h1+1 {
		t.Errorf("opponent hand %d -> %d, want +1", h1, got)
	}
	// Not on the opponent's turn.
	h0 = len(g.State.Players[0].Hand)
	g.must(t, 0, endMain)
	g.must(t, 0, attack(0))
	if got := len(g.State.Players[0].Hand); got != h0 {
		t.Errorf("Field Stalker fired on the opponent's turn: %d -> %d", h0, got)
	}
}

func TestWoadicMarauderDrawsWhenItChangesLanes(t *testing.T) {
	g := realGame(t)
	g.put(1, 0, "woadic-marauder")
	before := len(g.State.Players[1].Hand)
	g.cast(t, 1, "teleport", []TargetRef{creatureRef(1, 0)}, []TargetRef{laneRef(1, 3)})
	// Teleport left the hand (+1 give, -1 cast), Marauder drew one.
	if got := len(g.State.Players[1].Hand); got != before+1 {
		t.Errorf("hand %d -> %d, want +1", before, got)
	}
	// Moving some other Creature does not draw.
	g.put(1, 1, "evil-eye")
	before = len(g.State.Players[1].Hand)
	g.cast(t, 1, "teleport", []TargetRef{creatureRef(1, 1)}, []TargetRef{laneRef(1, 2)})
	if got := len(g.State.Players[1].Hand); got != before {
		t.Errorf("hand %d -> %d, want unchanged", before, got)
	}
}

func TestEnterTriggerTargetsAreListedByLegalActions(t *testing.T) {
	g := realGame(t)
	g.put(1, 0, "the-pig").Status = Flooped
	g.put(1, 1, "the-pig").Status = Flooped
	uid := g.give(1, "psionic-architect")
	n := 0
	for _, a := range g.LegalActions(1) {
		if a.Kind != ActPlayCard || a.Card != uid {
			continue
		}
		n++
		if _, err := g.Clone().Apply(1, a); err != nil {
			t.Errorf("%s: %v", a, err)
		}
	}
	// Lanes 2 and 3 are empty. Each offers: no target, or either Flooped Pig.
	// Lanes 0 and 1 hold Flooped Creatures, which cannot be replaced.
	if n != 2*3 {
		t.Errorf("plays listed = %d, want 6", n)
	}
}

func TestTriggersDoNotFireForCardsOutOfPlay(t *testing.T) {
	g := realGame(t)
	g.State.Players[0].Hand = append(g.State.Players[0].Hand, g.newInstance("field-stalker", 0))
	h0 := len(g.State.Players[0].Hand)
	g.must(t, 1, endMain)
	if got := len(g.State.Players[0].Hand); got != h0+1 {
		t.Errorf("hand %d -> %d: only the normal draw expected", h0, got)
	}
}
