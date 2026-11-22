package engine

import "cardwars/internal/card"

// CauseBurn is the Cause of the damage burning does at the start of a turn.
const CauseBurn = "burn"

// Burn tokens (Flame Princess's deck). Putting a burn token on a Creature or a player ("burning" it) deals 1
// damage at once, and each burn token on it deals 1 more at the start of its controller's turn. The tokens stay
// until they are removed or the Creature leaves play.

// burnCreature puts n burn tokens, one at a time, on the Creature at at, on behalf of player by. Each is 1 damage
// and counts as burning a Creature for what its controller's cards watch for.
func (g *Game) burnCreature(at pos, by PlayerID, n int) []Event {
	var ev []Event
	for i := 0; i < n; i++ {
		cr := g.creatureAt(at)
		if cr == nil {
			break
		}
		cr.Burn++
		ev = append(ev, Event{Kind: EvBurn, Player: at.P, Card: cr.UID, Lane: at.Lane, Amount: 1})
		if dealt := g.hurt(at, 1); dealt > 0 {
			ev = append(ev, Event{Kind: EvDamage, Player: at.P, Card: cr.UID, Lane: at.Lane, Amount: dealt})
		}
		ev = append(ev, g.fireBurns(by, at)...)
	}
	return ev
}

// burnPlayer puts n burn tokens on p, each dealing 1 damage at once.
func (g *Game) burnPlayer(p PlayerID, n int) []Event {
	var ev []Event
	pl := &g.State.Players[p]
	for i := 0; i < n; i++ {
		pl.Burn++
		pl.HP--
		ev = append(ev, Event{Kind: EvBurn, Player: p, Amount: 1}, Event{Kind: EvHPLoss, Player: p, Amount: 1})
	}
	return ev
}

// burnDamage is what burning does at the start of p's turn: each burning Creature of theirs takes 1 damage per
// token, and so does p if they are burning.
func (g *Game) burnDamage(p PlayerID) []Event {
	var ev []Event
	for lane := range g.State.Players[p].Lanes {
		at := pos{p, lane}
		if cr := g.creatureAt(at); cr != nil && cr.Burn > 0 {
			if dealt := g.hurt(at, cr.Burn); dealt > 0 {
				ev = append(ev, Event{Kind: EvDamage, Player: p, Card: cr.UID, Lane: lane, Amount: dealt, Cause: CauseBurn})
			}
		}
	}
	if pl := &g.State.Players[p]; pl.Burn > 0 {
		pl.HP -= pl.Burn
		ev = append(ev, Event{Kind: EvHPLoss, Player: p, Amount: pl.Burn, Cause: CauseBurn})
	}
	return ev
}

// burnTokens counts the burn tokens on the Creatures of the given players.
func (g *Game) burnTokens(players []PlayerID) int {
	n := 0
	for _, p := range players {
		for _, l := range g.State.Players[p].Lanes {
			if l.Creature != nil {
				n += l.Creature.Burn
			}
		}
	}
	return n
}

// withVictim runs fn with at as the Creature the triggers are about.
func (g *Game) withVictim(at pos, fn func() []Event) []Event {
	prev := g.victim
	g.victim = &at
	defer func() { g.victim = prev }()
	return fn()
}

// fireBurns runs the you_burn triggers of everything player by controls: they have just burned the Creature at at.
func (g *Game) fireBurns(by PlayerID, at pos) []Event {
	return g.withVictim(at, func() []Event {
		ev := g.fireHero(by, card.OnYouBurn)
		for i := range g.State.Players[by].Lanes {
			l := &g.State.Players[by].Lanes[i]
			here := pos{by, i}
			if l.Creature != nil {
				ev = append(ev, g.fire(card.OnYouBurn, effCtx{p: by, at: here, hasSrc: true, creature: l.Creature}, g.effectsOf(l.Creature), nil)...)
			}
			for _, b := range l.buildings() {
				ev = append(ev, g.fire(card.OnYouBurn, effCtx{p: by, at: here, hasSrc: true, card: b.CardID, building: b}, g.buildingEffects(here, b), nil)...)
			}
		}
		return ev
	})
}

// fireFoeLeaves runs the foe_burning_leaves triggers of everything the opponent of the burning Creature cr that has
// just left play controls.
func (g *Game) fireFoeLeaves(cr *Creature, side PlayerID) []Event {
	if cr.Burn == 0 {
		return nil
	}
	foe := side.Opp()
	g.State.LeftBurn = cr.Burn
	ev := g.fireHero(foe, card.OnFoeBurningLeaves)
	for i := range g.State.Players[foe].Lanes {
		l := &g.State.Players[foe].Lanes[i]
		here := pos{foe, i}
		if l.Creature != nil {
			ev = append(ev, g.fire(card.OnFoeBurningLeaves, effCtx{p: foe, at: here, hasSrc: true, creature: l.Creature}, g.effectsOf(l.Creature), nil)...)
		}
		for _, b := range l.buildings() {
			ev = append(ev, g.fire(card.OnFoeBurningLeaves, effCtx{p: foe, at: here, hasSrc: true, card: b.CardID, building: b}, g.buildingEffects(here, b), nil)...)
		}
	}
	return ev
}

// attackHero makes the Fight that is being started hit the opposing player instead of the Creature across.
func (g *Game) attackHero() {
	if g.State.Attacking != nil {
		g.State.Attacking.Hero = true
	}
}
