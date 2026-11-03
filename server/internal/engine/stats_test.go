package engine

import (
	"sync"
	"testing"

	"cardwars/internal/card"
)

var (
	realOnce sync.Once
	realCat  card.Catalog
)

// realGame is a game on the real catalog with Finn's and Jake's starter decks,
// in the main phase of turn 2 (player 1 active). Creatures are placed by hand.
func realGame(t *testing.T) *Game {
	t.Helper()
	realOnce.Do(func() {
		var err error
		if realCat, err = card.Load("../../data"); err != nil {
			t.Fatal(err)
		}
	})
	finn := DeckSpec{Landscapes: [NumLanes]card.Landscape{card.BluePlains, card.BluePlains, card.BluePlains, card.BluePlains}, Cards: realCat.Deck("Finn Vs Jake", "Finn's Deck")}
	jake := DeckSpec{Landscapes: cornfields, Cards: realCat.Deck("Finn Vs Jake", "Jake's Deck")}
	g, err := NewGame(realCat, Config{Seed: 1, First: 0, AllowUnimplemented: true}, [2]DeckSpec{jake, finn})
	if err != nil {
		t.Fatal(err)
	}
	g.start(t)
	g.must(t, 0, endMain)
	return g
}

func (g *Game) atkOf(p PlayerID, lane int) int { return g.atk(pos{p, lane}) }
func (g *Game) defOf(p PlayerID, lane int) int { return g.def(pos{p, lane}) }

// The rulebook's worked example: Husker Knight (Jake, Cornfield) vs The Pig
// (Finn) beside an X-Large Spirit Soldier.
func TestRulebookHuskerKnightExample(t *testing.T) {
	g := realGame(t)
	g.put(0, 0, "husker-knight")
	g.put(1, 0, "the-pig")
	g.put(1, 1, "x-large-spirit-soldier")

	// Jake controls 4 Cornfields: +1 ATK and +2 DEF each.
	if got := g.atkOf(0, 0); got != 4 {
		t.Errorf("Husker Knight ATK with 4 Cornfields = %d, want 4", got)
	}
	if got := g.defOf(0, 0); got != 8 {
		t.Errorf("Husker Knight DEF with 4 Cornfields = %d, want 8", got)
	}
	// The Pig is adjacent to X-Large Spirit Soldier: 1 + 1.
	if got := g.atkOf(1, 0); got != 2 {
		t.Errorf("The Pig ATK beside X-Large Spirit Soldier = %d, want 2", got)
	}
	if got := g.atkOf(1, 1); got != 1 {
		t.Errorf("X-Large Spirit Soldier ATK = %d, want 1 (it is not adjacent to itself)", got)
	}

	// "The Cornfield player only controls 3 Cornfields right now" after one
	// is flipped face down: Knight ATK 3, DEF 6, as in the rulebook.
	g.State.Players[0].Lanes[0].FaceDown = true
	if got := g.atkOf(0, 0); got != 3 {
		t.Errorf("Husker Knight ATK with 3 Cornfields = %d, want 3", got)
	}
	if got := g.defOf(0, 0); got != 6 {
		t.Errorf("Husker Knight DEF with 3 Cornfields = %d, want 6", got)
	}
	// Remove the Soldier: The Pig would have dealt only 1.
	g.State.Players[1].Lanes[1].Creature = nil
	if got := g.atkOf(1, 0); got != 1 {
		t.Errorf("The Pig ATK alone = %d, want 1", got)
	}
}

func TestNiceIceBabyBonusOnlyWhenUnopposed(t *testing.T) {
	g := realGame(t)
	g.put(1, 0, "nice-ice-baby")
	if got := g.atkOf(1, 0); got != 4 {
		t.Errorf("unopposed ATK = %d, want 1+3", got)
	}
	g.put(0, 0, "evil-eye")
	if got := g.atkOf(1, 0); got != 1 {
		t.Errorf("opposed ATK = %d, want 1", got)
	}
}

func TestBuildingsBuffTheirLane(t *testing.T) {
	g := realGame(t)
	g.put(1, 0, "the-pig") // ATK 1 DEF 4
	g.put(1, 1, "the-pig")
	g.State.Players[1].Lanes[0].Building = &Building{Instance: Instance{UID: 900, CardID: "blood-fortress"}}
	g.State.Players[1].Lanes[0].Building.Status = Ready
	g.State.Players[1].Lanes[1].Building = &Building{Instance: Instance{UID: 901, CardID: "celestial-castle"}}
	if g.atkOf(1, 0) != 2 || g.atkOf(1, 1) != 1 {
		t.Errorf("Blood Fortress buffs only its Lane: %d %d", g.atkOf(1, 0), g.atkOf(1, 1))
	}
	if g.defOf(1, 1) != 7 || g.defOf(1, 0) != 4 {
		t.Errorf("Celestial Castle buffs only its Lane: %d %d", g.defOf(1, 0), g.defOf(1, 1))
	}
	// An opposing Creature in the same Lane is not buffed by my Building.
	g.put(0, 0, "the-pig")
	if g.atkOf(0, 0) != 1 {
		t.Errorf("opponent's Creature buffed by my Building: %d", g.atkOf(0, 0))
	}
}

func TestCountBasedBonuses(t *testing.T) {
	g := realGame(t)
	// Corn Dog: 0/12, +1 DEF per Cornfield, +1 ATK while you control 3 or fewer.
	g.put(0, 0, "corn-dog")
	if g.atkOf(0, 0) != 0 || g.defOf(0, 0) != 16 {
		t.Errorf("4 Cornfields: %d/%d, want 0/16", g.atkOf(0, 0), g.defOf(0, 0))
	}
	g.State.Players[0].Lanes[3].FaceDown = true
	if g.atkOf(0, 0) != 1 || g.defOf(0, 0) != 15 {
		t.Errorf("3 Cornfields: %d/%d, want 1/15", g.atkOf(0, 0), g.defOf(0, 0))
	}

	// Corn Lord: +1 ATK per *other* Cornfield Creature; Rainbow cards do not count.
	g.put(0, 1, "corn-lord")
	g.put(0, 2, "corn-ronin")
	g.put(0, 3, "evil-eye") // Rainbow
	if got := g.atkOf(0, 1); got != 0+2 {
		t.Errorf("Corn Lord ATK = %d, want 2 (Corn Dog and Corn Ronin)", got)
	}
	// Corn Ronin: +1 per adjacent face-up Cornfield. Lane 2 touches lanes 1 and 3 (3 is face down).
	if got := g.atkOf(0, 2); got != 1+1 {
		t.Errorf("Corn Ronin ATK = %d, want 2", got)
	}
}

func TestWallOfEarsCountsBothPlayers(t *testing.T) {
	g := realGame(t)
	g.put(0, 0, "wall-of-ears") // 2/4
	if got := g.defOf(0, 0); got != 4+4 {
		t.Errorf("DEF = %d, want 8 with Jake's 4 Cornfields", got)
	}
	// Finn's side counts too once he has Cornfields in play.
	g.State.Players[1].Lanes[0].Landscape = card.Cornfield
	if got := g.defOf(0, 0); got != 4+5 {
		t.Errorf("DEF = %d, want 9", got)
	}
}

func TestStruzannJinnCountsFloopedCreatures(t *testing.T) {
	g := realGame(t)
	g.put(1, 0, "struzann-djinn") // 1/11
	f := g.put(1, 1, "the-pig")
	f.Status = Flooped
	if got := g.atkOf(1, 0); got != 1+2 {
		t.Errorf("ATK = %d, want 3", got)
	}
	g.put(0, 0, "the-pig").Status = Flooped // opposing Flooped Creatures do not count
	if got := g.atkOf(1, 0); got != 3 {
		t.Errorf("ATK = %d, want 3", got)
	}
}

func TestWoadicChiefCountsOnlyMySpellsThisTurn(t *testing.T) {
	g := realGame(t)
	g.put(1, 0, "woadic-chief") // 2/10, player 1 is active
	g.State.SpellsThisTurn = 2
	if got := g.atkOf(1, 0); got != 2+4 {
		t.Errorf("ATK = %d, want 6", got)
	}
	g.State.Active = 0 // on the opponent's turn the bonus is gone
	if got := g.atkOf(1, 0); got != 2 {
		t.Errorf("ATK on opponent's turn = %d, want 2", got)
	}
}

func TestPunkCatBuffsAnyCreatureThatChangedLanes(t *testing.T) {
	g := realGame(t)
	g.put(1, 0, "punk-cat")
	mover := g.put(0, 2, "the-pig")
	if got := g.atkOf(0, 2); got != 1 {
		t.Errorf("before moving: %d", got)
	}
	mover.ChangedLanes = true
	if got := g.atkOf(0, 2); got != 3 {
		t.Errorf("an opposing Creature that changed Lanes also gets +2: %d", got)
	}
}

func TestStatsClampAtZero(t *testing.T) {
	g := realGame(t)
	c := g.put(1, 0, "the-pig")
	c.TempATK = -10
	if g.atkOf(1, 0) != 0 {
		t.Errorf("ATK = %d, want clamped to 0", g.atkOf(1, 0))
	}
}

func TestCoolDogProtectsAdjacentLanes(t *testing.T) {
	g := realGame(t)
	g.put(0, 1, "cool-dog") // protects lanes 0 and 2 on player 0's side
	for _, lane := range []int{0, 2, 3} {
		g.put(0, lane, "evil-eye")
	}
	g.put(1, 0, "the-pig")
	g.put(1, 1, "the-pig")
	g.put(1, 2, "the-pig")
	g.put(1, 3, "the-pig")
	g.must(t, 1, endMain)
	g.mustFail(t, 1, attack(0), ErrCannotAttack)
	g.mustFail(t, 1, attack(2), ErrCannotAttack)
	g.must(t, 1, attack(1)) // Cool Dog itself may be Attacked
	if g.State.Active != 1 {
		t.Fatal("lane 3 can still Fight, so the turn is not over")
	}
	g.must(t, 1, attack(3)) // lane 3 is not adjacent to the Dog
	if g.State.Active != 0 {
		t.Error("lanes 0 and 2 cannot Fight, so the turn ends with those Creatures Ready")
	}
	for _, lane := range []int{0, 2} {
		if g.State.Players[1].Lanes[lane].Creature.Status != Ready {
			t.Errorf("lane %d Creature should stay Ready", lane)
		}
	}
}

func TestTurnEndsWhenOnlyUnattackableCreaturesRemain(t *testing.T) {
	g := realGame(t)
	g.put(0, 1, "cool-dog")
	g.put(0, 0, "evil-eye")
	g.put(1, 0, "the-pig") // can only attack lane 0, which is protected
	g.must(t, 1, endMain)
	if g.State.Active != 0 {
		t.Error("with nothing able to Fight the turn ends, leaving the Creature Ready")
	}
	if g.State.Players[1].Lanes[0].Creature.Status != Ready {
		t.Error("the Creature that could not Fight must stay Ready")
	}
}

func TestPerTurnEffectsEndWithTheTurn(t *testing.T) {
	g := realGame(t)
	c := g.put(1, 0, "the-pig")
	c.TempATK, c.TempDEF, c.ChangedLanes = 3, 2, true
	g.State.SpellsThisTurn = 2
	g.must(t, 1, endMain)
	g.must(t, 1, attack(0)) // the last Fight ends the turn
	if c.TempATK != 0 || c.TempDEF != 0 || c.ChangedLanes || g.State.SpellsThisTurn != 0 {
		t.Errorf("per-turn state survived the turn: %+v spells=%d", c, g.State.SpellsThisTurn)
	}
}
