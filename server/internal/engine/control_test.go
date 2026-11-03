package engine

import "testing"

// reaperGame is Jake's turn (player 0) with a Field Reaper and a spare card in hand.
func reaperGame(t *testing.T) (*Game, int, int) {
	t.Helper()
	g := realGame(t)
	g.State.Active = 0
	g.State.ActionsLeft = ActionsPerTurn
	spare := g.State.Players[0].Hand[0].UID
	reaper := g.give(0, "field-reaper")
	return g, reaper, spare
}

func handRef(p PlayerID, uid int) TargetRef {
	return TargetRef{Kind: "hand_card", Player: p, Card: uid}
}

func reap(reaper, lane int, discard int, targets ...[]TargetRef) Action {
	return Action{Kind: ActPlayCard, Card: reaper, Lane: lane,
		CostTargets: [][]TargetRef{{handRef(0, discard)}}, Targets: targets}
}

func TestFieldReaperTakesTheOpposingCreature(t *testing.T) {
	g, reaper, spare := reaperGame(t)
	dog := g.put(1, 1, "cool-dog")
	dog.Damage = 2
	g.must(t, 0, reap(reaper, 1, spare, []TargetRef{creatureRef(1, 1)}, []TargetRef{laneRef(0, 0)}))

	if g.State.Players[1].Lanes[1].Creature != nil {
		t.Fatal("the opposing Creature should have left its Lane")
	}
	mine := g.State.Players[0].Lanes[0].Creature
	if mine != dog || dog.Owner != 1 || dog.Damage != 2 {
		t.Fatalf("stolen Creature = %+v", dog)
	}
	if dog.Status == Ready || dog.Status == Flooped {
		t.Errorf("a moved opposing Creature stays exhausted, not Flooped: %s", dog.Status)
	}
	if g.State.Players[0].Lanes[1].Creature == nil || g.State.Players[0].Lanes[1].Creature.CardID != "field-reaper" {
		t.Error("Field Reaper should still be in its Lane")
	}
	// The discard cost was paid.
	found := false
	for _, in := range g.State.Players[0].Discard {
		found = found || in.UID == spare
	}
	if !found {
		t.Error("the chosen card should have been discarded")
	}
}

func TestStolenCreatureReadiesAtItsNewControllersTurn(t *testing.T) {
	g, reaper, spare := reaperGame(t)
	dog := g.put(1, 1, "cool-dog")
	g.must(t, 0, reap(reaper, 1, spare, []TargetRef{creatureRef(1, 1)}, []TargetRef{laneRef(0, 0)}))
	g.must(t, 0, endMain)
	g.must(t, 0, attack(1)) // the Reaper fights; the stolen Dog is exhausted, so the turn ends
	if g.State.Active != 1 {
		t.Fatalf("setup: active=%d", g.State.Active)
	}
	if dog.Status == Ready {
		t.Error("it must not ready on the opponent's turn")
	}
	g.must(t, 1, endMain)
	if g.State.Active != 0 || dog.Status != Ready {
		t.Errorf("active=%d status=%s: it readies at the start of my next turn", g.State.Active, dog.Status)
	}
}

func TestStolenCreatureDiesToItsOwnersDiscard(t *testing.T) {
	g, reaper, spare := reaperGame(t)
	dog := g.put(1, 1, "cool-dog")
	g.must(t, 0, reap(reaper, 1, spare, []TargetRef{creatureRef(1, 1)}, []TargetRef{laneRef(0, 0)}))
	dog.Damage = 6
	g.State.ActionsLeft = 1
	g.cast(t, 0, "corn-scepter", []TargetRef{creatureRef(0, 0)})
	if g.State.Players[0].Lanes[0].Creature != nil {
		t.Fatal("the Dog should be dead")
	}
	last := g.State.Players[1].Discard
	if len(last) == 0 || last[len(last)-1].UID != dog.UID {
		t.Errorf("owner's discard = %+v", last)
	}
}

func TestFieldReaperMovesItselfWhenNothingOpposes(t *testing.T) {
	g, reaper, spare := reaperGame(t)
	// Only candidate in this Lane is the Reaper itself.
	g.must(t, 0, reap(reaper, 1, spare, []TargetRef{creatureRef(0, 1)}, []TargetRef{laneRef(0, 2)}))
	if g.State.Players[0].Lanes[1].Creature != nil || g.State.Players[0].Lanes[2].Creature == nil {
		t.Error("Field Reaper should have moved to the adjacent Lane")
	}
}

func TestFieldReaperDoesNothingWhenAdjacentLanesAreFull(t *testing.T) {
	g, reaper, spare := reaperGame(t)
	dog := g.put(1, 1, "cool-dog")
	g.put(0, 0, "evil-eye")
	g.put(0, 2, "evil-eye")
	g.must(t, 0, reap(reaper, 1, spare)) // no destination exists: the trigger simply does nothing
	if g.State.Players[1].Lanes[1].Creature != dog {
		t.Error("the opposing Creature does not move")
	}
	// Choosing targets anyway is an error.
	r2 := g.give(0, "field-reaper")
	g.State.ActionsLeft = ActionsPerTurn
	g.mustFail(t, 0, reap(r2, 3, g.State.Players[0].Hand[0].UID, []TargetRef{creatureRef(1, 3)}, []TargetRef{laneRef(0, 2)}), ErrBadTarget)
}

func TestFieldReaperAdditionalCostRules(t *testing.T) {
	g, reaper, spare := reaperGame(t)
	g.mustFail(t, 0, Action{Kind: ActPlayCard, Card: reaper, Lane: 1}, ErrBadTarget)       // must choose a card
	g.mustFail(t, 0, reap(reaper, 1, reaper), ErrBadTarget)                                // not the card being played
	g.mustFail(t, 0, reap(reaper, 1, 9999), ErrBadTarget)                                  // not in hand
	g.mustFail(t, 0, reap(reaper, 1, spare, []TargetRef{creatureRef(0, 1)}), ErrBadTarget) // trigger slot 2 missing... lane is mandatory
	g.State.Players[0].Hand = g.State.Players[0].Hand[len(g.State.Players[0].Hand)-1:]     // only the Reaper left
	g.mustFail(t, 0, Action{Kind: ActPlayCard, Card: reaper, Lane: 1}, ErrNoTarget)        // nothing to discard
	// A card with no additional cost rejects cost targets.
	g2 := realGame(t)
	g2.mustFail(t, 1, Action{Kind: ActPlayCard, Card: g2.give(1, "the-pig"), Lane: 0, CostTargets: [][]TargetRef{{handRef(1, 1)}}}, ErrBadTarget)
}

func TestFieldReaperPaymentFailureLeavesTheGameUntouched(t *testing.T) {
	g, reaper, spare := reaperGame(t)
	before := g.String()
	_, err := g.Apply(0, reap(reaper, 1, spare, []TargetRef{creatureRef(1, 3)}, []TargetRef{laneRef(0, 0)}))
	if err == nil {
		t.Fatal("expected an error")
	}
	if g.String() != before {
		t.Error("a rejected action must not change the game (the discard cost must not be paid)")
	}
}

func TestLegalActionsListAdditionalCostChoices(t *testing.T) {
	g, reaper, _ := reaperGame(t)
	g.put(1, 1, "cool-dog")
	n := 0
	for _, a := range g.LegalActions(0) {
		if a.Kind == ActPlayCard && a.Card == reaper {
			n++
			if len(a.CostTargets) != 1 || len(a.CostTargets[0]) != 1 {
				t.Fatalf("every play must name a discard: %s %+v", a, a.CostTargets)
			}
			if _, err := g.Clone().Apply(0, a); err != nil {
				t.Errorf("%s: %v", a, err)
			}
		}
	}
	if n == 0 {
		t.Error("Field Reaper plays should be listed")
	}
}
