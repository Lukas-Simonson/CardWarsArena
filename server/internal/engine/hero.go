package engine

import (
	"slices"

	"cardwars/internal/card"
)

// A Hero is not in a Lane. It sits beside its controller's HP for the whole
// game, and its rules are those of a card in play: statics, triggers and
// activated abilities. Its position is Lane -1, so nothing that reads a Lane
// can mistake it for a card on the board.
const heroLane = -1

func (g *Game) heroEffects(p PlayerID) []card.Effect {
	h := g.State.Players[p].Hero
	if h.CardID == "" {
		return nil
	}
	return g.Card(h).Effects
}

func (g *Game) heroCtx(p PlayerID) effCtx {
	return effCtx{p: p, at: pos{p, heroLane}, hasSrc: true, hero: true, card: g.State.Players[p].Hero.CardID}
}

// fireHero runs the triggers of p's Hero for an event. A trigger with an `if`
// only fires while it holds, and one marked once_per_turn only the first time.
func (g *Game) fireHero(p PlayerID, on string) []Event {
	var ev []Event
	ctx := g.heroCtx(p)
	for i, e := range g.heroEffects(p) {
		if e.Kind != card.KindTrigger || e.On != on {
			continue
		}
		if e.If != nil && !g.cond(*e.If, ctx.source()) {
			continue
		}
		fired := &g.State.Players[p].HeroFired
		if e.OncePerTurn {
			if slices.Contains(*fired, i) {
				continue
			}
			*fired = append(*fired, i)
		}
		ev = append(ev, g.runOps(e.Do, ctx, nil)...)
	}
	return ev
}

// noteReturn records that p's Creature went back to p's hand during p's turn
// (from play or from the discard pile) and lets the Hero react.
func (g *Game) noteReturn(owner PlayerID) []Event {
	if g.State.Active != owner {
		return nil
	}
	g.State.Returned[owner]++
	return g.fireHero(owner, card.OnReturns)
}

// grantedTo lists the abilities a Hero gives the Creature c (Finn).
func (g *Game) grantedTo(c *Creature) []card.Effect {
	var out []card.Effect
	for p := range g.State.Players {
		for _, e := range g.heroEffects(PlayerID(p)) {
			if e.Kind != card.KindStatic || e.Grant == nil {
				continue
			}
			at, ok := g.posOf(c)
			if ok && g.selects(e.Grant.To, source{pos: pos{PlayerID(p), heroLane}}, at, c) {
				out = append(out, e.Grant.Ability)
			}
		}
	}
	return out
}

// posOf finds where a Creature in play is.
func (g *Game) posOf(c *Creature) (pos, bool) {
	for p := range g.State.Players {
		for i := range g.State.Players[p].Lanes {
			if g.State.Players[p].Lanes[i].Creature == c {
				return pos{PlayerID(p), i}, true
			}
		}
	}
	return pos{}, false
}
