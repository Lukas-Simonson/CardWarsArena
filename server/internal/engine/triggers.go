package engine

import "cardwars/internal/card"

// fire runs the triggers of one card for an event. A trigger whose mandatory
// target has no legal choice does nothing.
func (g *Game) fire(on string, ctx effCtx, in Instance, given [][]TargetRef) []Event {
	var ev []Event
	for _, e := range g.Card(in).Effects {
		if e.Kind != card.KindTrigger || e.On != on {
			continue
		}
		usable, err := g.checkTargets(e.Targets, given, ctx)
		if !usable || err != nil {
			continue
		}
		ev = append(ev, g.runOps(e.Do, ctx, given)...)
	}
	return ev
}

// fireTurnStart runs the turn_start triggers of everything p controls.
func (g *Game) fireTurnStart(p PlayerID) []Event {
	type item struct {
		in  Instance
		ctx effCtx
	}
	var items []item
	for i := range g.State.Players[p].Lanes {
		l := &g.State.Players[p].Lanes[i]
		if l.Creature != nil {
			items = append(items, item{l.Creature.Instance, effCtx{p: p, at: pos{p, i}, hasSrc: true, creature: l.Creature}})
		}
		if l.Building != nil {
			items = append(items, item{l.Building.Instance, effCtx{p: p, at: pos{p, i}, hasSrc: true}})
		}
	}
	var ev []Event
	for _, it := range items {
		ev = append(ev, g.fire(card.OnTurnStart, it.ctx, it.in, nil)...)
	}
	return ev
}
