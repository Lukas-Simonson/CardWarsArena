package engine

import (
	"slices"

	"cardwars/internal/card"
)

// fire runs the triggers of one card for an event. A trigger whose mandatory
// target has no legal choice does nothing.
func (g *Game) fire(on string, ctx effCtx, effects []card.Effect, given [][]TargetRef) []Event {
	var ev []Event
	// A Creature entering or leaving play opposite a Palace of Bone triggers nothing.
	if (on == card.OnEnterPlay || on == card.OnLeavePlay) && ctx.creature != nil && g.blocksTriggers(ctx.at.P, ctx.at.Lane) {
		return nil
	}
	for _, e := range effects {
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
func (g *Game) fireTurnStart(p PlayerID) []Event { return g.fireStart(p, card.OnTurnStart) }

// fireStart runs the triggers for the start of a turn or phase (turn_start, fight_start) of everything p controls.
func (g *Game) fireStart(p PlayerID, on string) []Event {
	type item struct {
		effects []card.Effect
		ctx     effCtx
	}
	var items []item
	for i := range g.State.Players[p].Lanes {
		l := &g.State.Players[p].Lanes[i]
		if l.Creature != nil {
			items = append(items, item{g.effectsOf(l.Creature), effCtx{p: p, at: pos{p, i}, hasSrc: true, creature: l.Creature}})
		}
		for _, b := range l.buildings() {
			items = append(items, item{g.buildingEffects(pos{p, i}, b), effCtx{p: p, at: pos{p, i}, hasSrc: true, card: b.CardID}})
		}
	}
	var ev []Event
	for _, it := range items {
		ev = append(ev, g.fire(on, it.ctx, it.effects, nil)...)
	}
	return ev
}

// fireWatchers runs the creature_enters triggers of everything the controller
// of the Creature that just entered play at (p, lane) has in play.
func (g *Game) fireWatchers(p PlayerID, lane int) []Event {
	ent := g.creatureAt(pos{p, lane})
	if ent == nil || g.blocksTriggers(p, lane) {
		return nil
	}
	var ev []Event
	for i := range g.State.Players[p].Lanes {
		l := &g.State.Players[p].Lanes[i]
		at := pos{p, i}
		if l.Creature != nil {
			ev = append(ev, g.watch(g.effectsOf(l.Creature), effCtx{p: p, at: at, hasSrc: true, creature: l.Creature, entering: &pos{p, lane}}, ent)...)
		}
		for _, b := range l.buildings() {
			ev = append(ev, g.watch(g.buildingEffects(at, b), effCtx{p: p, at: at, hasSrc: true, entering: &pos{p, lane}}, ent)...)
		}
	}
	return ev
}

func (g *Game) watch(effects []card.Effect, ctx effCtx, ent *Creature) []Event {
	var ev []Event
	for i, e := range effects {
		if e.Kind != card.KindTrigger || e.On != card.OnCreatureEnters {
			continue
		}
		if w := e.Watch; w != nil {
			if (w.Landscape != "" && !g.isType(*ctx.entering, w.Landscape)) ||
				(w.Other && ctx.creature == ent) ||
				(w.YourTurn && g.State.Active != ctx.p) {
				continue
			}
		}
		if e.OncePerTurn && ctx.creature != nil {
			if slices.Contains(ctx.creature.Fired, i) {
				continue
			}
			ctx.creature.Fired = append(ctx.creature.Fired, i)
		}
		ev = append(ev, g.runOps(e.Do, ctx, nil)...)
	}
	return ev
}
