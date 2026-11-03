package engine

import (
	"testing"

	"cardwars/internal/card"
)

// heroGame is the Cake and Fionna game (player 1 to move, on turn 2) with the
// given Heroes in place of the decks' own, so each Hero can be tried alone.
func heroGame(t *testing.T, h0, h1 string) *Game {
	t.Helper()
	g := fcGame(t)
	g.setHero(0, h0)
	g.setHero(1, h1)
	return g
}

func (g *Game) setHero(p PlayerID, id string) {
	g.State.Players[p].Hero = g.newInstance(id, p)
}

func heroActivate(idx int, targets ...[]TargetRef) Action {
	return Action{Kind: ActActivate, Source: "hero", Index: idx, Targets: targets}
}

func TestEveryHeroIsModelledAndDealt(t *testing.T) {
	realGame(t)
	for _, id := range []string{"bmo", "cake", "finn", "fionna", "jake", "lady-rainicorn", "lumpy-space-princess", "princess-bubblegum"} {
		h := realCat[id]
		if h == nil || h.Type != card.Hero || !h.Implemented() {
			t.Errorf("%s: not a modelled Hero", id)
		}
	}
	deck := DeckSpec{Landscapes: [NumLanes]card.Landscape{card.Cornfield, card.Cornfield, card.Cornfield, card.Cornfield}, Cards: realCat.Deck(fcSet, "Cake's Deck"), Hero: "cake"}
	g, err := NewGame(realCat, Config{Seed: 1, SkipDeckRules: true}, [2]DeckSpec{deck, deck})
	if err != nil {
		t.Fatal(err)
	}
	for p := range g.State.Players {
		if h := g.State.Players[p].Hero; h.CardID != "cake" || h.UID == 0 || h.Owner != PlayerID(p) {
			t.Errorf("seat %d Hero = %+v", p, h)
		}
	}
	deck.Hero = "pony"
	if _, err := NewGame(realCat, Config{Seed: 1, SkipDeckRules: true}, [2]DeckSpec{deck, deck}); err == nil {
		t.Error("a Creature was accepted as a Hero")
	}
	deck.Hero = "nope"
	if _, err := NewGame(realCat, Config{Seed: 1, SkipDeckRules: true}, [2]DeckSpec{deck, deck}); err == nil {
		t.Error("an unknown Hero was accepted")
	}
}

func TestJakeGivesFacedownCreaturesAttack(t *testing.T) {
	g := heroGame(t, "jake", "jake")
	g.put(0, 0, "pony")
	g.put(0, 1, "pony")
	g.put(1, 0, "pony")
	g.State.Players[0].Lanes[0].FaceDown = true
	g.State.Players[1].Lanes[0].FaceDown = true
	if g.atkOf(0, 0) != 3 || g.atkOf(0, 1) != 1 || g.atkOf(1, 0) != 3 {
		t.Errorf("ATK: %d %d %d, want 3 1 3", g.atkOf(0, 0), g.atkOf(0, 1), g.atkOf(1, 0))
	}
	g.setHero(1, "cake")
	if g.atkOf(1, 0) != 1 || g.atkOf(0, 0) != 3 {
		t.Errorf("Jake's bonus must stay on his side: %d and %d", g.atkOf(1, 0), g.atkOf(0, 0))
	}
}

func TestFionnaBoostsRainbowCreaturesOnOneLandscapeType(t *testing.T) {
	g := heroGame(t, "fionna", "cake")
	g.put(0, 0, "pony")            // a Rainbow Creature
	g.put(0, 1, "husker-champion") // a Cornfield Creature
	if g.atkOf(0, 0) != 1 {        // two Landscape types: Cornfield and Blue Plains
		t.Errorf("two Landscape types: ATK %d, want 1", g.atkOf(0, 0))
	}
	for i := range g.State.Players[0].Lanes {
		g.State.Players[0].Lanes[i].Landscape = card.Cornfield
	}
	if g.atkOf(0, 0) != 2 || g.atkOf(0, 1) != 1 {
		t.Errorf("one type: Rainbow %d, other %d, want 2 and 1", g.atkOf(0, 0), g.atkOf(0, 1))
	}
}

func TestBMODiscardsToDealDamage(t *testing.T) {
	g := heroGame(t, "cake", "bmo")
	foe := g.put(0, 2, "pony")
	uid := g.give(1, "pony")
	hand := len(g.State.Players[1].Hand)
	act := heroActivate(0, []TargetRef{handRef(1, uid)}, []TargetRef{crRef(0, 2)})
	if !hasAction(g.LegalActions(1), act) {
		t.Fatal("BMO's ability is not offered")
	}
	g.must(t, 1, act)
	if foe.Damage != 1 || len(g.State.Players[1].Hand) != hand-1 || g.State.ActionsLeft != ActionsPerTurn {
		t.Errorf("damage %d, hand %d -> %d, Actions %d", foe.Damage, hand, len(g.State.Players[1].Hand), g.State.ActionsLeft)
	}
	g.mustFail(t, 0, heroActivate(0), ErrNotYourTurn)
}

func TestPrincessBubblegumHurtsHerOwnCreatureOncePerTurn(t *testing.T) {
	g := heroGame(t, "cake", "princess-bubblegum")
	mine := g.put(1, 0, "pony")
	g.put(0, 0, "pony")
	g.mustFail(t, 1, heroActivate(0, []TargetRef{crRef(0, 0)}), ErrBadTarget) // only her own Creatures
	g.must(t, 1, heroActivate(0, []TargetRef{crRef(1, 0)}))
	if mine.Damage != 1 || g.atkOf(1, 0) != 2 {
		t.Errorf("damage %d, ATK %d, want 1 and 2", mine.Damage, g.atkOf(1, 0))
	}
	g.mustFail(t, 1, heroActivate(0, []TargetRef{crRef(1, 0)}), ErrLimit)
	// The next turn she may do it again, and the bonus has worn off.
	g.endTurn(1)
	g.endTurn(0)
	if g.atkOf(1, 0) != 1 {
		t.Errorf("ATK %d after the turn, want 1", g.atkOf(1, 0))
	}
	g.must(t, 1, heroActivate(0, []TargetRef{crRef(1, 0)}))
}

func TestFinnGivesUndamagedCreaturesAFloop(t *testing.T) {
	g := heroGame(t, "cake", "finn")
	mine := g.put(1, 0, "pony")
	g.put(0, 0, "pony")
	theirs := g.put(0, 1, "pony")
	g.State.Turn = 4
	g.mustFail(t, 0, floop(1, []TargetRef{crRef(1, 0)}), ErrNotYourTurn)
	foe := g.State.Players[0].Lanes[0].Creature
	g.must(t, 1, floop(0, []TargetRef{crRef(0, 0)}))
	if foe.Damage != 1 || mine.Status != Flooped {
		t.Errorf("target took %d, Pony is %s", foe.Damage, mine.Status)
	}
	// A damaged Creature has lost the ability, and so has one Finn does not control.
	mine.Status, mine.Damage = Ready, 1
	g.mustFail(t, 1, floop(0, []TargetRef{crRef(0, 0)}), ErrNoAbility)
	g.State.Active = 0
	g.mustFail(t, 0, floop(1, []TargetRef{crRef(1, 0)}), ErrNoAbility)
	if theirs.Damage != 0 {
		t.Error("the opponent's Creature was hurt")
	}
}

func TestCakeHealsCreaturesWithBuildingsWhenYouPlayOne(t *testing.T) {
	g := heroGame(t, "cake", "cake")
	a := g.put(0, 0, "pony")
	b := g.put(0, 1, "pony")
	a.Damage, b.Damage = 4, 4
	g.building(0, 0, "farmhouse")
	g.State.Active = 0
	g.place(0, g.newInstance("farmhouse", 0), 1)
	if a.Damage != 2 || b.Damage != 2 {
		t.Errorf("damage %d and %d, want 2 and 2", a.Damage, b.Damage)
	}
	c := g.put(0, 2, "pony")
	c.Damage = 4
	g.place(0, g.newInstance("farmhouse", 0), 3)
	if c.Damage != 4 {
		t.Errorf("a Creature with no Building was healed: %d", c.Damage)
	}
}

func TestLumpySpacePrincessDrawsOnTheFirstReplace(t *testing.T) {
	g := heroGame(t, "cake", "lumpy-space-princess")
	g.put(1, 0, "pony")
	g.put(1, 1, "pony")
	g.put(1, 2, "pony")
	hand := len(g.State.Players[1].Hand)
	g.place(1, g.newInstance("pony", 1), 3) // empty Lane: not a replace
	g.place(1, g.newInstance("pony", 1), 0)
	g.place(1, g.newInstance("pony", 1), 1)
	if got := len(g.State.Players[1].Hand); got != hand+1 {
		t.Errorf("hand %d -> %d, want one card drawn", hand, got)
	}
	g.State.Active = 0 // not on your turn
	g.State.Players[1].HeroFired = nil
	g.place(1, g.newInstance("pony", 1), 2)
	if got := len(g.State.Players[1].Hand); got != hand+1 {
		t.Errorf("drew on the opponent's turn: %d", got)
	}
}

func TestLadyRainicornHealsAfterTwoReturns(t *testing.T) {
	g := heroGame(t, "cake", "lady-rainicorn")
	for i := 0; i < 3; i++ {
		g.put(1, i, "pony")
	}
	g.State.Players[1].HP = 10
	g.removeCreature(pos{1, 0}, toHand)
	if g.State.Players[1].HP != 10 {
		t.Fatalf("healed after one return: %d", g.State.Players[1].HP)
	}
	g.removeCreature(pos{1, 1}, toHand)
	if g.State.Players[1].HP != 12 {
		t.Fatalf("HP %d after two returns, want 12", g.State.Players[1].HP)
	}
	g.removeCreature(pos{1, 2}, toHand)
	if g.State.Players[1].HP != 12 {
		t.Errorf("healed a second time: %d", g.State.Players[1].HP)
	}
	// A discard pile counts too, but not on the opponent's turn.
	g.State.Returned = [2]int{}
	g.State.Players[1].HeroFired = nil
	g.State.Active = 0
	g.put(1, 0, "pony")
	g.put(1, 1, "pony")
	g.removeCreature(pos{1, 0}, toHand)
	g.removeCreature(pos{1, 1}, toHand)
	if g.State.Players[1].HP != 12 {
		t.Errorf("healed during the opponent's turn: %d", g.State.Players[1].HP)
	}
}

func hasAction(list []Action, want Action) bool {
	for _, a := range list {
		if a.Kind == want.Kind && a.Source == want.Source && a.Index == want.Index && len(a.Targets) == len(want.Targets) {
			ok := true
			for i := range a.Targets {
				ok = ok && len(a.Targets[i]) == len(want.Targets[i]) && (len(a.Targets[i]) == 0 || a.Targets[i][0] == want.Targets[i][0])
			}
			if ok {
				return true
			}
		}
	}
	return false
}

func TestInstancesNamesTheHeroes(t *testing.T) {
	g := heroGame(t, "cake", "bmo")
	names := g.Instances()
	if names[g.State.Players[0].Hero.UID] != "cake" || names[g.State.Players[1].Hero.UID] != "bmo" {
		t.Errorf("Hero names missing: %v", names)
	}
}
