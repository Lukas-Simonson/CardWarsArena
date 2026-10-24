package engine

// canAttack reports whether p's Creature in lane may start a Fight right now.
func (g *Game) canAttack(p PlayerID, lane int) bool {
	cr := g.State.Players[p].Lanes[lane].Creature
	if cr == nil || cr.Status != Ready {
		return false
	}
	opp := pos{p.Opp(), lane}
	return g.creatureAt(opp) == nil || g.attackable(opp)
}

// hasAttacker reports whether p still has a Creature that can Fight. Ready
// Creatures that cannot (their target is protected) simply stay Ready.
func (g *Game) hasAttacker(p PlayerID) bool {
	for lane := 0; lane < NumLanes; lane++ {
		if g.canAttack(p, lane) {
			return true
		}
	}
	return false
}

func (g *Game) validateEndMain(p PlayerID) error { return g.requireMain(p) }

// doEndMain leaves the main phase. The very first turn of the game has no
// Fight phase. Otherwise the Fight phase begins, and the turn ends at once if
// nothing is Ready to Fight.
func (g *Game) doEndMain(p PlayerID) []Event {
	if g.State.Turn == 1 {
		return g.endTurn(p)
	}
	g.State.Phase = PhaseFight
	if !g.hasAttacker(p) {
		return g.endTurn(p)
	}
	return nil
}

func (g *Game) validateAttack(p PlayerID, a Action) error {
	if g.State.Phase != PhaseFight {
		return ErrWrongPhase
	}
	if p != g.State.Active {
		return ErrNotYourTurn
	}
	if a.Lane < 0 || a.Lane >= NumLanes {
		return ErrBadLane
	}
	if c := g.State.Players[p].Lanes[a.Lane].Creature; c == nil || c.Status != Ready {
		return ErrNoAttacker
	}
	if !g.canAttack(p, a.Lane) {
		return ErrCannotAttack
	}
	return nil
}

// doAttack Activates p's Creature in a.Lane to Fight.
func (g *Game) doAttack(p PlayerID, a Action) []Event {
	att := g.State.Players[p].Lanes[a.Lane].Creature
	att.Status = Fought
	ev := []Event{{Kind: EvAttack, Player: p, Card: att.UID, Lane: a.Lane}}

	opp := p.Opp()
	def := g.State.Players[opp].Lanes[a.Lane].Creature
	if def == nil {
		// Nothing in the way: the Creature hits the opposing Hero.
		if dmg := g.atk(pos{p, a.Lane}); dmg > 0 {
			g.State.Players[opp].HP -= dmg
			ev = append(ev, Event{Kind: EvHPLoss, Player: opp, Lane: a.Lane, Amount: dmg})
		}
	} else {
		// Both Creatures deal damage to each other simultaneously.
		toDef, toAtt := g.atk(pos{p, a.Lane}), g.atk(pos{opp, a.Lane})
		def.Damage += toDef
		att.Damage += toAtt
		if toDef > 0 {
			ev = append(ev, Event{Kind: EvDamage, Player: opp, Card: def.UID, Lane: a.Lane, Amount: toDef})
		}
		if toAtt > 0 {
			ev = append(ev, Event{Kind: EvDamage, Player: p, Card: att.UID, Lane: a.Lane, Amount: toAtt})
		}
	}

	ev = append(ev, g.settle()...)
	if g.State.Phase == PhaseOver {
		return ev
	}
	if !g.hasAttacker(p) {
		ev = append(ev, g.endTurn(p)...)
	}
	return ev
}

// settle applies state-based rules: Creatures with damage >= DEF are
// discarded, and a player at 0 Hit Points loses.
func (g *Game) settle() []Event {
	var ev []Event
	for p := range g.State.Players {
		pl := &g.State.Players[p]
		for i := range pl.Lanes {
			if c := pl.Lanes[i].Creature; c != nil && c.Damage >= g.def(pos{PlayerID(p), i}) {
				ev = append(ev, Event{Kind: EvDie, Player: PlayerID(p), Card: c.UID, Lane: i})
				ev = append(ev, g.removeCreature(pos{PlayerID(p), i}, toDiscard)...) // owner's discard pile
			}
		}
	}
	for p := range g.State.Players {
		if g.State.Players[p].HP <= 0 {
			g.State.Phase = PhaseOver
			g.State.Winner = PlayerID(p).Opp()
			ev = append(ev, Event{Kind: EvGameOver, Player: g.State.Winner})
			break
		}
	}
	return ev
}

// endTurn passes the turn to p's opponent. Unspent Actions are lost and every
// "this turn" effect ends.
func (g *Game) endTurn(p PlayerID) []Event {
	g.State.SpellsThisTurn = 0
	for i := range g.State.Players {
		for _, l := range g.State.Players[i].Lanes {
			if c := l.Creature; c != nil {
				c.TempATK, c.TempDEF, c.ChangedLanes = 0, 0, false
			}
		}
	}
	return g.startTurn(p.Opp())
}
