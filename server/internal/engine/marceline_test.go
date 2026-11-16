package engine

import (
	"math/rand/v2"
	"slices"
	"testing"

	"cardwars/internal/card"
)

// marcGame is Marceline's Deck (player 0: Useless Swamp in Lanes 0 and 1, Nice Lands in 2 and 3) against Finn's
// Deck (player 1), in player 0's main phase on turn 3 with both hands empty.
func marcGame(t *testing.T) *Game {
	t.Helper()
	realGame(t)
	ls := [NumLanes]card.Landscape{card.UselessSwamp, card.UselessSwamp, card.NiceLands, card.NiceLands}
	g, err := NewGame(realCat, Config{Seed: 1, First: 0}, [2]DeckSpec{
		{Landscapes: ls, Cards: realCat.Deck("Ice King Vs Marceline", "Marceline's Deck"), Hero: "marceline"},
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

func (g *Game) handSize(p PlayerID) int { return len(g.State.Players[p].Hand) }

func TestMarcelinesDeckIsFullyModelled(t *testing.T) {
	g := marcGame(t)
	for _, id := range realCat.Deck("Ice King Vs Marceline", "Marceline's Deck") {
		if !realCat[id].Implemented() {
			t.Errorf("%s is not modelled", id)
		}
	}
	if g.State.Players[0].Hero.CardID != "marceline" || !realCat["marceline"].Implemented() {
		t.Error("Marceline's deck did not come with a modelled Marceline")
	}
}

func TestBanShePrincessMayRemoveATokenFromAnAdjacentLandscape(t *testing.T) {
	g := marcGame(t)
	g.freezeAt(0, 0)
	g.freezeAt(0, 3) // not adjacent to Lane 2... it is, but Lane 1 is the one we play on
	g.freezeAt(1, 1) // the opponent's does not count
	id := g.give(0, "ban-she-princess")
	g.State.ActionsLeft = 2
	g.must(t, 0, Action{Kind: ActPlayCard, Card: id, Lane: 1, Targets: [][]TargetRef{{lsRef(0, 0)}}})
	if g.frozenAt(0, 0) || !g.frozenAt(0, 3) || !g.frozenAt(1, 1) {
		t.Errorf("tokens: %v %v %v", g.frozenAt(0, 0), g.frozenAt(0, 3), g.frozenAt(1, 1))
	}
	// A token that is not adjacent cannot be chosen; choosing none is allowed.
	id = g.give(0, "ban-she-queen")
	g.State.ActionsLeft = 2
	g.mustFail(t, 0, Action{Kind: ActPlayCard, Card: id, Lane: 2, Targets: [][]TargetRef{{lsRef(0, 0)}}}, ErrBadTarget)
	g.must(t, 0, Action{Kind: ActPlayCard, Card: id, Lane: 2})
}

func TestBogBumAndBogFrogBombOnlyGoOffWhileReady(t *testing.T) {
	for _, status := range []Status{Ready, Flooped, Fought} {
		g := marcGame(t)
		g.put(0, 0, "bog-bum")
		g.put(0, 1, "bog-frog-bomb")
		g.State.Players[0].Lanes[0].Creature.Status = status
		g.State.Players[0].Lanes[1].Creature.Status = status
		foe := g.put(1, 2, "wall-of-ears")
		g.give(1, "pony")
		g.give(1, "pony")
		g.destroyAt(pos{0, 0})
		g.destroyAt(pos{0, 1})
		want := status == Ready
		if (g.handSize(1) == 1) != want || (foe.Damage == 2) != want {
			t.Errorf("status %v: opponent's hand %d, damage %d", status, g.handSize(1), foe.Damage)
		}
	}
}

func TestCottonpultReadiesItselfAtFiveDamage(t *testing.T) {
	g := marcGame(t)
	cp := g.put(0, 2, "cottonpult")
	foe := g.put(1, 2, "wall-of-ears")
	g.must(t, 0, floop(2, []TargetRef{creatureRef(1, 2)}))
	if foe.Damage != 1 || cp.Status != Flooped {
		t.Errorf("damage %d, status %v", foe.Damage, cp.Status)
	}
	cp.Status, cp.Damage = Ready, 5
	g.must(t, 0, floop(2, []TargetRef{creatureRef(1, 2)}))
	if cp.Status != Ready || cp.Damage != 4 || foe.Damage != 2 {
		t.Errorf("status %v, damage %d and %d", cp.Status, cp.Damage, foe.Damage)
	}
}

func TestDrStuffensteinHealsOneAdjacentCreatureOrAllOfYours(t *testing.T) {
	g := marcGame(t)
	doc := g.put(0, 2, "dr-stuffenstein")
	next := g.put(0, 3, "wall-of-ears")
	far := g.put(0, 0, "wall-of-ears")
	next.Damage, far.Damage = 4, 4
	g.mustFail(t, 0, floop(2, []TargetRef{creatureRef(0, 0)}), ErrBadTarget)
	g.must(t, 0, floop(2, []TargetRef{creatureRef(0, 3)}))
	if next.Damage != 2 || far.Damage != 4 {
		t.Errorf("damage %d and %d", next.Damage, far.Damage)
	}
	doc.Status, doc.Damage = Ready, 5
	g.mustFail(t, 0, floop(2, []TargetRef{creatureRef(0, 3)}), ErrCondition)
	g.must(t, 0, Action{Kind: ActFloop, Lane: 2, Index: 1})
	if next.Damage != 0 || far.Damage != 2 || doc.Damage != 3 {
		t.Errorf("damage %d, %d and %d, want 0, 2 and 3", next.Damage, far.Damage, doc.Damage)
	}
}

func TestFlySwatterTakesAZeroCostCardOrShowsTheHand(t *testing.T) {
	g := marcGame(t)
	g.put(0, 2, "fly-swatter")
	g.give(1, "wall-of-ears") // cost 1
	g.give(1, "wall-of-ears")
	ev := g.must(t, 0, floop(2))
	reveals := 0
	for _, e := range ev {
		if e.Kind == EvReveal && e.Player == 1 {
			reveals++
		}
	}
	if g.State.Choice != nil || g.handSize(1) != 2 || reveals != 2 {
		t.Fatalf("no 0-cost card: choice %v, hand %d, %d cards shown", g.State.Choice, g.handSize(1), reveals)
	}

	g = marcGame(t)
	g.put(0, 2, "fly-swatter")
	zero := g.give(1, "spike-icicle") // cost 0
	g.give(1, "wall-of-ears")
	g.must(t, 0, floop(2))
	if g.State.Choice != nil || g.handSize(1) != 1 || len(g.State.Players[1].Discard) != 1 || g.State.Players[1].Discard[0].UID != zero {
		t.Errorf("one 0-cost card should go at once: choice %v hand %d", g.State.Choice, g.handSize(1))
	}

	g = marcGame(t)
	g.put(0, 2, "fly-swatter")
	a := g.give(1, "spike-icicle")
	b := g.give(1, "spike-icicle")
	pony := g.give(1, "wall-of-ears")
	g.must(t, 0, floop(2))
	ch := g.State.Choice
	if ch == nil || ch.Kind != ChoiceDiscard || ch.Player != 1 || ch.OnlyCost == nil {
		t.Fatalf("choice %+v", ch)
	}
	if got := g.LegalActions(1); len(got) != 2 {
		t.Errorf("legal %v: the two Spike Icicles", got)
	}
	g.mustFail(t, 1, Action{Kind: ActChoose, Card: pony}, ErrBadTarget)
	g.must(t, 1, Action{Kind: ActChoose, Card: a})
	if g.handSize(1) != 2 || !slices.ContainsFunc(g.State.Players[1].Hand, func(in Instance) bool { return in.UID == b }) {
		t.Errorf("hand %v", g.State.Players[1].Hand)
	}
}

func TestFuriousChickensPaperTigerAndMallardCountDamage(t *testing.T) {
	g := marcGame(t)
	hen := g.put(0, 2, "furious-hen") // 0 ATK
	tiger := g.put(0, 3, "papercut-tiger")
	hen.Damage = 3
	if g.atkOf(0, 2) != 3 {
		t.Errorf("hen ATK %d", g.atkOf(0, 2))
	}
	for dmg, want := range map[int]int{0: 2, 4: 2, 5: 8, 6: 3} { // 2, +5 at exactly 5, and Marceline's +1 from 5 on
		tiger.Damage = dmg
		if got := g.atkOf(0, 3); got != want {
			t.Errorf("Papercut Tiger with %d damage: ATK %d, want %d", dmg, got, want)
		}
	}
	mall := g.put(0, 0, "music-mallard")
	g.State.Players[0].Deck = append([]Instance{{UID: 801, CardID: "pony", Owner: 0}, {UID: 802, CardID: "pony", Owner: 0}, {UID: 803, CardID: "pony", Owner: 0}}, g.State.Players[0].Deck...)
	g.must(t, 0, floop(0))
	if g.handSize(0) != 1 {
		t.Errorf("drew %d", g.handSize(0))
	}
	mall.Status, mall.Damage = Ready, 5
	g.must(t, 0, floop(0))
	if g.handSize(0) != 3 {
		t.Errorf("hand %d, want 3", g.handSize(0))
	}
}

func TestManWitchAndFuneralHomeReactToTheOpponentDiscarding(t *testing.T) {
	g := marcGame(t)
	g.put(0, 0, "man-witch")
	g.Put(0, 1, "funeral-home")
	mine := g.put(0, 1, "wall-of-ears")
	g.put(0, 2, "man-witch")
	other := g.put(0, 3, "wall-of-ears")
	hp := g.State.Players[1].HP
	uid := g.give(1, "pony")
	g.discardFromHand(1, uid)
	if g.State.Players[1].HP != hp-2 || mine.TempATK != 1 || other.TempATK != 0 {
		t.Errorf("HP %d (want %d), ATK boosts %d and %d", g.State.Players[1].HP, hp-2, mine.TempATK, other.TempATK)
	}
	// Your own discards do nothing.
	hp0 := g.State.Players[0].HP
	g.discardFromHand(0, g.give(0, "pony"))
	if g.State.Players[0].HP != hp0 || mine.TempATK != 1 || g.State.Players[1].HP != hp-2 {
		t.Error("a discard by its controller set the triggers off")
	}
	// The opponent paying for a thaw counts too.
	g.freezeAt(1, 0)
	g.State.ActionsLeft = 2
	g.State.Active = 1
	g.must(t, 1, Action{Kind: ActThaw, Lane: 0, Card: g.give(1, "pony")})
	if g.State.Players[1].HP != hp-4 {
		t.Errorf("HP %d, want %d", g.State.Players[1].HP, hp-4)
	}
}

func TestUnicyclopsMakesEachPlayerDrawThenDiscard(t *testing.T) {
	g := marcGame(t)
	g.put(0, 0, "unicyclops")
	startOfTurn(t, g, 0)
	if g.State.Choice == nil || g.State.Choice.Kind != ChoiceDiscard {
		t.Fatalf("choice %+v", g.State.Choice)
	}
	// Both drew, so the hands are the one drawn for the turn plus this one; each discards one.
	n0, n1 := g.handSize(0), g.handSize(1)
	for g.State.Choice != nil {
		p := g.State.Choice.Player
		g.must(t, p, Action{Kind: ActChoose, Card: g.State.Players[p].Hand[0].UID})
	}
	if g.handSize(0) != n0-1 && g.handSize(1) != n1-1 {
		t.Errorf("hands %d, %d (were %d, %d)", g.handSize(0), g.handSize(1), n0, n1)
	}
}

func TestAutoPluckerHealsOrHurtsYourCreatureInItsLane(t *testing.T) {
	g := marcGame(t)
	g.Put(0, 2, "auto-plucker")
	mine := g.put(0, 2, "wall-of-ears")
	mine.Damage = 3
	bp := func(slot int) Action {
		tg := make([][]TargetRef, 2)
		tg[slot] = []TargetRef{creatureRef(0, 2)}
		return Action{Kind: ActFloop, Lane: 2, Source: "building", Targets: tg}
	}
	g.must(t, 0, bp(0))
	if mine.Damage != 2 {
		t.Errorf("heal: damage %d", mine.Damage)
	}
	g.State.Players[0].Lanes[2].Building.Status = Ready
	g.must(t, 0, bp(1))
	if mine.Damage != 3 {
		t.Errorf("hurt: damage %d", mine.Damage)
	}
}

func TestCardboardMansionCostsLessWithABigDiscardPile(t *testing.T) {
	g := marcGame(t)
	c := realCat["cardboard-mansion"]
	if a, _ := g.costFor(0, c, 0); a != 5 {
		t.Errorf("cost %d, want 5", a)
	}
	for i := 0; i < 12; i++ {
		g.State.Players[0].Discard = append(g.State.Players[0].Discard, Instance{UID: 900 + i, CardID: "pony", Owner: 0})
	}
	if a, l := g.costFor(0, c, 0); a != 3 || l != 3 {
		t.Errorf("cost %d and %d, want 3", a, l)
	}
	g.Put(0, 0, "cardboard-mansion")
	g.must(t, 0, Action{Kind: ActFloop, Lane: 0, Source: "building"})
	if g.State.ActionsLeft != 3 {
		t.Errorf("Actions %d, want 3", g.State.ActionsLeft)
	}
}

func TestTheBigHenHouseHurtsYourCreatureByOneOrTwoAndHealsYou(t *testing.T) {
	g := marcGame(t)
	g.Put(0, 2, "the-big-hen-house")
	mine := g.put(0, 2, "wall-of-ears")
	g.State.Players[0].HP = 20
	startOfTurn(t, g, 0)
	ch := g.State.Choice
	if ch == nil || ch.Kind != ChoiceAmount || !slices.Equal(ch.Options, []int{1, 2}) || ch.Player != 0 {
		t.Fatalf("choice %+v", ch)
	}
	g.mustFail(t, 0, Action{Kind: ActChoose, Card: 3}, ErrBadTarget)
	g.must(t, 0, Action{Kind: ActChoose, Card: 2})
	if mine.Damage != 2 || g.State.Players[0].HP != 21 {
		t.Errorf("damage %d, HP %d", mine.Damage, g.State.Players[0].HP)
	}
	// Without a Creature there is nothing to hurt and nothing to choose, but you still heal; and never above 25.
	g = marcGame(t)
	g.Put(0, 2, "the-big-hen-house")
	g.State.Players[0].HP = 25
	startOfTurn(t, g, 0)
	if g.State.Choice != nil || g.State.Players[0].HP != 25 {
		t.Errorf("choice %v, HP %d", g.State.Choice, g.State.Players[0].HP)
	}
}

func TestBlackHolePendantCountsTheOpponentsLandscapeTypes(t *testing.T) {
	g := marcGame(t)
	mine := g.put(0, 2, "wall-of-ears")
	g.State.Players[1].Lanes[1].Landscape = card.IcyLands
	g.State.Players[1].Lanes[2].Landscape = card.SandyLands
	g.cast(t, 0, "black-hole-pendant", []TargetRef{creatureRef(0, 2)})
	if mine.TempATK != 3 { // Cornfield, Icy Lands and Sandy Lands
		t.Errorf("+%d ATK, want 3", mine.TempATK)
	}
}

func TestBloodBathAndBumsAwayLetTheOpponentPayInstead(t *testing.T) {
	g := marcGame(t)
	hp := g.State.Players[1].HP
	cheap := g.give(1, "pony")
	g.cast(t, 0, "blood-bath")
	if g.State.Choice != nil || g.State.Players[1].HP != hp-5 {
		t.Errorf("no card of cost 2: choice %v, HP %d", g.State.Choice, g.State.Players[1].HP)
	}
	_ = cheap

	g = marcGame(t)
	dear := g.give(1, "drooling-dude") // cost 2 or more
	g.give(1, "pony")
	g.State.Players[1].Hand[0].CardID = "drooling-dude"
	g.cast(t, 0, "blood-bath")
	ch := g.State.Choice
	if ch == nil || ch.Kind != ChoiceRansom || ch.Player != 1 {
		t.Fatalf("choice %+v", ch)
	}
	got := g.LegalActions(1)
	for _, a := range got {
		if a.Card != 0 && a.Card != dear {
			t.Errorf("offered %v", a)
		}
	}
	g.must(t, 1, Action{Kind: ActChoose, Card: dear})
	if g.State.Players[1].HP != hp || g.handSize(1) != 1 {
		t.Errorf("paid with the card: HP %d, hand %d", g.State.Players[1].HP, g.handSize(1))
	}
	g.State.ActionsLeft = 2
	g.State.Players[1].Hand[0].CardID = "drooling-dude"
	g.cast(t, 0, "blood-bath")
	g.must(t, 1, Action{Kind: ActChoose})
	if g.State.Players[1].HP != hp-5 || g.handSize(1) != 1 {
		t.Errorf("declined: HP %d", g.State.Players[1].HP)
	}

	g = marcGame(t)
	a, b := g.put(1, 0, "wall-of-ears"), g.put(1, 3, "wall-of-ears")
	mine := g.put(0, 0, "wall-of-ears")
	g.give(1, "pony")
	g.cast(t, 0, "bums-away")
	g.must(t, 1, Action{Kind: ActChoose})
	if a.Damage != 2 || b.Damage != 2 || mine.Damage != 0 {
		t.Errorf("damage %d %d %d", a.Damage, b.Damage, mine.Damage)
	}
	g = marcGame(t)
	c := g.put(1, 0, "wall-of-ears")
	g.cast(t, 0, "bums-away") // an empty hand cannot pay
	if c.Damage != 2 || g.State.Choice != nil {
		t.Errorf("damage %d, choice %v", c.Damage, g.State.Choice)
	}
}

func TestBloodTransfusionRemovesACardFromTheGameAndHealsByItsCost(t *testing.T) {
	g := marcGame(t)
	hurt := g.put(0, 2, "wall-of-ears")
	hurt.Damage = 5
	in := Instance{UID: 950, CardID: "drooling-dude", Owner: 1}
	g.State.Players[1].Discard = append(g.State.Players[1].Discard, in)
	g.cast(t, 0, "blood-transfusion", []TargetRef{{Kind: "discard_card", Player: 1, Card: 950}}, []TargetRef{creatureRef(0, 2)})
	cost := *realCat["drooling-dude"].Cost
	if len(g.State.Players[1].Discard) != 0 || len(g.State.Players[1].Removed) != 1 || hurt.Damage != 5-cost {
		t.Errorf("discard %d, removed %d, damage %d (cost %d)", len(g.State.Players[1].Discard), len(g.State.Players[1].Removed), hurt.Damage, cost)
	}
	if v := g.Instances()[950]; v != "drooling-dude" {
		t.Errorf("a removed card should still be nameable, got %q", v)
	}
}

func TestPentaidHealsExactlyFiveOrNothing(t *testing.T) {
	g := marcGame(t)
	mine := g.put(0, 2, "wall-of-ears")
	mine.Damage = 4
	g.castFails(t, 0, "pentaid", ErrNoTarget, []TargetRef{creatureRef(0, 2)})
	mine.Damage = 7
	g.cast(t, 0, "pentaid", []TargetRef{creatureRef(0, 2)})
	if mine.Damage != 2 {
		t.Errorf("damage %d, want 2", mine.Damage)
	}
}

func TestMarcelineGivesDamagedCreaturesPlusOneAttack(t *testing.T) {
	g := marcGame(t)
	a := g.put(0, 2, "wall-of-ears")
	b := g.put(0, 3, "wall-of-ears")
	foe := g.put(1, 2, "wall-of-ears")
	a.Damage, foe.Damage = 5, 6
	base := g.atkOf(0, 3)
	if g.atkOf(0, 2) != base+1 || g.atkOf(1, 2) != base || b.Damage != 0 {
		t.Errorf("ATK %d, %d and %d", g.atkOf(0, 2), g.atkOf(0, 3), g.atkOf(1, 2))
	}
}

func TestRandomPlayoutsWithMarcelinesDeck(t *testing.T) {
	realGame(t)
	games := 120
	if testing.Short() {
		games = 20
	}
	ls := [NumLanes]card.Landscape{card.UselessSwamp, card.UselessSwamp, card.NiceLands, card.NiceLands}
	ice := [NumLanes]card.Landscape{card.IcyLands, card.IcyLands, card.IcyLands, card.IcyLands}
	finished := 0
	removed := 0
	for seed := uint64(1); seed <= uint64(games); seed++ {
		mine := DeckSpec{Landscapes: ls, Cards: realCat.Deck("Ice King Vs Marceline", "Marceline's Deck"), Hero: "marceline"}
		other := mine
		if seed%2 == 0 {
			other = DeckSpec{Landscapes: ice, Cards: realCat.Deck("Ice King Vs Marceline", "Ice King's Deck"), Hero: "ice-king"}
		}
		g, err := NewGame(realCat, Config{Seed: seed, First: PlayerID(seed % 2)}, [2]DeckSpec{mine, other})
		if err != nil {
			t.Fatal(err)
		}
		if _, done := playRandom(t, g, rand.New(rand.NewPCG(seed, 5)), 20000); done {
			finished++
		}
		removed += len(g.State.Players[0].Removed) + len(g.State.Players[1].Removed)
	}
	if finished < games*9/10 {
		t.Errorf("only %d of %d random games finished", finished, games)
	}
	t.Logf("cards removed from the game: %d", removed)
}
