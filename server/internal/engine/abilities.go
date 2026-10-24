package engine

import "cardwars/internal/card"

// abilitySource resolves the card an ability action refers to.
type abilitySource struct {
	ctx     effCtx
	effects []card.Effect
	status  *Status
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
		return abilitySource{ctx, g.effectsOf(l.Creature), &l.Creature.Status}, nil
	case "building":
		if l.Building == nil {
			return abilitySource{}, ErrNoAbility
		}
		return abilitySource{ctx, g.Card(l.Building.Instance).Effects, &l.Building.Status}, nil
	}
	return abilitySource{}, ErrNoAbility
}

// abilityEffect finds the Floop effect, or the Index-th activated ability.
func abilityEffect(effects []card.Effect, a Action) *card.Effect {
	n := 0
	for i := range effects {
		e := &effects[i]
		switch {
		case a.Kind == ActFloop && e.Kind == card.KindFloop:
			return e
		case a.Kind == ActActivate && e.Kind == card.KindActivate:
			if n == a.Index {
				return e
			}
			n++
		}
	}
	return nil
}

func (g *Game) validateAbility(p PlayerID, a Action) error {
	if err := g.requireMain(p); err != nil {
		return err
	}
	src, err := g.abilitySource(p, a)
	if err != nil {
		return err
	}
	e := abilityEffect(src.effects, a)
	if e == nil {
		return ErrNoAbility
	}
	if a.Kind == ActFloop {
		if g.State.Turn == 1 {
			return ErrFirstTurn
		}
		if *src.status != Ready {
			return ErrNotReady
		}
	} else if g.State.ActionsLeft < e.Cost {
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
	e := abilityEffect(src.effects, a)
	kind := EvActivate
	if a.Kind == ActFloop {
		*src.status = Flooped
		kind = EvFloop
	} else {
		g.State.ActionsLeft -= e.Cost
	}
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
					out = append(out, b)
				}
			}
		}
	}
	return out
}
