package engine

import "cardwars/internal/card"

// LegalActions lists every action p could legally take right now. It is built
// by proposing candidates and keeping those Validate accepts, so it can never
// disagree with Apply. Spells are offered once per card, with Lane 0.
func (g *Game) LegalActions(p PlayerID) []Action {
	if g.State.Choice != nil {
		var legal []Action
		for _, a := range g.choiceActions(p) {
			if g.Validate(p, a) == nil {
				legal = append(legal, a)
			}
		}
		return legal
	}
	if g.State.Pending != nil {
		if p != g.State.Active || g.State.Phase == PhaseOver {
			return nil
		}
		return g.pendingActions(p)
	}
	var cands []Action
	switch g.State.Phase {
	case PhaseMulligan:
		cands = append(cands, Action{Kind: ActKeepHand}, Action{Kind: ActMulligan})
	case PhaseMain:
		cands = append(cands, Action{Kind: ActDraw}, Action{Kind: ActEndMain})
		cands = append(cands, g.abilityActions(p)...)
		for _, in := range g.State.Players[p].Hand {
			if c := g.Card(in); c.Type == card.Spell {
				cands = append(cands, g.spellActions(p, in, c)...)
				continue
			}
			for lane := 0; lane < NumLanes; lane++ {
				cands = append(cands, g.laneActions(p, in, lane)...)
			}
		}
	case PhaseFight:
		for lane := 0; lane < NumLanes; lane++ {
			cands = append(cands, Action{Kind: ActAttack, Lane: lane})
		}
	}
	var legal []Action
	for _, a := range cands {
		if g.Validate(p, a) == nil {
			legal = append(legal, a)
		}
	}
	return legal
}

// maxTargetCombos caps how many target choices LegalActions lists per card.
// Clients choose targets interactively and are not limited by this.
const maxTargetCombos = 128

// spellActions proposes the plays of a Spell, one per target choice.
func (g *Game) spellActions(p PlayerID, in Instance, c *card.Card) []Action {
	e := spellEffect(c)
	choices := [][][]TargetRef{nil}
	if e != nil && len(e.Targets) > 0 {
		choices = g.targetCombos(e.Targets, effCtx{p: p}, maxTargetCombos)
	}
	var out []Action
	for _, cost := range g.costChoices(p, in) {
		for _, targets := range choices {
			out = append(out, Action{Kind: ActPlayCard, Card: in.UID, CostTargets: cost, Targets: targets, Labels: spellLabels(e)})
		}
	}
	return out
}

func spellLabels(e *card.Effect) []string {
	if e == nil {
		return nil
	}
	return labelsOf(e.Targets)
}

// costChoices lists the ways to pay a card's additional cost; a card with no
// additional cost has the single choice of paying nothing.
func (g *Game) costChoices(p PlayerID, in Instance) [][][]TargetRef {
	ce := costEffect(g.Card(in))
	if ce == nil {
		return [][][]TargetRef{nil}
	}
	return g.targetCombos(ce.Targets, effCtx{p: p, playing: in.UID}, maxTargetCombos)
}

// laneActions proposes the plays of a Creature or Building into one Lane, one
// per choice of additional cost and enter-play target.
func (g *Game) laneActions(p PlayerID, in Instance, lane int) []Action {
	costs := g.costChoices(p, in)
	if len(costs) == 0 {
		return []Action{{Kind: ActPlayCard, Card: in.UID, Lane: lane}} // cannot pay; Validate says why
	}
	e := enterEffect(g.Card(in))
	triggerChoices := [][][]TargetRef{nil}
	if e != nil && len(e.Targets) > 0 {
		plain := Action{Kind: ActPlayCard, Card: in.UID, Lane: lane}
		if _, _, err := g.checkPlayable(p, plain); err == nil {
			h := g.Clone()
			h.place(p, in, lane)
			if combos := h.targetCombos(e.Targets, h.placedCtx(p, lane), maxTargetCombos); len(combos) > 0 {
				triggerChoices = combos // otherwise nothing to target: the trigger does nothing
			}
		}
	}
	var out []Action
	for _, cost := range costs {
		for _, targets := range triggerChoices {
			out = append(out, Action{Kind: ActPlayCard, Card: in.UID, Lane: lane, CostTargets: cost, Targets: targets, Labels: spellLabels(e)})
		}
	}
	return out
}
