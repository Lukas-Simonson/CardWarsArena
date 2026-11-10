package engine

import "cardwars/internal/card"

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
	ev := g.fireStart(p, card.OnFightStart)
	if !g.hasAttacker(p) {
		ev = append(ev, g.endTurn(p)...)
	}
	return ev
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

// doAttack Activates p's Creature in a.Lane to Fight. Effects of the attack
// itself come first; one of them may stop to ask who fights (Pied Piper), and
// the Fight goes on once that is answered.
func (g *Game) doAttack(p PlayerID, a Action) []Event {
	att := g.State.Players[p].Lanes[a.Lane].Creature
	att.Status = Fought
	ev := []Event{{Kind: EvAttack, Player: p, Card: att.UID, Lane: a.Lane}}
	ctx := effCtx{p: p, at: pos{p, a.Lane}, hasSrc: true, creature: att}
	ev = append(ev, g.fire(card.OnAttacks, ctx, g.effectsOf(att), nil)...)
	if g.State.Choice != nil {
		return ev
	}
	return append(ev, g.fight(p, a.Lane, a.Lane)...)
}

// fight resolves a Fight in lane between the Creature of p in fromLane, which
// is normally the one in lane, and the opposing Creature there. It then ends
// the turn if nothing is left to Fight.
func (g *Game) fight(p PlayerID, lane, fromLane int) []Event {
	var ev []Event
	from := pos{p, fromLane}
	att := g.creatureAt(from)
	opp := p.Opp()
	def := g.State.Players[opp].Lanes[lane].Creature
	if def == nil {
		// Nothing in the way: the Creature hits the opposing Hero.
		if dmg := g.atk(from); dmg > 0 {
			g.State.Players[opp].HP -= dmg
			ev = append(ev, Event{Kind: EvHPLoss, Player: opp, Lane: lane, Amount: dmg})
			g.heroDamage = dmg
			ctx := effCtx{p: p, at: from, hasSrc: true, creature: att}
			ev = append(ev, g.fire(card.OnHeroDamage, ctx, g.effectsOf(att), nil)...)
			ev = append(ev, g.fire(card.OnDealsDamage, ctx, g.effectsOf(att), nil)...)
		}
	} else {
		// Both Creatures deal damage to each other simultaneously.
		toDef, toAtt := g.atk(from), g.atk(pos{opp, lane})
		if g.State.NoCombatDamage[opp] {
			toDef = 0 // Falling Star
		}
		if g.State.NoCombatDamage[p] {
			toAtt = 0
		}
		defAt := pos{opp, lane}
		toDef, toAtt = g.hurt(defAt, toDef), g.hurt(from, toAtt)
		if toDef > 0 {
			ev = append(ev, Event{Kind: EvDamage, Player: opp, Card: def.UID, Lane: lane, Amount: toDef})
		}
		if toAtt > 0 {
			ev = append(ev, Event{Kind: EvDamage, Player: p, Card: att.UID, Lane: fromLane, Amount: toAtt})
		}
		for _, d := range []struct {
			at     pos
			dealt  int
			dealer *Creature
		}{{from, toDef, att}, {defAt, toAtt, def}} {
			if d.dealt > 0 {
				g.heroDamage = d.dealt
				ctx := effCtx{p: d.at.P, at: d.at, hasSrc: true, creature: d.dealer}
				ev = append(ev, g.fire(card.OnDealsDamage, ctx, g.effectsOf(d.dealer), nil)...)
			}
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

// hurt puts damage on the Creature at at and returns how much went on:
// Blood Castle raises every amount it sees.
func (g *Game) hurt(at pos, amt int) int {
	if amt > 0 {
		amt += g.staticMod("damage_taken", at)
	}
	amt = max(amt, 0)
	g.creatureAt(at).Damage += amt
	return amt
}

// settle applies state-based rules: Creatures with damage >= DEF are
// discarded (repeating, since leaving play can trigger more damage), and a
// player at 0 Hit Points loses.
func (g *Game) settle() []Event {
	if g.State.Phase == PhaseOver {
		return nil // already decided; do not announce the result twice
	}
	var ev []Event
	for changed := true; changed; {
		changed = false
		for p := range g.State.Players {
			for i := range g.State.Players[p].Lanes {
				at := pos{PlayerID(p), i}
				if c := g.creatureAt(at); c != nil && c.Damage >= g.def(at) {
					ev = append(ev, Event{Kind: EvDie, Player: PlayerID(p), Card: c.UID, Lane: i})
					ev = append(ev, g.removeCreature(at, toDiscard)...) // owner's discard pile
					changed = true
				}
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
	var ev []Event
	// Borrowed Creatures go home (back to their owner's side of the same Lane).
	for i := range g.State.Players {
		for lane := range g.State.Players[i].Lanes {
			at := pos{PlayerID(i), lane}
			if c := g.creatureAt(at); c != nil && c.Borrowed {
				c.Borrowed = false
				if c.Owner != at.P {
					ev = append(ev, g.moveCreature(at, pos{c.Owner, lane})...)
				}
			}
		}
	}
	g.State.SpellsThisTurn = 0
	g.State.Played, g.State.RainbowHand = nil, nil
	g.State.Returned = [2]int{}
	for i := range g.State.Players {
		g.State.Players[i].HeroFired = nil
	}
	g.State.Entered = [2][NumLanes]int{}
	g.State.CreatureDiscount = 0
	g.State.NoCombatDamage = [2]bool{}
	for i := range g.State.Players {
		for lane := range g.State.Players[i].Lanes {
			l := &g.State.Players[i].Lanes[lane]
			l.ReplaceDiscount = 0
			if c := l.Creature; c != nil {
				c.TempATK, c.TempDEF, c.ChangedLanes, c.Replaced, c.Rainbow = 0, 0, false, false, false
				c.Silenced, c.Lent = false, ""
				c.Fired = nil
			}
		}
	}
	return append(ev, g.startTurn(p.Opp())...)
}
