package engine

import "cardwars/internal/card"

// LegalActions lists every action p could legally take right now. It is built
// by proposing candidates and keeping those Validate accepts, so it can never
// disagree with Apply. Spells are offered once per card, with Lane 0.
func (g *Game) LegalActions(p PlayerID) []Action {
	var cands []Action
	switch g.State.Phase {
	case PhaseMulligan:
		cands = append(cands, Action{Kind: ActKeepHand}, Action{Kind: ActMulligan})
	case PhaseMain:
		cands = append(cands, Action{Kind: ActDraw}, Action{Kind: ActEndMain})
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
const maxTargetCombos = 64

// spellActions proposes the plays of a Spell, one per target choice.
func (g *Game) spellActions(p PlayerID, in Instance, c *card.Card) []Action {
	e := spellEffect(c)
	if e == nil || len(e.Targets) == 0 {
		return []Action{{Kind: ActPlayCard, Card: in.UID}}
	}
	var out []Action
	for _, targets := range g.targetCombos(e.Targets, effCtx{p: p}, maxTargetCombos) {
		out = append(out, Action{Kind: ActPlayCard, Card: in.UID, Targets: targets})
	}
	return out
}

// laneActions proposes the plays of a Creature or Building into one Lane, one
// per target choice of its enter-play trigger.
func (g *Game) laneActions(p PlayerID, in Instance, lane int) []Action {
	plain := Action{Kind: ActPlayCard, Card: in.UID, Lane: lane}
	e := enterEffect(g.Card(in))
	if e == nil || len(e.Targets) == 0 {
		return []Action{plain}
	}
	if _, _, err := g.checkPlayable(p, plain); err != nil {
		return []Action{plain} // not playable anyway; Validate will say why
	}
	h := g.Clone()
	h.place(p, in, lane)
	ctx := h.placedCtx(p, lane)
	combos := h.targetCombos(e.Targets, ctx, maxTargetCombos)
	if len(combos) == 0 {
		return []Action{plain} // nothing to target: the trigger does nothing
	}
	var out []Action
	for _, targets := range combos {
		out = append(out, Action{Kind: ActPlayCard, Card: in.UID, Lane: lane, Targets: targets})
	}
	return out
}
