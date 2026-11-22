package engine

import (
	"slices"
	"testing"

	"cardwars/internal/card"
)

// fernGame is Fern's Deck (player 0: Nice Lands in Lanes 0 and 1, Cornfield in 2 and 3) against Finn's Deck
// (player 1), in player 0's main phase on turn 3 with both hands empty and 2 Actions.
func fernGame(t *testing.T) *Game {
	t.Helper()
	realGame(t)
	ls := [NumLanes]card.Landscape{card.NiceLands, card.NiceLands, card.Cornfield, card.Cornfield}
	g, err := NewGame(realCat, Config{Seed: 1, First: 0}, [2]DeckSpec{
		{Landscapes: ls, Cards: realCat.Deck("Flame Princess Vs Fern", "Fern's Deck"), Hero: "fern"},
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

// playAt plays id from player 0's hand into a Lane and returns what it cost in Actions.
func (g *Game) playAt(t *testing.T, id string, lane int, targets ...[]TargetRef) int {
	t.Helper()
	before := g.State.ActionsLeft
	g.must(t, 0, Action{Kind: ActPlayCard, Card: g.give(0, id), Lane: lane, Targets: targets})
	return before - g.State.ActionsLeft
}

func TestFernsDeckIsFullyModelled(t *testing.T) {
	g := fernGame(t)
	for _, id := range realCat.Deck("Flame Princess Vs Fern", "Fern's Deck") {
		if !realCat[id].Implemented() {
			t.Errorf("%s is not modelled", id)
		}
	}
	if g.State.Players[0].Hero.CardID != "fern" || !realCat["fern"].Implemented() {
		t.Error("Fern's deck did not come with a modelled Fern")
	}
	for id, feat := range map[string]card.Feat{"adored-cobnoblin": card.Gobble, "treasured-cobnoblin": card.Gobble,
		"gracious-armourdillo": card.Whump, "valiant-armourdillo": card.Whump, "refined-armourdillo": ""} {
		if realCat[id].Feat != feat {
			t.Errorf("%s has feat %q, want %q", id, realCat[id].Feat, feat)
		}
	}
}

func TestAReplacedGobbleCreatureGoesUnderTheOneThatReplacedIt(t *testing.T) {
	g := fernGame(t)
	old := g.put(0, 0, "adored-cobnoblin") // 1/4
	old.Damage, old.Tokens = 1, 2
	oldUID := old.UID
	g.playAt(t, "darling-cobnoblin", 0) // 2/3
	cr := g.State.Players[0].Lanes[0].Creature
	if len(cr.Gobbled) != 1 || cr.Gobbled[0].UID != oldUID {
		t.Fatalf("underneath: %v", cr.Gobbled)
	}
	if g.atk(pos{0, 0}) != 2+1+2 || g.def(pos{0, 0}) != 3+4 || cr.Damage != 1 || cr.Tokens != 2 {
		t.Errorf("atk %d def %d damage %d tokens %d", g.atk(pos{0, 0}), g.def(pos{0, 0}), cr.Damage, cr.Tokens)
	}
	if len(g.State.Players[0].Discard) != 0 {
		t.Errorf("the Gobble Creature was discarded: %v", g.State.Players[0].Discard)
	}
	// Replacing one that has no Gobble discards all of it, with what lay underneath.
	g.State.ActionsLeft = 2
	g.playAt(t, "rhinoceros", 0)
	if got := len(g.State.Players[0].Discard); got != 2 {
		t.Errorf("discard pile %d cards, want the Cobnoblin on top and the one it gobbled", got)
	}
	if rh := g.State.Players[0].Lanes[0].Creature; len(rh.Gobbled) != 0 || rh.Damage != 0 {
		t.Errorf("a Creature without Gobble took %v and %d damage", rh.Gobbled, rh.Damage)
	}
}

func TestGobbleStacksGrowAndGoTogether(t *testing.T) {
	g := fernGame(t)
	g.put(0, 0, "treasured-cobnoblin") // 1/4
	g.State.ActionsLeft = 9
	g.playAt(t, "precious-cobnoblin", 0) // 2/3 with Gobble
	g.playAt(t, "adored-cobnoblin", 0)   // 1/4 with Gobble
	cr := g.State.Players[0].Lanes[0].Creature
	if len(cr.Gobbled) != 2 || g.atk(pos{0, 0}) != 1+2+1 || g.def(pos{0, 0}) != 4+3+4 {
		t.Fatalf("stack %v atk %d def %d", cr.Gobbled, g.atk(pos{0, 0}), g.def(pos{0, 0}))
	}
	g.destroyAt(pos{0, 0})
	if got := len(g.State.Players[0].Discard); got != 3 {
		t.Errorf("a stack of three left %d cards in the discard pile", got)
	}
}

func TestWhumpCostsLessByTheCostOfWhatItReplaces(t *testing.T) {
	g := fernGame(t)
	g.nice()
	g.State.ActionsLeft = 3
	if got := g.playAt(t, "courageous-armourdillo", 3); got != 3 { // empty Lane: full price
		t.Errorf("into an empty Lane it cost %d Actions", got)
	}
	g.State.ActionsLeft = 2
	g.put(0, 0, "gallant-armourdillo") // cost 2
	if got := g.playAt(t, "courageous-armourdillo", 0); got != 1 {
		t.Errorf("over a cost 2 Creature it cost %d Actions, want 1", got)
	}
	// It needs the Landscapes of the reduced cost, and no more.
	g = fernGame(t)
	g.State.ActionsLeft = 5
	g.State.Players[0].Lanes[1].FaceDown, g.State.Players[0].Lanes[0].FaceDown = true, true
	g.put(0, 3, "gallant-armourdillo")
	g.State.Players[0].Lanes[2].Landscape = card.NiceLands
	g.State.Players[0].Lanes[3].Landscape = card.NiceLands
	g.castFailsPlay(t, "gracious-armourdillo", 0, ErrLandscapes) // cost 4 with 2 Nice Lands
	g.playAt(t, "gracious-armourdillo", 3)                       // 4-2 = 2
}

func (g *Game) castFailsPlay(t *testing.T, id string, lane int, want error) {
	t.Helper()
	g.mustFail(t, 0, Action{Kind: ActPlayCard, Card: g.give(0, id), Lane: lane}, want)
}

// nice turns all four of player 0's Landscapes into Nice Lands.
func (g *Game) nice() {
	for i := range g.State.Players[0].Lanes {
		g.State.Players[0].Lanes[i].Landscape = card.NiceLands
	}
}

func TestPolishedArmourdilloMakesOtherWhumpCreaturesCostLess(t *testing.T) {
	g := fernGame(t)
	g.nice()
	g.State.ActionsLeft = 9
	if got := g.playAt(t, "courageous-armourdillo", 3); got != 3 {
		t.Fatalf("cost %d", got)
	}
	g.put(0, 0, "polished-armourdillo")
	if got := g.playAt(t, "gracious-armourdillo", 2); got != 3 { // 4 less 1
		t.Errorf("with a Polished Armourdillo it cost %d, want 3", got)
	}
	if got := g.playAt(t, "gallant-armourdillo", 1); got != 2 { // no Whump: unchanged
		t.Errorf("a Creature without Whump cost %d, want 2", got)
	}
}

func TestFernMakesTheNextCostOneCreatureFreeAfterAReplace(t *testing.T) {
	g := fernGame(t)
	g.put(0, 0, "rhinoceros")
	g.playAt(t, "darling-cobnoblin", 0) // replaces
	if !g.State.FreeCostSet {
		t.Fatal("replacing a Creature did nothing")
	}
	if got := g.playAt(t, "treasured-cobnoblin", 2); got != 0 {
		t.Errorf("the next cost 1 Creature cost %d Actions", got)
	}
	if got := g.playAt(t, "treasured-cobnoblin", 3); got != 1 {
		t.Errorf("the one after cost %d Actions", got)
	}
	// Only once a turn, and a Creature that costs something else does not use it up.
	g = fernGame(t)
	g.State.ActionsLeft = 9
	g.put(0, 0, "rhinoceros")
	g.put(0, 1, "rhinoceros")
	g.playAt(t, "darling-cobnoblin", 0)
	g.playAt(t, "gallant-armourdillo", 3) // cost 2, empty Lane: still waiting
	if !g.State.FreeCostSet {
		t.Error("a cost 2 Creature used the free play up")
	}
	g.State.FreeCostSet = false
	g.playAt(t, "darling-cobnoblin", 1) // a second replace in the turn
	if g.State.FreeCostSet {
		t.Error("Fern fired twice in one turn")
	}
}

func TestEloquentArmourdilloGetsATokenOnlyWhenItReplaces(t *testing.T) {
	g := fernGame(t)
	g.playAt(t, "eloquent-armourdillo", 3)
	if cr := g.State.Players[0].Lanes[3].Creature; cr.Tokens != 0 || g.atk(pos{0, 3}) != 2 {
		t.Errorf("tokens %d", cr.Tokens)
	}
	g.State.ActionsLeft = 2
	g.playAt(t, "eloquent-armourdillo", 3)
	if cr := g.State.Players[0].Lanes[3].Creature; cr.Tokens != 1 || g.atk(pos{0, 3}) != 3 {
		t.Errorf("tokens %d atk %d", cr.Tokens, g.atk(pos{0, 3}))
	}
}

func TestRhinocerosGetsATokenOnlyWhenFirst(t *testing.T) {
	g := fernGame(t)
	g.playAt(t, "rhinoceros", 3)
	if g.State.Players[0].Lanes[3].Creature.Tokens != 1 {
		t.Error("the first card played should have got a token")
	}
	g.playAt(t, "rhinoceros", 2)
	if g.State.Players[0].Lanes[2].Creature.Tokens != 0 {
		t.Error("the second card played got a token")
	}
}

func (g *Game) floopAt(t *testing.T, lane int, targets ...[]TargetRef) []Event {
	t.Helper()
	return g.must(t, 0, Action{Kind: ActFloop, Lane: lane, Targets: targets})
}

func TestDarlingCobnoblinPlacesATokenOnAFriendlyCreature(t *testing.T) {
	g := fernGame(t)
	g.put(0, 0, "darling-cobnoblin")
	friend := g.put(0, 1, "the-pig")
	foe := g.put(1, 1, "the-pig")
	g.mustFail(t, 0, Action{Kind: ActFloop, Lane: 0, Targets: [][]TargetRef{{creatureRef(1, 1)}}}, ErrBadTarget)
	g.floopAt(t, 0, []TargetRef{creatureRef(0, 1)})
	if friend.Tokens != 1 || foe.Tokens != 0 || g.atk(pos{0, 1}) != 2 {
		t.Errorf("tokens %d/%d atk %d", friend.Tokens, foe.Tokens, g.atk(pos{0, 1}))
	}
}

func TestCherishedCobnoblinMovesOneOrTwoTokens(t *testing.T) {
	g := fernGame(t)
	g.put(0, 0, "cherished-cobnoblin")
	from, to := g.put(1, 0, "the-pig"), g.put(0, 1, "the-pig")
	from.Tokens = 3
	acts := 0
	for _, a := range g.LegalActions(0) {
		if a.Kind == ActFloop && a.Lane == 0 {
			acts++
		}
	}
	if acts == 0 {
		t.Fatal("no Floop offered")
	}
	// Two tokens come off the foe's Creature and go onto ours.
	g.floopAt(t, 0, nil, []TargetRef{creatureRef(1, 0)}, []TargetRef{creatureRef(0, 1)})
	if from.Tokens != 1 || to.Tokens != 2 {
		t.Errorf("tokens %d -> %d", from.Tokens, to.Tokens)
	}
	// Asking for two from a Creature with one moves that one.
	g.State.Players[0].Lanes[0].Creature.Status = Ready
	g.floopAt(t, 0, nil, []TargetRef{creatureRef(1, 0)}, []TargetRef{creatureRef(0, 1)})
	if from.Tokens != 0 || to.Tokens != 3 {
		t.Errorf("tokens %d -> %d", from.Tokens, to.Tokens)
	}
	// With no tokens anywhere it cannot be used.
	g.State.Players[0].Lanes[0].Creature.Status = Ready
	to.Tokens = 0
	for _, a := range g.LegalActions(0) {
		if a.Kind == ActFloop && a.Lane == 0 {
			t.Errorf("a Floop is offered with no tokens to move: %v", a)
		}
	}
}

func TestPreciousCobnoblinHealsOrRemovesAToken(t *testing.T) {
	g := fernGame(t)
	g.put(0, 0, "precious-cobnoblin")
	friend := g.put(0, 1, "wall-of-ears")
	friend.Damage, friend.Tokens = 3, 2
	g.floopAt(t, 0, []TargetRef{creatureRef(0, 1)})
	if friend.Damage != 1 || friend.Tokens != 2 {
		t.Errorf("heal: damage %d tokens %d", friend.Damage, friend.Tokens)
	}
	g.State.Players[0].Lanes[0].Creature.Status = Ready
	g.floopAt(t, 0, nil, []TargetRef{creatureRef(0, 1)})
	if friend.Damage != 1 || friend.Tokens != 1 {
		t.Errorf("token: damage %d tokens %d", friend.Damage, friend.Tokens)
	}
}

func TestAdoredCobnoblinAndTreasuredCobnoblinFloops(t *testing.T) {
	g := fernGame(t)
	g.put(0, 0, "adored-cobnoblin")
	g.put(0, 1, "treasured-cobnoblin")
	g.State.ActionsLeft = 2
	g.floopAt(t, 0)
	g.floopAt(t, 1)
	if g.handSize(0) != 1 {
		t.Errorf("hand %d", g.handSize(0))
	}
	if got := g.playAt(t, "darling-cobnoblin", 3); got != 0 {
		t.Errorf("a cost 1 Creature after Adored Cobnoblin cost %d", got)
	}
}

func TestChampionsEmblemGainsAnActionOnlyIfDamageIsLeft(t *testing.T) {
	g := fernGame(t)
	hurt := g.put(0, 0, "wall-of-ears")
	hurt.Damage = 5
	g.cast(t, 0, "champion-s-emblem", []TargetRef{creatureRef(0, 0)})
	if hurt.Damage != 2 || g.State.ActionsLeft != 2 {
		t.Errorf("damage %d actions %d", hurt.Damage, g.State.ActionsLeft)
	}
	g.State.ActionsLeft = 2
	hurt.Damage = 3
	g.cast(t, 0, "champion-s-emblem", []TargetRef{creatureRef(0, 0)})
	if hurt.Damage != 0 || g.State.ActionsLeft != 1 {
		t.Errorf("damage %d actions %d", hurt.Damage, g.State.ActionsLeft)
	}
}

func TestChivalrousArmourdilloMayHealAnotherCreatureAndDraw(t *testing.T) {
	g := fernGame(t)
	g.State.Players[0].Deck = append([]Instance{g.newInstance("the-pig", 0)}, g.State.Players[0].Deck...)
	hurt := g.put(1, 1, "wall-of-ears")
	hurt.Damage = 5
	g.playAt(t, "chivalrous-armourdillo", 3, []TargetRef{creatureRef(1, 1)})
	if hurt.Damage != 2 || g.handSize(0) != 1 {
		t.Errorf("damage %d hand %d", hurt.Damage, g.handSize(0))
	}
	// It may also decline.
	g = fernGame(t)
	g.playAt(t, "chivalrous-armourdillo", 3)
}

// attackFrom makes player 0's Creature in lane Fight, in the Fight phase of turn 3.
func (g *Game) attackFrom(t *testing.T, lane int) []Event {
	t.Helper()
	g.State.Phase = PhaseFight
	return g.must(t, 0, Action{Kind: ActAttack, Lane: lane})
}

func TestCourageousArmourdilloHealsYouAndHurtsTheOpponentWhenItDefeatsACreature(t *testing.T) {
	g := fernGame(t)
	g.State.Players[0].HP = 20
	g.put(0, 0, "courageous-armourdillo") // 4 ATK
	g.put(1, 0, "the-pig")                // 4 DEF
	g.attackFrom(t, 0)
	if g.State.Players[0].HP != 22 || g.State.Players[1].HP != 23 {
		t.Errorf("HP %d / %d", g.State.Players[0].HP, g.State.Players[1].HP)
	}
	// Beating nothing in the way is not defeating a Creature.
	g = fernGame(t)
	g.State.Players[0].HP = 20
	g.put(0, 0, "courageous-armourdillo")
	g.attackFrom(t, 0)
	if g.State.Players[0].HP != 20 || g.State.Players[1].HP != 21 {
		t.Errorf("HP %d / %d", g.State.Players[0].HP, g.State.Players[1].HP)
	}
}

func TestKangarooBatGetsATokenWhenItHitsTheOpponent(t *testing.T) {
	g := fernGame(t)
	bat := g.put(0, 0, "kangaroo-bat")
	g.attackFrom(t, 0)
	if bat.Tokens != 1 || g.atk(pos{0, 0}) != 3 {
		t.Errorf("tokens %d", bat.Tokens)
	}
}

func TestHerbalHurlersHealCreaturesByWhatTheyDeal(t *testing.T) {
	g := fernGame(t)
	g.cast(t, 0, "herbal-hurlers")
	mine := g.put(0, 0, "rhinoceros") // 3/3
	mine.Damage = 1
	g.put(1, 0, "wall-of-ears")
	g.attackFrom(t, 0)
	if mine.Damage != 0 || g.State.Players[0].Lanes[0].Creature == nil {
		t.Errorf("damage %d, want it healed by what it dealt", mine.Damage)
	}
	// It lasts only for the turn.
	g.endTurnFor(t)
	if g.State.Lifesteal[0] {
		t.Error("Herbal Hurlers lasted past the turn")
	}
}

func (g *Game) endTurnFor(t *testing.T) {
	t.Helper()
	g.endTurn(g.State.Active)
}

func TestKernelRenewalDealsTheDamageOnOneOfYourCreatures(t *testing.T) {
	g := fernGame(t)
	mine := g.put(0, 0, "wall-of-ears")
	mine.Damage = 4
	foe := g.put(1, 0, "wall-of-ears")
	g.cast(t, 0, "kernel-renewal", []TargetRef{creatureRef(1, 0)}, []TargetRef{creatureRef(0, 0)})
	if foe.Damage != 4 || mine.Damage != 4 {
		t.Errorf("damage %d / %d", foe.Damage, mine.Damage)
	}
}

func TestRagepopTotemGivesAttackAndAnAttackerDrawsWhenItDefeats(t *testing.T) {
	g := fernGame(t)
	mine := g.put(0, 0, "the-pig") // 1/4
	g.put(1, 0, "the-pig")
	g.cast(t, 0, "ragepop-totem", []TargetRef{creatureRef(0, 0)})
	if g.atk(pos{0, 0}) != 3 || !mine.Rage {
		t.Fatalf("atk %d", g.atk(pos{0, 0}))
	}
	g.State.Players[1].Lanes[0].Creature.Damage = 2 // so 3 ATK is enough
	hand := g.handSize(0)
	g.attackFrom(t, 0)
	if g.handSize(0) != hand+1 {
		t.Errorf("hand %d, want %d", g.handSize(0), hand+1)
	}
}

func TestSacredScytheDestroysACreatureAndItsOwnerHeals(t *testing.T) {
	g := fernGame(t)
	g.State.Players[1].HP = 10
	g.State.Players[0].HP = 10
	g.put(1, 2, "wall-of-ears")
	g.cast(t, 0, "sacred-scythe", []TargetRef{creatureRef(1, 2)})
	if g.State.Players[1].Lanes[2].Creature != nil || g.State.Players[1].HP != 15 || g.State.Players[0].HP != 10 {
		t.Errorf("HP %d / %d", g.State.Players[0].HP, g.State.Players[1].HP)
	}
	// Destroying your own Creature heals you, and never above 25.
	g.put(0, 1, "wall-of-ears")
	g.State.Players[0].HP = 23
	g.cast(t, 0, "sacred-scythe", []TargetRef{creatureRef(0, 1)})
	if g.State.Players[0].HP != StartingHP {
		t.Errorf("HP %d", g.State.Players[0].HP)
	}
}

func TestValiantArmourdilloDestroysTheCreatureOpposingOnlyWhenPlayedFromHand(t *testing.T) {
	g := fernGame(t)
	g.nice()
	g.State.ActionsLeft = 9
	g.put(1, 0, "wall-of-ears")
	g.put(0, 0, "rhinoceros") // replaced, so that Whump makes it affordable
	g.playAt(t, "valiant-armourdillo", 0)
	if g.State.Players[1].Lanes[0].Creature != nil {
		t.Error("the opposing Creature was not destroyed")
	}
	// Played for free from the deck by Gallant Armourdillo it does not.
	g = fernGame(t)
	g.nice()
	g.put(1, 1, "wall-of-ears")
	g.put(0, 1, "rhinoceros")
	g.State.Players[0].Deck = append([]Instance{g.newInstance("valiant-armourdillo", 0)}, g.State.Players[0].Deck...)
	g.playAt(t, "gallant-armourdillo", 1)
	if g.State.Pending == nil {
		t.Fatal("Gallant Armourdillo did not play the top card")
	}
	g.must(t, 0, Action{Kind: ActPlayPending, Lane: 1})
	if g.State.Players[1].Lanes[1].Creature == nil {
		t.Error("a Valiant Armourdillo played from the deck destroyed a Creature")
	}
}

func TestGallantArmourdilloOnlyPlaysTheTopCardWhenItReplaces(t *testing.T) {
	g := fernGame(t)
	g.playAt(t, "gallant-armourdillo", 3)
	if g.State.Pending != nil {
		t.Error("it played a card without replacing")
	}
	g.State.ActionsLeft = 2
	deck := len(g.State.Players[0].Deck)
	g.playAt(t, "gallant-armourdillo", 3)
	if g.State.Pending == nil || len(g.State.Players[0].Deck) != deck-1 {
		t.Fatalf("pending %v deck %d", g.State.Pending, len(g.State.Players[0].Deck))
	}
	for _, a := range g.LegalActions(0) {
		if a.Kind != ActPlayPending {
			t.Errorf("another action while a card waits to be played: %v", a)
		}
	}
}

func TestGentealArmourdilloMayTakeAnAttack(t *testing.T) {
	for _, name := range []string{"genteel-armourdillo", "gracious-armourdillo"} {
		g := fernGame(t)
		g.State.Active = 1
		g.State.Phase = PhaseFight
		g.State.Turn = 4
		atk := g.put(1, 0, "wall-of-ears")
		atk.Damage = 0
		g.put(0, 0, "the-pig")
		dill := g.put(0, 2, name)
		g.must(t, 1, Action{Kind: ActAttack, Lane: 0})
		ch := g.State.Choice
		if ch == nil || ch.Kind != ChoiceDefend || ch.Player != 0 || len(ch.Revealed) != 1 {
			t.Fatalf("%s: choice %+v", name, ch)
		}
		g.must(t, 0, Action{Kind: ActChoose, Card: dill.UID})
		if g.State.Players[0].Lanes[0].Creature.Damage != 0 {
			t.Errorf("%s: the Creature across was hit", name)
		}
		if g.State.Choice != nil {
			t.Errorf("%s: choice still open", name)
		}
		// Declining lets the Fight go where it was going.
		g = fernGame(t)
		g.State.Active, g.State.Phase, g.State.Turn = 1, PhaseFight, 4
		g.put(1, 0, "the-pig")
		g.put(0, 0, "the-pig")
		g.put(0, 2, name)
		g.must(t, 1, Action{Kind: ActAttack, Lane: 0})
		g.must(t, 0, Action{Kind: ActChoose})
		if g.State.Players[0].Lanes[0].Creature.Damage != 1 {
			t.Errorf("%s: declined, but the Fight went elsewhere", name)
		}
	}
}

func TestResonantSpellholdReturnsASpellOfLesserCost(t *testing.T) {
	g := fernGame(t)
	g.nice()
	g.put(0, 1, "rhinoceros")
	g.State.Players[0].Lanes[1].Building = &Building{Instance: Instance{UID: 900, CardID: "resonant-spellhold"}}
	cheap := g.newInstance("ragepop-totem", 0)    // cost 0
	dear := g.newInstance("champion-s-emblem", 0) // cost 1
	g.State.Players[0].Discard = []Instance{cheap, dear}
	g.State.ActionsLeft = 9
	g.playAt(t, "chivalrous-armourdillo", 1) // cost 1: only the cost 0 Spell is lesser
	ch := g.State.Choice
	if ch == nil || ch.Kind != ChoiceTarget || len(ch.Targets) != 1 || ch.Targets[0].Card != cheap.UID {
		t.Fatalf("choice %+v", ch)
	}
	g.must(t, 0, Action{Kind: ActChoose, Targets: [][]TargetRef{{ch.Targets[0]}}})
	if !slices.ContainsFunc(g.State.Players[0].Hand, func(in Instance) bool { return in.UID == cheap.UID }) {
		t.Error("the Spell did not come back")
	}
	// A Creature in another Lane does not set it off.
	g.playAt(t, "chivalrous-armourdillo", 2)
	if g.State.Choice != nil {
		t.Errorf("another Lane asked: %+v", g.State.Choice)
	}
}

func TestAscensionSiloDrawsOncePerTurnWhenYourCreatureLeaves(t *testing.T) {
	g := fernGame(t)
	g.put(0, 0, "wall-of-ears")
	g.State.Players[0].Lanes[0].Building = &Building{Instance: Instance{UID: 900, CardID: "ascension-silo"}}
	g.destroyAt(pos{0, 0})
	if g.handSize(0) != 1 {
		t.Fatalf("hand %d", g.handSize(0))
	}
	g.put(0, 0, "wall-of-ears")
	g.destroyAt(pos{0, 0})
	if g.handSize(0) != 1 {
		t.Errorf("it drew twice in a turn: hand %d", g.handSize(0))
	}
}

func TestLifewardTowerHealsItsLanesCreatureAtTheStartOfTheTurn(t *testing.T) {
	g := fernGame(t)
	mine := g.put(0, 0, "wall-of-ears")
	other := g.put(0, 1, "wall-of-ears")
	mine.Damage, other.Damage = 3, 3
	g.State.Players[0].Lanes[0].Building = &Building{Instance: Instance{UID: 900, CardID: "lifeward-tower"}}
	g.fireTurnStart(0)
	if mine.Damage != 2 || other.Damage != 3 {
		t.Errorf("damage %d / %d", mine.Damage, other.Damage)
	}
}

func TestMossglenRetreatOffersToRemoveATokenOrAFrozenToken(t *testing.T) {
	g := fernGame(t)
	mine := g.put(0, 0, "wall-of-ears")
	mine.Tokens = 2
	g.freezeAt(0, 3)
	g.State.Players[0].Lanes[1].Building = &Building{Instance: Instance{UID: 900, CardID: "mossglen-retreat"}}
	g.fireTurnStart(0)
	offers := 0
	for _, a := range g.LegalActions(0) {
		if a.Kind == ActActivate && a.Lane == 1 && a.Source == "building" {
			offers++
			if !a.Offer {
				t.Errorf("an offer is not marked as one: %v", a)
			}
		}
	}
	if offers != 2 {
		t.Fatalf("%d offers, want one for the Creature's token and one for the Landscape's", offers)
	}
	g.must(t, 0, Action{Kind: ActActivate, Lane: 1, Source: "building", Targets: [][]TargetRef{{creatureRef(0, 0)}, nil}})
	if mine.Tokens != 1 {
		t.Errorf("tokens %d", mine.Tokens)
	}
	// The offer is used up; the next turn's start makes a new one.
	if len(g.State.Players[0].Lanes[1].Building.Offered) != 0 {
		t.Error("the offer is still there")
	}
}

func TestRefinedArmourdilloIsFreeAfterACostTwoCreature(t *testing.T) {
	g := fernGame(t)
	if got := g.playAt(t, "refined-armourdillo", 3); got != 1 {
		t.Errorf("first it cost %d", got)
	}
	g.State.ActionsLeft = 4
	g.playAt(t, "gallant-armourdillo", 0)
	if got := g.playAt(t, "refined-armourdillo", 1); got != 0 {
		t.Errorf("after a cost 2 Creature it cost %d Actions", got)
	}
}
