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
			if g.Card(in).Type == card.Spell {
				cands = append(cands, Action{Kind: ActPlayCard, Card: in.UID})
				continue
			}
			for lane := 0; lane < NumLanes; lane++ {
				cands = append(cands, Action{Kind: ActPlayCard, Card: in.UID, Lane: lane})
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
