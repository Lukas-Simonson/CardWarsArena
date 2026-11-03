package engine

import (
	"slices"
	"strings"

	"cardwars/internal/card"
)

// buildings lists the Buildings on this side of a Lane: the one that is
// there, then any extra ones.
func (l *Lane) buildings() []*Building {
	if l.Building == nil {
		return nil
	}
	return append([]*Building{l.Building}, l.Extra...)
}

// dropBuildings takes every Building off the Lane and returns them.
func (l *Lane) dropBuildings() []*Building {
	out := l.buildings()
	l.Building, l.Extra = nil, nil
	return out
}

// takeBuilding takes the Building that is there off the Lane; an extra one,
// if there is one, becomes the Building that is there.
func (l *Lane) takeBuilding() *Building {
	b := l.Building
	l.Building = nil
	if len(l.Extra) > 0 {
		l.Building, l.Extra = l.Extra[0], l.Extra[1:]
	}
	return b
}

func hasRestrict(effects []card.Effect, r string) bool {
	for _, e := range effects {
		if e.Kind == card.KindStatic && e.Restrict == r {
			return true
		}
	}
	return false
}

// buildingEffects is the rules a Building at at currently has. Normally that
// is its printed effects. An opposing Flooped The Dog takes them all away, and
// Learning Center also has those of the Buildings next to it.
func (g *Game) buildingEffects(at pos, b *Building) []card.Effect {
	if g.buildingSilenced(at) {
		return nil
	}
	own := g.Card(b.Instance).Effects
	if !hasRestrict(own, card.RestrictCopyAdjacentBuildings) {
		return own
	}
	out := append([]card.Effect(nil), own...)
	for _, d := range []int{-1, 1} {
		lane := at.Lane + d
		if lane < 0 || lane >= NumLanes || g.buildingSilenced(pos{at.P, lane}) {
			continue
		}
		for _, next := range g.State.Players[at.P].Lanes[lane].buildings() {
			for _, e := range g.Card(next.Instance).Effects {
				if e.Kind != card.KindStatic || e.Restrict != card.RestrictCopyAdjacentBuildings {
					out = append(out, e)
				}
			}
		}
	}
	return out
}

// buildingSilenced reports whether the Buildings on at have lost their rules
// to a Flooped The Dog opposite them.
func (g *Game) buildingSilenced(at pos) bool {
	foe := g.creatureAt(pos{at.P.Opp(), at.Lane})
	return foe != nil && foe.Status == Flooped && hasRestrict(g.effectsOf(foe), card.RestrictSilenceOpposingBuildings)
}

// eachBuilding visits every Building p has in play.
func (g *Game) eachBuilding(p PlayerID, fn func(at pos, b *Building)) {
	for i := range g.State.Players[p].Lanes {
		for _, b := range g.State.Players[p].Lanes[i].buildings() {
			fn(pos{p, i}, b)
		}
	}
}

// buildingCount is how many Buildings p controls, or how many of one card.
func (g *Game) buildingCount(p PlayerID, id string) int {
	n := 0
	g.eachBuilding(p, func(_ pos, b *Building) {
		if id == "" || b.CardID == id {
			n++
		}
	})
	return n
}

// laneRainbow reports whether a Building makes every Creature in a Lane Rainbow.
func (g *Game) laneRainbow(lane int) bool {
	for p := PlayerID(0); p < 2; p++ {
		for _, b := range g.State.Players[p].Lanes[lane].buildings() {
			if hasRestrict(g.buildingEffects(pos{p, lane}, b), card.RestrictLaneRainbow) {
				return true
			}
		}
	}
	return false
}

// creatureTypes are the Landscape types the Creature at at has now. Usually
// that is the printed one. A Rainbow Creature has no other type.
func (g *Game) creatureTypes(at pos) []card.Landscape {
	cr := g.creatureAt(at)
	if cr.Rainbow || g.laneRainbow(at.Lane) {
		return []card.Landscape{card.Rainbow}
	}
	out := []card.Landscape{g.Card(cr.Instance).Landscape}
	if hasRestrict(g.effectsOf(cr), card.RestrictAlsoRainbow) {
		out = append(out, card.Rainbow)
	}
	return out
}

// isType reports whether the Creature at at is a Creature of the Landscape type.
func (g *Game) isType(at pos, ls card.Landscape) bool {
	return slices.Contains(g.creatureTypes(at), ls)
}

// rainbowCard reports whether a card in hand is a Rainbow card right now.
func (g *Game) rainbowCard(in Instance) bool {
	return g.Card(in).Landscape == card.Rainbow || slices.Contains(g.State.RainbowHand, in.UID)
}

// extraLandscapes is how many face-up Landscapes of a type p's Buildings add.
func (g *Game) extraLandscapes(p PlayerID, ls card.Landscape) int {
	n := 0
	g.eachBuilding(p, func(at pos, b *Building) {
		for _, e := range g.buildingEffects(at, b) {
			if e.Kind == card.KindStatic && e.Restrict == card.RestrictCountsAsLandscape && (ls == "" || e.As == ls) {
				n++
			}
		}
	})
	return n
}

// landscapeCount is p's face-up Landscapes of a type, including the ones
// Buildings add.
func (g *Game) landscapeCount(p PlayerID, ls card.Landscape) int {
	return g.faceUp(p, ls) + g.extraLandscapes(p, ls)
}

// playedTitled counts the cards the active player played this turn whose
// name has one of the words.
func (g *Game) playedTitled(words []string) int {
	n := 0
	for _, pc := range g.State.Played {
		name := g.cat[pc.CardID].Name
		for _, w := range words {
			if strings.Contains(name, w) {
				n++
				break
			}
		}
	}
	return n
}

// playedOfType counts the cards of a Landscape type the active player played this turn.
func (g *Game) playedOfType(ls card.Landscape) int {
	n := 0
	for _, pc := range g.State.Played {
		if (ls == card.Rainbow && pc.Rainbow) || (!pc.Rainbow && g.cat[pc.CardID].Landscape == ls) {
			n++
		}
	}
	return n
}

// CreatureTypes are the Landscape types of p's Creature in a Lane now, after
// anything that changes them. For display and tooling.
func (g *Game) CreatureTypes(p PlayerID, lane int) []card.Landscape {
	if g.creatureAt(pos{p, lane}) == nil {
		return nil
	}
	return g.creatureTypes(pos{p, lane})
}

// IsRainbowInHand reports whether the card with this UID has been made a
// Rainbow card for the rest of the turn.
func (g *Game) IsRainbowInHand(uid int) bool { return slices.Contains(g.State.RainbowHand, uid) }
