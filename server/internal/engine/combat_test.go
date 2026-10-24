package engine

import "testing"

func attack(lane int) Action { return Action{Kind: ActAttack, Lane: lane} }

var endMain = Action{Kind: ActEndMain}

// midGame returns a game on turn 2 (player 1 active, main phase) so fighting
// is allowed. Both players' creatures are placed directly (test helper).
func midGame(t *testing.T) *Game {
	t.Helper()
	g := newTestGame(t, 0, nil, nil)
	g.start(t)
	g.must(t, 0, endMain) // first turn: no Fight, passes to player 1
	if g.State.Turn != 2 || g.State.Active != 1 {
		t.Fatalf("setup: turn %d active %d", g.State.Turn, g.State.Active)
	}
	return g
}

func (g *Game) put(p PlayerID, lane int, id string) *Creature {
	g.State.NextUID++
	c := &Creature{Instance: Instance{UID: g.State.NextUID, CardID: id}, Status: Ready}
	g.State.Players[p].Lanes[lane].Creature = c
	return c
}

func TestFirstTurnHasNoFight(t *testing.T) {
	g := newTestGame(t, 0, []string{"Grunt"}, nil)
	g.start(t)
	g.play(t, 0, "Grunt", 0)
	g.must(t, 0, endMain)
	if g.State.Active != 1 || g.State.Phase != PhaseMain || g.State.Turn != 2 {
		t.Fatalf("active=%d phase=%s turn=%d", g.State.Active, g.State.Phase, g.State.Turn)
	}
	if g.State.Players[1].HP != StartingHP {
		t.Error("first player damaged the opponent on turn 1")
	}
	if g.State.Players[0].Lanes[0].Creature.Status != Ready {
		t.Error("creature should still be Ready")
	}
}

func TestSecondPlayerFightsOnFirstTurn(t *testing.T) {
	g := midGame(t)
	g.put(1, 0, "Brute")
	g.must(t, 1, endMain)
	if g.State.Phase != PhaseFight {
		t.Fatalf("phase = %s", g.State.Phase)
	}
	g.must(t, 1, attack(0))
	if hp := g.State.Players[0].HP; hp != StartingHP-4 {
		t.Errorf("HP = %d, want %d", hp, StartingHP-4)
	}
}

func TestEmptyBoardEndsTurnImmediately(t *testing.T) {
	g := midGame(t)
	g.must(t, 1, endMain)
	if g.State.Active != 0 || g.State.Turn != 3 || g.State.Phase != PhaseMain {
		t.Errorf("active=%d turn=%d phase=%s", g.State.Active, g.State.Turn, g.State.Phase)
	}
}

func TestTurnEndsAfterLastCreatureFights(t *testing.T) {
	g := midGame(t)
	g.put(1, 0, "Grunt")
	g.put(1, 2, "Grunt")
	g.must(t, 1, endMain)
	g.must(t, 1, attack(2)) // the attacker chooses the order
	if g.State.Active != 1 {
		t.Fatal("turn ended before every Ready Creature fought")
	}
	g.must(t, 1, attack(0))
	if g.State.Active != 0 {
		t.Error("turn should end once all Ready Creatures have fought")
	}
}

func TestCreaturesFightSimultaneously(t *testing.T) {
	g := midGame(t)
	att := g.put(1, 1, "Grunt") // ATK 2 DEF 3
	def := g.put(0, 1, "Brute") // ATK 4 DEF 5
	g.must(t, 1, endMain)
	g.must(t, 1, attack(1))
	if def.Damage != 2 || att.Damage != 4 {
		t.Errorf("defender took %d (want 2), attacker took %d (want 4)", def.Damage, att.Damage)
	}
}

func TestDeathAtDamageEqualToDef(t *testing.T) {
	g := midGame(t)
	g.put(1, 1, "Grunt") // ATK 2 DEF 3
	g.put(0, 1, "Brute") // ATK 4 DEF 5: deals 4 >= 3, kills the attacker
	g.must(t, 1, endMain)
	g.must(t, 1, attack(1))
	if g.State.Players[1].Lanes[1].Creature != nil {
		t.Error("attacker should be discarded")
	}
	if d := g.State.Players[1].Discard; len(d) != 1 || d[0].CardID != "Grunt" {
		t.Errorf("attacker's owner discard = %+v", d)
	}
	if g.State.Players[0].Lanes[1].Creature == nil {
		t.Error("defender survives with 2 damage")
	}
}

func TestBothCreaturesCanDie(t *testing.T) {
	g := midGame(t)
	g.put(1, 3, "Free Rat")
	g.put(0, 3, "Free Rat")
	g.must(t, 1, endMain)
	g.must(t, 1, attack(3)) // 1 ATK vs 1 DEF each way: both die
	if g.State.Players[1].Lanes[3].Creature != nil || g.State.Players[0].Lanes[3].Creature != nil {
		t.Error("both Free Rats should die")
	}
	if len(g.State.Players[0].Discard) != 1 || len(g.State.Players[1].Discard) != 1 {
		t.Error("each player's Rat goes to its own owner's discard")
	}
}

func TestDamagePersistsBetweenFights(t *testing.T) {
	g := midGame(t)
	g.put(1, 0, "Grunt")
	wall := g.put(0, 0, "Wall") // ATK 0 DEF 6
	g.must(t, 1, endMain)
	g.must(t, 1, attack(0))
	if wall.Damage != 2 {
		t.Fatalf("wall damage = %d", wall.Damage)
	}
	g.must(t, 0, endMain) // player 0's turn: the Wall fights the Grunt and takes 2 more
	g.must(t, 0, attack(0))
	if wall.Damage != 4 {
		t.Errorf("damage should persist and accumulate, got %d", wall.Damage)
	}
}

func TestCannotAttackTwiceOrWithEmptyLane(t *testing.T) {
	g := midGame(t)
	g.put(1, 0, "Grunt")
	g.put(1, 1, "Grunt")
	g.must(t, 1, endMain)
	g.must(t, 1, attack(0))
	g.mustFail(t, 1, attack(0), ErrNoAttacker)
	g.mustFail(t, 1, attack(2), ErrNoAttacker)
	g.mustFail(t, 1, attack(9), ErrBadLane)
	g.mustFail(t, 0, attack(1), ErrNotYourTurn)
}

func TestFloopedCreatureDoesNotFightButStillDefends(t *testing.T) {
	g := midGame(t)
	flooped := g.put(1, 0, "Grunt")
	flooped.Status = Flooped
	g.put(1, 1, "Grunt")
	g.must(t, 1, endMain)
	g.mustFail(t, 1, attack(0), ErrNoAttacker)
	g.must(t, 1, attack(1)) // only the Ready Creature fights, then the turn ends
	if g.State.Active != 0 {
		t.Fatal("turn should end: the only Ready Creature has fought")
	}
	// On player 0's turn the Flooped Creature is Readied, and defends either way.
	if flooped.Status != Flooped {
		t.Error("opponent's Creatures must not be readied on my opponent's turn start")
	}
	g.put(0, 0, "Brute")
	g.must(t, 0, endMain)
	g.must(t, 0, attack(0))
	if flooped.Damage != 4 {
		t.Errorf("a Flooped Creature still takes damage when defending: %d", flooped.Damage)
	}
}

func TestReadyAtTurnStart(t *testing.T) {
	g := midGame(t)
	c := g.put(1, 0, "Grunt")
	g.must(t, 1, endMain)
	g.must(t, 1, attack(0))
	if c.Status != Fought {
		t.Fatal("setup")
	}
	g.must(t, 0, endMain) // nothing on player 0's side: turn passes straight back
	if c.Status != Ready {
		t.Errorf("status = %s, want ready at start of owner's turn", c.Status)
	}
}

func TestUnspentActionsAreLost(t *testing.T) {
	g := midGame(t)
	g.must(t, 1, endMain)
	g.must(t, 0, endMain) // back to player 1
	if g.State.Active != 0 && g.State.ActionsLeft != ActionsPerTurn {
		t.Errorf("actions = %d", g.State.ActionsLeft)
	}
	if g.State.ActionsLeft != ActionsPerTurn {
		t.Errorf("actions = %d, want %d (unspent Actions do not carry over)", g.State.ActionsLeft, ActionsPerTurn)
	}
}

func TestWinningEndsTheGame(t *testing.T) {
	g := midGame(t)
	g.put(1, 0, "Brute") // 4 damage
	g.State.Players[0].HP = 4
	g.must(t, 1, endMain)
	ev := g.must(t, 1, attack(0))
	if g.State.Phase != PhaseOver || g.State.Winner != 1 {
		t.Fatalf("phase=%s winner=%d", g.State.Phase, g.State.Winner)
	}
	if last := ev[len(ev)-1]; last.Kind != EvGameOver || last.Player != 1 {
		t.Errorf("last event = %+v", last)
	}
	g.mustFail(t, 0, Action{Kind: ActDraw}, ErrGameOver)
	g.mustFail(t, 1, endMain, ErrGameOver)
}

func TestHPMayGoNegative(t *testing.T) {
	g := midGame(t)
	g.put(1, 0, "Brute")
	g.State.Players[0].HP = 1
	g.must(t, 1, endMain)
	g.must(t, 1, attack(0))
	if g.State.Players[0].HP != -3 || g.State.Winner != 1 {
		t.Errorf("hp=%d winner=%d", g.State.Players[0].HP, g.State.Winner)
	}
}

func TestLosingACreatureOnYourTurnExposesYou(t *testing.T) {
	// Rulebook: "If you lose a Creature during your own turn, your opponent's
	// Creature will be able to attack you directly during his turn."
	g := midGame(t)
	g.put(1, 0, "Brute") // ATK 4 DEF 5
	g.put(0, 0, "Wall")  // ATK 0, DEF 6
	g.State.Players[0].Lanes[0].Creature.Damage = 5
	g.must(t, 1, endMain)
	g.must(t, 1, attack(0)) // Brute deals 4: Wall (5+4 >= 6) dies
	if g.State.Players[0].Lanes[0].Creature != nil {
		t.Fatal("Wall should be dead")
	}
	g.must(t, 0, endMain) // player 0 has nothing to Fight with; turn passes
	g.must(t, 1, endMain)
	g.must(t, 1, attack(0)) // Brute is Ready again and the Lane is empty
	if hp := g.State.Players[0].HP; hp != StartingHP-4 {
		t.Errorf("HP = %d, want direct damage of 4", hp)
	}
}

func TestGameOverIsAnnouncedOnce(t *testing.T) {
	g := midGame(t)
	g.put(1, 0, "Brute")
	g.State.Players[0].HP = 1
	g.must(t, 1, endMain)
	ev := g.must(t, 1, attack(0))
	n := 0
	for _, e := range ev {
		if e.Kind == EvGameOver {
			n++
		}
	}
	if n != 1 {
		t.Errorf("game_over events = %d, want 1", n)
	}
}
