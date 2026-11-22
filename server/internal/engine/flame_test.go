package engine

import (
	"testing"

	"cardwars/internal/card"
)

// flameGame is Flame Princess's Deck (player 0, all Lava Flats) against Finn's Deck (player 1), in player 0's main
// phase on turn 3 with both hands empty.
func flameGame(t *testing.T) *Game {
	t.Helper()
	realGame(t)
	ls := [NumLanes]card.Landscape{card.LavaFlats, card.LavaFlats, card.LavaFlats, card.LavaFlats}
	g, err := NewGame(realCat, Config{Seed: 1, First: 0}, [2]DeckSpec{
		{Landscapes: ls, Cards: realCat.Deck("Flame Princess Vs Fern", "FP's Deck"), Hero: "flame-princess"},
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

func TestFlamePrincesssDeckIsFullyModelled(t *testing.T) {
	g := flameGame(t)
	for _, id := range realCat.Deck("Flame Princess Vs Fern", "FP's Deck") {
		if !realCat[id].Implemented() {
			t.Errorf("%s is not modelled", id)
		}
	}
	if g.State.Players[0].Hero.CardID != "flame-princess" || !realCat["flame-princess"].Implemented() {
		t.Error("Flame Princess's deck did not come with a modelled Flame Princess")
	}
}

func TestABurnTokenHurtsAtOnceAndAgainAtTheStartOfTheTurn(t *testing.T) {
	g := flameGame(t)
	foe := g.put(1, 1, "wall-of-ears")
	g.cast(t, 0, "hot-shot", []TargetRef{creatureRef(1, 1)}) // burn, then 1 more damage
	if foe.Burn != 1 || foe.Damage != 2 {
		t.Fatalf("burn %d damage %d", foe.Burn, foe.Damage)
	}
	g.burnDamage(1)
	if foe.Damage != 3 {
		t.Errorf("damage %d after the start of its controller's turn", foe.Damage)
	}
	g.burnDamage(0) // not its controller's turn
	if foe.Damage != 3 {
		t.Errorf("it burned on the wrong player's turn: %d", foe.Damage)
	}
}

func TestBurnedPlayersTakeDamageEachTurn(t *testing.T) {
	g := flameGame(t)
	g.burnPlayer(1, 2)
	if g.State.Players[1].HP != StartingHP-2 || g.State.Players[1].Burn != 2 {
		t.Fatalf("HP %d burn %d", g.State.Players[1].HP, g.State.Players[1].Burn)
	}
	g.burnDamage(1)
	if g.State.Players[1].HP != StartingHP-4 {
		t.Errorf("HP %d", g.State.Players[1].HP)
	}
}

func TestBurningCreatureTokensGoWhenItLeavesPlay(t *testing.T) {
	g := flameGame(t)
	foe := g.put(1, 0, "wall-of-ears")
	g.burnCreature(pos{1, 0}, 0, 2)
	if foe.Burn != 2 {
		t.Fatal("not burning")
	}
	g.destroyAt(pos{1, 0})
	g.put(1, 0, "wall-of-ears")
	if g.State.Players[1].Lanes[0].Creature.Burn != 0 {
		t.Error("the new Creature inherited burn tokens")
	}
}

func TestFireDogDealsDamageForEveryBurnToken(t *testing.T) {
	g := flameGame(t)
	a, b := g.put(1, 0, "wall-of-ears"), g.put(0, 1, "wall-of-ears")
	g.burnCreature(pos{1, 0}, 0, 2)
	g.burnCreature(pos{0, 1}, 0, 1)
	a.Damage, b.Damage = 0, 0
	g.playAt(t, "fire-dog", 3, []TargetRef{creatureRef(1, 0)})
	if a.Damage != 3 {
		t.Errorf("damage %d, want 3 for the 3 burn tokens", a.Damage)
	}
}

func TestFlameingoBlazelingBurnsACreatureItDamagesOnce(t *testing.T) {
	g := flameGame(t)
	g.put(0, 0, "flameingo-blazeling")
	foe := g.put(1, 0, "wall-of-ears")
	g.attackFrom(t, 0)
	if foe.Burn != 1 {
		t.Errorf("burn %d", foe.Burn)
	}
	// Already burning: no more.
	g = flameGame(t)
	g.put(0, 0, "flameingo-blazeling")
	foe = g.put(1, 0, "wall-of-ears")
	g.burnCreature(pos{1, 0}, 0, 1)
	g.attackFrom(t, 0)
	if foe.Burn != 1 {
		t.Errorf("a burning Creature was burned again: %d", foe.Burn)
	}
}

func TestFlameingoBurnwardenTradesABurnTokenForDamage(t *testing.T) {
	g := flameGame(t)
	g.put(0, 0, "flameingo-burnwarden")
	foe := g.put(1, 1, "wall-of-ears")
	for _, a := range g.LegalActions(0) {
		if a.Kind == ActActivate && a.Lane == 0 && a.Source != "hero" {
			t.Fatalf("offered with nothing burning: %v", a)
		}
	}
	g.burnCreature(pos{1, 1}, 0, 2)
	hp := g.State.Players[1].HP
	for i := 0; i < 2; i++ { // any number of times
		g.must(t, 0, Action{Kind: ActActivate, Lane: 0, Targets: [][]TargetRef{{creatureRef(1, 1)}}})
	}
	if foe.Burn != 0 || g.State.Players[1].HP != hp-2 {
		t.Errorf("burn %d HP %d", foe.Burn, g.State.Players[1].HP)
	}
}

func TestFlameingoCaptainBurnsTheOpponentItHits(t *testing.T) {
	g := flameGame(t)
	g.put(0, 0, "flameingo-captain")
	g.put(0, 3, "wall-of-ears") // still to Fight, so the turn goes on
	g.attackFrom(t, 0)
	if g.State.Players[1].Burn != 1 || g.State.Players[1].HP != StartingHP-3-1 { // the hit, and the token's 1
		t.Errorf("burn %d HP %d", g.State.Players[1].Burn, g.State.Players[1].HP)
	}
}

func TestFireplumeChiefCostsLessForBurnTokensAndBurnsWhatItDamages(t *testing.T) {
	g := flameGame(t)
	g.State.ActionsLeft = 9
	if got := g.playAt(t, "flameingo-fireplume-chief", 3); got != 4 {
		t.Fatalf("cost %d", got)
	}
	g = flameGame(t)
	g.State.ActionsLeft = 9
	g.put(1, 0, "wall-of-ears")
	g.burnCreature(pos{1, 0}, 0, 3)
	if got := g.playAt(t, "flameingo-fireplume-chief", 3); got != 1 {
		t.Errorf("with 3 burn tokens it cost %d, want 1", got)
	}
	foe := g.put(1, 3, "wall-of-ears")
	g.attackFrom(t, 3)
	if foe.Burn != 1 {
		t.Errorf("burn %d", foe.Burn)
	}
}

func TestFlameingoFlameweaverBurnsBothCreaturesInALane(t *testing.T) {
	g := flameGame(t)
	g.put(0, 0, "flameingo-flameweaver")
	mine, foe, other := g.put(0, 2, "wall-of-ears"), g.put(1, 2, "wall-of-ears"), g.put(1, 3, "wall-of-ears")
	g.floopAt(t, 0, []TargetRef{laneRef(0, 2)})
	if mine.Burn != 1 || foe.Burn != 1 || other.Burn != 0 {
		t.Errorf("burn %d %d %d", mine.Burn, foe.Burn, other.Burn)
	}
}

func TestHeatEaterDrawsOnTheFirstBurnEachTurn(t *testing.T) {
	g := flameGame(t)
	g.put(0, 0, "flameingo-heat-eater")
	g.put(1, 0, "wall-of-ears")
	g.burnCreature(pos{1, 0}, 0, 1)
	g.burnCreature(pos{1, 0}, 0, 1)
	if g.handSize(0) != 1 {
		t.Errorf("hand %d", g.handSize(0))
	}
	// Burning by the opponent's cards does not count.
	g.burnCreature(pos{1, 0}, 1, 1)
	if g.handSize(0) != 1 {
		t.Errorf("hand %d", g.handSize(0))
	}
}

func TestHeatLeapersAndHotHuts(t *testing.T) {
	g := flameGame(t)
	leapers := g.put(0, 0, "heat-leapers")
	g.building(0, 1, "hot-huts")
	foe := g.put(1, 0, "wall-of-ears")
	mine := g.put(0, 2, "wall-of-ears")
	g.burnCreature(pos{1, 0}, 0, 1) // 1 burn + 1 Hot Huts
	if leapers.Tokens != 1 || foe.Damage != 2 {
		t.Errorf("tokens %d damage %d", leapers.Tokens, foe.Damage)
	}
	g.burnCreature(pos{0, 2}, 0, 1) // your own: no extra damage
	if mine.Damage != 1 || leapers.Tokens != 2 {
		t.Errorf("own Creature damage %d, tokens %d", mine.Damage, leapers.Tokens)
	}
}

func TestFlameingoHotshotAsksOnlyWhenItAttacksABurningCreatureAndThenFights(t *testing.T) {
	g := flameGame(t)
	g.put(0, 0, "flameingo-hotshot")
	g.put(1, 0, "wall-of-ears")
	other := g.put(1, 2, "wall-of-ears")
	g.attackFrom(t, 0)
	if g.State.Choice != nil {
		t.Fatal("asked, but the Creature it attacks is not burning")
	}
	g = flameGame(t)
	hot := g.put(0, 0, "flameingo-hotshot")
	g.put(0, 3, "wall-of-ears") // still to Fight, so the turn goes on
	foe := g.put(1, 0, "wall-of-ears")
	other = g.put(1, 2, "wall-of-ears")
	g.burnCreature(pos{1, 0}, 0, 1)
	g.attackFrom(t, 0)
	ch := g.State.Choice
	if ch == nil || ch.Kind != ChoiceTarget {
		t.Fatalf("choice %+v", ch)
	}
	if hot.Damage != 0 {
		t.Fatal("the Fight happened before the choice was made")
	}
	g.must(t, 0, Action{Kind: ActChoose, Targets: [][]TargetRef{{creatureRef(1, 2)}}})
	if other.Damage != 3 || other.Burn != 1 { // 2 damage, 1 burn damage
		t.Errorf("other: damage %d burn %d", other.Damage, other.Burn)
	}
	if hot.Damage == 0 || foe.Damage == 0 {
		t.Errorf("the Fight did not go on: %d %d", hot.Damage, foe.Damage)
	}
}

func TestFlameingoIgnisbeakHitsThePlayerInsteadOfABurningCreature(t *testing.T) {
	g := flameGame(t)
	g.put(0, 0, "flameingo-ignisbeak") // 3 ATK
	g.put(0, 3, "wall-of-ears")        // still to Fight, so the turn goes on
	foe := g.put(1, 0, "wall-of-ears")
	g.burnCreature(pos{1, 0}, 0, 1)
	hp, dmg := g.State.Players[1].HP, foe.Damage
	g.attackFrom(t, 0)
	if g.State.Players[1].HP != hp-3 || foe.Damage != dmg {
		t.Errorf("HP %d, Creature damage %d", g.State.Players[1].HP, foe.Damage)
	}
}

func TestFuriousPhoenixHampireAndKangarooCat(t *testing.T) {
	g := flameGame(t)
	g.put(0, 0, "furious-phoenix")
	g.put(1, 0, "wall-of-ears")
	g.burnCreature(pos{1, 0}, 0, 2)
	if g.atk(pos{0, 0}) != 1+2 {
		t.Errorf("atk %d", g.atk(pos{0, 0}))
	}
	g = flameGame(t)
	g.State.Players[0].HP = 10
	g.put(0, 0, "hampire-pig-of-the-night") // 3 ATK
	g.put(1, 0, "the-pig")
	g.attackFrom(t, 0)
	if g.State.Players[0].HP != 13 {
		t.Errorf("HP %d", g.State.Players[0].HP)
	}
	g = flameGame(t)
	cat := g.put(0, 0, "kangaroo-cat")
	g.put(0, 3, "wall-of-ears") // still to Fight, so the turn goes on
	g.attackFrom(t, 0)
	if cat.Tokens != 1 {
		t.Fatalf("tokens %d", cat.Tokens)
	}
	cat.Status = Ready
	g.attackFrom(t, 0)
	if cat.Tokens != 1 {
		t.Errorf("it got a token the second time: %d", cat.Tokens)
	}
}

func TestHotWheelBurnsOnceForTheCostOfTheCardItDraws(t *testing.T) {
	g := flameGame(t)
	g.put(0, 0, "hot-wheel")
	foe := g.put(1, 0, "wall-of-ears")
	g.State.Players[0].Deck = append([]Instance{g.newInstance("fire-dog", 0), g.newInstance("flameingo-captain", 0)}, g.State.Players[0].Deck...)
	g.must(t, 0, Action{Kind: ActActivate, Lane: 0, Targets: [][]TargetRef{{creatureRef(1, 0)}}})
	if foe.Burn != 0 { // Fire Dog costs 0
		t.Errorf("burn %d", foe.Burn)
	}
	g.State.ActionsLeft = 2
	g.must(t, 0, Action{Kind: ActActivate, Lane: 0, Targets: [][]TargetRef{{creatureRef(1, 0)}}})
	if foe.Burn != 2 { // Flameingo Captain costs 2
		t.Errorf("burn %d", foe.Burn)
	}
}

func TestMoltenPotionVolcanicVitalityAndFlamePrincess(t *testing.T) {
	g := flameGame(t)
	a, b, mine := g.put(1, 0, "wall-of-ears"), g.put(1, 1, "wall-of-ears"), g.put(0, 0, "wall-of-ears")
	g.burnCreature(pos{1, 1}, 0, 2)
	b.Damage = 0
	g.cast(t, 0, "molten-potion", []TargetRef{creatureRef(1, 0)})
	if a.Burn != 1 || a.Damage != 3 || b.Damage != 2 || mine.Damage != 0 { // a: 1 burn + 2; b: 2
		t.Errorf("a %d/%d b %d mine %d", a.Burn, a.Damage, b.Damage, mine.Damage)
	}
	g.State.ActionsLeft = 2
	g.cast(t, 0, "volcanic-vitality") // a has 1 token, b has 2
	if a.Burn != 2 || b.Burn != 4 || mine.Burn != 0 {
		t.Errorf("burn %d %d %d", a.Burn, b.Burn, mine.Burn)
	}
	// Flame Princess burns every opposing Creature once a game.
	g = flameGame(t)
	a, b = g.put(1, 0, "wall-of-ears"), g.put(1, 3, "wall-of-ears")
	g.must(t, 0, Action{Kind: ActActivate, Source: "hero"})
	if a.Burn != 1 || b.Burn != 1 {
		t.Errorf("burn %d %d", a.Burn, b.Burn)
	}
	g.mustFail(t, 0, Action{Kind: ActActivate, Source: "hero"}, ErrLimit)
	g.endTurn(0)
	g.endTurn(1)
	g.State.Phase = PhaseMain
	g.mustFail(t, 0, Action{Kind: ActActivate, Source: "hero"}, ErrLimit)
}

func TestPyroventFortressAndScorchstoneGateway(t *testing.T) {
	g := flameGame(t)
	g.building(0, 0, "pyrovent-fortress")
	foe := g.put(1, 1, "wall-of-ears")
	foe.Burn = 1
	g.must(t, 0, Action{Kind: ActFloop, Source: "building", Lane: 0, Targets: [][]TargetRef{{creatureRef(1, 1)}}})
	if foe.Burn != 0 || foe.Damage != 3 {
		t.Errorf("burn %d damage %d", foe.Burn, foe.Damage)
	}
	g = flameGame(t)
	g.building(0, 0, "scorchstone-gateway")
	foe = g.put(1, 0, "wall-of-ears")
	g.must(t, 0, Action{Kind: ActFloop, Source: "building", Lane: 0, Targets: [][]TargetRef{{creatureRef(1, 0)}}})
	if foe.Burn != 1 || g.State.ActionsLeft != 1 {
		t.Errorf("burn %d actions %d", foe.Burn, g.State.ActionsLeft)
	}
}

func TestScorchcrawlerAndCozyKilnWhenABurningCreatureLeaves(t *testing.T) {
	g := flameGame(t)
	g.put(0, 0, "scorchcrawler")
	g.building(0, 1, "cozy-kiln")
	target := g.put(1, 2, "wall-of-ears")
	dying := g.put(1, 0, "wall-of-ears")
	dying.Burn = 3
	g.destroyAt(pos{1, 0})
	if g.State.Players[1].Burn != 1 {
		t.Errorf("Cozy Kiln: player burn %d", g.State.Players[1].Burn)
	}
	ch := g.State.Choice
	if ch == nil || ch.Kind != ChoiceTarget {
		t.Fatalf("Scorchcrawler did not ask: %+v", ch)
	}
	g.must(t, 0, Action{Kind: ActChoose, Targets: [][]TargetRef{{creatureRef(1, 2)}}})
	if target.Burn != 3 {
		t.Errorf("burn %d, want one per token on the Creature that left", target.Burn)
	}
	// A Creature that was not burning sets nothing off.
	g.put(1, 3, "wall-of-ears")
	g.destroyAt(pos{1, 3})
	if g.State.Choice != nil || g.State.Players[1].Burn != 1 {
		t.Error("a Creature that was not burning set it off")
	}
}

func TestBroilsCoilsAndTitanbaneAmulet(t *testing.T) {
	g := flameGame(t)
	weak := g.put(1, 0, "the-pig")
	weak.Damage = 2
	g.cast(t, 0, "broils-coils", []TargetRef{creatureRef(1, 0)})
	if g.State.ActionsLeft != 2 {
		t.Errorf("actions %d, want the 1 spent and the 1 gained", g.State.ActionsLeft)
	}
	g = flameGame(t)
	g.put(1, 0, "wall-of-ears")
	g.cast(t, 0, "broils-coils", []TargetRef{creatureRef(1, 0)})
	if g.State.ActionsLeft != 1 {
		t.Errorf("actions %d: the Creature survived", g.State.ActionsLeft)
	}
	g = flameGame(t)
	g.State.ActionsLeft = 2
	g.put(1, 0, "rhinoceros") // 3 ATK
	g.cast(t, 0, "titanbane-amulet", []TargetRef{creatureRef(1, 0)})
	if g.State.Players[0].HP != StartingHP-3 || g.State.Players[1].Lanes[0].Creature != nil {
		t.Errorf("HP %d", g.State.Players[0].HP)
	}
}
