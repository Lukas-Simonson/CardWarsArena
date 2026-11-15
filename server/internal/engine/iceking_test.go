package engine

import (
	"math/rand/v2"
	"slices"
	"testing"

	"cardwars/internal/card"
)

// iceGame is Ice King's Deck (player 0, Icy Lands everywhere) against Finn's Deck (player 1, Cornfield), in
// player 0's main phase on turn 3 with both hands empty.
func iceGame(t *testing.T) *Game {
	t.Helper()
	realGame(t)
	ls := [NumLanes]card.Landscape{card.IcyLands, card.IcyLands, card.IcyLands, card.IcyLands}
	g, err := NewGame(realCat, Config{Seed: 1, First: 0}, [2]DeckSpec{
		{Landscapes: ls, Cards: realCat.Deck("Ice King Vs Marceline", "Ice King's Deck"), Hero: "ice-king"},
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

func (g *Game) freezeAt(p PlayerID, lane int) { g.State.Players[p].Lanes[lane].Frozen = true }

func TestIceKingsDeckIsFullyModelled(t *testing.T) {
	g := iceGame(t)
	for _, id := range realCat.Deck("Ice King Vs Marceline", "Ice King's Deck") {
		if !realCat[id].Implemented() {
			t.Errorf("%s is not modelled", id)
		}
	}
	if g.State.Players[0].Hero.CardID != "ice-king" || !realCat["ice-king"].Implemented() {
		t.Error("Ice King's deck did not come with a modelled Ice King")
	}
}

func TestAbdominalSnowmanAndColdSoldierHitHardWhenUnopposed(t *testing.T) {
	g := iceGame(t)
	g.put(0, 0, "abdominal-snowman") // 3 ATK
	g.put(0, 1, "cold-soldier")      // 2 ATK
	g.put(1, 1, "pony")
	if g.atkOf(0, 0) != 6 || g.atkOf(0, 1) != 2 {
		t.Errorf("ATK %d and %d, want 6 and 2", g.atkOf(0, 0), g.atkOf(0, 1))
	}
}

func TestBoarderColliePicksUpAnActionOnAFrozenLane(t *testing.T) {
	g := iceGame(t)
	g.put(0, 0, "boarder-collie")
	g.State.ActionsLeft = 1
	g.must(t, 0, floop(0)) // three empty Landscapes: it asks where
	ch := g.State.Choice
	if ch == nil || ch.Kind != ChoiceMove || !slices.Equal(ch.Lanes, []int{1, 2, 3}) {
		t.Fatalf("choice %+v", ch)
	}
	g.freezeAt(1, 2) // the opponent's Landscape opposite Lane 3
	g.must(t, 0, Action{Kind: ActChoose, Lane: 2})
	if g.creatureAt(pos{0, 2}) == nil || g.State.ActionsLeft != 2 {
		t.Errorf("Actions left = %d, want 2 (it moved to a Lane with a frozen Landscape)", g.State.ActionsLeft)
	}

	// A Lane with no frozen Landscape gains nothing; with a single empty Landscape it goes without asking.
	g = iceGame(t)
	g.put(0, 0, "boarder-collie")
	for _, lane := range []int{1, 2} {
		g.put(0, lane, "pony")
	}
	g.State.ActionsLeft = 1
	g.must(t, 0, floop(0))
	if g.State.Choice != nil || g.creatureAt(pos{0, 3}) == nil || g.State.ActionsLeft != 1 {
		t.Errorf("choice %v, Actions left %d", g.State.Choice, g.State.ActionsLeft)
	}
	g.put(0, 0, "pony")
	g.State.Players[0].Lanes[3].Creature.Status = Ready
	g.mustFail(t, 0, floop(3), ErrCondition) // nowhere left to go: its Floop is not offered
}

func TestEmperorPenguinChillsEveryFrozenCreature(t *testing.T) {
	g := iceGame(t)
	g.put(0, 0, "emperor-penguin")
	g.put(0, 1, "cold-soldier")
	g.put(1, 1, "cold-soldier")
	g.freezeAt(0, 1)
	g.freezeAt(1, 0)
	if g.atkOf(0, 1) != 2-1 || g.atkOf(1, 1) != 2 || g.atkOf(0, 0) != 1 {
		t.Errorf("ATK %d %d %d", g.atkOf(0, 1), g.atkOf(1, 1), g.atkOf(0, 0))
	}
	g.freezeAt(0, 0) // the Penguin itself, on a frozen Landscape
	if g.atkOf(0, 0) != 0 {
		t.Errorf("the Penguin's own ATK is %d", g.atkOf(0, 0))
	}
}

func TestFrostDragonAndSnowBusinessFreezeInTheirOwnLane(t *testing.T) {
	g := iceGame(t)
	g.put(0, 1, "frost-dragon")
	g.must(t, 0, activate(1, 0, []TargetRef{lsRef(1, 1)}))
	if !g.frozenAt(1, 1) || g.State.ActionsLeft != 1 {
		t.Errorf("frozen %v, Actions %d", g.frozenAt(1, 1), g.State.ActionsLeft)
	}
	g.mustFail(t, 0, activate(1, 0, []TargetRef{lsRef(1, 2)}), ErrBadTarget)

	g.Put(0, 2, "snow-business")
	g.must(t, 0, Action{Kind: ActActivate, Lane: 2, Source: "building", Targets: [][]TargetRef{{lsRef(0, 2)}}})
	if !g.frozenAt(0, 2) || g.State.ActionsLeft != 0 {
		t.Errorf("Snow Business: frozen %v, Actions %d", g.frozenAt(0, 2), g.State.ActionsLeft)
	}
}

func TestFrozenFishMayFreezeAfterDamagingACreature(t *testing.T) {
	g := iceGame(t)
	g.put(0, 0, "frozen-fish")
	g.put(1, 0, "pony")
	g.State.Phase = PhaseFight
	g.must(t, 0, attack(0))
	ch := g.State.Choice
	if ch == nil || ch.Kind != ChoiceTarget || !ch.Optional || len(ch.Targets) != 2 {
		t.Fatalf("choice %+v", ch)
	}
	if got := g.LegalActions(0); len(got) != 3 {
		t.Errorf("legal %v: two Landscapes or none", got)
	}
	g.mustFail(t, 0, Action{Kind: ActChoose, Targets: [][]TargetRef{{lsRef(0, 2)}}}, ErrBadTarget) // not in its Lane
	g.must(t, 0, Action{Kind: ActChoose, Targets: [][]TargetRef{{lsRef(1, 0)}}})
	if !g.frozenAt(1, 0) || g.frozenAt(0, 0) || g.State.Choice != nil {
		t.Errorf("frozen %v %v, choice %v", g.frozenAt(1, 0), g.frozenAt(0, 0), g.State.Choice)
	}

	// It may decline.
	g = iceGame(t)
	g.put(0, 0, "frozen-fish")
	g.put(1, 0, "pony")
	g.State.Phase = PhaseFight
	g.must(t, 0, attack(0))
	g.must(t, 0, Action{Kind: ActChoose})
	if g.frozenAt(0, 0) || g.frozenAt(1, 0) {
		t.Error("declining froze something")
	}

	// Hitting the player is not hitting a Creature.
	g = iceGame(t)
	g.put(0, 0, "frozen-fish")
	g.State.Phase = PhaseFight
	g.must(t, 0, attack(0))
	if g.State.Choice != nil {
		t.Errorf("hitting the opposing Hero asked %+v", g.State.Choice)
	}

	// Defending counts too, and the choice is its controller's.
	g = iceGame(t)
	g.put(1, 0, "pony")
	g.put(0, 0, "frozen-fish")
	g.State.Active, g.State.Phase = 1, PhaseFight
	g.must(t, 1, attack(0))
	if g.State.Choice == nil || g.State.Choice.Player != 0 {
		t.Errorf("choice %+v", g.State.Choice)
	}
}

func TestIcemeisterBurnsEveryCreatureOnAFrozenLandscapeAtTheStartOfYourTurn(t *testing.T) {
	g := iceGame(t)
	g.put(0, 0, "icemeister")
	mine := g.put(0, 1, "cold-soldier")
	foe := g.put(1, 2, "pony")
	other := g.put(1, 3, "pony")
	g.freezeAt(0, 1)
	g.freezeAt(1, 2)
	startOfTurn(t, g, 0)
	if mine.Damage != 1 || foe.Damage != 1 || other.Damage != 0 || g.State.Players[0].Lanes[0].Creature.Damage != 0 {
		t.Errorf("damage %d %d %d", mine.Damage, foe.Damage, other.Damage)
	}
}

func TestCreaturesThatCountFrozenLandscapes(t *testing.T) {
	g := iceGame(t)
	g.put(0, 0, "sprucy-lucy") // 2 ATK
	g.put(0, 1, "reign-deer")
	hurt := g.put(0, 2, "snow-angel")
	hurt.Damage = 5
	ally := g.put(0, 3, "cold-soldier")
	ally.Damage = 3
	g.freezeAt(0, 0)
	g.freezeAt(1, 3)
	if g.atkOf(0, 0) != 4 {
		t.Errorf("Sprucy Lucy ATK %d, want 4", g.atkOf(0, 0))
	}
	g.State.Players[0].Hand = nil
	g.State.Players[0].Deck = append([]Instance{{UID: 801, CardID: "pony", Owner: 0}, {UID: 802, CardID: "pony", Owner: 0}, {UID: 803, CardID: "pony", Owner: 0}}, g.State.Players[0].Deck...)
	g.must(t, 0, floop(1))
	if len(g.State.Players[0].Hand) != 2 {
		t.Errorf("Reign Deer drew %d, want 2", len(g.State.Players[0].Hand))
	}
	g.must(t, 0, floop(2))
	if hurt.Damage != 3 || ally.Damage != 1 {
		t.Errorf("Snow Angel: damage %d and %d, want 3 and 1", hurt.Damage, ally.Damage)
	}
}

func TestSnowDogBoostsOnlyAdjacentCreaturesThisTurn(t *testing.T) {
	g := iceGame(t)
	g.put(0, 1, "snow-dog")
	a := g.put(0, 0, "cold-soldier")
	b := g.put(0, 2, "cold-soldier")
	c := g.put(0, 3, "cold-soldier")
	g.put(1, 0, "pony")
	g.put(1, 2, "pony")
	g.put(1, 3, "pony")
	g.must(t, 0, floop(1))
	if a.TempATK != 2 || b.TempATK != 2 || c.TempATK != 0 {
		t.Errorf("boosts %d %d %d", a.TempATK, b.TempATK, c.TempATK)
	}
}

func TestSlayRiderNeedsACreatureOnAFrozenLandscape(t *testing.T) {
	g := iceGame(t)
	g.put(0, 0, "slay-rider")
	foe := g.put(1, 1, "wall-of-ears")
	g.mustFail(t, 0, floop(0, []TargetRef{creatureRef(1, 1)}), ErrNoTarget)
	g.freezeAt(1, 1)
	g.must(t, 0, floop(0, []TargetRef{creatureRef(1, 1)}))
	if foe.Damage != 3 {
		t.Errorf("damage %d, want 3", foe.Damage)
	}
}

func TestSnowBunnyDestroysItselfToFreeze(t *testing.T) {
	g := iceGame(t)
	g.put(0, 0, "snow-bunny")
	g.must(t, 0, activate(0, 0, []TargetRef{lsRef(1, 3)}))
	if g.creatureAt(pos{0, 0}) != nil || !g.frozenAt(1, 3) || g.State.ActionsLeft != 2 {
		t.Errorf("bunny %v frozen %v Actions %d", g.creatureAt(pos{0, 0}), g.frozenAt(1, 3), g.State.ActionsLeft)
	}
	if n := len(g.State.Players[0].Discard); n != 1 {
		t.Errorf("discard holds %d", n)
	}
}

func TestCrystalPalaceHitsACreatureWheneverALandscapeInItsLaneFreezes(t *testing.T) {
	g := iceGame(t)
	g.Put(0, 1, "crystal-palace")
	foe := g.put(1, 3, "wall-of-ears")
	// Two Creatures in play: its controller picks one.
	mine := g.put(0, 0, "cold-soldier")
	g.freeze(1, 1)
	ch := g.State.Choice
	if ch == nil || ch.Kind != ChoiceTarget || ch.Optional || len(ch.Targets) != 2 {
		t.Fatalf("choice %+v", ch)
	}
	g.must(t, 0, Action{Kind: ActChoose, Targets: [][]TargetRef{{creatureRef(1, 3)}}})
	if foe.Damage != 2 || mine.Damage != 0 {
		t.Errorf("damage %d and %d", foe.Damage, mine.Damage)
	}
	// Another Lane freezing does nothing, and neither does a token that is already there.
	g.freeze(1, 2)
	g.freeze(1, 1)
	if g.State.Choice != nil {
		t.Errorf("choice %+v", g.State.Choice)
	}
	// With one Creature in play there is only one thing to hit.
	g.State.Players[0].Lanes[0].Creature = nil
	g.freeze(0, 1)
	if g.State.Choice != nil || foe.Damage != 4 {
		t.Errorf("damage %d, choice %v", foe.Damage, g.State.Choice)
	}
	// A Building on the other side counts for its own controller.
	g.Put(1, 0, "crystal-palace")
	g.freeze(0, 0)
	if g.State.Choice != nil || g.creatureAt(pos{1, 3}) == nil {
		t.Errorf("choice %v", g.State.Choice)
	}
}

func TestIceOlationCellKeepsFrozenCreaturesFromFightingOrUsingAbilities(t *testing.T) {
	g := iceGame(t)
	g.Put(0, 0, "ice-olation-cell")
	dog := g.put(0, 0, "snow-dog")
	g.put(0, 1, "snow-dog")
	g.freezeAt(0, 1) // frozen, but not in the Cell's Lane
	g.must(t, 0, floop(1))
	g.freezeAt(0, 0)
	g.mustFail(t, 0, floop(0), ErrInert)
	foe := g.put(1, 0, "pony")
	g.freezeAt(1, 0)
	g.State.Phase = PhaseFight
	g.mustFail(t, 0, attack(0), ErrCannotAttack)
	dog.Status = Ready
	g.State.Phase = PhaseMain
	// The Cell does not care whose side a Creature is on.
	if !g.inertAt(pos{1, 0}) || foe == nil {
		t.Error("the opposing Creature on a frozen Landscape in the Cell's Lane should be inert too")
	}
	// Nor does it silence Creatures whose Landscape is not frozen.
	g.State.Players[0].Lanes[0].Frozen = false
	g.State.Active = 0
	g.must(t, 0, floop(0))
}

func TestFreezeRayAndSnowWayAndFreezingPoint(t *testing.T) {
	g := iceGame(t)
	g.State.Players[0].Deck = append([]Instance{{UID: 801, CardID: "pony", Owner: 0}}, g.State.Players[0].Deck...)
	g.cast(t, 0, "freeze-ray", []TargetRef{lsRef(1, 0)})
	if !g.frozenAt(1, 0) || len(g.State.Players[0].Hand) != 1 {
		t.Errorf("Freeze Ray: frozen %v, hand %d", g.frozenAt(1, 0), len(g.State.Players[0].Hand))
	}

	g = iceGame(t)
	g.castFails(t, 0, "snow-way", ErrBadTarget, []TargetRef{lsRef(1, 0)}, []TargetRef{lsRef(1, 0)})
	g.cast(t, 0, "snow-way", []TargetRef{lsRef(1, 0)}, []TargetRef{lsRef(0, 3)})
	if !g.frozenAt(1, 0) || !g.frozenAt(0, 3) || g.frozenCount([]PlayerID{0, 1}) != 2 {
		t.Errorf("Snow Way froze %d Landscapes", g.frozenCount([]PlayerID{0, 1}))
	}

	g = iceGame(t)
	g.freezeAt(1, 2)
	g.cast(t, 0, "freezing-point")
	for lane := 0; lane < NumLanes; lane++ {
		if !g.frozenAt(1, lane) || g.frozenAt(0, lane) {
			t.Errorf("Lane %d: opponent %v, you %v", lane, g.frozenAt(1, lane), g.frozenAt(0, lane))
		}
	}
	// It was played as player 1 would too: their opponent is the one frozen.
	g = iceGame(t)
	g.runOps(realCat["freezing-point"].Effects[0].Do, effCtx{p: 1, at: pos{1, 0}, card: "freezing-point"}, nil)
	if g.frozenAt(1, 0) || !g.frozenAt(0, 0) {
		t.Error("player 1's Freezing Point froze the wrong side")
	}
}

func TestFrostyFrolicMakesThemDiscardForTheirOwnFrozenLandscapes(t *testing.T) {
	g := iceGame(t)
	g.freezeAt(0, 0) // yours do not count
	g.freezeAt(1, 1)
	g.freezeAt(1, 2)
	g.State.Players[1].Hand = nil
	for i := 0; i < 4; i++ {
		g.give(1, "pony")
	}
	g.cast(t, 0, "frosty-frolic")
	ch := g.State.Choice
	if ch == nil || ch.Kind != ChoiceDiscard || ch.Player != 1 || ch.Count != 2 {
		t.Fatalf("choice %+v", ch)
	}
}

func TestFrozenHeartDealsThreeOnAFrozenLandscape(t *testing.T) {
	g := iceGame(t)
	cold := g.put(1, 0, "wall-of-ears")
	hot := g.put(1, 1, "wall-of-ears")
	g.freezeAt(1, 0)
	g.cast(t, 0, "frozen-heart", []TargetRef{creatureRef(1, 0)})
	g.State.ActionsLeft = 2
	g.cast(t, 0, "frozen-heart", []TargetRef{creatureRef(1, 1)})
	if cold.Damage != 3 || hot.Damage != 1 {
		t.Errorf("damage %d and %d, want 3 and 1", cold.Damage, hot.Damage)
	}
}

func TestIceKingGivesTheFirstCreatureThatHitsYourOpponentPlusOneAttack(t *testing.T) {
	g := iceGame(t)
	g.put(0, 0, "cold-soldier") // 2 ATK, +3 unopposed
	g.put(0, 1, "cold-soldier")
	g.put(1, 2, "pony")
	g.put(0, 2, "cold-soldier")
	g.State.Phase = PhaseFight
	hp := g.State.Players[1].HP
	g.must(t, 0, attack(2)) // into a Creature: no bonus
	if g.State.Players[0].Lanes[2].Creature.TempATK != 0 {
		t.Error("attacking a Creature is not attacking the opponent")
	}
	g.must(t, 0, attack(0))
	if got := hp - g.State.Players[1].HP; got != 6 {
		t.Errorf("first hit on the opponent dealt %d, want 6 (5 + 1)", got)
	}
	hp = g.State.Players[1].HP
	g.must(t, 0, attack(1))
	if got := hp - g.State.Players[1].HP; got != 5 {
		t.Errorf("second hit dealt %d, want 5", got)
	}
}

// Whole games with Ice King's deck finish and never lose a card, with Choices of every kind answered.
func TestRandomPlayoutsWithIceKingsDeck(t *testing.T) {
	realGame(t)
	games := 120
	if testing.Short() {
		games = 20
	}
	ice := [NumLanes]card.Landscape{card.IcyLands, card.IcyLands, card.IcyLands, card.IcyLands}
	gls := [NumLanes]card.Landscape{card.IcyLands, card.IcyLands, card.BluePlains, card.BluePlains}
	finished := 0
	for seed := uint64(1); seed <= uint64(games); seed++ {
		mine := DeckSpec{Landscapes: ice, Cards: realCat.Deck("Ice King Vs Marceline", "Ice King's Deck"), Hero: "ice-king"}
		other := mine
		if seed%2 == 0 {
			other = DeckSpec{Landscapes: gls, Cards: realCat.Deck("Lemongrab Vs Gunter", "Gunter's Deck"), Hero: "gunter"}
		}
		g, err := NewGame(realCat, Config{Seed: seed, First: PlayerID(seed % 2)}, [2]DeckSpec{mine, other})
		if err != nil {
			t.Fatal(err)
		}
		if _, done := playRandom(t, g, rand.New(rand.NewPCG(seed, 5)), 20000); done {
			finished++
		}
	}
	if finished < games*9/10 {
		t.Errorf("only %d of %d random games finished", finished, games)
	}
}
