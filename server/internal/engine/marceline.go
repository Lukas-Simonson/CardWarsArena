package engine

import (
	"math/rand/v2"

	"cardwars/internal/card"
)

// Discarding as a cost of something else, and what reacts to it (Marceline's deck).

// fireOpponentDiscard runs the opponent_discards triggers of everything the opponent of p, who just discarded a
// card from their hand, has in play.
func (g *Game) fireOpponentDiscard(p PlayerID) []Event {
	q := p.Opp()
	var ev []Event
	for lane := range g.State.Players[q].Lanes {
		at := pos{q, lane}
		l := &g.State.Players[q].Lanes[lane]
		if l.Creature != nil {
			ev = append(ev, g.fire(card.OnOpponentDiscards, effCtx{p: q, at: at, hasSrc: true, creature: l.Creature}, g.effectsOf(l.Creature), nil)...)
		}
		for _, b := range l.buildings() {
			ev = append(ev, g.fire(card.OnOpponentDiscards, effCtx{p: q, at: at, hasSrc: true, card: b.CardID}, g.buildingEffects(at, b), nil)...)
		}
	}
	return ev
}

// discardRandom makes p discard n cards at random (Bog Bum).
func (g *Game) discardRandom(p PlayerID, n int) []Event {
	var ev []Event
	for ; n > 0 && len(g.State.Players[p].Hand) > 0; n-- {
		hand := g.State.Players[p].Hand
		ev = append(ev, g.discardFromHand(p, hand[rand.New(&g.rng).IntN(len(hand))].UID)...)
	}
	return ev
}

// handCost reports the cost of a card, or -1 when it has none.
func (g *Game) handCost(in Instance) int {
	if c := g.Card(in).Cost; c != nil {
		return *c
	}
	return -1
}

// discardCost makes p discard a card of exactly the given cost, or show their hand if they have none (Fly Swatter).
func (g *Game) discardCost(p PlayerID, cost int) []Event {
	var match []Instance
	for _, in := range g.State.Players[p].Hand {
		if g.handCost(in) == cost {
			match = append(match, in)
		}
	}
	switch len(match) {
	case 0:
		var ev []Event
		for _, in := range g.State.Players[p].Hand {
			ev = append(ev, Event{Kind: EvReveal, Player: p, Card: in.UID})
		}
		return ev
	case 1:
		return g.discardFromHand(p, match[0].UID)
	}
	g.ask(Choice{Player: p, Kind: ChoiceDiscard, Count: 1, OnlyCost: &cost})
	return nil
}

// mayDiscard (Blood Bath, Bums Away!) lets p discard a card of at least the given cost; if they do not, the
// ops in else happen to them. With no such card in hand they happen at once.
func (g *Game) mayDiscard(p PlayerID, op card.Op, c effCtx) []Event {
	minCost := 0
	if op.Cost != nil {
		minCost = *op.Cost
	}
	can := false
	for _, in := range g.State.Players[p].Hand {
		can = can || g.handCost(in) >= minCost
	}
	if !can {
		return g.runOps(op.Else, c, nil)
	}
	uid := 0
	if c.creature != nil {
		uid = c.creature.UID
	}
	g.ask(Choice{Player: p, Kind: ChoiceRansom, Card: c.cardID(), Lane: c.at.Lane, MoveSide: c.at.P, SourceUID: uid, MinCost: &minCost, Do: op.Else, hasSrc: c.hasSrc})
	return nil
}

// answerRansom discards the card chosen, or lets the ops that were waiting happen to the player.
func (g *Game) answerRansom(ch *Choice, a Action) []Event {
	if a.Card != 0 {
		return g.discardFromHand(ch.Player, a.Card)
	}
	return g.runOps(ch.Do, g.ctxOfChoice(ch, ch.Player.Opp()), nil)
}

// askAmount stops the ops at the op that has Options: its controller picks the amount, and the ops from it on
// run when they have.
func (g *Game) askAmount(ops []card.Op, c effCtx) {
	uid := 0
	if c.creature != nil {
		uid = c.creature.UID
	}
	unit := ""
	switch ops[0].Do {
	case "damage_creature", "damage_hero":
		unit = "damage"
	case "heal_creature", "heal_hero":
		unit = "heal"
	}
	g.ask(Choice{Player: c.p, Kind: ChoiceAmount, Card: c.cardID(), Lane: c.at.Lane, MoveSide: c.at.P, SourceUID: uid,
		Options: ops[0].Options, Unit: unit, Do: ops, hasSrc: c.hasSrc})
}

// answerAmount runs the waiting ops with the number picked.
func (g *Game) answerAmount(ch *Choice, n int) []Event {
	ops := append([]card.Op(nil), ch.Do...)
	ops[0].Options, ops[0].Amount = nil, &card.Value{N: n}
	return g.runOps(ops, g.ctxOfChoice(ch, ch.Player), nil)
}

// moveAllDamage puts all the damage on the Creature at from onto the one at to (Bag Ban-She Angel).
func (g *Game) moveAllDamage(from, to pos) []Event {
	a, b := g.creatureAt(from), g.creatureAt(to)
	if a == nil || b == nil || a == b || a.Damage == 0 {
		return nil
	}
	n := a.Damage
	a.Damage = 0
	b.Damage += n
	return []Event{
		{Kind: EvHeal, Player: from.P, Card: a.UID, Lane: from.Lane, Amount: n},
		{Kind: EvDamage, Player: to.P, Card: b.UID, Lane: to.Lane, Amount: n},
	}
}

// removeCard takes the card out of a discard pile and out of the game (Blood Transfusion).
func (g *Game) removeCard(r TargetRef) []Event {
	in, ok := g.takeFromDiscard(r.Player, r.Card)
	if !ok {
		return nil
	}
	g.removedCost = max(g.handCost(in), 0)
	g.State.Players[in.Owner].Removed = append(g.State.Players[in.Owner].Removed, in)
	return []Event{{Kind: EvRemove, Player: in.Owner, Card: in.UID}}
}
