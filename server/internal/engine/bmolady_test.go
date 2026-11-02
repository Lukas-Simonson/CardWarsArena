package engine

import (
	"math/rand/v2"
	"testing"

	"cardwars/internal/card"
)

// ladyGame is BMO's Deck (player 0, Useless Swamp) against Lady's Deck
// (player 1, SandyLands) under the full rules, in the main phase of turn 2
// with player 1 active. Cards are put on the board directly.
func ladyGame(t *testing.T) *Game {
	t.Helper()
	realGame(t) // loads the catalog
	swamp := [NumLanes]card.Landscape{card.UselessSwamp, card.UselessSwamp, card.UselessSwamp, card.UselessSwamp}
	bmo := DeckSpec{swamp, realCat.Deck("BMO Vs Lady Rainicorn", "BMO's Deck")}
	lady := DeckSpec{sandy, realCat.Deck("BMO Vs Lady Rainicorn", "Lady's Deck")}
	g, err := NewGame(realCat, Config{Seed: 1, First: 0}, [2]DeckSpec{bmo, lady})
	if err != nil {
		t.Fatal(err)
	}
	g.start(t)
	g.must(t, 0, endMain)
	// Each side has both Landscape types, so either deck's cards can be played.
	g.State.Players[0].Lanes[2].Landscape, g.State.Players[0].Lanes[3].Landscape = card.SandyLands, card.SandyLands
	g.State.Players[1].Lanes[2].Landscape, g.State.Players[1].Lanes[3].Landscape = card.UselessSwamp, card.UselessSwamp
	return g
}

// fillDiscard gives p n cards in their discard pile.
func (g *Game) fillDiscard(p PlayerID, n int, id string) {
	for i := 0; i < n; i++ {
		g.State.Players[p].Discard = append(g.State.Players[p].Discard, g.newInstance(id, p))
	}
}

func (g *Game) building(p PlayerID, lane int, id string) *Building {
	g.State.NextUID++
	b := &Building{Instance: Instance{UID: g.State.NextUID, CardID: id, Owner: p}, Status: Ready}
	g.State.Players[p].Lanes[lane].Building = b
	return b
}

func crRef(p PlayerID, lane int) TargetRef { return TargetRef{Kind: "creature", Player: p, Lane: lane} }

func TestAllStarterDecksAreFullyModelled(t *testing.T) {
	realGame(t)
	for _, d := range []struct{ set, name string }{
		{"Finn Vs Jake", "Finn's Deck"}, {"Finn Vs Jake", "Jake's Deck"},
		{"BMO Vs Lady Rainicorn", "BMO's Deck"}, {"BMO Vs Lady Rainicorn", "Lady's Deck"},
		{"Princess Bubblegum Vs Lumpy Space Princess", "PB's Deck"}, {"Princess Bubblegum Vs Lumpy Space Princess", "LSP's Deck"},
	} {
		for _, id := range realCat.Deck(d.set, d.name) {
			if !realCat[id].Implemented() {
				t.Errorf("%s: %s is not modelled", d.name, id)
			}
		}
	}
}

func TestStatsFromTheDiscardPile(t *testing.T) {
	g := ladyGame(t)
	g.put(0, 0, "dark-angel") // 0/5, +1 ATK per 5 cards in the discard pile
	g.put(0, 1, "mouthball")  // 2/4, +2 DEF per 5
	if g.atkOf(0, 0) != 0 || g.defOf(0, 1) != 4 {
		t.Fatalf("no discards: %d %d", g.atkOf(0, 0), g.defOf(0, 1))
	}
	g.fillDiscard(0, 9, "fummy")
	if g.atkOf(0, 0) != 1 || g.defOf(0, 1) != 6 {
		t.Errorf("9 discards: %d %d, want 1 and 6", g.atkOf(0, 0), g.defOf(0, 1))
	}
	g.fillDiscard(0, 1, "fummy")
	if g.atkOf(0, 0) != 2 || g.defOf(0, 1) != 8 {
		t.Errorf("10 discards: %d %d, want 2 and 8", g.atkOf(0, 0), g.defOf(0, 1))
	}
}

func TestImmortalMaizeWalkerTriplesOnCornfield(t *testing.T) {
	g := ladyGame(t)
	g.put(0, 0, "immortal-maize-walker") // 2 ATK
	if g.atkOf(0, 0) != 2 {
		t.Fatalf("on a Useless Swamp: %d", g.atkOf(0, 0))
	}
	g.State.Players[0].Lanes[0].Landscape = card.Cornfield
	if g.atkOf(0, 0) != 6 {
		t.Errorf("on a Cornfield: %d, want 6", g.atkOf(0, 0))
	}
	g.State.Players[0].Lanes[0].FaceDown = true
	if g.atkOf(0, 0) != 2 {
		t.Errorf("a face-down Landscape has no type: %d", g.atkOf(0, 0))
	}
}

func TestSandKnightsAndGreenCactiball(t *testing.T) {
	g := ladyGame(t)
	g.put(1, 0, "sand-knights") // 1/8, +2 ATK with a Blue Plains Landscape
	if g.atkOf(1, 0) != 1 {
		t.Fatalf("no Blue Plains: %d", g.atkOf(1, 0))
	}
	g.State.Players[1].Lanes[3].Landscape = card.BluePlains
	if g.atkOf(1, 0) != 3 {
		t.Errorf("with Blue Plains: %d, want 3", g.atkOf(1, 0))
	}
	g.put(1, 1, "green-cactiball")
	if g.atkOf(1, 1) != 2 {
		t.Errorf("one Green Cactiball: %d, want 2", g.atkOf(1, 1))
	}
	g.put(1, 2, "green-cactiball")
	if g.atkOf(1, 1) != 4 || g.atkOf(1, 2) != 4 {
		t.Errorf("two Green Cactiballs: %d %d, want 4 each", g.atkOf(1, 1), g.atkOf(1, 2))
	}
}

func TestSandsnakeAndSandhornDevil(t *testing.T) {
	g := ladyGame(t)
	foe := g.put(0, 1, "mace-stump") // 3/3 opposite lane 1
	g.enter(t, 1, "sandsnake", 1, []TargetRef{crRef(0, 1)})
	if g.State.Players[0].Lanes[1].Creature != nil {
		t.Errorf("Sandsnake should destroy a 3/3 (damage %d)", foe.Damage)
	}
	// With nothing opposite, the trigger does nothing and the play is legal.
	g.enter(t, 1, "sandsnake", 2)

	ally := g.put(1, 3, "wandering-bald-man") // 0/10
	enemy := g.put(0, 0, "dark-angel")        // 0/5
	g.enter(t, 1, "sandhorn-devil", 0)
	if ally.Damage != 1 || enemy.Damage != 1 {
		t.Errorf("Sandhorn Devil hits every Creature: ally %d, enemy %d", ally.Damage, enemy.Damage)
	}
	if devil := g.State.Players[1].Lanes[0].Creature; devil == nil || devil.Damage != 1 {
		t.Errorf("including itself: %+v", devil)
	}
}

func TestEntersPlayWatchers(t *testing.T) {
	g := ladyGame(t)
	g.enter(t, 1, "sandwitch", 0)
	if g.State.Players[0].HP != StartingHP-1 {
		t.Errorf("SandWitch hurts the opponent when it enters: HP %d", g.State.Players[0].HP)
	}
	g.enter(t, 1, "shark", 1)
	if g.State.Players[0].HP != StartingHP-2 {
		t.Errorf("SandWitch sees Shark enter: HP %d", g.State.Players[0].HP)
	}
	if got := g.atkOf(1, 1); got != 3 {
		t.Errorf("Shark (2 ATK) gives itself +1 ATK: %d, want 3", got)
	}
	g.enter(t, 1, "sand-eyebat", 2)  // 1/10
	g.enter(t, 1, "wall-of-sand", 3) // 1/12
	if got := g.defOf(1, 2); got != 11 {
		t.Errorf("Sand Eyebat gains 1 DEF when another Creature enters: %d, want 11", got)
	}
	// Wall of Sand: +2 ATK once per turn for other SandyLands Creatures; Shark adds +1 to them.
	g.enter(t, 1, "fummy", 0) // SandyLands, replaces SandWitch
	// Wall of Sand: 1 base, +1 from Shark when it entered, +2 from its own trigger.
	if got := g.atkOf(1, 3); got != 4 {
		t.Errorf("Wall of Sand after a SandyLands Creature: %d, want 4", got)
	}
	g.enter(t, 1, "sand-knights", 1)
	if got := g.atkOf(1, 3); got != 4 {
		t.Errorf("Wall of Sand only gains once a turn: %d, want 4", got)
	}
	// Everything "this turn" ends; DEF from Sand Eyebat stays.
	g.endTurn(1)
	if got := g.atkOf(1, 3); got != 1 {
		t.Errorf("Wall of Sand next turn: %d, want 1", got)
	}
	if got := g.defOf(1, 2); got < 11 {
		t.Errorf("Sand Eyebat keeps its DEF: %d", got)
	}
}

func TestSharkOnlyDuringYourTurn(t *testing.T) {
	g := ladyGame(t)
	g.put(1, 0, "peach-djinni") // 0/6, +1 ATK when a SandyLands Creature enters under your control
	g.enter(t, 1, "fummy", 1)
	if got := g.atkOf(1, 0); got != 1 {
		t.Errorf("Peach Djinni: %d, want 1", got)
	}
	g.enter(t, 1, "mace-stump", 2) // Useless Swamp: not SandyLands
	if got := g.atkOf(1, 0); got != 1 {
		t.Errorf("a Useless Swamp Creature does not count: %d", got)
	}
	g.endTurn(1)
	if got := g.atkOf(1, 0); got != 0 {
		t.Errorf("+1 ATK ends with the turn: %d", got)
	}
}

func TestTurnStartTriggers(t *testing.T) {
	g := ladyGame(t)
	g.put(0, 0, "chest-burster")
	g.put(0, 1, "extraordinary-spider")
	g.put(0, 2, "wandering-bald-man")
	g.fillDiscard(0, 10, "fummy")
	g.State.Players[1].Hand = nil // the opponent holds no cards
	deck := len(g.State.Players[0].Deck)
	g.must(t, 1, endMain)
	if g.State.Active != 0 {
		t.Fatal("setup: expected player 0's turn")
	}
	// Chest Burster 3 + Spider 2 (10 discards).
	if got := g.State.Players[1].HP; got != StartingHP-5 {
		t.Errorf("HP = %d, want %d", got, StartingHP-5)
	}
	// One draw, one milled.
	if got := len(g.State.Players[0].Deck); got != deck-2 {
		t.Errorf("deck %d, want %d (a draw and a mill)", got, deck-2)
	}
	if got := len(g.State.Players[0].Discard); got < 11 {
		t.Errorf("Wandering Bald Man should have milled a card: %d", got)
	}
}

func TestChestBursterNeedsAnEmptyHand(t *testing.T) {
	g := ladyGame(t)
	g.put(0, 0, "chest-burster")
	g.must(t, 1, endMain)
	if got := g.State.Players[1].HP; got != StartingHP {
		t.Errorf("the opponent has cards in hand: HP %d", got)
	}
}

func TestDestroySelfAbilities(t *testing.T) {
	g := ladyGame(t)
	g.put(0, 0, "dark-angel")
	g.put(0, 1, "green-mermaid")
	g.State.Active = 0
	g.State.ActionsLeft = 0
	before := g.State.Players[1].HP
	e1 := g.put(1, 0, "wandering-bald-man") // 0/10
	g.must(t, 0, activate(1, 0))
	if g.State.Players[0].Lanes[1].Creature != nil {
		t.Error("Green Mermaid destroys itself")
	}
	if e1.Damage != 1 || g.State.Players[1].HP != before {
		t.Errorf("1 damage to each opposing Creature: %d", e1.Damage)
	}
	if n := len(g.State.Players[0].Discard); n != 1 {
		t.Errorf("Green Mermaid goes to the discard pile: %d", n)
	}

	// Mace Stump: destroy itself, the opponent discards 1 per 5 cards in the discard pile.
	g.put(0, 2, "mace-stump")
	g.fillDiscard(0, 9, "fummy") // plus the Mermaid and, after destroying, the Stump: 11
	hand := len(g.State.Players[1].Hand)
	g.must(t, 0, activate(2, 0))
	// 11 cards in the discard pile: the opponent chooses 2 cards to discard.
	g.discardChoices(t, 1, 2)
	if got := len(g.State.Players[1].Hand); got != hand-2 {
		t.Errorf("hand %d -> %d, want 2 fewer", hand, got)
	}
}

func TestGreenMermanDamagesByTheMilledCost(t *testing.T) {
	g := ladyGame(t)
	g.State.Active = 0
	g.put(0, 0, "green-merman")
	foe := g.put(1, 0, "wandering-bald-man") // 0/10
	deck := &g.State.Players[0].Deck
	top := (*deck)[0]
	want := 0
	if c := g.Card(top).Cost; c != nil {
		want = *c
	}
	g.must(t, 0, floop(0))
	if foe.Damage != want {
		t.Errorf("damage %d, want the milled %s's cost %d", foe.Damage, g.Card(top).Name, want)
	}
}

func TestDrDeathDestroysTwoCreatures(t *testing.T) {
	g := ladyGame(t)
	g.State.Active = 0
	g.put(0, 0, "dr-death")
	g.put(0, 1, "fummy")
	g.put(1, 0, "wandering-bald-man")
	g.must(t, 0, floop(0, []TargetRef{crRef(0, 1)}, []TargetRef{crRef(1, 0)}))
	if g.State.Players[0].Lanes[1].Creature != nil || g.State.Players[1].Lanes[0].Creature != nil {
		t.Error("both the sacrificed Creature and the target should be destroyed")
	}
	if g.State.Players[0].Lanes[0].Creature == nil {
		t.Error("Dr. Death stays")
	}
}

func TestBouncingFloops(t *testing.T) {
	g := ladyGame(t)
	g.put(1, 1, "beach-mummy")
	foe := g.put(0, 0, "dark-angel")
	g.must(t, 1, floop(1, []TargetRef{crRef(0, 0)}))
	if g.State.Players[0].Lanes[0].Creature != nil || !handHas(g, 0, foe.UID) {
		t.Error("Beach Mummy returns a Creature in an adjacent Lane to its owner's hand")
	}
	g.put(1, 2, "fummy")
	g.building(1, 2, "sand-sphinx")
	g.must(t, 1, Action{Kind: ActFloop, Lane: 2, Source: "building", Targets: [][]TargetRef{{crRef(1, 2)}}})
	if g.State.Players[1].Lanes[2].Creature != nil {
		t.Error("Sand Sphinx returns the Creature in its Lane")
	}
}

func handHas(g *Game, p PlayerID, uid int) bool {
	for _, in := range g.State.Players[p].Hand {
		if in.UID == uid {
			return true
		}
	}
	return false
}

func TestFummyGainsAnAction(t *testing.T) {
	g := ladyGame(t)
	g.put(1, 0, "fummy")
	g.must(t, 1, floop(0))
	if g.State.ActionsLeft != ActionsPerTurn+1 {
		t.Errorf("Actions %d", g.State.ActionsLeft)
	}
}

func TestReturnFromTheDiscardPile(t *testing.T) {
	g := ladyGame(t)
	g.put(1, 0, "gray-eyebat")
	g.put(1, 1, "tree-of-undeath")
	g.put(1, 2, "red-eyeling")
	g.State.Players[1].Discard = nil
	g.fillDiscard(1, 1, "wandering-bald-man") // a Rainbow Creature with cost 0
	g.fillDiscard(1, 1, "dark-angel")         // Useless Swamp Creature
	hand := len(g.State.Players[1].Hand)

	g.must(t, 1, activate(0, 0)) // random Useless Swamp Creature: the Dark Angel
	if len(g.State.Players[1].Hand) != hand+1 || len(g.State.Players[1].Discard) != 1 {
		t.Fatalf("Gray Eyebat: hand %d discard %d", len(g.State.Players[1].Hand), len(g.State.Players[1].Discard))
	}
	if g.State.ActionsLeft != ActionsPerTurn-1 {
		t.Errorf("Gray Eyebat costs 1 Action: %d", g.State.ActionsLeft)
	}
	g.must(t, 1, floop(1)) // any Creature: the Bald Man
	if len(g.State.Players[1].Discard) != 0 {
		t.Errorf("Tree of Undeath: discard %d", len(g.State.Players[1].Discard))
	}
	// Red-Eyeling returns a chosen cost 0 card; nothing is left to choose.
	g.fillDiscard(1, 1, "dark-angel") // cost 1: not a legal choice
	g.fillDiscard(1, 1, "ms-mummy")   // cost 0
	var mummy int
	for _, in := range g.State.Players[1].Discard {
		if in.CardID == "ms-mummy" {
			mummy = in.UID
		}
	}
	g.mustFail(t, 1, floop(2, []TargetRef{{Kind: "discard_card", Player: 1, Card: g.State.Players[1].Discard[0].UID}}), ErrBadTarget)
	g.must(t, 1, floop(2, []TargetRef{{Kind: "discard_card", Player: 1, Card: mummy}}))
	if !handHas(g, 1, mummy) {
		t.Error("Red-Eyeling should return the chosen cost 0 card")
	}
}

func TestSpiritTowerBorrowsACreatureForATurn(t *testing.T) {
	g := ladyGame(t)
	foe := g.put(0, 1, "dark-angel")
	g.building(1, 1, "spirit-tower")
	g.State.Players[1].Lanes[1].Creature = nil
	tower := func(targets ...[]TargetRef) Action {
		return Action{Kind: ActFloop, Lane: 1, Source: "building", Targets: targets}
	}
	g.must(t, 1, tower([]TargetRef{crRef(0, 1)}, []TargetRef{{Kind: "lane", Player: 1, Lane: 1}}))
	if g.State.ActionsLeft != ActionsPerTurn-1 {
		t.Errorf("the Floop costs 1 Action: %d", g.State.ActionsLeft)
	}
	if c := g.State.Players[1].Lanes[1].Creature; c != foe || c.Status != Ready {
		t.Fatalf("the Creature should be on my side and Ready: %+v", c)
	}
	g.must(t, 1, endMain)
	g.must(t, 1, Action{Kind: ActAttack, Lane: 1})
	if g.State.Players[0].Lanes[1].Creature == nil && g.State.Players[1].Lanes[1].Creature != nil {
		t.Fatal("it should have gone home at the end of the turn")
	}
	if c := g.State.Players[0].Lanes[1].Creature; c == nil || c.Owner != 0 {
		t.Errorf("back on its owner's side: %+v", c)
	}
}

func TestPalaceOfBoneStopsOpposingTriggers(t *testing.T) {
	g := ladyGame(t)
	g.building(0, 0, "palace-of-bone")
	g.put(0, 0, "wandering-bald-man")
	// Lady plays SandWitch opposite Palace of Bone: it triggers nothing.
	hp := g.State.Players[0].HP
	g.enter(t, 1, "sandwitch", 0)
	if g.State.Players[0].HP != hp {
		t.Errorf("SandWitch opposite the Palace should not trigger: HP %d", g.State.Players[0].HP)
	}
	g.enter(t, 1, "sandwitch", 1)
	if g.State.Players[0].HP != hp-2 { // the new one and the first one, which sees it enter
		t.Errorf("elsewhere it does: HP %d, want %d", g.State.Players[0].HP, hp-2)
	}
}

func TestPlayConditionsAndCostMods(t *testing.T) {
	g := ladyGame(t)
	// Hot Eyebat needs 10 cards in the discard pile.
	g.enterFails(t, 1, "hot-eyebat", 0, ErrCondition)
	g.fillDiscard(1, 10, "fummy")
	g.enter(t, 1, "hot-eyebat", 0)

	// Teeth Leaf: 2 fewer Actions with 10 in the discard pile (still needs 2 Landscapes).
	g.State.Players[1].Lanes[1].Landscape = card.UselessSwamp
	g.State.Players[1].Lanes[2].Landscape = card.UselessSwamp
	g.State.ActionsLeft = 0
	uid := g.give(1, "teeth-leaf")
	g.must(t, 1, Action{Kind: ActPlayCard, Card: uid, Lane: 1})

	// Lost Golem: 1 less per other Creature entered this turn (cost 3).
	g.State.Players[1].Lanes[0].Landscape = card.SandyLands
	for _, l := range []int{0, 1, 2, 3} {
		g.State.Players[1].Lanes[l].Landscape = card.SandyLands
	}
	g.State.ActionsLeft = 0
	g.State.Entered = [2][NumLanes]int{}
	g.State.Entered[1][0] = 3 // three Creatures this turn: free
	uid = g.give(1, "lost-golem")
	g.must(t, 1, Action{Kind: ActPlayCard, Card: uid, Lane: 3})
}

func TestUnemptyCoffinDiscountsTheNextCreature(t *testing.T) {
	g := ladyGame(t)
	g.State.ActionsLeft = 2
	coffin := g.give(1, "unempty-coffin")
	g.must(t, 1, Action{Kind: ActPlayCard, Card: coffin})
	golem := g.give(1, "lost-golem") // cost 3 with only 2 Actions left: 2 fewer Actions
	for l := range g.State.Players[1].Lanes {
		g.State.Players[1].Lanes[l].Landscape = card.SandyLands
	}
	g.must(t, 1, Action{Kind: ActPlayCard, Card: golem, Lane: 0})
	if g.State.ActionsLeft != 1 {
		t.Errorf("Actions left %d, want 1", g.State.ActionsLeft)
	}
	if g.State.CreatureDiscount != 0 {
		t.Error("the discount is used up")
	}
	// A discount unused at the end of the turn is gone.
	g.must(t, 1, Action{Kind: ActPlayCard, Card: g.give(1, "unempty-coffin")})
	g.endTurn(1)
	if g.State.CreatureDiscount != 0 {
		t.Error("the discount ends with the turn")
	}
}

func TestSpells(t *testing.T) {
	t.Run("Abraca Amadeus and Bone Wand make the opponent discard", func(t *testing.T) {
		g := ladyGame(t)
		g.fillDiscard(1, 10, "fummy")
		hand := len(g.State.Players[0].Hand)
		g.State.ActionsLeft = 2
		g.must(t, 1, Action{Kind: ActPlayCard, Card: g.give(1, "abraca-amadeus")})
		if g.State.Choice == nil || g.State.Choice.Player != 0 || g.State.Choice.Count != 2 {
			t.Fatalf("the opponent must choose 2 cards to discard: %+v", g.State.Choice)
		}
		// Nothing else can happen until they have chosen.
		g.mustFail(t, 1, Action{Kind: ActDraw}, ErrChoice)
		g.mustFail(t, 0, Action{Kind: ActDraw}, ErrChoice)
		g.mustFail(t, 1, Action{Kind: ActChoose, Card: g.State.Players[0].Hand[0].UID}, ErrNotYourTurn)
		g.mustFail(t, 0, Action{Kind: ActChoose, Card: 99999}, ErrNoSuchCard)
		if got := g.Awaiting(); len(got) != 1 || got[0] != 0 {
			t.Errorf("awaiting %v, want only the opponent", got)
		}
		g.discardChoices(t, 0, 2)
		if got := len(g.State.Players[0].Hand); got != hand-2 {
			t.Errorf("hand %d -> %d, want 2 fewer", hand, got)
		}
		if g.State.Choice != nil || g.Awaiting()[0] != 1 {
			t.Error("the active player goes on once the choice is made")
		}
		wand := g.give(1, "bone-wand")
		g.mustFail(t, 1, Action{Kind: ActPlayCard, Card: wand}, ErrCondition)
		g.put(1, 0, "mace-stump")
		g.must(t, 1, Action{Kind: ActPlayCard, Card: wand})
		g.discardChoices(t, 0, 1)
		if got := len(g.State.Players[0].Hand); got != hand-3 {
			t.Errorf("Bone Wand: hand %d", got)
		}
	})
	t.Run("Magic Ring-Ding", func(t *testing.T) {
		g := ladyGame(t)
		g.State.Active = 0
		g.fillDiscard(0, 10, "fummy")
		a, b := g.put(0, 0, "mace-stump"), g.put(0, 1, "mace-stump")
		foe := g.put(1, 0, "fummy")
		g.State.ActionsLeft = 2
		g.must(t, 0, Action{Kind: ActPlayCard, Card: g.give(0, "magic-ring-ding")})
		if a.TempATK != 2 || b.TempATK != 2 || foe.TempATK != 0 {
			t.Errorf("+2 ATK to each of your Creatures only: %d %d %d", a.TempATK, b.TempATK, foe.TempATK)
		}
	})
	t.Run("Tome of Ankhs draws per empty lane", func(t *testing.T) {
		g := ladyGame(t)
		g.put(1, 0, "fummy")
		hand := len(g.State.Players[1].Hand)
		g.State.ActionsLeft = 2
		g.must(t, 1, Action{Kind: ActPlayCard, Card: g.give(1, "tome-of-ankhs")})
		if got := len(g.State.Players[1].Hand); got != hand+3 { // +3 draws; the Tome left the hand
			t.Errorf("hand %d -> %d, want 3 more cards", hand, got)
		}
	})
	t.Run("Zazo's Magic Seeds", func(t *testing.T) {
		g := ladyGame(t)
		tgt := g.put(1, 1, "fummy")
		g.enter(t, 1, "fummy", 0) // adjacent
		g.enter(t, 1, "fummy", 2) // adjacent
		g.State.ActionsLeft = 2
		g.must(t, 1, Action{Kind: ActPlayCard, Card: g.give(1, "zazo-s-magic-seeds"), Targets: [][]TargetRef{{crRef(1, 1)}}})
		if tgt.TempATK != 4 {
			t.Errorf("+2 ATK per Creature that entered an adjacent Lane: %d, want 4", tgt.TempATK)
		}
	})
	t.Run("The Mariachi", func(t *testing.T) {
		g := ladyGame(t)
		g.put(1, 0, "the-mariachi")
		foe := g.put(0, 0, "dark-angel") // 0/5
		g.enter(t, 1, "fummy", 1)
		g.put(0, 2, "fummy")
		g.State.Entered[0][2] = 1 // a Creature entered under the other player too
		g.must(t, 1, floop(0, []TargetRef{crRef(0, 0)}))
		if foe.Damage != 2 {
			t.Errorf("1 damage per Creature entered this turn (both sides): %d, want 2", foe.Damage)
		}
	})
	t.Run("Snake-Eye Ring and Ogre Gas", func(t *testing.T) {
		g := ladyGame(t)
		g.fillDiscard(1, 1, "dark-angel")
		g.State.ActionsLeft = 2
		hand := len(g.State.Players[1].Hand)
		g.must(t, 1, Action{Kind: ActPlayCard, Card: g.give(1, "snake-eye-ring")})
		if got := len(g.State.Players[1].Hand); got != hand+1 || len(g.State.Players[1].Discard) != 1 { // the Ring itself is discarded
			t.Errorf("Snake-Eye Ring: hand %d, discard %d", got, len(g.State.Players[1].Discard))
		}
		deck := len(g.State.Players[1].Deck)
		discard := len(g.State.Players[1].Discard)
		g.must(t, 1, Action{Kind: ActPlayCard, Card: g.give(1, "ogre-gas")})
		ch := g.State.Choice
		if ch == nil || ch.Kind != ChoiceReveal || len(ch.Revealed) != 3 {
			t.Fatalf("Ogre Gas reveals 3 cards: %+v", ch)
		}
		keep := ch.Revealed[1]
		g.mustFail(t, 1, Action{Kind: ActChoose, Card: g.State.Players[1].Hand[0].UID}, ErrNoSuchCard)
		g.must(t, 1, Action{Kind: ActChoose, Card: keep.UID})
		bottom := g.State.Players[1].Deck[len(g.State.Players[1].Deck)-1]
		if bottom.UID != keep.UID {
			t.Errorf("the chosen card goes to the bottom: %d, want %d", bottom.UID, keep.UID)
		}
		if got := len(g.State.Players[1].Deck); got != deck-2 {
			t.Errorf("one of the top 3 goes to the bottom, two are discarded: deck %d -> %d", deck, got)
		}
		if got := len(g.State.Players[1].Discard); got != discard+3 { // two cards and the spell
			t.Errorf("discard %d -> %d", discard, got)
		}
	})
}

// mummyActions are p's activations of the Ms. Mummy in Lane 0.
func mummyActions(g *Game, p PlayerID) []Action {
	var out []Action
	for _, a := range g.LegalActions(p) {
		if a.Kind == ActActivate && a.Lane == 0 {
			out = append(out, a)
		}
	}
	return out
}

// At the start of your turn Ms. Mummy offers her ability on the card, free;
// nothing stops the game to ask.
func TestMsMummyOffersHerAbilityAtTheStartOfYourTurn(t *testing.T) {
	g := ladyGame(t)
	g.put(1, 0, "ms-mummy")
	ally := g.put(1, 1, "fummy") // SandyLands, 2/7
	g.State.Active = 0
	g.endTurn(0) // player 1's turn begins
	if g.State.Choice != nil {
		t.Fatalf("no question is asked: %+v", g.State.Choice)
	}
	// Without a target, or helping the one SandyLands Creature.
	acts := mummyActions(g, 1)
	if len(acts) != 2 || len(acts[0].Targets[0]) != 0 || len(acts[1].Targets[0]) != 1 {
		t.Fatalf("activations = %+v", acts)
	}
	left := g.State.ActionsLeft
	g.must(t, 1, acts[1])
	if g.State.Players[1].Lanes[0].Creature != nil || !handHas(g, 1, g.cardUID(1, "ms-mummy")) {
		t.Error("Ms. Mummy returns to her owner's hand")
	}
	if g.State.ActionsLeft != left {
		t.Errorf("the ability is free: Actions %d -> %d", left, g.State.ActionsLeft)
	}
	if ally.PermDEF != 1 || g.defOf(1, 1) != 8 {
		t.Errorf("the chosen Creature gains 1 DEF for good: %d", g.defOf(1, 1))
	}
	g.endTurn(1)
	if ally.PermDEF != 1 {
		t.Error("the DEF is not lost at the end of the turn")
	}
}

// The offer is for the start of the turn only: any other action lets it pass.
func TestMsMummyOfferLapsesWhenYouDoSomethingElse(t *testing.T) {
	for name, other := range map[string]Action{
		"draw":     {Kind: ActDraw},
		"end main": {Kind: ActEndMain},
	} {
		t.Run(name, func(t *testing.T) {
			g := ladyGame(t)
			g.put(1, 0, "ms-mummy")
			g.State.Active = 0
			g.endTurn(0)
			if len(mummyActions(g, 1)) == 0 {
				t.Fatal("the offer is there at the start of the turn")
			}
			g.must(t, 1, other)
			if len(mummyActions(g, 1)) != 0 {
				t.Error("the offer is gone once something else was done")
			}
			if g.State.Players[1].Lanes[0].Creature == nil {
				t.Error("Ms. Mummy stays where she is")
			}
		})
	}
}

// Using one Mummy leaves the other's offer; offers are made afresh each turn
// rather than piling up.
func TestOffersAreSeparateAndDoNotPileUp(t *testing.T) {
	g := ladyGame(t)
	g.put(1, 0, "ms-mummy")
	g.put(1, 1, "ms-mummy")
	lane1 := func() []Action {
		var out []Action
		for _, a := range g.LegalActions(1) {
			if a.Kind == ActActivate && a.Lane == 1 {
				out = append(out, a)
			}
		}
		return out
	}
	g.State.Active = 0
	g.endTurn(0)
	first := len(lane1())
	g.must(t, 1, mummyActions(g, 1)[0])
	if len(lane1()) != first || first == 0 {
		t.Fatalf("the second Mummy can still be used: %d, was %d", len(lane1()), first)
	}
	g.endTurn(1)
	g.endTurn(0) // player 1's next turn
	if got := len(lane1()); got != first {
		t.Errorf("a new turn offers once more, not on top of the old: %d, want %d", got, first)
	}
}

// With no SandyLands Creature to help she may still go home.
func TestMsMummyWithoutATarget(t *testing.T) {
	g := ladyGame(t)
	g.put(1, 0, "ms-mummy")
	g.State.Active = 0
	g.endTurn(0)
	acts := mummyActions(g, 1)
	if len(acts) != 1 {
		t.Fatalf("activations = %+v", acts)
	}
	g.must(t, 1, acts[0])
	if g.State.Players[1].Lanes[0].Creature != nil {
		t.Error("she returned to her owner's hand")
	}
}

func TestChoicesQueue(t *testing.T) {
	g := ladyGame(t)
	g.ask(Choice{Player: 1, Kind: ChoiceDiscard, Card: "first", Count: 1})
	g.ask(Choice{Player: 1, Kind: ChoiceDiscard, Card: "second", Count: 1})
	if g.State.Choice == nil || g.State.Choice.Card != "first" || len(g.State.Queue) != 1 {
		t.Fatalf("two questions ask one after the other: %+v / %d queued", g.State.Choice, len(g.State.Queue))
	}
	g.closeChoice()
	if g.State.Choice == nil || g.State.Choice.Card != "second" || len(g.State.Queue) != 0 {
		t.Fatal("the second question follows the first")
	}
	g.closeChoice()
	if g.State.Choice != nil {
		t.Error("no more questions")
	}
}

func TestDiscardingYourWholeHandAsksNothing(t *testing.T) {
	g := ladyGame(t)
	g.fillDiscard(1, 30, "fummy")
	g.State.Players[0].Hand = g.State.Players[0].Hand[:1]
	g.State.ActionsLeft = 2
	g.must(t, 1, Action{Kind: ActPlayCard, Card: g.give(1, "abraca-amadeus")})
	if g.State.Choice != nil || len(g.State.Players[0].Hand) != 0 {
		t.Error("with fewer cards than to discard there is nothing to choose")
	}
}

// cardUID is the UID of the card named id in p's hand.
func (g *Game) cardUID(p PlayerID, id string) int {
	for _, in := range g.State.Players[p].Hand {
		if in.CardID == id {
			return in.UID
		}
	}
	return 0
}

// discardChoices makes p choose their first n hand cards to discard.
func (g *Game) discardChoices(t *testing.T, p PlayerID, n int) {
	t.Helper()
	for i := 0; i < n; i++ {
		g.must(t, p, Action{Kind: ActChoose, Card: g.State.Players[p].Hand[0].UID})
	}
}

// Every card of all six decks can be played through the real rules, so random
// games must keep every invariant.
func TestRandomPlayoutsOfAllSixDecks(t *testing.T) {
	realGame(t)
	decks := []struct {
		set, name string
		lands     [NumLanes]card.Landscape
	}{
		{"Finn Vs Jake", "Finn's Deck", [NumLanes]card.Landscape{card.BluePlains, card.BluePlains, card.BluePlains, card.BluePlains}},
		{"Finn Vs Jake", "Jake's Deck", cornfields},
		{"BMO Vs Lady Rainicorn", "BMO's Deck", [NumLanes]card.Landscape{card.UselessSwamp, card.UselessSwamp, card.UselessSwamp, card.UselessSwamp}},
		{"BMO Vs Lady Rainicorn", "Lady's Deck", sandy},
		{"Princess Bubblegum Vs Lumpy Space Princess", "PB's Deck", [NumLanes]card.Landscape{card.NiceLands, card.NiceLands, card.NiceLands, card.NiceLands}},
		{"Princess Bubblegum Vs Lumpy Space Princess", "LSP's Deck", [NumLanes]card.Landscape{card.BluePlains, card.BluePlains, card.UselessSwamp, card.UselessSwamp}},
	}
	games := 60
	if testing.Short() {
		games = 12
	}
	seen := map[EventKind]int{}
	finished := 0
	for i := 0; i < games; i++ {
		a, b := decks[i%len(decks)], decks[(i/len(decks)+i+1)%len(decks)]
		spec := func(d struct {
			set, name string
			lands     [NumLanes]card.Landscape
		}) DeckSpec {
			return DeckSpec{d.lands, realCat.Deck(d.set, d.name)}
		}
		seed := uint64(i + 1)
		g, err := NewGame(realCat, Config{Seed: seed, First: PlayerID(seed % 2)}, [2]DeckSpec{spec(a), spec(b)})
		if err != nil {
			t.Fatal(err)
		}
		if _, done := playRandomSeen(t, g, rand.New(rand.NewPCG(seed, 5)), 20000, seen); done {
			finished++
		}
	}
	if finished < games*9/10 {
		t.Errorf("only %d of %d games finished", finished, games)
	}
}
