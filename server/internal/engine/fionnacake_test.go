package engine

import (
	"math/rand/v2"
	"testing"

	"cardwars/internal/card"
)

const fcSet = "Fionna Vs Cake"

// fcGame is Cake's Deck (player 0) against Fionna's Deck (player 1), on player
// 1's turn. Both sides have two Cornfields and two Blue Plains, and the decks
// are not held to the 40-card rule so a short one can still be played.
func fcGame(t *testing.T) *Game {
	t.Helper()
	realGame(t)
	mix := [NumLanes]card.Landscape{card.Cornfield, card.Cornfield, card.BluePlains, card.BluePlains}
	g, err := NewGame(realCat, Config{Seed: 1, First: 0, SkipDeckRules: true}, [2]DeckSpec{
		{mix, realCat.Deck(fcSet, "Cake's Deck")}, {mix, realCat.Deck(fcSet, "Fionna's Deck")},
	})
	if err != nil {
		t.Fatal(err)
	}
	g.start(t)
	g.must(t, 0, endMain)
	return g
}

func TestFionnaCakeDecksAreFullyModelled(t *testing.T) {
	fcGame(t)
	for _, name := range []string{"Cake's Deck", "Fionna's Deck"} {
		for _, id := range realCat.Deck(fcSet, name) {
			if !realCat[id].Implemented() {
				t.Errorf("%s: %s is not modelled", name, id)
			}
		}
	}
}

func TestHuskersGrowWithABuildingInTheirLane(t *testing.T) {
	g := fcGame(t)
	g.put(0, 0, "husker-champion") // 1/4
	g.put(0, 1, "husker-valkyrie") // 2/6
	if g.atkOf(0, 0) != 1 || g.defOf(0, 1) != 6 {
		t.Fatalf("without a Building: %d ATK, %d DEF", g.atkOf(0, 0), g.defOf(0, 1))
	}
	g.building(0, 0, "farmhouse")
	g.building(0, 1, "farmhouse")
	if g.atkOf(0, 0) != 3 || g.defOf(0, 0) != 6 || g.atkOf(0, 1) != 4 || g.defOf(0, 1) != 8 {
		t.Errorf("with a Building: %d/%d and %d/%d", g.atkOf(0, 0), g.defOf(0, 0), g.atkOf(0, 1), g.defOf(0, 1))
	}
}

func TestFloopedCreaturesAreStrongerWithDruidAndQueen(t *testing.T) {
	g := fcGame(t)
	g.put(0, 0, "druid-of-the-cob") // 1/8
	queen := g.put(0, 1, "kernel-queen")
	tornado := g.put(0, 2, "tornado-wall-of-fire") // 0/10, +3 while Flooped
	if g.atkOf(0, 1) != 1 || g.atkOf(0, 2) != 0 {
		t.Fatalf("ready: Queen %d, Tornado %d", g.atkOf(0, 1), g.atkOf(0, 2))
	}
	queen.Status, tornado.Status = Flooped, Flooped
	// Queen: 1 + 2 Flooped Creatures + Druid's 1 (she is Flooped); Tornado: 0 + 3 + Druid's 1.
	if g.atkOf(0, 1) != 4 || g.atkOf(0, 2) != 4 || g.atkOf(0, 0) != 1 {
		t.Errorf("Flooped: Queen %d, Tornado %d, Druid %d", g.atkOf(0, 1), g.atkOf(0, 2), g.atkOf(0, 0))
	}
}

func TestBloodCastleRaisesDamageToTheOpposingCreature(t *testing.T) {
	g := fcGame(t)
	g.building(0, 0, "blood-castle")
	mine := g.put(1, 0, "pony")  // opposite the Castle: 1/5
	other := g.put(1, 1, "pony") // another Lane
	g.put(0, 0, "hunkclops")     // 3/3 attacks Pony
	g.put(0, 1, "hunkclops")
	g.must(t, 1, endMain)
	g.State.Active = 0
	g.State.Phase = PhaseFight
	g.must(t, 0, Action{Kind: ActAttack, Lane: 0})
	if mine.Damage != 4 { // 3 and 1 more
		t.Errorf("Pony under Blood Castle took %d, want 4", mine.Damage)
	}
	g.must(t, 0, Action{Kind: ActAttack, Lane: 1})
	if other.Damage != 3 {
		t.Errorf("Pony elsewhere took %d, want 3", other.Damage)
	}
	// It also raises damage from effects, but not from nothing.
	g.State.Phase = PhaseMain
	g.State.Active = 0
	slicer := g.put(0, 0, "mr-slicer")
	g.State.Players[1].Lanes[0].Creature = nil
	foe := g.put(1, 0, "tiny-elephant") // 2/6
	g.State.Turn = 4
	g.must(t, 0, floop(0, []TargetRef{crRef(1, 0)}))
	if slicer.Status != Flooped || foe.Damage != 3 {
		t.Errorf("Mr. Slicer under Blood Castle did %d, want 3", foe.Damage)
	}
}

func TestOilRefineriesAndLearningCenter(t *testing.T) {
	g := fcGame(t)
	for lane := 0; lane < NumLanes; lane++ {
		g.put(1, lane, "pony") // 1/5
	}
	g.building(1, 0, "oil-refinery")
	if g.defOf(1, 0) != 7 {
		t.Errorf("one Oil Refinery: DEF %d, want 7", g.defOf(1, 0))
	}
	g.building(1, 1, "learning-center") // has the Refinery's rules, but is not one
	if g.defOf(1, 1) != 7 || g.defOf(1, 0) != 7 {
		t.Errorf("Learning Center beside it: DEF %d and %d, want 7 and 7", g.defOf(1, 1), g.defOf(1, 0))
	}
	g.building(1, 3, "oil-refinery")
	if g.defOf(1, 0) != 9 || g.defOf(1, 1) != 9 || g.defOf(1, 3) != 9 || g.defOf(1, 2) != 5 {
		t.Errorf("two Refineries: DEF %d %d %d %d", g.defOf(1, 0), g.defOf(1, 1), g.defOf(1, 2), g.defOf(1, 3))
	}
}

func TestBurlyLumberjackKeepsExtraBuildings(t *testing.T) {
	g := fcGame(t)
	g.put(1, 0, "burly-lumberjack")
	g.building(1, 0, "oil-refinery")
	g.State.ActionsLeft = 2
	hut := g.give(1, "farmhouse")
	g.must(t, 1, Action{Kind: ActPlayCard, Card: hut, Lane: 0})
	l := &g.State.Players[1].Lanes[0]
	if l.Building == nil || len(l.Extra) != 1 || l.Extra[0].CardID != "farmhouse" || g.State.Players[1].Discard != nil && len(g.State.Players[1].Discard) > 0 {
		t.Fatalf("Building %v Extra %v", l.Building, l.Extra)
	}
	if g.landscapeCount(1, card.Cornfield) != 3 { // two Cornfields and the Farmhouse
		t.Errorf("Cornfields = %d, want 3", g.landscapeCount(1, card.Cornfield))
	}
	// Without the Lumberjack the next Building replaces both.
	g.State.Players[1].Lanes[0].Creature = nil
	g.State.ActionsLeft = 2
	third := g.give(1, "celestial-fortress")
	g.must(t, 1, Action{Kind: ActPlayCard, Card: third, Lane: 0})
	if len(l.Extra) != 0 || l.Building.CardID != "celestial-fortress" || len(g.State.Players[1].Discard) != 2 {
		t.Errorf("after replacing: Building %v Extra %v discard %d", l.Building, l.Extra, len(g.State.Players[1].Discard))
	}
}

func TestDestroyingABuildingPromotesTheNextOne(t *testing.T) {
	g := fcGame(t)
	l := &g.State.Players[1].Lanes[0]
	g.building(1, 0, "oil-refinery")
	g.State.NextUID++
	l.Extra = []*Building{{Instance: Instance{UID: g.State.NextUID, CardID: "farmhouse", Owner: 1}}}
	g.put(1, 0, "archer-danica")
	g.put(0, 0, "pony")
	g.State.Players[1].Lanes[0].Creature.Status = Ready
	g.must(t, 1, floop(0, []TargetRef{{Kind: "building", Player: 1, Lane: 0}}))
	if l.Building == nil || l.Building.CardID != "farmhouse" || len(l.Extra) != 0 {
		t.Errorf("Building %v Extra %v", l.Building, l.Extra)
	}
	if pony := g.State.Players[0].Lanes[0].Creature; pony == nil || pony.Damage != 3 {
		t.Errorf("Archer Danica should hit the opposing Pony for 3: %+v", pony)
	}
}

func TestCharmingCityMakesEveryCreatureThereRainbow(t *testing.T) {
	g := fcGame(t)
	g.put(1, 0, "crazy-cat-lady") // Rainbow Creatures you control have +1 ATK
	g.put(1, 1, "husker-champion")
	g.put(0, 1, "husker-champion")
	if g.isType(pos{1, 1}, card.Rainbow) || g.atkOf(1, 1) != 1 {
		t.Fatal("a Husker is not a Rainbow Creature")
	}
	g.building(0, 1, "charming-city") // the opponent's Building, in the Lane both Huskers are in
	if !g.isType(pos{1, 1}, card.Rainbow) || g.isType(pos{1, 1}, card.Cornfield) {
		t.Errorf("types in Charming City's Lane: %v", g.creatureTypes(pos{1, 1}))
	}
	if g.atkOf(1, 1) != 2 { // Cat Lady's bonus
		t.Errorf("ATK %d, want 2", g.atkOf(1, 1))
	}
	if g.isType(pos{1, 0}, card.Rainbow) {
		t.Error("Crazy Cat Lady is not Rainbow in another Lane")
	}
}

func TestTinyElephantIsBothAndWoadicMatriarchCountsRainbow(t *testing.T) {
	g := fcGame(t)
	g.put(1, 0, "tiny-elephant")
	g.put(1, 1, "woadic-matriarch") // 1/7, +1 for each Rainbow Creature
	if !g.isType(pos{1, 0}, card.BluePlains) || !g.isType(pos{1, 0}, card.Rainbow) {
		t.Errorf("Tiny Elephant is %v", g.creatureTypes(pos{1, 0}))
	}
	if g.atkOf(1, 1) != 2 {
		t.Errorf("Matriarch ATK %d, want 2", g.atkOf(1, 1))
	}
	g.put(1, 2, "pony")
	g.put(1, 3, "hunkclops")
	if g.atkOf(1, 1) != 4 {
		t.Errorf("Matriarch with four Rainbows... ATK %d, want 4", g.atkOf(1, 1))
	}
}

func TestPuppyParadeHealsAndMakesRainbowForTheTurn(t *testing.T) {
	g := fcGame(t)
	a := g.put(1, 0, "husker-champion")
	b := g.put(1, 1, "kernel-queen")
	a.Damage, b.Damage = 3, 1
	g.State.ActionsLeft = 2
	g.cast(t, 1, "puppy-parade")
	if a.Damage != 1 || b.Damage != 0 || !a.Rainbow || !b.Rainbow {
		t.Errorf("damage %d/%d rainbow %v/%v", a.Damage, b.Damage, a.Rainbow, b.Rainbow)
	}
	if g.isType(pos{1, 0}, card.Cornfield) || !g.isType(pos{1, 0}, card.Rainbow) {
		t.Errorf("types %v", g.creatureTypes(pos{1, 0}))
	}
	g.endTurn(1)
	if a.Rainbow {
		t.Error("the Rainbow lasts only this turn")
	}
}

func TestFarmhouseCountsAsAnExtraCornfield(t *testing.T) {
	g := fcGame(t)
	if g.landscapeCount(0, card.Cornfield) != 2 {
		t.Fatal("setup")
	}
	g.building(0, 3, "farmhouse")
	if g.landscapeCount(0, card.Cornfield) != 3 || g.landscapeCount(0, card.BluePlains) != 2 {
		t.Errorf("Cornfields %d, Blue Plains %d", g.landscapeCount(0, card.Cornfield), g.landscapeCount(0, card.BluePlains))
	}
	// Cabin of Many Woods needs five and costs 1 less for each Flooped Creature.
	g.State.Active = 0
	g.State.ActionsLeft = 2
	cabin := g.give(0, "cabin-of-many-woods")
	g.mustFail(t, 0, Action{Kind: ActPlayCard, Card: cabin, Lane: 0}, ErrNoActions)
	for lane := 0; lane < 3; lane++ {
		g.put(0, lane, "pony").Status = Flooped
	}
	g.State.Players[0].Lanes[3].Landscape = card.Cornfield
	g.State.Players[0].Lanes[2].Landscape = card.Cornfield
	// Cost 5 less 3 = 2 Actions; 2 Cornfields needed, and there are four plus the Farmhouse.
	g.must(t, 0, Action{Kind: ActPlayCard, Card: cabin, Lane: 0})
	if g.State.ActionsLeft != 0 {
		t.Errorf("Actions left %d, want 0", g.State.ActionsLeft)
	}
	if g.defOf(0, 0) != 5+5 { // Pony 1/5 plus the Cabin's 5 DEF
		t.Errorf("DEF %d, want 10", g.defOf(0, 0))
	}
}

func TestWoadicEnchantressStopsFloopsInItsLane(t *testing.T) {
	g := fcGame(t)
	g.put(0, 0, "woadic-enchantress")
	g.put(1, 0, "mr-slicer")
	g.put(1, 1, "mr-slicer")
	g.put(0, 1, "pony")
	g.State.Turn = 4
	g.mustFail(t, 1, floop(0, []TargetRef{crRef(0, 0)}), ErrNoFloop)
	g.must(t, 1, floop(1, []TargetRef{crRef(0, 1)}))
}

func TestTheDogSilencesOpposingBuildingsWhileFlooped(t *testing.T) {
	g := fcGame(t)
	dog := g.put(1, 0, "the-dog")
	g.building(0, 0, "celestial-fortress") // -2 DEF to the opposing Creature, the Dog
	g.put(0, 0, "pony")
	g.State.Turn = 4
	if g.defOf(1, 0) != 5 { // The Dog is 1/7, minus 2
		t.Fatalf("DEF %d, want 5", g.defOf(1, 0))
	}
	g.must(t, 1, floop(0))
	if dog.Status != Flooped || g.defOf(1, 0) != 7 {
		t.Errorf("Flooped Dog: %s, DEF %d, want 7", dog.Status, g.defOf(1, 0))
	}
}

func TestPlayingDogsAndRainbowCardsPowersUpBusinessDogAndScholar(t *testing.T) {
	g := fcGame(t)
	g.State.ActionsLeft = 5
	bd := g.give(1, "business-dog")
	g.must(t, 1, Action{Kind: ActPlayCard, Card: bd, Lane: 0})
	if g.atkOf(1, 0) != 3 { // itself counts: 1 + 2
		t.Errorf("Business Dog ATK %d, want 3", g.atkOf(1, 0))
	}
	g.must(t, 1, Action{Kind: ActPlayCard, Card: g.give(1, "puppy-parade"), Lane: 0})
	if g.atkOf(1, 0) != 5 {
		t.Errorf("after Puppy Parade ATK %d, want 5", g.atkOf(1, 0))
	}
	g.State.ActionsLeft = 5
	g.put(1, 1, "infant-scholar")
	if g.atkOf(1, 1) != 1 {
		t.Fatalf("Scholar without a Rainbow card played: ATK %d", g.atkOf(1, 1))
	}
	g.must(t, 1, Action{Kind: ActPlayCard, Card: g.give(1, "pony"), Lane: 2})
	if g.atkOf(1, 1) != 4 {
		t.Errorf("Scholar after a Rainbow Creature: ATK %d, want 4", g.atkOf(1, 1))
	}
}

func TestWoadicWeirdoMakesACardRainbow(t *testing.T) {
	g := fcGame(t)
	g.State.Turn = 4
	g.put(1, 0, "woadic-weirdo")
	g.State.ActionsLeft = 2
	g.State.Players[1].Lanes[1].Landscape = card.IcyLands
	g.State.Players[1].Lanes[2].Landscape = card.IcyLands
	g.State.Players[1].Lanes[3].Landscape = card.IcyLands
	g.State.Players[1].Lanes[0].Landscape = card.IcyLands
	husker := g.give(1, "husker-valkyrie") // Cornfield: needs a Cornfield, and the deck has none
	g.mustFail(t, 1, Action{Kind: ActPlayCard, Card: husker, Lane: 1}, ErrLandscapes)
	g.must(t, 1, floop(0, []TargetRef{handRef(1, husker)}))
	if !g.IsRainbowInHand(husker) {
		t.Fatal("the card did not become Rainbow")
	}
	g.must(t, 1, Action{Kind: ActPlayCard, Card: husker, Lane: 1})
	if g.IsRainbowInHand(husker) {
		t.Error("a played card is no longer a hand card")
	}
}

func TestFiddlingFerretGainsAnActionForDiscardingRainbow(t *testing.T) {
	g := fcGame(t)
	g.State.Turn = 4
	g.put(1, 0, "fiddling-ferret")
	g.State.Players[1].Hand = nil
	pony := g.give(1, "pony")
	g.must(t, 1, floop(0))
	// It drew a card, so there are two to choose from.
	if g.State.Choice == nil || g.State.Choice.Kind != ChoiceDiscard {
		t.Fatalf("choice %+v", g.State.Choice)
	}
	before := g.State.ActionsLeft
	g.must(t, 1, Action{Kind: ActChoose, Card: pony})
	if g.State.ActionsLeft != before+1 {
		t.Errorf("Actions %d, want %d", g.State.ActionsLeft, before+1)
	}
}

func TestFriendshipBraceletMovesChosenDamage(t *testing.T) {
	g := fcGame(t)
	a := g.put(1, 0, "husker-champion")
	b := g.put(1, 1, "kernel-queen")
	a.Damage = 3
	g.State.ActionsLeft = 2
	g.cast(t, 1, "friendship-bracelet", []TargetRef{crRef(1, 0)}, []TargetRef{crRef(1, 1)})
	ch := g.State.Choice
	if ch == nil || ch.Kind != ChoiceMoveDamage || ch.Count != 3 {
		t.Fatalf("choice %+v", ch)
	}
	if got := len(g.choiceActions(1)); got != 3 {
		t.Errorf("%d amounts offered, want 3", got)
	}
	g.mustFail(t, 1, Action{Kind: ActChoose, Card: 4}, ErrBadTarget)
	g.must(t, 1, Action{Kind: ActChoose, Card: 2})
	if a.Damage != 1 || b.Damage != 2 || g.State.Choice != nil {
		t.Errorf("damage %d and %d", a.Damage, b.Damage)
	}
	// Moving damage onto a Creature that cannot take it destroys it.
	g.cast(t, 1, "friendship-bracelet", []TargetRef{crRef(1, 0)}, []TargetRef{crRef(1, 1)}) // 1 damage: no choice
	if b.Damage != 3 || g.State.Choice != nil {
		t.Errorf("one damage moves at once: %d", b.Damage)
	}
}

func TestDeforestationExhaustsACreatureAndSearchesForABuilding(t *testing.T) {
	g := fcGame(t)
	g.State.ActionsLeft = 2
	pony := g.put(1, 0, "pony")
	pl := &g.State.Players[1]
	pl.Deck = append(pl.Deck, g.newInstance("oil-refinery", 1), g.newInstance("farmhouse", 1))
	uid := g.give(1, "deforestation")
	deck := len(pl.Deck)
	// A Creature must be exhausted to pay for it.
	g.mustFail(t, 1, Action{Kind: ActPlayCard, Card: uid, Targets: [][]TargetRef{{laneRef(1, 2)}}}, ErrBadTarget)
	g.must(t, 1, Action{Kind: ActPlayCard, Card: uid, CostTargets: [][]TargetRef{{crRef(1, 0)}}, Targets: [][]TargetRef{{laneRef(1, 2)}}})
	if pony.Status != Flooped {
		t.Errorf("Pony is %s", pony.Status)
	}
	ch := g.State.Choice
	if ch == nil || ch.Kind != ChoiceSearch || len(ch.Revealed) < 2 {
		t.Fatalf("choice %+v", ch)
	}
	for _, in := range ch.Revealed {
		if g.Card(in).Type != card.Building {
			t.Errorf("%s offered but is not a Building", in.CardID)
		}
	}
	pick := ch.Revealed[0]
	g.mustFail(t, 1, Action{Kind: ActChoose, Card: 9999}, ErrNoSuchCard)
	g.must(t, 1, Action{Kind: ActChoose, Card: pick.UID})
	if b := pl.Lanes[2].Building; b == nil || b.UID != pick.UID {
		t.Errorf("Lane 2 has %v", pl.Lanes[2].Building)
	}
	if len(pl.Deck) != deck-1 || g.State.Choice != nil {
		t.Errorf("deck %d (was %d)", len(pl.Deck), deck)
	}
}

func TestDeforestationWithNoBuildingLeftInTheDeckJustShuffles(t *testing.T) {
	g := fcGame(t)
	g.State.ActionsLeft = 2
	g.put(1, 0, "pony")
	pl := &g.State.Players[1]
	pl.Deck = slicesDeleteBuildings(g, pl.Deck)
	uid := g.give(1, "deforestation")
	g.must(t, 1, Action{Kind: ActPlayCard, Card: uid, CostTargets: [][]TargetRef{{crRef(1, 0)}}, Targets: [][]TargetRef{{laneRef(1, 2)}}})
	if g.State.Choice != nil || pl.Lanes[2].Building != nil {
		t.Errorf("choice %+v building %v", g.State.Choice, pl.Lanes[2].Building)
	}
}

func slicesDeleteBuildings(g *Game, deck []Instance) []Instance {
	var out []Instance
	for _, in := range deck {
		if g.Card(in).Type != card.Building {
			out = append(out, in)
		}
	}
	return out
}

func TestHarvestMoonPutsABuildingFromTheDiscardPileIntoPlay(t *testing.T) {
	g := fcGame(t)
	g.State.ActionsLeft = 2
	g.State.Players[1].Discard = append(g.State.Players[1].Discard, g.newInstance("oil-refinery", 1))
	bld := g.State.Players[1].Discard[0]
	g.cast(t, 1, "harvest-moon", []TargetRef{{Kind: "discard_card", Player: 1, Card: bld.UID}}, []TargetRef{laneRef(1, 1)})
	if b := g.State.Players[1].Lanes[1].Building; b == nil || b.UID != bld.UID {
		t.Errorf("Lane 1 has %v", g.State.Players[1].Lanes[1].Building)
	}
	for _, in := range g.State.Players[1].Discard {
		if in.UID == bld.UID {
			t.Error("the Building is still in the discard pile")
		}
	}
}

func TestImpossiblePossumReturnsForADiscard(t *testing.T) {
	g := fcGame(t)
	g.put(1, 0, "impossible-possum") // 2/8
	foe := g.put(0, 0, "hunkclops")
	g.State.Players[1].Lanes[0].Creature.Damage = 7
	g.State.Players[0].Lanes[0].Creature.Status = Ready
	g.State.Active = 0
	g.State.Phase = PhaseFight
	g.State.Turn = 4
	g.must(t, 0, Action{Kind: ActAttack, Lane: 0}) // 3 damage kills it
	_ = foe
	ch := g.State.Choice
	if ch == nil || ch.Kind != ChoiceRebound || ch.Player != 1 {
		t.Fatalf("choice %+v", ch)
	}
	if g.State.Players[1].Lanes[0].Creature != nil {
		t.Fatal("the Possum should be gone")
	}
	hand := len(g.State.Players[1].Hand)
	discard := g.State.Players[1].Hand[0].UID
	g.must(t, 1, Action{Kind: ActChoose, Card: discard})
	if len(g.State.Players[1].Hand) != hand { // one out, the Possum in
		t.Errorf("hand %d, was %d", len(g.State.Players[1].Hand), hand)
	}
	found := false
	for _, in := range g.State.Players[1].Hand {
		found = found || in.CardID == "impossible-possum"
	}
	if !found {
		t.Error("the Possum did not come back")
	}
}

func TestImpossiblePossumCanStayInTheDiscardPile(t *testing.T) {
	g := fcGame(t)
	g.put(1, 0, "impossible-possum")
	g.State.Players[1].Lanes[0].Creature.Damage = 7
	g.put(0, 0, "hunkclops")
	g.State.Active, g.State.Phase, g.State.Turn = 0, PhaseFight, 4
	g.must(t, 0, Action{Kind: ActAttack, Lane: 0})
	g.must(t, 1, Action{Kind: ActChoose})
	for _, in := range g.State.Players[1].Hand {
		if in.CardID == "impossible-possum" {
			t.Fatal("it came back anyway")
		}
	}
	if g.State.Choice != nil {
		t.Error("the choice is still open")
	}
}

func TestBounceDoesNotTriggerPossum(t *testing.T) {
	g := fcGame(t)
	g.put(1, 0, "impossible-possum")
	g.removeCreature(pos{1, 0}, toHand)
	if g.State.Choice != nil {
		t.Error("returning to hand is not going to the discard pile")
	}
}

func TestPiedPiperLetsAFloopedCreatureFightInstead(t *testing.T) {
	g := fcGame(t)
	g.State.Active, g.State.Phase, g.State.Turn = 0, PhaseFight, 4
	piper := g.put(0, 0, "pied-piper") // 1/9
	hunk := g.put(0, 1, "hunkclops")   // 3/3
	hunk.Status = Flooped
	foe := g.put(1, 0, "pony") // 1/5
	g.must(t, 0, Action{Kind: ActAttack, Lane: 0})
	ch := g.State.Choice
	if ch == nil || ch.Kind != ChoiceRedirect || len(ch.Revealed) != 1 || ch.Revealed[0].UID != hunk.UID {
		t.Fatalf("choice %+v", ch)
	}
	if foe.Damage != 0 {
		t.Fatal("the Fight should wait for the answer")
	}
	g.must(t, 0, Action{Kind: ActChoose, Card: hunk.UID})
	if foe.Damage != 3 || hunk.Damage != 1 || piper.Damage != 0 {
		t.Errorf("Pony %d, Hunkclops %d, Piper %d damage", foe.Damage, hunk.Damage, piper.Damage)
	}
	if piper.Status != Fought || hunk.Status != Flooped {
		t.Errorf("Piper %s, Hunkclops %s", piper.Status, hunk.Status)
	}
}

func TestPiedPiperMayFightItself(t *testing.T) {
	g := fcGame(t)
	g.State.Active, g.State.Phase, g.State.Turn = 0, PhaseFight, 4
	piper := g.put(0, 0, "pied-piper")
	g.put(0, 1, "hunkclops").Status = Flooped
	foe := g.put(1, 0, "pony")
	g.must(t, 0, Action{Kind: ActAttack, Lane: 0})
	g.must(t, 0, Action{Kind: ActChoose})
	if foe.Damage != 1 || piper.Damage != 1 {
		t.Errorf("Pony %d, Piper %d damage", foe.Damage, piper.Damage)
	}
}

func TestPiedPiperWithNoFloopedCreatureJustFights(t *testing.T) {
	g := fcGame(t)
	g.State.Active, g.State.Phase, g.State.Turn = 0, PhaseFight, 4
	g.put(0, 0, "pied-piper")
	foe := g.put(1, 0, "pony")
	g.must(t, 0, Action{Kind: ActAttack, Lane: 0})
	if g.State.Choice != nil || foe.Damage != 1 {
		t.Errorf("choice %+v, damage %d", g.State.Choice, foe.Damage)
	}
}

func TestVampireLordHealsWhatItDeals(t *testing.T) {
	g := fcGame(t)
	g.State.Active, g.State.Phase, g.State.Turn = 0, PhaseFight, 4
	lord := g.put(0, 0, "vampire-lord") // 1/8
	lord.Damage = 3
	g.put(1, 0, "hunkclops") // 3/3 hits back
	v2 := g.put(0, 1, "vampire-lord")
	v2.Damage = 2
	g.State.Players[1].HP = 20
	g.must(t, 0, Action{Kind: ActAttack, Lane: 0})
	if lord.Damage != 5 { // 3 + 3 taken - 1 healed
		t.Errorf("damage %d, want 5", lord.Damage)
	}
	g.must(t, 0, Action{Kind: ActAttack, Lane: 1})
	if g.State.Players[1].HP != 19 || v2.Damage != 1 {
		t.Errorf("HP %d, damage %d", g.State.Players[1].HP, v2.Damage)
	}
}

func TestDrawnCardsPowerSwordBouquetAndSingingSword(t *testing.T) {
	g := fcGame(t)
	g.State.ActionsLeft = 2
	g.put(1, 0, "pony")
	foe := g.put(0, 0, "tiny-elephant")
	g.State.Drawn[1] = 0
	g.State.ActionsLeft = 6
	g.cast(t, 1, "sword-bouquet", []TargetRef{crRef(0, 0)})
	if foe.Damage != 0 {
		t.Errorf("no cards drawn: %d damage", foe.Damage)
	}
	g.must(t, 1, Action{Kind: ActDraw})
	g.must(t, 1, Action{Kind: ActDraw})
	if g.State.Drawn[1] != 2 {
		t.Fatalf("drawn %d", g.State.Drawn[1])
	}
	g.cast(t, 1, "sword-bouquet", []TargetRef{crRef(0, 0)})
	if foe.Damage != 2 {
		t.Errorf("two cards drawn: %d damage, want 2", foe.Damage)
	}
	g.cast(t, 1, "singing-sword", []TargetRef{crRef(1, 0)})
	if g.atkOf(1, 0) != 2 { // 1 + 1 for every 2 drawn
		t.Errorf("Pony ATK %d, want 2", g.atkOf(1, 0))
	}
}

func TestLonelyPandaDrawsOnlyWhenAlone(t *testing.T) {
	g := fcGame(t)
	g.State.ActionsLeft = 2
	hand := len(g.State.Players[1].Hand)
	g.must(t, 1, Action{Kind: ActPlayCard, Card: g.give(1, "lonely-panda"), Lane: 0})
	if len(g.State.Players[1].Hand) != hand+1 {
		t.Errorf("alone: hand %d, want %d", len(g.State.Players[1].Hand), hand+1)
	}
	hand = len(g.State.Players[1].Hand)
	g.State.ActionsLeft = 2
	g.must(t, 1, Action{Kind: ActPlayCard, Card: g.give(1, "lonely-panda"), Lane: 1})
	if len(g.State.Players[1].Hand) != hand { // the second one is not alone
		t.Errorf("not alone: hand %d, want %d", len(g.State.Players[1].Hand), hand)
	}
}

func TestToughLumberjillHitsForEachBuilding(t *testing.T) {
	g := fcGame(t)
	g.building(1, 0, "farmhouse")
	g.building(1, 1, "oil-refinery")
	g.State.ActionsLeft = 2
	g.must(t, 1, Action{Kind: ActPlayCard, Card: g.give(1, "tough-lumberjill"), Lane: 2})
	if g.State.Players[0].HP != StartingHP-2 {
		t.Errorf("HP %d, want %d", g.State.Players[0].HP, StartingHP-2)
	}
}

func TestPsionicSwashbucklerHitsAFloopedCreature(t *testing.T) {
	g := fcGame(t)
	g.State.ActionsLeft = 2
	ready := g.put(0, 0, "hunkclops")
	flooped := g.put(0, 1, "hunkclops")
	flooped.Status = Flooped
	uid := g.give(1, "psionic-swashbuckler")
	g.mustFail(t, 1, Action{Kind: ActPlayCard, Card: uid, Lane: 0, Targets: [][]TargetRef{{crRef(0, 0)}}}, ErrBadTarget)
	g.must(t, 1, Action{Kind: ActPlayCard, Card: uid, Lane: 0, Targets: [][]TargetRef{{crRef(0, 1)}}})
	if ready.Damage != 0 || g.State.Players[0].Lanes[1].Creature != nil { // 3 damage on a 3/3
		t.Errorf("ready %d, flooped alive %v", ready.Damage, g.State.Players[0].Lanes[1].Creature != nil)
	}
}

func TestLumbercaddyAndPopcornButteredfly(t *testing.T) {
	g := fcGame(t)
	g.State.Turn = 4
	g.put(1, 0, "lumbercaddy")
	g.put(1, 1, "popcorn-butteredfly")
	g.building(1, 0, "farmhouse")
	g.building(1, 1, "oil-refinery")
	g.must(t, 1, floop(0, []TargetRef{{Kind: "building", Player: 1, Lane: 0}}, []TargetRef{laneRef(1, 2)}))
	if g.State.Players[1].Lanes[0].Building != nil || g.State.Players[1].Lanes[2].Building == nil {
		t.Error("the Farmhouse did not move")
	}
	hand := len(g.State.Players[1].Hand)
	g.must(t, 1, floop(1))
	if len(g.State.Players[1].Hand) != hand+2 {
		t.Errorf("hand %d, want %d", len(g.State.Players[1].Hand), hand+2)
	}
}

func TestMiscFloopsAndSpells(t *testing.T) {
	g := fcGame(t)
	g.State.Turn = 4
	// Corns Templar heals itself; Helping Hand takes a Building back from the discard pile.
	templar := g.put(1, 0, "corns-templar")
	templar.Damage = 3
	g.must(t, 1, floop(0))
	if templar.Damage != 2 {
		t.Errorf("damage %d", templar.Damage)
	}
	g.put(1, 1, "helping-hand")
	g.State.Players[1].Discard = append(g.State.Players[1].Discard, g.newInstance("farmhouse", 1))
	b := g.State.Players[1].Discard[0]
	g.must(t, 1, floop(1, []TargetRef{{Kind: "discard_card", Player: 1, Card: b.UID}}))
	if g.uid(t, 1, "farmhouse") != b.UID {
		t.Error("the Farmhouse is not in hand")
	}
	// Spells: Void Thimble, River of Swords, Log Rhythm, Furious Furor, Glorious Gramophone.
	g.State.ActionsLeft = 10
	g.cast(t, 1, "void-thimble")
	if g.State.Players[1].HP != StartingHP-2 || g.State.ActionsLeft != 11 {
		t.Errorf("HP %d, Actions %d", g.State.Players[1].HP, g.State.ActionsLeft)
	}
	g.cast(t, 1, "river-of-swords")
	if g.atkOf(1, 0) != 3 { // Templar 1 + 2
		t.Errorf("Templar ATK %d", g.atkOf(1, 0))
	}
	g.building(1, 1, "farmhouse")
	g.State.Players[1].Lanes[0].Creature.TempATK = 0
	g.State.Players[1].Lanes[1].Creature.TempATK = 0
	g.cast(t, 1, "log-rhythm")
	if g.atkOf(1, 1) != 3 || g.atkOf(1, 0) != 1 { // only the Lane with a Building
		t.Errorf("ATK %d and %d", g.atkOf(1, 1), g.atkOf(1, 0))
	}
	templar.Status, g.State.Players[1].Lanes[1].Creature.Status = Flooped, Flooped
	g.cast(t, 1, "furious-furor", []TargetRef{crRef(1, 0)})
	if g.atkOf(1, 0) != 1+2*2 {
		t.Errorf("Furor: ATK %d, want 5", g.atkOf(1, 0))
	}
	g.State.Players[1].Hand = nil
	g.cast(t, 1, "glorious-gramophone")
	if g.State.Choice == nil || g.State.Choice.Kind != ChoiceDiscard || g.State.Choice.Count != 2 || len(g.State.Players[1].Hand) != 4 {
		t.Errorf("choice %+v hand %d", g.State.Choice, len(g.State.Players[1].Hand))
	}
}

func TestEmboldenedRetrieverDrawsWhenItAttacks(t *testing.T) {
	g := fcGame(t)
	g.State.Active, g.State.Phase, g.State.Turn = 0, PhaseFight, 4
	g.put(0, 0, "emboldened-retriever")
	hand := len(g.State.Players[0].Hand)
	g.must(t, 0, Action{Kind: ActAttack, Lane: 0})
	if len(g.State.Players[0].Hand) != hand+1 {
		t.Errorf("hand %d, want %d", len(g.State.Players[0].Hand), hand+1)
	}
}

func TestFionnaCakeRandomPlayoutsKeepInvariants(t *testing.T) {
	realGame(t)
	games := 150
	if testing.Short() {
		games = 15
	}
	mixC := [NumLanes]card.Landscape{card.Cornfield, card.Cornfield, card.Cornfield, card.Cornfield}
	mixB := [NumLanes]card.Landscape{card.BluePlains, card.BluePlains, card.BluePlains, card.BluePlains}
	seen := map[EventKind]int{}
	for seed := uint64(1); seed <= uint64(games); seed++ {
		g, err := NewGame(realCat, Config{Seed: seed, First: PlayerID(seed % 2), SkipDeckRules: true}, [2]DeckSpec{
			{mixC, realCat.Deck(fcSet, "Cake's Deck")}, {mixB, realCat.Deck(fcSet, "Fionna's Deck")},
		})
		if err != nil {
			t.Fatal(err)
		}
		playRandomSeen(t, g, rand.New(rand.NewPCG(seed, 11)), 20000, seen)
	}
	t.Logf("events: %v", seen)
}
