package bot

import (
	"math/rand/v2"

	"cardwars/internal/engine"
)

// Heuristic looks one action ahead: it tries every legal action on a copy of the
// game, scores the position that results, and plays the best. It does not look
// at the opponent's hand or either deck, but the copy plays out hidden draws and
// random effects exactly as the real game will, so those are not hidden from it.
type Heuristic struct{}

// Weights of the position score, in "points": one point is about a Hit Point.
const (
	winScore       = 1e6
	perATK         = 1.0  // each point of a Creature's ATK
	perDEF         = 0.5  // each point of DEF left (DEF minus damage)
	perCreature    = 1.0  // having a Creature at all
	readyBonus     = 0.5  // a Ready Creature can still Fight
	perBuilding    = 2.0  // each Building in play
	perCardInHand  = 1.0  // base value of a card in hand
	perCostInHand  = 0.5  // plus this for each point of cost: dear cards are worth keeping
	perActionLeft  = 0.3  // an unspent Action is worth a little, so it takes a gain to spend one
	perOppHandCard = 0.5  // each card the opponent holds
	noise          = 0.01 // breaks ties so equal moves vary
)

// Choose returns the best action p has, or false if p has none.
func (Heuristic) Choose(g *engine.Game, p engine.PlayerID, rng *rand.Rand) (engine.Action, bool) {
	legal := g.LegalActions(p)
	if len(legal) == 0 {
		return engine.Action{}, false
	}
	if g.State.Phase == engine.PhaseMulligan {
		return chooseMulligan(g, p, legal), true
	}
	if len(legal) == 1 {
		return legal[0], true
	}
	best, bestScore := legal[0], -2*winScore
	for _, a := range legal {
		h := g.Clone()
		if _, err := h.Apply(p, a); err != nil {
			continue
		}
		if score := evaluate(h, p) + rng.Float64()*noise; score > bestScore {
			best, bestScore = a, score
		}
	}
	return best, true
}

// evaluate scores the game for player p: higher is better for p.
func evaluate(g *engine.Game, p engine.PlayerID) float64 {
	st := &g.State
	if st.Phase == engine.PhaseOver {
		if st.Winner == p {
			return winScore
		}
		return -winScore
	}
	score := float64(st.Players[p].HP - st.Players[p.Opp()].HP)
	score += side(g, p) - side(g, p.Opp())
	for _, in := range st.Players[p].Hand {
		score += perCardInHand
		if c := g.Card(in); c.Cost != nil {
			score += perCostInHand * float64(*c.Cost)
		}
	}
	score -= perOppHandCard * float64(len(st.Players[p.Opp()].Hand))
	if st.Active == p {
		score += perActionLeft * float64(st.ActionsLeft)
	}
	return score
}

// side is what one player has on the board.
func side(g *engine.Game, p engine.PlayerID) float64 {
	total := 0.0
	for lane, l := range g.State.Players[p].Lanes {
		if l.Creature != nil {
			left := g.Defense(p, lane) - l.Creature.Damage
			total += perCreature + perATK*float64(g.Attack(p, lane)) + perDEF*float64(max(left, 0))
			if l.Creature.Status == engine.Ready {
				total += readyBonus
			}
		}
		total += perBuilding * float64(len(buildings(l)))
	}
	return total
}

func buildings(l engine.Lane) []*engine.Building {
	if l.Building == nil {
		return nil
	}
	return append([]*engine.Building{l.Building}, l.Extra...)
}
