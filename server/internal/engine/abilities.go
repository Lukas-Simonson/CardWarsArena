package engine

import "cardwars/internal/card"

// abilitySource resolves the card an ability action refers to.
type abilitySource struct {
	ctx     effCtx
	effects []card.Effect
	status  *Status
	// real counts the card's own activated abilities; offers follow them.
	real int
}

func (g *Game) abilitySource(p PlayerID, a Action) (abilitySource, error) {
	if a.Lane < 0 || a.Lane >= NumLanes {
		return abilitySource{}, ErrBadLane
	}
	l := &g.State.Players[p].Lanes[a.Lane]
	ctx := effCtx{p: p, at: pos{p, a.Lane}, hasSrc: true}
	switch a.Source {
	case "", "creature":
		if l.Creature == nil {
			return abilitySource{}, ErrNoAbility
		}
		ctx.creature = l.Creature
		effects := g.effectsOf(l.Creature)
		src := abilitySource{ctx: ctx, effects: effects, status: &l.Creature.Status, real: countActivates(effects)}
		if !l.Creature.Silenced {
			// Offers are free activated abilities after the card's own.
			src.effects = append(append([]card.Effect(nil), effects...), offerEffects(l.Creature.Offered)...)
		}
		return src, nil
	case "building":
		if l.Building == nil {
			return abilitySource{}, ErrNoAbility
		}
		ctx.card = l.Building.CardID
		effects := g.buildingEffects(ctx.at, l.Building)
		return abilitySource{ctx: ctx, effects: effects, status: &l.Building.Status, real: countActivates(effects)}, nil
	}
	return abilitySource{}, ErrNoAbility
}

func countActivates(effects []card.Effect) int {
	n := 0
	for _, e := range effects {
		if e.Kind == card.KindActivate {
			n++
		}
	}
	return n
}

func offerEffects(offers []Offer) []card.Effect {
	out := make([]card.Effect, len(offers))
	for i, o := range offers {
		out[i] = card.Effect{Kind: card.KindActivate, Targets: o.Targets, Do: o.Then}
	}
	return out
}

// usesOffer reports whether a is the use of an Offer.
func (g *Game) usesOffer(p PlayerID, a Action) bool {
	if a.Kind != ActActivate {
		return false
	}
	src, err := g.abilitySource(p, a)
	return err == nil && a.Index >= src.real
}

// clearOffers withdraws every Offer: the moment to use them has passed.
func (g *Game) clearOffers() {
	for p := range g.State.Players {
		for i := range g.State.Players[p].Lanes {
			if c := g.State.Players[p].Lanes[i].Creature; c != nil {
				c.Offered = nil
			}
		}
	}
}

// abilityEffect finds the Floop effect, or the Index-th activated ability.
func abilityEffect(effects []card.Effect, a Action) (*card.Effect, int) {
	n := 0
	for i := range effects {
		e := &effects[i]
		switch {
		case a.Kind == ActFloop && e.Kind == card.KindFloop:
			return e, i
		case a.Kind == ActActivate && e.Kind == card.KindActivate:
			if n == a.Index {
				return e, i
			}
			n++
		}
	}
	return nil, 0
}

// usesLeft reports whether a Creature may use effect i again this turn.
func usesLeft(cr *Creature, e *card.Effect, i int) bool {
	if e.Limit == 0 || cr == nil {
		return true
	}
	used := 0
	for _, f := range cr.Fired {
		if f == i {
			used++
		}
	}
	return used < e.Limit
}

func (g *Game) validateAbility(p PlayerID, a Action) error {
	if err := g.requireMain(p); err != nil {
		return err
	}
	src, err := g.abilitySource(p, a)
	if err != nil {
		return err
	}
	e, idx := abilityEffect(src.effects, a)
	if e == nil {
		return ErrNoAbility
	}
	if !usesLeft(src.ctx.creature, e, idx) {
		return ErrLimit
	}
	if e.If != nil && !g.cond(*e.If, src.ctx.source()) {
		return ErrCondition
	}
	if a.Kind == ActFloop {
		if g.State.Turn == 1 {
			return ErrFirstTurn
		}
		if src.ctx.creature != nil && g.floopsBlocked(a.Lane) {
			return ErrNoFloop
		}
		if *src.status != Ready {
			return ErrNotReady
		}
	}
	if g.State.ActionsLeft < e.Cost {
		// Paying Actions for an ability needs no particular Landscape.
		return ErrNoActions
	}
	usable, err := g.checkTargets(e.Targets, a.Targets, src.ctx)
	if !usable {
		return ErrNoTarget
	}
	return err
}

// doAbility pays the cost of a validated ability and resolves it.
func (g *Game) doAbility(p PlayerID, a Action) []Event {
	src, _ := g.abilitySource(p, a)
	e, idx := abilityEffect(src.effects, a)
	if e.Limit > 0 && src.ctx.creature != nil {
		src.ctx.creature.Fired = append(src.ctx.creature.Fired, idx)
	}
	kind := EvActivate
	if a.Kind == ActFloop {
		*src.status = Flooped
		kind = EvFloop
	}
	if a.Kind == ActActivate && a.Index >= src.real {
		// An Offer is used up, but the Creature's other Offers stay.
		cr := src.ctx.creature
		i := a.Index - src.real
		cr.Offered = append(append([]Offer(nil), cr.Offered[:i]...), cr.Offered[i+1:]...)
	}
	g.State.ActionsLeft -= e.Cost // a Floop may cost Actions too (Spirit Tower)
	uid := 0
	if src.ctx.creature != nil {
		uid = src.ctx.creature.UID
	} else {
		uid = g.State.Players[p].Lanes[a.Lane].Building.UID
	}
	ev := []Event{{Kind: kind, Player: p, Card: uid, Lane: a.Lane}}
	return append(ev, g.runOps(e.Do, src.ctx, a.Targets)...)
}

// abilityActions proposes every Floop and activated ability p could use.
func (g *Game) abilityActions(p PlayerID) []Action {
	var out []Action
	for lane := 0; lane < NumLanes; lane++ {
		for _, source := range []string{"creature", "building"} {
			probe := Action{Lane: lane, Source: source}
			src, err := g.abilitySource(p, probe)
			if err != nil {
				continue
			}
			activate := 0
			for _, e := range src.effects {
				a := Action{Lane: lane, Source: source}
				switch e.Kind {
				case card.KindFloop:
					a.Kind = ActFloop
				case card.KindActivate:
					a.Kind, a.Index = ActActivate, activate
					activate++
				default:
					continue
				}
				if len(e.Targets) == 0 {
					out = append(out, a)
					continue
				}
				for _, targets := range g.targetCombos(e.Targets, src.ctx, maxTargetCombos) {
					b := a
					b.Targets = targets
					b.Labels = labelsOf(e.Targets)
					out = append(out, b)
				}
			}
		}
	}
	return out
}

// floopsBlocked reports whether a Creature in a Lane may not use Floop
// abilities because of a Creature in that Lane on either side (Woadic Enchantress).
func (g *Game) floopsBlocked(lane int) bool {
	for p := PlayerID(0); p < 2; p++ {
		if cr := g.State.Players[p].Lanes[lane].Creature; cr != nil && hasRestrict(g.effectsOf(cr), card.RestrictLaneNoFloop) {
			return true
		}
	}
	return false
}
