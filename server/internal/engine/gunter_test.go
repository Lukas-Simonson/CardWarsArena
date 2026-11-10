package engine

import (
	"slices"
	"testing"

	"cardwars/internal/card"
)

// gunterGame is Gunter's Deck (player 0, Icy Lands in Lanes 0 and 1, Blue Plains in 2 and 3) against Finn's
// Deck (player 1), in player 0's main phase on turn 3, with both hands empty and the decks full.
func gunterGame(t *testing.T) *Game {
	t.Helper()
	realGame(t)
	ls := [NumLanes]card.Landscape{card.IcyLands, card.IcyLands, card.BluePlains, card.BluePlains}
	g, err := NewGame(realCat, Config{Seed: 1, First: 0}, [2]DeckSpec{
		{Landscapes: ls, Cards: realCat.Deck("Lemongrab Vs Gunter", "Gunter's Deck"), Hero: "gunter"},
		{Landscapes: cornfields, Cards: realCat.Deck("Finn Vs Jake", "Finn's Deck"), Hero: "finn"},
	})
	if err != nil {
		t.Fatal(err)
	}
	g.start(t)
	g.State.Turn = 3
	for p := range g.State.Players {
		g.State.Players[p].Hand = nil
	}
	return g
}

func (g *Game) frozenAt(p PlayerID, lane int) bool { return g.State.Players[p].Lanes[lane].Frozen }

func (g *Game) playIn(t *testing.T, p PlayerID, id string, lane int) []Event {
	t.Helper()
	return g.must(t, p, Action{Kind: ActPlayCard, Card: g.give(p, id), Lane: lane})
}

func (g *Game) freezeBoth(t *testing.T, lane int) {
	t.Helper()
	g.cast(t, 0, "snowblower", []TargetRef{laneRef(0, lane)})
}

func TestGuntersDeckIsFullyModelled(t *testing.T) {
	g := gunterGame(t)
	for _, id := range realCat.Deck("Lemongrab Vs Gunter", "Gunter's Deck") {
		if !realCat[id].Implemented() {
			t.Errorf("%s is not modelled", id)
		}
	}
	if g.State.Players[0].Hero.CardID != "gunter" {
		t.Error("Gunter's deck did not come with Gunter")
	}
}

func TestFreezingPutsOneTokenOnTheLandscapeAndKeepsItsType(t *testing.T) {
	g := gunterGame(t)
	g.freezeBoth(t, 0)
	if !g.frozenAt(0, 0) || !g.frozenAt(1, 0) || g.frozenAt(0, 1) {
		t.Fatalf("a Lane's two Landscapes should be frozen: %v %v %v", g.frozenAt(0, 0), g.frozenAt(1, 0), g.frozenAt(0, 1))
	}
	if g.State.Players[0].Lanes[0].Landscape != card.IcyLands {
		t.Error("a frozen Landscape lost its type")
	}
	if n := g.frozenCount([]PlayerID{0, 1}); n != 2 {
		t.Errorf("%d frozen Landscapes", n)
	}
	// Freezing a frozen Landscape again gives it no second token and is not an error.
	g.State.ActionsLeft = 2
	g.freezeBoth(t, 0)
	if n := g.frozenCount([]PlayerID{0, 1}); n != 2 {
		t.Errorf("a second freeze changed the count to %d", n)
	}
	// A token stays when the Landscape is turned over, and a face-down Landscape can be frozen.
	g.State.Players[0].Lanes[0].FaceDown = true
	if !g.frozenAt(0, 0) {
		t.Error("flipping removed the token")
	}
	g.State.Players[0].Lanes[1].FaceDown = true
	g.freeze(0, 1)
	if !g.frozenAt(0, 1) {
		t.Error("a face-down Landscape could not be frozen")
	}
}

func TestNoCreatureMayBePlayedOntoAFrozenLandscape(t *testing.T) {
	g := gunterGame(t)
	g.freezeBoth(t, 0)
	g.State.ActionsLeft = 2
	pup := g.give(0, "icy-commando")
	g.mustFail(t, 0, Action{Kind: ActPlayCard, Card: pup, Lane: 0}, ErrFrozen)
	g.must(t, 0, Action{Kind: ActPlayCard, Card: pup, Lane: 1}) // another Lane is fine
	// The opponent's Landscape in that Lane is frozen too.
	g.State.Active = 1
	g.State.ActionsLeft = 2
	g.mustFail(t, 1, Action{Kind: ActPlayCard, Card: g.give(1, "pony"), Lane: 0}, ErrFrozen)
	g.State.Active, g.State.ActionsLeft = 0, 2
	// Buildings are not stopped.
	g.playIn(t, 0, "fancy-spa", 0)
	if g.State.Players[0].Lanes[0].Building == nil {
		t.Error("a Building could not be played on a frozen Landscape")
	}
}

func TestACreatureAlreadyThereStaysAndCannotBeReplaced(t *testing.T) {
	g := gunterGame(t)
	g.put(0, 0, "icy-commando")
	g.freezeBoth(t, 0)
	if g.State.Players[0].Lanes[0].Creature == nil {
		t.Fatal("the Creature already there was removed")
	}
	g.State.ActionsLeft = 2
	g.mustFail(t, 0, Action{Kind: ActPlayCard, Card: g.give(0, "glacier-racer"), Lane: 0}, ErrFrozen)
	// Not even Frosted Deanimator, which may go onto a frozen Landscape, may replace one.
	g.mustFail(t, 0, Action{Kind: ActPlayCard, Card: g.give(0, "frosted-deanimator"), Lane: 0}, ErrFrozen)
}

func TestFrostedDeanimatorMayGoOntoAnEmptyFrozenLandscape(t *testing.T) {
	g := gunterGame(t)
	g.freezeBoth(t, 0)
	g.State.ActionsLeft = 2
	g.playIn(t, 0, "frosted-deanimator", 0)
	if g.State.Players[0].Lanes[0].Creature == nil {
		t.Error("Frosted Deanimator was not played onto the frozen Landscape")
	}
}

func TestRemovingAFrozenTokenCostsACardFromYourHand(t *testing.T) {
	g := gunterGame(t)
	g.freezeBoth(t, 0)
	g.State.ActionsLeft = 2
	card1 := g.give(0, "icy-commando")
	pup := g.give(0, "glacier-racer")
	g.mustFail(t, 0, Action{Kind: ActThaw, Lane: 1, Card: card1}, ErrNotFrozen)
	g.mustFail(t, 0, Action{Kind: ActThaw, Lane: 0, Card: 9999}, ErrNoSuchCard)

	hand := len(g.State.Players[0].Hand)
	discard := len(g.State.Players[0].Discard)
	ev := g.must(t, 0, Action{Kind: ActThaw, Lane: 0, Card: card1})
	if g.frozenAt(0, 0) || !g.frozenAt(1, 0) {
		t.Error("only the token on your own side should come off")
	}
	if len(g.State.Players[0].Hand) != hand-1 || len(g.State.Players[0].Discard) != discard+1 || g.State.ActionsLeft != 2 {
		t.Errorf("hand %d, discard %d, actions %d: removing a token costs one card and no Actions", len(g.State.Players[0].Hand), len(g.State.Players[0].Discard), g.State.ActionsLeft)
	}
	if !slices.ContainsFunc(ev, func(e Event) bool { return e.Kind == EvThaw && e.Lane == 0 }) {
		t.Error("no thaw event")
	}
	// At once, a Creature may be played there again.
	g.must(t, 0, Action{Kind: ActPlayCard, Card: pup, Lane: 0})
	// You cannot remove a token on the opponent's side: the action only names your own Landscapes.
	g.mustFail(t, 0, Action{Kind: ActThaw, Lane: 0, Card: g.give(0, "icy-commando")}, ErrNotFrozen)
}

func TestRemovingTokensIsOfferedAndOnlyInTheMainPhase(t *testing.T) {
	g := gunterGame(t)
	g.freezeBoth(t, 2)
	g.give(0, "icy-commando")
	g.give(0, "glacier-racer")
	n := 0
	for _, a := range g.LegalActions(0) {
		if a.Kind == ActThaw {
			n++
			if a.Lane != 2 {
				t.Errorf("offered to thaw Lane %d", a.Lane)
			}
		}
	}
	if n != 2 {
		t.Errorf("%d thaw actions offered, want one for each card in hand", n)
	}
	g.State.Phase = PhaseFight
	g.mustFail(t, 0, Action{Kind: ActThaw, Lane: 2, Card: g.State.Players[0].Hand[0].UID}, ErrWrongPhase)
}

func TestMovingAFrozenLandscapesCreatureIsAllowed(t *testing.T) {
	g := gunterGame(t)
	g.freezeBoth(t, 0)
	foe := g.put(1, 3, "pony")
	g.State.ActionsLeft = 2
	g.State.Players[0].HP = 15 // Magenapping needs 15 Hit Points or fewer
	// Magenapping moves a Creature onto a frozen, empty Landscape.
	g.cast(t, 0, "magenapping", []TargetRef{{Kind: "creature", Player: 1, Lane: 3}}, []TargetRef{laneRef(0, 0)})
	if g.State.Players[0].Lanes[0].Creature != foe {
		t.Error("the Creature was not moved onto the frozen Landscape")
	}
}

func TestEffectsThatCountFrozenLandscapes(t *testing.T) {
	g := gunterGame(t)
	commando := g.put(0, 2, "icy-commando") // 1/7, +1 ATK for each frozen Landscape
	if g.atkOf(0, 2) != 1 {
		t.Fatalf("Icy Commando ATK = %d", g.atkOf(0, 2))
	}
	g.freezeBoth(t, 0)
	if g.atkOf(0, 2) != 3 {
		t.Errorf("with two frozen Landscapes Icy Commando has ATK %d, want 3", g.atkOf(0, 2))
	}
	g.State.ActionsLeft = 2
	g.State.Players[0].Lanes[3].Frozen = true // the opponent's frozen Landscapes count too
	g.State.Players[1].Lanes[3].Frozen = true
	if g.atkOf(0, 2) != 5 {
		t.Errorf("Icy Commando ATK = %d, want 5", g.atkOf(0, 2))
	}
	_ = commando
}

func TestGlacierRacerNeedsBothLandscapesFrozen(t *testing.T) {
	g := gunterGame(t)
	g.put(0, 2, "glacier-racer") // 2/6
	g.State.Players[0].Lanes[2].Frozen = true
	if g.atkOf(0, 2) != 3 { // only Gunter's +1 for being on a frozen Landscape
		t.Errorf("ATK with one Landscape frozen = %d, want 3", g.atkOf(0, 2))
	}
	g.State.Players[1].Lanes[2].Frozen = true
	if g.atkOf(0, 2) != 7 { // +4, and Gunter's +1
		t.Errorf("ATK with both frozen = %d, want 7", g.atkOf(0, 2))
	}
}

func TestGunterGivesFrozenCreaturesAttack(t *testing.T) {
	g := gunterGame(t)
	g.put(0, 0, "glacier-racer")
	g.put(0, 1, "glacier-racer")
	g.put(1, 1, "pony")
	before := g.atkOf(0, 0)
	g.State.Players[0].Lanes[0].Frozen = true
	if g.atkOf(0, 0) != before+1 || g.atkOf(0, 1) != before {
		t.Errorf("ATK %d and %d, want %d and %d", g.atkOf(0, 0), g.atkOf(0, 1), before+1, before)
	}
	g.State.Players[1].Lanes[1].Frozen = true // the opponent's own Creature is not helped by your Hero
	if g.atkOf(1, 1) != baseStat(realCat["pony"].Attack) {
		t.Errorf("the opponent's Creature has ATK %d", g.atkOf(1, 1))
	}
}

func TestFloatingIcePalaceHelpsOnlyOnYourTurn(t *testing.T) {
	g := gunterGame(t)
	g.put(0, 3, "glacier-racer")
	g.Put(0, 3, "floating-ice-palace")
	g.State.Players[1].Lanes[0].Frozen = true
	g.State.Players[1].Lanes[1].Frozen = true
	base := baseStat(realCat["glacier-racer"].Attack)
	if g.atkOf(0, 3) != base+2 {
		t.Errorf("ATK on your turn = %d, want %d", g.atkOf(0, 3), base+2)
	}
	g.State.Active = 1
	if g.atkOf(0, 3) != base {
		t.Errorf("ATK on the opponent's turn = %d, want %d", g.atkOf(0, 3), base)
	}
}

func TestIcyInfiltratorFreezesItsOwnLandscape(t *testing.T) {
	g := gunterGame(t)
	g.playIn(t, 0, "icy-infiltrator", 2)
	if !g.frozenAt(0, 2) || g.frozenAt(1, 2) {
		t.Errorf("own %v, opposite %v: only its own Landscape freezes", g.frozenAt(0, 2), g.frozenAt(1, 2))
	}
}

func TestFrostedSnowwomanFreezesBothLandscapesInItsLane(t *testing.T) {
	g := gunterGame(t)
	g.playIn(t, 0, "frosted-snowwoman", 0)
	if !g.frozenAt(0, 0) || !g.frozenAt(1, 0) {
		t.Error("both Landscapes in its Lane should be frozen")
	}
	if g.State.Players[0].Lanes[0].Creature == nil {
		t.Error("it should stay on the Landscape it froze")
	}
}

func TestTheCoolerFreezesItsLandscapeWhenItLeavesPlay(t *testing.T) {
	g := gunterGame(t)
	g.put(0, 3, "the-cooler")
	if g.frozenAt(0, 3) {
		t.Fatal("setup: already frozen")
	}
	g.destroyAt(pos{0, 3})
	if !g.frozenAt(0, 3) || g.frozenAt(1, 3) {
		t.Errorf("own %v, opposite %v: only its own Landscape freezes", g.frozenAt(0, 3), g.frozenAt(1, 3))
	}
}

func TestIcyIntruderFreezesAChosenLandscapeForAnAction(t *testing.T) {
	g := gunterGame(t)
	g.put(0, 0, "icy-intruder")
	g.State.ActionsLeft = 2
	g.must(t, 0, floop(0, []TargetRef{{Kind: "landscape", Player: 1, Lane: 3}}))
	if !g.frozenAt(1, 3) || g.frozenAt(0, 3) {
		t.Error("the chosen Landscape was not frozen")
	}
	if g.State.ActionsLeft != 1 {
		t.Errorf("Actions left = %d, want 1: its Floop costs one", g.State.ActionsLeft)
	}
	g.State.ActionsLeft = 0
	g.put(0, 1, "icy-intruder")
	g.mustFail(t, 0, floop(1, []TargetRef{{Kind: "landscape", Player: 1, Lane: 2}}), ErrNoActions)
}

func TestSnowTrainingCampOnlyWorksWhileNothingIsFrozen(t *testing.T) {
	g := gunterGame(t)
	g.Put(0, 1, "snow-training-camp")
	g.must(t, 0, Action{Kind: ActFloop, Lane: 1, Source: "building"})
	if !g.frozenAt(0, 1) || !g.frozenAt(1, 1) {
		t.Error("both Landscapes in its Lane should be frozen")
	}
	g.State.Players[0].Lanes[1].Building.Status = Ready
	g.mustFail(t, 0, Action{Kind: ActFloop, Lane: 1, Source: "building"}, ErrCondition)
}

func TestSnowBallerRemovesItsOwnTokenToDealDamage(t *testing.T) {
	g := gunterGame(t)
	g.put(0, 3, "snow-baller")
	foe := g.put(1, 0, "pony")
	g.mustFail(t, 0, Action{Kind: ActActivate, Lane: 3, Targets: [][]TargetRef{{{Kind: "creature", Player: 1, Lane: 0}}}}, ErrCondition)
	g.State.Players[0].Lanes[3].Frozen = true
	g.must(t, 0, Action{Kind: ActActivate, Lane: 3, Targets: [][]TargetRef{{{Kind: "creature", Player: 1, Lane: 0}}}})
	if g.frozenAt(0, 3) {
		t.Error("the token stayed")
	}
	if foe.Damage != 3 {
		t.Errorf("damage = %d, want 3", foe.Damage)
	}
	if g.State.ActionsLeft != 2 {
		t.Error("it should cost no Actions")
	}
}

func TestSnowvalancheHitsOnlyOpposingCreaturesOnFrozenLandscapes(t *testing.T) {
	g := gunterGame(t)
	a := g.put(1, 0, "wall-of-ears")
	b := g.put(1, 1, "wall-of-ears")
	mine := g.put(0, 0, "icy-commando")
	g.State.Players[1].Lanes[0].Frozen = true
	g.State.Players[0].Lanes[0].Frozen = true
	g.cast(t, 0, "snowvalanche")
	if a.Damage != 2 || b.Damage != 0 || mine.Damage != 0 {
		t.Errorf("damage %d, %d and %d, want 2, 0 and 0", a.Damage, b.Damage, mine.Damage)
	}
}

func TestSpazzyColaAndWakeUpCallAndTappedOut(t *testing.T) {
	g := gunterGame(t)
	g.put(0, 0, "icy-commando")
	g.put(1, 0, "wall-of-ears")
	a0, b0 := g.atkOf(0, 0), g.atkOf(1, 0)
	g.cast(t, 0, "spazzy-cola")
	if g.atkOf(0, 0) != a0+1 || g.atkOf(1, 0) != b0+1 {
		t.Error("every Creature should have +1 ATK this turn, the opponent's too")
	}

	g = gunterGame(t)
	x, y := g.put(0, 0, "icy-commando"), g.put(0, 1, "icy-commando")
	x.Status, y.Status = Flooped, Flooped
	g.State.Players[0].Lanes[2].Creature = &Creature{Instance: Instance{UID: 900, CardID: "icy-commando", Owner: 0}, Status: Fought}
	g.cast(t, 0, "wake-up-call")
	if x.Status != Ready || y.Status != Ready {
		t.Error("Wake Up Call should Ready your exhausted Creatures")
	}

	g = gunterGame(t)
	g.put(0, 0, "icy-commando").Status = Flooped
	g.put(0, 1, "icy-commando").Status = Fought
	target := g.put(0, 2, "icy-commando")
	before := g.atkOf(0, 2)
	g.cast(t, 0, "tapped-out", []TargetRef{{Kind: "creature", Player: 0, Lane: 2}})
	if g.atkOf(0, 2) != before+4 {
		t.Errorf("ATK %d, want %d: +2 for each of two exhausted Creatures", g.atkOf(0, 2), before+4)
	}
	_ = target
}

func TestFatigueLibrarianEntersExhaustedAndStaysSoUntilItMoves(t *testing.T) {
	for _, id := range []string{"fatigued-librarian", "weakened-warrior"} {
		g := gunterGame(t)
		g.State.ActionsLeft = 2
		g.playIn(t, 0, id, 2)
		cr := g.State.Players[0].Lanes[2].Creature
		if cr.Status == Ready {
			t.Fatalf("%s entered Ready", id)
		}
		g.State.Players[0].Hand = nil
		g.must(t, 0, endMain)
		g.must(t, 1, endMain)
		if g.State.Active != 0 || cr.Status == Ready {
			t.Errorf("%s: it Readied at the start of its controller's turn (active %d, status %s)", id, g.State.Active, cr.Status)
		}
		g.moveCreature(pos{0, 2}, pos{0, 3})
		if cr.Status != Ready {
			t.Errorf("%s: changing Lanes should Ready it, status %s", id, cr.Status)
		}
	}
}

func TestFancySpaDrawsWhenYouHaveAnExhaustedCreatureInItsLane(t *testing.T) {
	g := gunterGame(t)
	g.Put(0, 1, "fancy-spa")
	cr := g.put(0, 1, "icy-commando")
	g.put(0, 0, "icy-commando") // Ready, so there is a Fight phase
	cr.Status = Flooped
	g.State.Players[0].Deck = append([]Instance{g.newInstance("pony", 0)}, g.State.Players[0].Deck...)
	hand := len(g.State.Players[0].Hand)
	g.must(t, 0, endMain)
	if len(g.State.Players[0].Hand) != hand+1 {
		t.Errorf("hand %d, want %d: the Spa draws for an exhausted Creature in its Lane", len(g.State.Players[0].Hand), hand+1)
	}

	g = gunterGame(t)
	g.Put(0, 1, "fancy-spa")
	g.put(0, 1, "icy-commando") // Ready
	hand = len(g.State.Players[0].Hand)
	g.must(t, 0, endMain)
	if len(g.State.Players[0].Hand) != hand {
		t.Error("the Spa drew with no exhausted Creature in its Lane")
	}
}

func TestWearyTradingPostMovesAnExhaustedCreatureHere(t *testing.T) {
	g := gunterGame(t)
	g.Put(0, 0, "weary-trading-post")
	tired := g.put(0, 3, "icy-commando")
	tired.Status = Flooped
	g.must(t, 0, Action{Kind: ActFloop, Lane: 0, Source: "building", Targets: [][]TargetRef{{{Kind: "creature", Player: 0, Lane: 3}}, {laneRef(0, 0)}}})
	if g.State.Players[0].Lanes[0].Creature != tired || g.State.Players[0].Lanes[3].Creature != nil {
		t.Error("the exhausted Creature was not moved to the Post's Lane")
	}
	// With a Creature already in the Lane there is nowhere to move it.
	g.State.Players[0].Lanes[0].Building.Status = Ready
	g.State.Players[0].Lanes[1].Creature = &Creature{Instance: Instance{UID: 901, CardID: "icy-commando", Owner: 0}, Status: Flooped}
	g.mustFail(t, 0, Action{Kind: ActFloop, Lane: 0, Source: "building", Targets: [][]TargetRef{{{Kind: "creature", Player: 0, Lane: 1}}, {laneRef(0, 0)}}}, ErrNoTarget)
}

func startOfTurn(t *testing.T, g *Game, p PlayerID) {
	t.Helper()
	g.State.Players[p].Hand = nil
	for lane := range g.State.Players[p.Opp()].Lanes {
		if cr := g.State.Players[p.Opp()].Lanes[lane].Creature; cr != nil {
			cr.Status = Fought // nothing for the other side to Fight with, so its turn ends at once
		}
	}
	g.State.Active = p.Opp()
	g.State.Phase = PhaseMain
	g.must(t, p.Opp(), endMain)
	if g.State.Active != p {
		t.Fatalf("active %d, want %d", g.State.Active, p)
	}
}

func TestStartOfTurnMovers(t *testing.T) {
	// One empty Landscape: it goes there without asking.
	g := gunterGame(t)
	g.put(0, 0, "drained-cleric")
	g.put(0, 1, "icy-commando")
	g.put(0, 2, "icy-commando")
	startOfTurn(t, g, 0)
	if g.State.Choice != nil || g.creatureAt(pos{0, 3}) == nil || g.creatureAt(pos{0, 0}) != nil {
		t.Errorf("Drained Cleric should move to Lane 4 (choice %v)", g.State.Choice)
	}

	// Several: its controller chooses.
	g = gunterGame(t)
	cr := g.put(0, 0, "drained-cleric")
	startOfTurn(t, g, 0)
	if g.State.Choice == nil || g.State.Choice.Kind != ChoiceMove || !slices.Equal(g.State.Choice.Lanes, []int{1, 2, 3}) {
		t.Fatalf("expected a choice of Lanes 2 to 4, got %+v", g.State.Choice)
	}
	if got := g.LegalActions(0); len(got) != 3 || got[0].Kind != ActChoose {
		t.Errorf("legal actions %v", got)
	}
	g.mustFail(t, 0, Action{Kind: ActChoose, Lane: 0}, ErrBadLane)
	g.must(t, 0, Action{Kind: ActChoose, Lane: 2})
	if g.creatureAt(pos{0, 2}) != cr || g.State.Choice != nil {
		t.Error("the Creature did not move to the Lane that was chosen")
	}

	// None: Drained Cleric discards a card, Woadic Ring Leader is destroyed, Tired Wombat moves to the opponent.
	g = gunterGame(t)
	for lane := 1; lane < 4; lane++ {
		g.put(0, lane, "icy-commando")
	}
	g.put(0, 0, "drained-cleric")
	g.State.Players[0].Deck = append([]Instance{g.newInstance("pony", 0)}, g.State.Players[0].Deck...)
	discard := len(g.State.Players[0].Discard)
	startOfTurn(t, g, 0) // draws one card, which is all it has to discard
	if g.State.Choice != nil || len(g.State.Players[0].Hand) != 0 || len(g.State.Players[0].Discard) != discard+1 {
		t.Errorf("Drained Cleric with nowhere to go should discard a card: choice %+v, hand %d", g.State.Choice, len(g.State.Players[0].Hand))
	}

	g = gunterGame(t)
	for lane := 1; lane < 4; lane++ {
		g.put(0, lane, "icy-commando")
	}
	g.put(0, 0, "woadic-ring-leader")
	startOfTurn(t, g, 0)
	if g.creatureAt(pos{0, 0}) != nil {
		t.Error("Woadic Ring Leader should be destroyed when it cannot move")
	}

	g = gunterGame(t)
	for lane := 1; lane < 4; lane++ {
		g.put(0, lane, "icy-commando")
	}
	wombat := g.put(0, 0, "tired-wombat")
	g.put(1, 0, "pony") // the opponent's empty Landscapes are Lanes 2 to 4
	startOfTurn(t, g, 0)
	if g.State.Choice == nil || g.State.Choice.MoveSide != 1 || !slices.Equal(g.State.Choice.Lanes, []int{1, 2, 3}) {
		t.Fatalf("Tired Wombat should choose among the opponent's empty Landscapes: %+v", g.State.Choice)
	}
	g.must(t, 0, Action{Kind: ActChoose, Lane: 3})
	if g.State.Players[1].Lanes[3].Creature != wombat {
		t.Error("Tired Wombat did not move to the opponent's side")
	}
}

func TestCuteOverloadDrawsForEachCreatureDestroyed(t *testing.T) {
	g := gunterGame(t)
	g.put(0, 0, "icy-commando")
	g.put(0, 1, "icy-commando")
	g.put(0, 2, "icy-commando")
	hand := len(g.State.Players[0].Hand)
	g.cast(t, 0, "cute-overload", []TargetRef{{Kind: "creature", Player: 0, Lane: 0}, {Kind: "creature", Player: 0, Lane: 2}})
	if g.creatureAt(pos{0, 0}) != nil || g.creatureAt(pos{0, 2}) != nil || g.creatureAt(pos{0, 1}) == nil {
		t.Error("it should destroy exactly the chosen Creatures")
	}
	if len(g.State.Players[0].Hand) != hand+2 {
		t.Errorf("hand %d, want %d: one card for each of two Creatures", len(g.State.Players[0].Hand), hand+2)
	}
	// "Any number" includes none.
	g.State.ActionsLeft = 2
	hand = len(g.State.Players[0].Hand)
	g.cast(t, 0, "cute-overload", nil)
	if len(g.State.Players[0].Hand) != hand || g.creatureAt(pos{0, 1}) == nil {
		t.Error("choosing no Creatures should do nothing")
	}
}

func TestIndustrialAssassinDestroysACreatureOfYoursToDraw(t *testing.T) {
	g := gunterGame(t)
	g.put(0, 0, "industrial-assassin")
	g.put(0, 1, "icy-commando")
	hand := len(g.State.Players[0].Hand)
	g.must(t, 0, floop(0, []TargetRef{{Kind: "creature", Player: 0, Lane: 1}}))
	if g.creatureAt(pos{0, 1}) != nil || len(g.State.Players[0].Hand) != hand+1 {
		t.Error("it should destroy the chosen Creature and draw a card")
	}
}

func TestMagenappingHasPlayConditions(t *testing.T) {
	g := gunterGame(t)
	g.State.ActionsLeft = 2
	g.put(0, 0, "icy-commando")
	g.put(0, 1, "icy-commando")
	foe := g.put(1, 0, "pony")
	spell := g.give(0, "magenapping")
	args := [][]TargetRef{{{Kind: "creature", Player: 1, Lane: 0}}, {laneRef(0, 2)}}
	g.mustFail(t, 0, Action{Kind: ActPlayCard, Card: spell, Targets: args}, ErrCondition) // two Creatures
	g.State.Players[0].Lanes[1].Creature = nil
	g.State.Players[0].HP = 16
	g.mustFail(t, 0, Action{Kind: ActPlayCard, Card: spell, Targets: args}, ErrCondition) // too many Hit Points
	g.State.Players[0].HP = 15
	g.must(t, 0, Action{Kind: ActPlayCard, Card: spell, Targets: args})
	if g.State.Players[0].Lanes[2].Creature != foe {
		t.Error("Magenapping did not move the Creature to your empty Landscape")
	}
}
