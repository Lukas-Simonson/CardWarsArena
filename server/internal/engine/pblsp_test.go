package engine

import (
	"testing"

	"cardwars/internal/card"
)

const pbSet = "Princess Bubblegum Vs Lumpy Space Princess"

// pbGame is PB's Deck (player 0) against LSP's Deck (player 1), on player 1's
// turn. Both sides have two NiceLands and two Blue Plains, enough for the
// dearest cards of those types.
func pbGame(t *testing.T) *Game {
	t.Helper()
	realGame(t) // loads the catalog
	mix := [NumLanes]card.Landscape{card.NiceLands, card.NiceLands, card.BluePlains, card.BluePlains}
	g, err := NewGame(realCat, Config{Seed: 1, First: 0}, [2]DeckSpec{
		{Landscapes: mix, Cards: realCat.Deck(pbSet, "PB's Deck"), Hero: "princess-bubblegum"}, {Landscapes: mix, Cards: realCat.Deck(pbSet, "LSP's Deck"), Hero: "lumpy-space-princess"},
	})
	if err != nil {
		t.Fatal(err)
	}
	g.start(t)
	g.must(t, 0, endMain)
	g.State.Active = 0
	g.endTurn(0) // player 1's turn begins
	return g
}

// castSpell plays a Spell of player 1's with the given targets and Actions to spare.
func (g *Game) castSpell(t *testing.T, id string, targets ...[]TargetRef) {
	t.Helper()
	uid := g.give(1, id)
	g.State.ActionsLeft = ActionsPerTurn
	g.must(t, 1, Action{Kind: ActPlayCard, Card: uid, Targets: targets})
}

// useWithDiscard uses the Discard-a-card ability of player 1's Creature in a
// Lane, discarding the first card in hand.
func (g *Game) useWithDiscard(t *testing.T, lane int) error {
	t.Helper()
	var targets [][]TargetRef
	if h := g.State.Players[1].Hand; len(h) > 0 {
		targets = [][]TargetRef{{handRef(1, h[0].UID)}}
	}
	_, err := g.Apply(1, activate(lane, 0, targets...))
	return err
}

func TestExactDamageGivesATKAndNoDamageGivesMore(t *testing.T) {
	g := pbGame(t)
	albino := g.put(1, 0, "albino-eyebat") // 2/7, +2 ATK with exactly 2 damage
	for damage, want := range map[int]int{0: 2, 1: 2, 2: 4, 3: 2} {
		albino.Damage = damage
		if got := g.atkOf(1, 0); got != want {
			t.Errorf("Albino Eyebat with %d damage: ATK %d, want %d", damage, got, want)
		}
	}
	wall := g.put(1, 1, "wall-of-chocolate") // 1/9, +3 ATK with no damage
	if g.atkOf(1, 1) != 4 {
		t.Errorf("undamaged Wall of Chocolate: ATK %d, want 4", g.atkOf(1, 1))
	}
	wall.Damage = 1
	if g.atkOf(1, 1) != 1 {
		t.Errorf("damaged Wall of Chocolate: ATK %d, want 1", g.atkOf(1, 1))
	}
	for id, exact := range map[string]int{"angel-heart": 3, "cotton-eyebat": 4, "ms-fluff": 7} {
		c := g.put(1, 2, id)
		base := g.atkOf(1, 2)
		c.Damage = exact
		if got := g.atkOf(1, 2); got != base+exact {
			t.Errorf("%s with exactly %d damage: ATK %d, want %d", id, exact, got, base+exact)
		}
	}
}

func TestWindmillOfHealthRewardsAnUndamagedCreature(t *testing.T) {
	g := pbGame(t)
	g.building(1, 0, "windmill-of-health")
	cow := g.put(1, 0, "cow") // 1/5
	g.put(1, 1, "cow")
	if g.atkOf(1, 0) != 3 || g.atkOf(1, 1) != 1 {
		t.Errorf("only the Creature in its Lane: %d %d, want 3 and 1", g.atkOf(1, 0), g.atkOf(1, 1))
	}
	cow.Damage = 1
	if g.atkOf(1, 0) != 1 {
		t.Errorf("damaged: %d, want 1", g.atkOf(1, 0))
	}
}

func TestHealing(t *testing.T) {
	t.Run("Angel of Chocolate heals all damage for 1 Action", func(t *testing.T) {
		g := pbGame(t)
		angel := g.put(1, 0, "angel-of-chocolate")
		angel.Damage = 5
		g.must(t, 1, activate(0, 0))
		if angel.Damage != 0 || g.State.ActionsLeft != ActionsPerTurn-1 {
			t.Errorf("damage %d, Actions %d", angel.Damage, g.State.ActionsLeft)
		}
	})
	t.Run("Blue Candy heals up to 3", func(t *testing.T) {
		g := pbGame(t)
		cr := g.put(1, 0, "fairy-shepherd")
		cr.Damage = 5
		g.castSpell(t, "blue-candy", []TargetRef{crRef(1, 0)})
		if cr.Damage != 2 {
			t.Errorf("damage %d, want 2", cr.Damage)
		}
		g.castSpell(t, "blue-candy", []TargetRef{crRef(1, 0)})
		if cr.Damage != 0 {
			t.Errorf("healing stops at none: %d", cr.Damage)
		}
	})
	t.Run("Cutie heals you, but never above 25", func(t *testing.T) {
		g := pbGame(t)
		g.put(1, 0, "cutie")
		g.State.Players[1].HP = 20
		g.must(t, 1, floop(0))
		if g.State.Players[1].HP != 21 {
			t.Errorf("HP %d, want 21", g.State.Players[1].HP)
		}
		g.State.Players[1].HP = StartingHP
		g.State.Players[1].Lanes[0].Creature.Status = Ready
		ev := g.must(t, 1, floop(0))
		for _, e := range ev {
			if e.Kind == EvHPGain {
				t.Errorf("at full health nothing is healed: %+v", e)
			}
		}
		if g.State.Players[1].HP != StartingHP {
			t.Errorf("HP %d", g.State.Players[1].HP)
		}
	})
	t.Run("Pieclops heals the Creatures beside it", func(t *testing.T) {
		g := pbGame(t)
		for lane := 0; lane < 4; lane++ {
			g.put(1, lane, "cow").Damage = 2
		}
		g.State.Players[1].Lanes[1].Creature = nil
		g.enter(t, 1, "pieclops", 1)
		d := func(lane int) int { return g.State.Players[1].Lanes[lane].Creature.Damage }
		if d(0) != 1 || d(2) != 1 || d(3) != 2 {
			t.Errorf("damage %d %d %d, want 1 1 2", d(0), d(2), d(3))
		}
	})
}

func TestGreenCandyHealsOrDamagesExactlyOneCreature(t *testing.T) {
	g := pbGame(t)
	cr := g.put(1, 0, "fairy-shepherd")
	cr.Damage = 3
	uid := g.give(1, "green-candy")
	if err := g.Validate(1, Action{Kind: ActPlayCard, Card: uid, Targets: [][]TargetRef{{}, {}}}); err != ErrBadTarget {
		t.Errorf("choosing nothing: %v", err)
	}
	both := [][]TargetRef{{crRef(1, 0)}, {crRef(1, 0)}}
	if err := g.Validate(1, Action{Kind: ActPlayCard, Card: uid, Targets: both}); err != ErrBadTarget {
		t.Errorf("choosing both: %v", err)
	}
	n := 0
	for _, a := range g.LegalActions(1) {
		if a.Kind == ActPlayCard && a.Card == uid {
			n++
		}
	}
	if n != 2 {
		t.Errorf("with one Creature the Candy heals or damages it: %d plays", n)
	}
	g.must(t, 1, Action{Kind: ActPlayCard, Card: uid, Targets: [][]TargetRef{{crRef(1, 0)}, {}}})
	if cr.Damage != 2 {
		t.Errorf("healed: %d", cr.Damage)
	}
	g.castSpell(t, "green-candy", []TargetRef{}, []TargetRef{crRef(1, 0)})
	if cr.Damage != 3 {
		t.Errorf("damaged: %d", cr.Damage)
	}
}

// Apple Pieclops offers, at the start of your turn, to heal some Creatures and
// damage others.
func TestApplePieclopsHealsOrDamagesEachOfYourCreatures(t *testing.T) {
	g := pbGame(t)
	g.put(1, 0, "apple-pieclops")
	a, b := g.put(1, 1, "cow"), g.put(1, 2, "cow")
	a.Damage = 2
	g.State.Active = 0
	g.endTurn(0)
	g.must(t, 1, activate(0, 0, []TargetRef{crRef(1, 1)}, []TargetRef{crRef(1, 2)}))
	if a.Damage != 1 || b.Damage != 1 {
		t.Errorf("healed one, hurt the other: %d %d", a.Damage, b.Damage)
	}
}

// Each of your Creatures is healed, damaged or left alone, so 4 Creatures have
// 3^4 choices, all of them listed, each labelled for what it does.
func TestApplePieclopsListsEveryChoiceOnce(t *testing.T) {
	g := pbGame(t)
	g.put(1, 0, "apple-pieclops")
	for lane := 1; lane < 4; lane++ {
		g.put(1, lane, "cow")
	}
	g.State.Active = 0
	g.endTurn(0)
	n := 0
	for _, a := range g.LegalActions(1) {
		if a.Kind != ActActivate || a.Lane != 0 {
			continue
		}
		n++
		if len(a.Labels) != 2 || a.Labels[0] != "Heal 1 damage from" || a.Labels[1] != "Deal 1 damage to" {
			t.Fatalf("labels = %v", a.Labels)
		}
		for _, r := range a.Targets[0] {
			if contains(a.Targets[1], r) {
				t.Fatalf("%+v is both healed and damaged", r)
			}
		}
	}
	if n != 81 {
		t.Errorf("%d choices, want 81", n)
	}
}

func TestReplacingACreatureTriggersTheReplacers(t *testing.T) {
	g := pbGame(t)
	hand := func() int { return len(g.State.Players[1].Hand) }

	before := hand()
	g.enter(t, 1, "blueberry-djinni", 2) // an empty Lane: no replace
	if hand() != before {
		t.Errorf("entering an empty Lane draws nothing: %d -> %d", before, hand())
	}
	before = hand()
	g.enter(t, 1, "blueberry-djinni", 2) // replaces the first
	if hand() != before+3 {              // 2 from the Djinni, 1 from LSP, who draws for the first replace each turn
		t.Errorf("Blueberry Djinni and LSP draw 3 when replacing: %d -> %d", before, hand())
	}

	g.put(1, 0, "cow")
	before = hand()
	g.enter(t, 1, "goat", 0)
	if hand() != before+1 {
		t.Errorf("Goat draws a card when replacing: %d -> %d", before, hand())
	}

	g.put(1, 2, "cow")
	g.enter(t, 1, "future-scholar", 2)
	if g.State.ActionsLeft != 1 { // 2 paid, 1 gained
		t.Errorf("Future Scholar gains an Action: %d left, want 1", g.State.ActionsLeft)
	}

	g.put(1, 2, "cow")
	g.enter(t, 1, "brain-gooey", 2)
	if g.atkOf(1, 2) != 3 {
		t.Errorf("Brain Gooey 1 ATK +2: %d", g.atkOf(1, 2))
	}
	g.endTurn(1)
	g.endTurn(0)
	if g.atkOf(1, 2) != 1 {
		t.Errorf("only for the turn: %d", g.atkOf(1, 2))
	}

	g.put(0, 2, "cow")
	g.put(1, 2, "cow")
	g.enter(t, 1, "headphone-jerk", 2)
	if d := g.State.Players[0].Lanes[2].Creature.Damage; d != 3 {
		t.Errorf("Headphone Jerk deals 3 to the opposing Creature: %d", d)
	}
	g.put(0, 2, "cow")
	g.State.Players[1].Lanes[2].Creature = nil
	g.enter(t, 1, "headphone-jerk", 2)
	if d := g.State.Players[0].Lanes[2].Creature.Damage; d != 0 {
		t.Errorf("not replacing: %d damage", d)
	}
}

func TestGrapeDjinniPutsACardFromDiscardOnTheDeck(t *testing.T) {
	g := pbGame(t)
	g.fillDiscard(1, 1, "goat")
	card := g.State.Players[1].Discard[0]
	g.put(1, 2, "cow")
	g.enter(t, 1, "grape-djinni", 2, []TargetRef{{Kind: "discard_card", Player: 1, Card: card.UID}})
	if top := g.State.Players[1].Deck[0]; top.UID != card.UID {
		t.Errorf("top of the deck is %d, want %d", top.UID, card.UID)
	}
	g.put(1, 3, "cow")
	g.State.Players[1].Lanes[3].Creature = nil
	g.fillDiscard(1, 1, "goat")
	other := g.State.Players[1].Discard[len(g.State.Players[1].Discard)-1]
	g.enter(t, 1, "grape-djinni", 3, []TargetRef{{Kind: "discard_card", Player: 1, Card: other.UID}})
	if g.State.Players[1].Deck[0].UID == other.UID {
		t.Error("without replacing nothing is put back")
	}
}

func TestDiscardAbilitiesCostACardAndHaveLimits(t *testing.T) {
	t.Run("Dogboy once per turn", func(t *testing.T) {
		g := pbGame(t)
		g.put(1, 0, "dogboy")
		hand := len(g.State.Players[1].Hand)
		if err := g.useWithDiscard(t, 0); err != nil {
			t.Fatal(err)
		}
		if g.atkOf(1, 0) != 2 || len(g.State.Players[1].Hand) != hand-1 {
			t.Errorf("ATK %d, hand %d -> %d", g.atkOf(1, 0), hand, len(g.State.Players[1].Hand))
		}
		if err := g.useWithDiscard(t, 0); err != ErrLimit {
			t.Errorf("second use: %v", err)
		}
		g.endTurn(1)
		g.endTurn(0)
		if err := g.useWithDiscard(t, 0); err != nil {
			t.Errorf("next turn: %v", err)
		}
	})
	t.Run("Dragon Foot five times", func(t *testing.T) {
		g := pbGame(t)
		g.put(1, 0, "dragon-foot")
		for i := 0; i < 5; i++ {
			g.give(1, "cow")
			if err := g.useWithDiscard(t, 0); err != nil {
				t.Fatalf("use %d: %v", i+1, err)
			}
		}
		g.give(1, "cow")
		if err := g.useWithDiscard(t, 0); err != ErrLimit {
			t.Errorf("sixth use: %v", err)
		}
		if g.atkOf(1, 0) != 1+5 {
			t.Errorf("ATK %d, want 6", g.atkOf(1, 0))
		}
	})
	t.Run("an empty hand cannot pay", func(t *testing.T) {
		g := pbGame(t)
		g.put(1, 0, "herculeye")
		g.State.Players[1].Hand = nil
		if err := g.useWithDiscard(t, 0); err != ErrNoTarget {
			t.Errorf("%v", err)
		}
	})
	t.Run("Infinite Figure and Pink Merwitch", func(t *testing.T) {
		g := pbGame(t)
		g.put(1, 0, "infinite-figure")
		g.put(1, 1, "pink-merwitch")
		foes := []*Creature{g.put(0, 0, "cow"), g.put(0, 1, "cow"), g.put(0, 2, "cow")}
		if err := g.useWithDiscard(t, 0); err != nil {
			t.Fatal(err)
		}
		if foes[0].Damage != 1 || foes[1].Damage != 0 {
			t.Errorf("only the opposing Creature in its Lane: %d %d", foes[0].Damage, foes[1].Damage)
		}
		if err := g.useWithDiscard(t, 1); err != nil {
			t.Fatal(err)
		}
		if foes[0].Damage != 2 || foes[1].Damage != 1 || foes[2].Damage != 1 {
			t.Errorf("every opposing Creature: %d %d %d", foes[0].Damage, foes[1].Damage, foes[2].Damage)
		}
	})
	t.Run("Squatting Bald Man heals itself", func(t *testing.T) {
		g := pbGame(t)
		man := g.put(1, 0, "squatting-bald-man")
		man.Damage = 3
		if err := g.useWithDiscard(t, 0); err != nil || man.Damage != 2 {
			t.Errorf("%v, damage %d", err, man.Damage)
		}
	})
}

func TestCaveOfSolitudeProtectsUntilYourNextTurn(t *testing.T) {
	g := pbGame(t)
	g.building(1, 0, "cave-of-solitude")
	g.put(1, 0, "cow")
	g.put(0, 0, "cow")
	hand := g.State.Players[1].Hand[0].UID
	g.must(t, 1, Action{Kind: ActActivate, Lane: 0, Source: "building", Targets: [][]TargetRef{{handRef(1, hand)}}})
	if g.CanBeAttacked(1, 0) {
		t.Error("the Creature cannot be attacked")
	}
	spec := card.TargetSpec{What: "creature", Side: "opponent"}
	if len(g.candidates(spec, effCtx{p: 0}, nil)) != 0 || len(g.candidates(spec, effCtx{p: 1}, nil)) != 1 {
		t.Error("the opponent cannot target it, but its controller can target the other side")
	}
	own := card.TargetSpec{What: "creature", Side: "you"}
	if len(g.candidates(own, effCtx{p: 1}, nil)) != 1 {
		t.Error("its controller may still target it")
	}
	g.endTurn(1)
	if g.CanBeAttacked(1, 0) {
		t.Error("still protected during the opponent's turn")
	}
	g.endTurn(0)
	if !g.CanBeAttacked(1, 0) {
		t.Error("protection ends when its controller's turn starts")
	}
}

func TestFallingStarKeepsYourCreaturesFromHarm(t *testing.T) {
	g := pbGame(t)
	mine, theirs := g.put(1, 0, "cow"), g.put(0, 0, "cow")
	g.castSpell(t, "falling-star")
	g.must(t, 1, endMain)
	g.must(t, 1, Action{Kind: ActAttack, Lane: 0})
	if mine.Damage != 0 || theirs.Damage != 1 {
		t.Errorf("damage: mine %d, theirs %d, want 0 and 1", mine.Damage, theirs.Damage)
	}
	if g.State.NoCombatDamage != [2]bool{} {
		t.Error("it lasts only the turn")
	}
}

func TestPyramidiaMakesReplacingCheaper(t *testing.T) {
	g := pbGame(t)
	g.building(1, 0, "pyramidia")
	g.put(1, 0, "cow")
	g.put(1, 1, "cow")
	g.must(t, 1, Action{Kind: ActFloop, Lane: 0, Source: "building"})
	g.enter(t, 1, "angel-of-vanilla", 1) // costs 1, replaces in a Lane with no discount
	if g.State.ActionsLeft != ActionsPerTurn-1 {
		t.Errorf("no discount elsewhere: %d Actions left", g.State.ActionsLeft)
	}
	uid := g.give(1, "angel-of-vanilla")
	g.State.ActionsLeft = ActionsPerTurn
	g.must(t, 1, Action{Kind: ActPlayCard, Card: uid, Lane: 0})
	if g.State.ActionsLeft != ActionsPerTurn {
		t.Errorf("replacing in its Lane is 1 cheaper: %d Actions left", g.State.ActionsLeft)
	}
	uid = g.give(1, "angel-of-vanilla")
	g.must(t, 1, Action{Kind: ActPlayCard, Card: uid, Lane: 0})
	if g.State.ActionsLeft != ActionsPerTurn-1 {
		t.Errorf("only the once: %d Actions left", g.State.ActionsLeft)
	}
}

func TestTravelinSkeletonSwapsLanes(t *testing.T) {
	g := pbGame(t)
	a, b := g.put(1, 0, "travelin-skeleton"), g.put(1, 2, "cow")
	g.mustFail(t, 1, floop(0, []TargetRef{crRef(1, 0)}), ErrBadTarget)
	g.must(t, 1, floop(0, []TargetRef{crRef(1, 2)}))
	if g.State.Players[1].Lanes[0].Creature != b || g.State.Players[1].Lanes[2].Creature != a {
		t.Error("the Creatures swapped Lanes")
	}
	if !a.ChangedLanes || !b.ChangedLanes {
		t.Error("both changed Lanes")
	}
}

func TestVolcano(t *testing.T) {
	g := pbGame(t)
	g.building(0, 1, "blood-fortress")
	foe := g.put(0, 1, "cow")
	mine := g.put(1, 1, "cow")
	other := g.put(0, 2, "cow")
	uid := g.give(1, "volcano")
	g.State.ActionsLeft = ActionsPerTurn
	bld := TargetRef{Kind: "building", Player: 0, Lane: 1}
	g.mustFail(t, 1, Action{Kind: ActPlayCard, Card: uid, Targets: [][]TargetRef{{bld}, {crRef(0, 2)}}}, ErrBadTarget)
	g.must(t, 1, Action{Kind: ActPlayCard, Card: uid, Targets: [][]TargetRef{{bld}, {crRef(0, 1)}}})
	if g.State.Players[0].Lanes[1].Building != nil || foe.Damage != 3 || mine.Damage != 0 || other.Damage != 0 {
		t.Errorf("building %v, damage %d %d %d", g.State.Players[0].Lanes[1].Building, foe.Damage, mine.Damage, other.Damage)
	}
	if !g.State.Players[1].Lanes[1].FaceDown || g.State.Players[0].Lanes[1].FaceDown {
		t.Error("your Landscape in that Lane is flipped face down, and only yours")
	}
	// Dealing the damage is optional.
	g.building(0, 3, "blood-fortress")
	g.castSpell(t, "volcano", []TargetRef{{Kind: "building", Player: 0, Lane: 3}}, []TargetRef{})
	if g.State.Players[0].Lanes[3].Building != nil || !g.State.Players[1].Lanes[3].FaceDown {
		t.Error("it works without choosing a Creature")
	}
}

func TestPantsOfAwesomeMovesToAnEmptyBluePlains(t *testing.T) {
	g := pbGame(t)
	cr := g.put(1, 0, "cow")
	g.State.Players[1].Lanes[3].Landscape = card.UselessSwamp
	uid := g.give(1, "pants-of-awesome")
	g.State.ActionsLeft = ActionsPerTurn
	g.mustFail(t, 1, Action{Kind: ActPlayCard, Card: uid, Targets: [][]TargetRef{{crRef(1, 0)}, {laneRef(1, 3)}}}, ErrBadTarget) // a Useless Swamp
	hand := len(g.State.Players[1].Hand)
	g.must(t, 1, Action{Kind: ActPlayCard, Card: uid, Targets: [][]TargetRef{{crRef(1, 0)}, {laneRef(1, 2)}}})
	if g.State.Players[1].Lanes[2].Creature != cr || g.State.Players[1].Lanes[0].Creature != nil {
		t.Error("the Creature moved to the Blue Plains")
	}
	if len(g.State.Players[1].Hand) != hand-1+1 {
		t.Errorf("hand %d -> %d: the spell is played and a card drawn", hand, len(g.State.Players[1].Hand))
	}
}

func TestSkeletalHandMakesYourOpponentDiscardPerSpell(t *testing.T) {
	g := pbGame(t)
	g.put(1, 0, "skeletal-hand")
	pl := &g.State.Players[1]
	pl.Deck = append([]Instance{g.newInstance("strength-crystal", 1), g.newInstance("cow", 1), g.newInstance("volcano", 1)}, pl.Deck...)
	g.must(t, 1, floop(0))
	ch := g.State.Choice
	if ch == nil || ch.Player != 0 || ch.Kind != ChoiceDiscard || ch.Count != 2 {
		t.Fatalf("the opponent discards one card per Spell: %+v", ch)
	}
	if len(pl.Discard) < 3 {
		t.Errorf("three cards were discarded: %d", len(pl.Discard))
	}
}

func TestNightTowerNeedsAnEmptyOpposingLane(t *testing.T) {
	g := pbGame(t)
	g.building(1, 0, "night-tower")
	g.building(1, 1, "night-tower")
	g.put(0, 1, "cow")
	g.must(t, 1, Action{Kind: ActFloop, Lane: 0, Source: "building"})
	if ch := g.State.Choice; ch == nil || ch.Player != 0 || ch.Count != 1 {
		t.Fatalf("an empty Lane makes them discard: %+v", ch)
	}
	g.discardChoices(t, 0, 1)
	g.must(t, 1, Action{Kind: ActFloop, Lane: 1, Source: "building"})
	if g.State.Choice != nil {
		t.Errorf("a Creature opposite makes it do nothing: %+v", g.State.Choice)
	}
}

func TestSteakchopWantsDiscardsOrDies(t *testing.T) {
	start := func(t *testing.T) *Game {
		g := pbGame(t)
		g.put(1, 0, "steakchop")
		g.State.Active = 0
		g.endTurn(0)
		return g
	}
	t.Run("discard two", func(t *testing.T) {
		g := start(t)
		ch := g.State.Choice
		if ch == nil || ch.Kind != ChoiceUpkeep || ch.Player != 1 || ch.Count != 2 {
			t.Fatalf("choice = %+v", ch)
		}
		hand := len(g.State.Players[1].Hand)
		if got := len(g.LegalActions(1)); got != hand+1 {
			t.Errorf("each card in hand, or destroy it: %d actions for %d cards", got, hand)
		}
		g.must(t, 1, Action{Kind: ActChoose, Card: g.State.Players[1].Hand[0].UID})
		if g.State.Choice == nil || g.State.Choice.Kind != ChoiceDiscard || g.State.Choice.Count != 1 {
			t.Fatalf("one more to discard: %+v", g.State.Choice)
		}
		for _, a := range g.LegalActions(1) {
			if a.Card == 0 {
				t.Error("having started, it cannot be destroyed instead")
			}
		}
		g.must(t, 1, Action{Kind: ActChoose, Card: g.State.Players[1].Hand[0].UID})
		if g.State.Choice != nil || g.State.Players[1].Lanes[0].Creature == nil || len(g.State.Players[1].Hand) != hand-2 {
			t.Errorf("Steakchop stays: choice %v, hand %d", g.State.Choice, len(g.State.Players[1].Hand))
		}
	})
	t.Run("destroy it", func(t *testing.T) {
		g := start(t)
		g.must(t, 1, Action{Kind: ActChoose})
		if g.State.Players[1].Lanes[0].Creature != nil || g.State.Choice != nil {
			t.Error("Steakchop is destroyed")
		}
		if d := g.State.Players[1].Discard; len(d) == 0 || d[len(d)-1].CardID != "steakchop" {
			t.Error("it goes to its owner's discard pile")
		}
	})
	t.Run("too few cards", func(t *testing.T) {
		g := pbGame(t)
		g.put(1, 0, "steakchop")
		g.State.Players[1].Hand = nil
		g.State.Players[1].Deck = g.State.Players[1].Deck[:1] // the free draw leaves one card
		g.State.Active = 0
		g.endTurn(0)
		if g.State.Choice != nil || g.State.Players[1].Lanes[0].Creature != nil {
			t.Errorf("with one card Steakchop is simply destroyed: %+v", g.State.Choice)
		}
	})
}

func TestSnakemintAndTheStartOfTurnHealers(t *testing.T) {
	g := pbGame(t)
	g.put(1, 0, "snakemint")
	g.State.Players[1].HP = 20
	g.must(t, 1, endMain)
	g.must(t, 1, Action{Kind: ActAttack, Lane: 0}) // 1 ATK at an empty Lane
	if g.State.Players[0].HP != StartingHP-1 || g.State.Players[1].HP != 21 {
		t.Errorf("HP %d / %d", g.State.Players[0].HP, g.State.Players[1].HP)
	}

	g = pbGame(t)
	g.put(1, 0, "rainbow-eyebat")
	g.State.Players[1].Lanes[3].Landscape = card.UselessSwamp
	g.State.Players[1].HP = 10
	g.State.Active = 0
	g.endTurn(0)
	if g.State.Players[1].HP != 13 { // NiceLands, Blue Plains, Useless Swamp
		t.Errorf("heals 1 per Landscape type: HP %d, want 13", g.State.Players[1].HP)
	}

	g = pbGame(t)
	rex := g.put(1, 0, "niceasaurus-rex")
	hand := len(g.State.Players[1].Hand)
	g.State.Active = 0
	g.endTurn(0)
	if len(g.State.Players[1].Hand) != hand+1 {
		t.Errorf("undamaged: just the turn's draw, hand %d -> %d", hand, len(g.State.Players[1].Hand))
	}
	rex.Damage = 1
	hand = len(g.State.Players[1].Hand)
	g.endTurn(1)
	g.endTurn(0)
	if len(g.State.Players[1].Hand) != hand+2 {
		t.Errorf("damaged: an extra card, hand %d -> %d", hand, len(g.State.Players[1].Hand))
	}
}

func TestDiscardAndStatSpells(t *testing.T) {
	g := pbGame(t)
	g.State.Players[1].Lanes[3].Landscape = card.UselessSwamp // Toilet of Doom
	fat := g.put(1, 0, "fatapillar")                          // 0/3, +2 ATK per 5 in the discard pile
	g.building(1, 1, "mausoleum")
	mate := g.put(1, 1, "cow") // 1/5, +1 DEF per 5
	g.fillDiscard(1, 10, "goat")
	if g.atkOf(1, 0) != 4 || g.defOf(1, 1) != 7 {
		t.Fatalf("Fatapillar %d, Cow with Mausoleum DEF %d", g.atkOf(1, 0), g.defOf(1, 1))
	}
	g.castSpell(t, "toilet-of-doom", []TargetRef{crRef(1, 1)})
	if g.atkOf(1, 1) != 1+2 {
		t.Errorf("Toilet of Doom: %d", g.atkOf(1, 1))
	}
	g.State.SpellsThisTurn = 1
	g.castSpell(t, "subliminal-strength", []TargetRef{crRef(1, 0)})
	if fat.TempATK != 4 { // two Spells including this one
		t.Errorf("Subliminal Strength after one other Spell: +%d, want +4", fat.TempATK)
	}
	mate.Damage = 2
	g.castSpell(t, "ring-of-fluffy", []TargetRef{crRef(1, 1)})
	if mate.TempATK != 2+2 {
		t.Errorf("Ring of Fluffy adds the damage: TempATK %d", mate.TempATK)
	}
}

func TestPiestormBriefPowerAndStrengthCrystal(t *testing.T) {
	g := pbGame(t)
	fresh, hurt := g.put(1, 0, "cow"), g.put(1, 1, "cow")
	hurt.Damage = 1
	g.castSpell(t, "piestorm")
	if fresh.TempATK != 2 || hurt.TempATK != 0 {
		t.Errorf("only undamaged Creatures: %d %d", fresh.TempATK, hurt.TempATK)
	}
	swamp := g.put(1, 3, "dragon-foot") // a Useless Swamp Creature
	uid := g.give(1, "brief-power")
	if err := g.Validate(1, Action{Kind: ActPlayCard, Card: uid, Targets: [][]TargetRef{{crRef(1, 0)}}}); err == nil {
		t.Error("Brief Power targets a Useless Swamp Creature only")
	}
	g.must(t, 1, Action{Kind: ActPlayCard, Card: uid, Targets: [][]TargetRef{{crRef(1, 3)}}})
	if swamp.TempATK != 2 {
		t.Errorf("Brief Power: %d", swamp.TempATK)
	}
	hand := len(g.State.Players[1].Hand)
	g.castSpell(t, "strength-crystal")
	if len(g.State.Players[1].Hand) != hand+5 {
		t.Errorf("draws 5: %d -> %d", hand, len(g.State.Players[1].Hand))
	}
}

func TestFairyShepherdProtectsAdjacentNiceLandsCreatures(t *testing.T) {
	g := pbGame(t)
	g.put(1, 1, "fairy-shepherd")
	g.put(1, 0, "pieclops") // 2/7 NiceLands
	g.put(1, 2, "cow")      // 1/5 Rainbow
	if g.defOf(1, 0) != 9 || g.defOf(1, 2) != 5 {
		t.Errorf("DEF %d %d, want 9 and 5", g.defOf(1, 0), g.defOf(1, 2))
	}
}
