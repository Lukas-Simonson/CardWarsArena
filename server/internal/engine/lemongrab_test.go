package engine

import (
	"math/rand/v2"
	"testing"

	"cardwars/internal/card"
)

// lemonGame is Lemongrab's Deck (player 0: Cornfield, Sandy Lands, Blue Plains, Icy Lands) against Finn's Deck
// (player 1, all Cornfield), in player 0's main phase on turn 3 with both hands empty.
func lemonGame(t *testing.T) *Game {
	t.Helper()
	realGame(t)
	ls := [NumLanes]card.Landscape{card.Cornfield, card.SandyLands, card.BluePlains, card.IcyLands}
	g, err := NewGame(realCat, Config{Seed: 1, First: 0}, [2]DeckSpec{
		{Landscapes: ls, Cards: realCat.Deck("Lemongrab Vs Gunter", "Lemongrab's Deck"), Hero: "lemongrab"},
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

func (g *Game) faceDown(p PlayerID, lane int) bool { return g.State.Players[p].Lanes[lane].FaceDown }

func TestLemongrabsDeckIsFullyModelled(t *testing.T) {
	g := lemonGame(t)
	for _, id := range realCat.Deck("Lemongrab Vs Gunter", "Lemongrab's Deck") {
		if !realCat[id].Implemented() {
			t.Errorf("%s is not modelled", id)
		}
	}
	if g.State.Players[0].Hero.CardID != "lemongrab" || !realCat["lemongrab"].Implemented() {
		t.Error("Lemongrab's deck did not come with a modelled Lemongrab")
	}
}

func TestZebracornsAndCornEyeCountLandscapeTypes(t *testing.T) {
	g := lemonGame(t)
	g.put(0, 0, "bouncing-zebracorn") // 1 ATK
	g.put(0, 1, "fancy-zebracorn")    // 4 DEF
	g.put(0, 2, "ol-corn-eye")        // 1 DEF
	g.put(0, 3, "rebounding-zebracorn")
	if g.atkOf(0, 0) != 1+4 || g.defOf(0, 1) != 4+4 || g.defOf(0, 2) != 1+4 || g.atkOf(0, 3) != 4 {
		t.Errorf("four types: %d %d %d %d", g.atkOf(0, 0), g.defOf(0, 1), g.defOf(0, 2), g.atkOf(0, 3))
	}
	g.State.Players[0].Lanes[1].FaceDown = true // a face-down Landscape has no type
	g.State.Players[0].Lanes[3].Landscape = card.Cornfield
	if g.atkOf(0, 0) != 1+2 {
		t.Errorf("two types: %d", g.atkOf(0, 0))
	}
}

func TestFaceDownLandscapesPowerCornfieldCreatures(t *testing.T) {
	g := lemonGame(t)
	g.put(0, 0, "earth-mover") // 2 ATK, +2 for each face-down Landscape in play
	g.put(0, 1, "quake-maker") // 1 ATK
	if g.atkOf(0, 0) != 2 {
		t.Fatalf("no face-down Landscapes: %d", g.atkOf(0, 0))
	}
	g.State.Players[0].Lanes[2].FaceDown = true
	g.State.Players[1].Lanes[3].FaceDown = true // the opponent's counts too
	if g.atkOf(0, 0) != 2+4 || g.atkOf(0, 1) != 1+4 {
		t.Errorf("two face down: %d %d", g.atkOf(0, 0), g.atkOf(0, 1))
	}
	g.put(1, 0, "pony")
	g.cast(t, 0, "amaizing-avalanche")
	if g.atkOf(0, 0) != 2+4+2 || g.atkOf(0, 1) != 1+4+2 {
		t.Errorf("Amaizing Avalanche: %d %d", g.atkOf(0, 0), g.atkOf(0, 1))
	}
	if g.atkOf(1, 0) != baseStat(realCat["pony"].Attack) {
		t.Error("Amaizing Avalanche helped the opponent")
	}
}

func TestParrotroopersMoveToAnyEmptyLandscapeAndCannotBeReplaced(t *testing.T) {
	g := lemonGame(t)
	g.put(0, 1, "bouncing-zebracorn")
	g.put(1, 0, "pony")
	g.put(1, 1, "pony")
	g.put(1, 2, "pony")
	// The only empty Landscape of the opponent's is Lane 4; Lane 1 and 3 on my side are empty too.
	uid := g.give(0, "dragoon-parrotrooper")
	moves := map[[2]int]bool{}
	for _, a := range g.LegalActions(0) {
		if a.Kind == ActPlayCard && a.Card == uid && a.Lane == 0 && len(a.Targets) == 1 {
			moves[[2]int{int(a.Targets[0][0].Player), a.Targets[0][0].Lane}] = true
		}
	}
	if len(moves) != 3 || !moves[[2]int{1, 3}] || !moves[[2]int{0, 2}] || !moves[[2]int{0, 3}] {
		t.Fatalf("it may move to any empty Landscape on either side: %v", moves)
	}
	g.must(t, 0, Action{Kind: ActPlayCard, Card: uid, Lane: 0, Targets: [][]TargetRef{{laneRef(1, 3)}}})
	dragoon := g.State.Players[1].Lanes[3].Creature
	if dragoon == nil || dragoon.CardID != "dragoon-parrotrooper" || g.creatureAt(pos{0, 0}) != nil {
		t.Fatalf("the Parrotrooper did not move to the opponent's side")
	}
	// The Creature beside it has -1 ATK; an opposing player cannot replace it.
	g.State.Active = 1
	g.State.ActionsLeft = 2
	if g.atkOf(1, 2) != baseStat(realCat["pony"].Attack)-1 {
		t.Errorf("the adjacent Creature has %d ATK", g.atkOf(1, 2))
	}
	g.mustFail(t, 1, Action{Kind: ActPlayCard, Card: g.give(1, "pony"), Lane: 3}, ErrNoReplace)
}

func TestCannotReplaceAParrotrooper(t *testing.T) {
	g := lemonGame(t)
	g.put(0, 0, "static-parrotrooper")
	g.mustFail(t, 0, Action{Kind: ActPlayCard, Card: g.give(0, "pony"), Lane: 0}, ErrNoReplace)
	for _, a := range g.LegalActions(0) {
		if a.Kind == ActPlayCard && a.Lane == 0 {
			t.Fatalf("a Creature may be played onto a Parrotrooper: %v", a)
		}
	}
}

func TestStartOfTurnCostsOfParrotroopers(t *testing.T) {
	g := lemonGame(t)
	g.put(0, 0, "static-parrotrooper")
	startOfTurn(t, g, 0)
	if g.State.Players[0].HP != StartingHP-1 {
		t.Errorf("Static Parrotrooper: %d HP", g.State.Players[0].HP)
	}

	g = lemonGame(t)
	g.put(0, 0, "jinxed-parrotrooper")
	g.State.Players[0].Deck = append([]Instance{g.newInstance("pony", 0)}, g.State.Players[0].Deck...)
	discard := len(g.State.Players[0].Discard)
	startOfTurn(t, g, 0)
	if len(g.State.Players[0].Hand) != 0 || len(g.State.Players[0].Discard) != discard+1 {
		t.Errorf("Jinxed Parrotrooper should have made the player discard the card it drew: hand %d", len(g.State.Players[0].Hand))
	}
}

func TestHuskyParrotrooperFlipsItsLandscapeAndFlipsItBack(t *testing.T) {
	g := lemonGame(t)
	g.put(0, 1, "bouncing-zebracorn")
	uid := g.give(0, "husky-parrotrooper")
	g.must(t, 0, Action{Kind: ActPlayCard, Card: uid, Lane: 0, Targets: [][]TargetRef{{laneRef(0, 2)}}})
	if g.creatureAt(pos{0, 2}) == nil || !g.faceDown(0, 2) || g.faceDown(0, 0) {
		t.Fatalf("Husky should be in Lane 3 on a face-down Landscape")
	}
	g.State.Players[0].Lanes[2].Creature.Damage = 99
	g.settle()
	if g.faceDown(0, 2) {
		t.Error("the Landscape should be face up again once Husky leaves play")
	}
}

func TestMantleMasherAndRockNRoller(t *testing.T) {
	for _, id := range []string{"mantle-masher", "rock-n-roller"} {
		g := lemonGame(t)
		g.State.Players[0].Lanes[1].Landscape = card.Cornfield // Rock 'n Roller costs 2
		g.put(1, 0, "pony")
		uid := g.give(0, id)
		g.State.ActionsLeft = 2
		g.must(t, 0, Action{Kind: ActPlayCard, Card: uid, Lane: 0, Targets: [][]TargetRef{{{Kind: "landscape", Player: 1, Lane: 0}}}})
		if !g.faceDown(1, 0) || g.faceDown(0, 0) {
			t.Fatalf("%s: the chosen Landscape should be face down", id)
		}
		g.State.Players[0].Lanes[0].Creature.Damage = 99
		g.settle()
		if g.faceDown(1, 0) {
			t.Errorf("%s: the Landscape should be face up once it leaves play", id)
		}
	}
}

func TestRockOutAndSinkholeFlipUntilYourNextTurn(t *testing.T) {
	g := lemonGame(t)
	g.cast(t, 0, "rock-out", []TargetRef{{Kind: "landscape", Player: 1, Lane: 2}})
	if !g.faceDown(1, 2) {
		t.Fatal("Rock Out! did not flip the Landscape")
	}
	startOfTurn(t, g, 1) // the opponent's turn: still face down
	if !g.faceDown(1, 2) {
		t.Error("the Landscape came back on the opponent's turn")
	}
	startOfTurn(t, g, 0)
	if g.faceDown(1, 2) {
		t.Error("the Landscape should be face up at the start of my next turn")
	}

	g = lemonGame(t)
	g.State.Turn = 3
	sink := g.State.Players[0].Lanes[1]
	_ = sink
	g.State.Players[0].Lanes[1].Building = &Building{Instance: g.newInstance("sinkhole", 0), Status: Ready}
	card1 := g.give(0, "pony")
	g.must(t, 0, Action{Kind: ActFloop, Lane: 1, Source: "building", Targets: [][]TargetRef{{{Kind: "hand_card", Player: 0, Card: card1}}, {{Kind: "landscape", Player: 0, Lane: 1}}}})
	if !g.faceDown(0, 1) || len(g.State.Players[0].Hand) != 0 {
		t.Fatal("Sinkhole should discard a card and flip the Landscape")
	}
	startOfTurn(t, g, 1)
	startOfTurn(t, g, 0)
	if g.faceDown(0, 1) {
		t.Error("Sinkhole's flip should end at the start of my next turn")
	}
}

func TestSiloOfFreedomChangesTheTypeForAWhile(t *testing.T) {
	g := lemonGame(t)
	g.State.Players[0].Lanes[0].Building = &Building{Instance: g.newInstance("silo-of-freedom", 0), Status: Ready}
	c := g.give(0, "pony")
	g.must(t, 0, Action{Kind: ActActivate, Lane: 0, Source: "building", Targets: [][]TargetRef{{{Kind: "hand_card", Player: 0, Card: c}}}})
	if g.State.Choice == nil || g.State.Choice.Kind != ChoiceLandscape {
		t.Fatalf("expected a choice of Landscape type: %+v", g.State.Choice)
	}
	if got := len(g.LegalActions(0)); got != len(card.Landscapes) {
		t.Errorf("%d types to choose from", got)
	}
	g.mustFail(t, 0, Action{Kind: ActChoose, Card: 0}, ErrBadTarget)
	g.must(t, 0, Action{Kind: ActChoose, Card: 1 + landscapeIndex(card.NiceLands)})
	if g.State.Players[0].Lanes[0].Landscape != card.NiceLands || g.faceUp(0, card.Cornfield) != 0 {
		t.Fatal("the Landscape did not become Nice Lands")
	}
	startOfTurn(t, g, 1)
	startOfTurn(t, g, 0)
	if g.State.Players[0].Lanes[0].Landscape != card.Cornfield {
		t.Errorf("the Landscape should be Cornfield again, it is %s", g.State.Players[0].Lanes[0].Landscape)
	}
}

func TestCarmelCamelCollectsCreaturesThatLeave(t *testing.T) {
	g := lemonGame(t)
	camel := g.put(0, 1, "carmel-camel") // 0/4
	victim := g.put(0, 0, "bouncing-zebracorn")
	g.put(1, 0, "pony")
	g.State.Players[0].Lanes[0].Creature.Damage = 99
	g.settle()
	ch := g.State.Choice
	if ch == nil || ch.Kind != ChoiceCamel || ch.Player != 0 || ch.Revealed[0].UID != victim.UID {
		t.Fatalf("the Camel should be offered the Creature: %+v", ch)
	}
	if got := len(g.LegalActions(0)); got != 2 {
		t.Errorf("legal actions: %d, want put it under or not", got)
	}
	g.mustFail(t, 0, Action{Kind: ActChoose, Card: 12345}, ErrNoSuchCard)
	g.must(t, 0, Action{Kind: ActChoose, Card: camel.UID})
	if len(camel.Under) != 1 || g.atkOf(0, 1) != 1 || g.defOf(0, 1) != 5 {
		t.Fatalf("the Camel should hold the Creature and be 1/5: under %d, %d/%d", len(camel.Under), g.atkOf(0, 1), g.defOf(0, 1))
	}
	for _, in := range g.State.Players[0].Discard {
		if in.UID == victim.UID {
			t.Error("the Creature is also in the discard pile")
		}
	}
	// A Creature the Camel declines goes where it was going.
	g.put(0, 2, "pony")
	g.State.Players[0].Lanes[2].Creature.Damage = 99
	g.settle()
	g.must(t, 0, Action{Kind: ActChoose})
	if len(camel.Under) != 1 || g.State.Players[0].Discard[len(g.State.Players[0].Discard)-1].CardID != "pony" {
		t.Error("a declined Creature should be discarded")
	}
	// The Camel leaving discards what is under it.
	g.State.Players[0].Lanes[1].Creature.Damage = 99
	g.settle()
	found := false
	for _, in := range g.State.Players[0].Discard {
		found = found || in.UID == victim.UID
	}
	if !found || g.State.Choice != nil {
		t.Error("the cards under the Camel should be discarded when it leaves play")
	}
	// A returned Creature that is declined goes to the hand.
	g = lemonGame(t)
	g.put(0, 1, "carmel-camel")
	g.put(0, 0, "bouncing-zebracorn")
	g.cast(t, 0, "drop-zone", []TargetRef{creatureRef(0, 0)})
	if g.State.Choice == nil || g.State.Choice.Count != 1 {
		t.Fatalf("expected the Camel's choice about a returned Creature: %+v", g.State.Choice)
	}
	g.must(t, 0, Action{Kind: ActChoose})
	if len(g.State.Players[0].Hand) != 1 || g.State.Players[0].Hand[0].CardID != "bouncing-zebracorn" {
		t.Error("the declined Creature should have gone to the hand")
	}
}

func TestGrandMummyAndSandmagusAtTheStartOfTurn(t *testing.T) {
	g := lemonGame(t)
	g.put(0, 0, "grand-mummy")
	g.put(0, 1, "pony")
	startOfTurn(t, g, 0)
	hand := len(g.State.Players[0].Hand)
	g.must(t, 0, Action{Kind: ActActivate, Lane: 0, Index: 0})
	if g.creatureAt(pos{0, 0}) != nil || len(g.State.Players[0].Hand) != hand+1+2 {
		t.Errorf("Grand Mummy should return and draw 2: hand %d -> %d", hand, len(g.State.Players[0].Hand))
	}

	g = lemonGame(t)
	g.put(0, 0, "sandmagus")
	g.put(0, 1, "pony")
	startOfTurn(t, g, 0)
	before := g.atkOf(0, 1)
	g.must(t, 0, Action{Kind: ActActivate, Lane: 0, Index: 0})
	if g.atkOf(0, 1) != before+1 || g.creatureAt(pos{0, 0}) != nil {
		t.Errorf("Sandmagus should return and give the others +1 ATK: %d -> %d", before, g.atkOf(0, 1))
	}
}

func TestLemongrabSpells(t *testing.T) {
	// Drop Zone returns a Creature I own, even from the opponent's side.
	g := lemonGame(t)
	cr := g.put(0, 0, "pony")
	g.State.Players[0].Lanes[0].Creature = nil
	g.State.Players[1].Lanes[2].Creature = cr
	g.cast(t, 0, "drop-zone", []TargetRef{creatureRef(1, 2)})
	if g.State.Players[1].Lanes[2].Creature != nil || len(g.State.Players[0].Hand) != 1 {
		t.Error("Drop Zone should return my Creature from the opponent's side")
	}
	g.put(1, 3, "pony")
	// ...but not one I do not own.
	for _, a := range g.LegalActions(0) {
		if a.Kind == ActPlayCard && len(a.Targets) == 1 && a.Targets[0][0].Player == 1 && a.Targets[0][0].Lane == 3 && g.Card(g.State.Players[0].Hand[0]).ID == "drop-zone" {
			t.Error("Drop Zone offered a Creature I do not own")
		}
	}

	// Quick Pick Me Up gains Actions equal to the cost.
	g = lemonGame(t)
	g.put(0, 0, "bouncing-zebracorn") // cost 2
	g.State.ActionsLeft = 2
	g.cast(t, 0, "quick-pick-me-up", []TargetRef{creatureRef(0, 0)})
	if g.State.ActionsLeft != 2-1+2 {
		t.Errorf("Actions left %d, want 3", g.State.ActionsLeft)
	}

	// Ham Fist: -5 ATK and a card.
	g = lemonGame(t)
	g.put(0, 0, "bouncing-zebracorn")
	hand := len(g.State.Players[0].Hand)
	g.cast(t, 0, "ham-fist", []TargetRef{creatureRef(0, 0)})
	if g.atkOf(0, 0) != 0 || len(g.State.Players[0].Hand) != hand+1 {
		t.Errorf("Ham Fist: ATK %d, hand %d", g.atkOf(0, 0), len(g.State.Players[0].Hand))
	}

	// Free Fall: +1 ATK for each Creature that entered play this turn.
	g = lemonGame(t)
	g.State.ActionsLeft = 5
	g.put(0, 3, "earth-mover")
	g.playIn(t, 0, "pony", 0)
	g.playIn(t, 0, "pony", 1)
	g.cast(t, 0, "free-fall")
	if g.atkOf(0, 0) != baseStat(realCat["pony"].Attack)+2 {
		t.Errorf("Free Fall: %d ATK", g.atkOf(0, 0))
	}
}

func TestLunchpadParrotmilitaryOutpostAndSandasaurus(t *testing.T) {
	g := lemonGame(t)
	g.State.Players[0].Lanes[1].Building = &Building{Instance: g.newInstance("lunchpad", 0), Status: Ready}
	g.State.ActionsLeft = 5
	g.State.Turn = 3
	g.playIn(t, 0, "pony", 1)
	g.State.Players[0].Deck = append([]Instance{g.newInstance("pony", 0), g.newInstance("pony", 0)}, g.State.Players[0].Deck...)
	g.State.Players[0].Lanes[1].Building.Status = Ready
	g.must(t, 0, Action{Kind: ActFloop, Lane: 1, Source: "building"})
	if len(g.State.Players[0].Hand) != 1 {
		t.Errorf("Lunchpad: drew %d, want 1", len(g.State.Players[0].Hand))
	}

	g = lemonGame(t)
	g.State.Players[0].Lanes[0].Building = &Building{Instance: g.newInstance("parrotmilitary-outpost", 0), Status: Ready}
	g.put(0, 0, "drooling-dude")
	g.put(1, 0, "drooling-dude")
	g.put(0, 1, "drooling-dude")
	base := baseStat(realCat["drooling-dude"].Attack)
	if g.atkOf(0, 0) != base-1 || g.atkOf(1, 0) != base-1 || g.atkOf(0, 1) != base {
		t.Errorf("Outpost: %d %d %d", g.atkOf(0, 0), g.atkOf(1, 0), g.atkOf(0, 1))
	}

	g = lemonGame(t)
	g.put(0, 0, "sandasaurus-rex")
	if g.atkOf(0, 0) != 2*3 {
		t.Errorf("Sandasaurus Rex with 3 empty Landscapes: %d", g.atkOf(0, 0))
	}
}

func TestSkaPionCostsLessWhileAnOpponentHasMyCreature(t *testing.T) {
	g := lemonGame(t)
	c := realCat["ska-pion"]
	a, l := g.costFor(0, c, 0)
	if a != 1 || l != 1 {
		t.Fatalf("normal cost %d/%d", a, l)
	}
	cr := g.put(0, 0, "pony")
	g.State.Players[0].Lanes[0].Creature = nil
	g.State.Players[1].Lanes[0].Creature = cr
	if a, l = g.costFor(0, c, 1); a != 0 || l != 0 {
		t.Errorf("cost with my Creature on their side: %d/%d", a, l)
	}
}

func TestTNTimmyAndLemongrab(t *testing.T) {
	g := lemonGame(t)
	tnt := g.put(0, 0, "tntimmy")
	g.put(1, 0, "drooling-dude")
	g.put(1, 1, "drooling-dude")
	g.put(0, 1, "drooling-dude")
	_ = tnt
	g.State.Players[0].Lanes[0].Creature.Damage = 99
	g.settle()
	if g.State.Players[1].Lanes[0].Creature.Damage != 1 || g.State.Players[1].Lanes[1].Creature.Damage != 1 || g.State.Players[0].Lanes[1].Creature.Damage != 0 {
		t.Error("TNTimmy should deal 1 damage to each opposing Creature only")
	}

	// Lemongrab heals 1 at the start of the Fight Phase while I have an empty Landscape, never above 25.
	g = lemonGame(t)
	g.State.Players[0].HP = 20
	g.put(0, 0, "drooling-dude")
	g.must(t, 0, endMain)
	if g.State.Players[0].HP != 21 {
		t.Errorf("Lemongrab: %d HP", g.State.Players[0].HP)
	}
	g = lemonGame(t)
	g.State.Players[0].HP = 20
	for lane := 0; lane < NumLanes; lane++ {
		g.put(0, lane, "drooling-dude")
	}
	g.must(t, 0, endMain)
	if g.State.Players[0].HP != 20 {
		t.Errorf("Lemongrab healed with no empty Landscape: %d HP", g.State.Players[0].HP)
	}
	g = lemonGame(t)
	g.put(0, 0, "drooling-dude")
	g.must(t, 0, endMain)
	if g.State.Players[0].HP != StartingHP {
		t.Errorf("Lemongrab healed above %d: %d", StartingHP, g.State.Players[0].HP)
	}
}

// Whole random games with Lemongrab's deck keep every rule of the game intact: cards are never lost, a
// Camel's Creatures are accounted for, and the games finish.
func TestRandomPlayoutsWithLemongrabsDeck(t *testing.T) {
	realGame(t)
	games := 120
	if testing.Short() {
		games = 20
	}
	ls := [NumLanes]card.Landscape{card.Cornfield, card.Cornfield, card.SandyLands, card.SandyLands}
	gls := [NumLanes]card.Landscape{card.IcyLands, card.IcyLands, card.BluePlains, card.BluePlains}
	finished := 0
	seen := map[EventKind]int{}
	for seed := uint64(1); seed <= uint64(games); seed++ {
		lemon := DeckSpec{Landscapes: ls, Cards: realCat.Deck("Lemongrab Vs Gunter", "Lemongrab's Deck"), Hero: "lemongrab"}
		other := lemon
		if seed%2 == 0 {
			other = DeckSpec{Landscapes: gls, Cards: realCat.Deck("Lemongrab Vs Gunter", "Gunter's Deck"), Hero: "gunter"}
		}
		g, err := NewGame(realCat, Config{Seed: seed, First: PlayerID(seed % 2)}, [2]DeckSpec{lemon, other})
		if err != nil {
			t.Fatal(err)
		}
		if _, done := playRandomSeen(t, g, rand.New(rand.NewPCG(seed, 5)), 20000, seen); done {
			finished++
		}
	}
	if finished < games*9/10 {
		t.Errorf("only %d of %d random games finished", finished, games)
	}
	t.Logf("events seen: under=%d retype=%d flip=%d", seen[EvUnder], seen[EvRetype], seen[EvFlip])
}
