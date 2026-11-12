package engine

import "cardwars/internal/card"

// abilitySource resolves the card an ability action refers to.
type abilitySource struct {
	ctx     effCtx
	effects []card.Effect
	status  *Status
	// real counts the card's own activated abilities; offers follow them.
	real int
	// own counts the card's own effects; a Creature's effects after them were granted to it (Finn's Hero).
	own int
	// fired records the uses of limited abilities this turn; nil for Buildings.
	fired *[]int
}

func (g *Game) abilitySource(p PlayerID, a Action) (abilitySource, error) {
	if a.Source == "hero" {
		effects := g.heroEffects(p)
		if len(effects) == 0 {
			return abilitySource{}, ErrNoAbility
		}
		return abilitySource{ctx: g.heroCtx(p), effects: effects, real: countActivates(effects), fired: &g.State.Players[p].HeroFired}, nil
	}
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
		src := abilitySource{ctx: ctx, effects: effects, status: &l.Creature.Status, real: countActivates(effects), fired: &l.Creature.Fired, own: len(g.Card(l.Creature.Instance).Effects)}
		if l.Creature.Silenced {
			src.own = len(effects)
		}
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

// abilityEffect finds the Index-th Floop ability or the Index-th activated ability. A card may have several
// Floop abilities (its own, and one its Hero grants it); using one Floops the card, so only one is used.
func abilityEffect(effects []card.Effect, a Action) (*card.Effect, int) {
	floops, activates := 0, 0
	for i := range effects {
		e := &effects[i]
		switch {
		case a.Kind == ActFloop && e.Kind == card.KindFloop:
			if floops == a.Index {
				return e, i
			}
			floops++
		case a.Kind == ActActivate && e.Kind == card.KindActivate:
			if activates == a.Index {
				return e, i
			}
			activates++
		}
	}
	return nil, 0
}

// usesLeft reports whether effect i may be used again this turn.
func usesLeft(fired *[]int, e *card.Effect, i int) bool {
	if e.Limit == 0 || fired == nil {
		return true
	}
	used := 0
	for _, f := range *fired {
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
	if !usesLeft(src.fired, e, idx) {
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
	if e.Limit > 0 && src.fired != nil {
		*src.fired = append(*src.fired, idx)
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
	var uid int
	switch {
	case src.ctx.creature != nil:
		uid = src.ctx.creature.UID
	case src.ctx.hero:
		uid = g.State.Players[p].Hero.UID
	default:
		uid = g.State.Players[p].Lanes[a.Lane].Building.UID
	}
	ev := []Event{{Kind: kind, Player: p, Card: uid, Lane: a.Lane}}
	return append(ev, g.runOps(e.Do, src.ctx, a.Targets)...)
}

// abilityActions proposes every Floop and activated ability p could use.
func (g *Game) abilityActions(p PlayerID) []Action {
	out := g.abilitiesOf(p, Action{Source: "hero"}, nil)
	for lane := 0; lane < NumLanes; lane++ {
		for _, source := range []string{"creature", "building"} {
			out = g.abilitiesOf(p, Action{Lane: lane, Source: source}, out)
		}
	}
	return out
}

// abilitiesOf adds to out every ability of the card that probe names.
func (g *Game) abilitiesOf(p PlayerID, probe Action, out []Action) []Action {
	src, err := g.abilitySource(p, probe)
	if err != nil {
		return out
	}
	activate, floop := 0, 0
	for i, e := range src.effects {
		a := Action{Lane: probe.Lane, Source: probe.Source}
		switch e.Kind {
		case card.KindFloop:
			a.Kind, a.Index = ActFloop, floop
			floop++
			if i >= src.own && src.ctx.creature != nil {
				a.Via = "hero" // granted by the Hero, not printed on the card
			}
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
