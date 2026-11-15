package engine

import (
	"slices"

	"cardwars/internal/card"
)

// Landscapes that change for a while (Rock Out!, Sinkhole, Silo of Freedom), and Creatures that gather
// the Creatures leaving play underneath them (Carmel Camel).

// restoreLanes puts back the Landscapes that were changed "until the start of your next turn" when p's
// turn starts: flipped ones face up again, retyped ones to what they were.
func (g *Game) restoreLanes(p PlayerID) []Event {
	var ev []Event
	for side := range g.State.Players {
		for i := range g.State.Players[side].Lanes {
			l := &g.State.Players[side].Lanes[i]
			if l.FlipBack == int(p)+1 {
				l.FlipBack = 0
				if l.FaceDown {
					l.FaceDown = false
					ev = append(ev, Event{Kind: EvFlip, Player: PlayerID(side), Lane: i})
				}
			}
			if l.TypeBack == int(p)+1 {
				l.TypeBack = 0
				if l.WasType != "" {
					l.Landscape, l.WasType = l.WasType, ""
					ev = append(ev, Event{Kind: EvRetype, Player: PlayerID(side), Lane: i, Amount: landscapeIndex(l.Landscape) + 1})
				}
			}
		}
	}
	return ev
}

func landscapeIndex(ls card.Landscape) int { return slices.Index(card.Landscapes, ls) }

// flipLandscape turns one Landscape over. When it really changes, Until makes it go back at the start of
// the controller's next turn, and Track has the Creature remember it.
func (g *Game) flipLandscape(op card.Op, c effCtx, side PlayerID, lane int) []Event {
	l := &g.State.Players[side].Lanes[lane]
	down := op.Face == "down"
	changed := l.FaceDown != down
	l.FaceDown = down
	if changed && op.Until != "" && down {
		l.FlipBack = int(c.p) + 1
	}
	if changed && op.Track && c.creature != nil {
		c.creature.Flipped = append(c.creature.Flipped, Spot{side, lane})
	}
	return []Event{{Kind: EvFlip, Player: side, Lane: lane}}
}

// unflipTracked turns face up again every Landscape the Creature flipped down.
func (g *Game) unflipTracked(c effCtx) []Event {
	cr := c.creature
	if cr == nil {
		return nil
	}
	var ev []Event
	for _, s := range cr.Flipped {
		l := &g.State.Players[s.Player].Lanes[s.Lane]
		if l.FaceDown {
			l.FaceDown = false
			ev = append(ev, Event{Kind: EvFlip, Player: s.Player, Lane: s.Lane})
		}
	}
	cr.Flipped = nil
	return ev
}

// retype changes the type of p's Landscape in a Lane until the start of p's next turn (Silo of Freedom).
func (g *Game) retype(p PlayerID, lane int, to card.Landscape) []Event {
	l := &g.State.Players[p].Lanes[lane]
	if l.FaceDown || l.Landscape == to {
		return nil
	}
	if l.TypeBack == 0 {
		l.WasType = l.Landscape
	}
	l.Landscape, l.TypeBack = to, int(p)+1
	return []Event{{Kind: EvRetype, Player: p, Lane: lane, Amount: landscapeIndex(to) + 1}}
}

// askLandscape (Silo of Freedom) has p pick the type of their Landscape in a Lane.
func (g *Game) askLandscape(p PlayerID, lane int, c effCtx) {
	if c.hasSrc && !g.State.Players[p].Lanes[lane].FaceDown {
		g.ask(Choice{Player: p, Kind: ChoiceLandscape, Card: c.cardID(), Lane: lane})
	}
}

// camelLanes lists the Lanes of cr's owner that hold a Creature that may collect cr as it leaves play.
func (g *Game) camelLanes(cr *Creature) []int {
	var out []int
	for i, l := range g.State.Players[cr.Owner].Lanes {
		if l.Creature != nil && l.Creature != cr && hasRestrict(g.effectsOf(l.Creature), card.RestrictCollectLeaving) {
			out = append(out, i)
		}
	}
	return out
}

// settleLeaver puts a Creature that has left play into its owner's hand or discard pile.
func (g *Game) settleLeaver(cr *Creature, lane int, to destination) []Event {
	owner := &g.State.Players[cr.Owner]
	if to == toHand {
		owner.Hand = append(owner.Hand, cr.Instance)
		ev := []Event{{Kind: EvBounce, Player: cr.Owner, Card: cr.UID, Lane: lane}}
		return append(ev, g.noteReturn(cr.Owner)...)
	}
	owner.Discard = append(owner.Discard, cr.Instance)
	return nil
}

// camelAnswer finishes a Creature's leaving play once its owner has said whether a Camel collects it.
func (g *Game) camelAnswer(p PlayerID, ch *Choice, camelUID int) []Event {
	in := ch.Revealed[0]
	for i, l := range g.State.Players[p].Lanes {
		if cm := l.Creature; cm != nil && cm.UID == camelUID && camelUID != 0 && slices.Contains(ch.Lanes, i) {
			cm.Under = append(cm.Under, in)
			return []Event{{Kind: EvUnder, Player: p, Card: in.UID, Lane: i}}
		}
	}
	to := toDiscard
	if ch.Count == 1 {
		to = toHand
	}
	cr := &Creature{Instance: in}
	ev := g.settleLeaver(cr, ch.Lane, to)
	if to == toDiscard {
		ctx := effCtx{p: ch.MoveSide, at: pos{ch.MoveSide, ch.Lane}, hasSrc: true, creature: cr}
		ev = append(ev, g.fire(card.OnDiscarded, ctx, g.effectsOf(cr), nil)...)
	}
	return ev
}

// effectsCamel is the card id of the first Camel in the Lanes, for naming it in the Choice.
func (g *Game) effectsCamel(lanes []int, owner PlayerID) string {
	return g.State.Players[owner].Lanes[lanes[0]].Creature.CardID
}
