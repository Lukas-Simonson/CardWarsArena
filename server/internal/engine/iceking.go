package engine

import "cardwars/internal/card"

// Triggers that ask their controller to choose (Crystal Palace, Frozen Fish), buildings that stop frozen
// Creatures (Ice-olation Cell) and what happens once a Creature has moved (Boarder Collie).

// fireFrozen runs the landscape_frozen triggers of the Buildings in lane, on both sides: a Landscape in
// their Lane has just got a frozen token.
func (g *Game) fireFrozen(lane int) []Event {
	var ev []Event
	for p := PlayerID(0); p < 2; p++ {
		at := pos{p, lane}
		for _, b := range g.State.Players[p].Lanes[lane].buildings() {
			ev = append(ev, g.fire(card.OnLandscapeFrozen, effCtx{p: p, at: at, hasSrc: true, card: b.CardID}, g.buildingEffects(at, b), nil)...)
		}
	}
	return ev
}

// askTrigger resolves a trigger that has a target to choose. With nothing to choose it does nothing, and with
// one forced choice it just happens; otherwise its controller is asked (and may decline an optional target).
func (g *Game) askTrigger(e card.Effect, ctx effCtx) []Event {
	spec := e.Targets[0]
	cands := g.candidates(spec, ctx, nil)
	if len(cands) == 0 {
		return nil
	}
	if len(cands) == 1 && !spec.Optional {
		return g.runOps(e.Do, ctx, [][]TargetRef{{cands[0]}})
	}
	uid := 0
	if ctx.creature != nil {
		uid = ctx.creature.UID
	}
	g.ask(Choice{Player: ctx.p, Kind: ChoiceTarget, Card: ctx.cardID(), Lane: ctx.at.Lane, SourceUID: uid, MoveSide: ctx.at.P,
		Targets: cands, Optional: spec.Optional, Label: spec.Label, Do: e.Do})
	return nil
}

// answerTarget carries out a target Choice. The card that asked may have left play meanwhile (Frozen Fish that
// died in the same Fight): what it was going to do still happens.
func (g *Game) answerTarget(ch *Choice, a Action) []Event {
	if len(a.Targets) != 1 || len(a.Targets[0]) != 1 {
		return nil
	}
	ctx := effCtx{p: ch.Player, at: pos{ch.MoveSide, ch.Lane}, hasSrc: true, card: ch.Card}
	if cr := g.creatureAt(ctx.at); cr != nil && cr.UID == ch.SourceUID && ch.SourceUID != 0 {
		ctx.creature, ctx.card = cr, ""
	}
	return g.runOps(ch.Do, ctx, a.Targets)
}

// afterMove runs what a moving Creature does once it has arrived at to (Boarder Collie).
func (g *Game) afterMove(ops []card.Op, p PlayerID, to pos, cr *Creature) []Event {
	if len(ops) == 0 || cr == nil || g.creatureAt(to) != cr {
		return nil
	}
	return g.runOps(ops, effCtx{p: p, at: to, hasSrc: true, creature: cr}, nil)
}

// inertAt reports whether the Creature at at cannot Fight or use abilities: its Landscape is frozen and a
// Building in its Lane, on either side, says so.
func (g *Game) inertAt(at pos) bool {
	if !g.frozen(at) {
		return false
	}
	for p := PlayerID(0); p < 2; p++ {
		for _, b := range g.State.Players[p].Lanes[at.Lane].buildings() {
			if hasRestrict(g.buildingEffects(pos{p, at.Lane}, b), card.RestrictFrozenInert) {
				return true
			}
		}
	}
	return false
}
