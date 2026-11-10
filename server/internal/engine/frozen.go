package engine

import "cardwars/internal/card"

// Frozen tokens. A Landscape can hold one; it keeps its type and stays when the Landscape is flipped.
// No Creature may be played onto a frozen Landscape (except one that says it may), and a Creature on
// one cannot be replaced. Creatures may still be moved onto or off it. To take a token off its own
// side of the board, a player discards a card from their hand during their turn.

func (g *Game) frozen(at pos) bool { return g.State.Players[at.P].Lanes[at.Lane].Frozen }

// frozenCount is how many frozen Landscapes the players have, both sides.
func (g *Game) frozenCount(players []PlayerID) int {
	n := 0
	for _, p := range players {
		for _, l := range g.State.Players[p].Lanes {
			if l.Frozen {
				n++
			}
		}
	}
	return n
}

// freeze puts a frozen token on a Landscape. It already having one is not an error: nothing happens.
func (g *Game) freeze(p PlayerID, lane int) []Event {
	l := &g.State.Players[p].Lanes[lane]
	if l.Frozen {
		return nil
	}
	l.Frozen = true
	return []Event{{Kind: EvFreeze, Player: p, Lane: lane}}
}

// thaw takes the frozen token off a Landscape, if it has one.
func (g *Game) thaw(p PlayerID, lane int) []Event {
	l := &g.State.Players[p].Lanes[lane]
	if !l.Frozen {
		return nil
	}
	l.Frozen = false
	return []Event{{Kind: EvThaw, Player: p, Lane: lane}}
}

// checkFrozen refuses playing a Creature onto a frozen Landscape of p's, or over the Creature on one.
// Buildings and Spells are not affected, and a card that says so may go onto an empty frozen Landscape.
func (g *Game) checkFrozen(p PlayerID, lane int, c *card.Card) error {
	if c.Type != card.Creature || !g.State.Players[p].Lanes[lane].Frozen {
		return nil
	}
	if g.State.Players[p].Lanes[lane].Creature != nil || !hasRestrict(c.Effects, card.RestrictPlayOnFrozen) {
		return ErrFrozen
	}
	return nil
}

func (g *Game) validateThaw(p PlayerID, a Action) error {
	if err := g.requireMain(p); err != nil {
		return err
	}
	if a.Lane < 0 || a.Lane >= NumLanes {
		return ErrBadLane
	}
	if !g.State.Players[p].Lanes[a.Lane].Frozen {
		return ErrNotFrozen
	}
	if g.handIndex(p, a.Card) < 0 {
		return ErrNoSuchCard
	}
	return nil
}

// doThaw discards the chosen card and removes the token. It costs no Actions.
func (g *Game) doThaw(p PlayerID, a Action) []Event {
	ev := g.discardFromHand(p, a.Card)
	return append(ev, g.thaw(p, a.Lane)...)
}

// thawActions lists every way p could remove a token: each frozen Landscape of theirs with each card in hand.
func (g *Game) thawActions(p PlayerID) []Action {
	var out []Action
	for lane, l := range g.State.Players[p].Lanes {
		if !l.Frozen {
			continue
		}
		for _, in := range g.State.Players[p].Hand {
			out = append(out, Action{Kind: ActThaw, Lane: lane, Card: in.UID})
		}
	}
	return out
}

// freezeTargets works out which Landscapes a freeze op reaches: the card's own (Self) or the chosen
// ones, and with Both the Landscape opposite as well.
func (g *Game) freezeTargets(op card.Op, c effCtx, given [][]TargetRef) []pos {
	var at []pos
	if op.Self {
		if c.hasSrc {
			at = append(at, c.at)
		}
	} else {
		for _, r := range slotRefs(op.Slot, given) {
			at = append(at, pos{r.Player, r.Lane})
		}
	}
	if !op.Both {
		return at
	}
	var out []pos
	for _, a := range at {
		out = append(out, pos{a.P, a.Lane}, pos{a.P.Opp(), a.Lane})
	}
	return out
}

// emptyLanes lists the Lanes on one side with no Creature, other than a Lane to skip (-1 for none).
func (g *Game) emptyLanes(side PlayerID, skip int) []int {
	var out []int
	for lane, l := range g.State.Players[side].Lanes {
		if l.Creature == nil && lane != skip {
			out = append(out, lane)
		}
	}
	return out
}

// moveSelf (Drained Cleric, Tired Wombat, Woadic Ring Leader) moves the Creature to an empty Landscape:
// one its controller has, else (if the card says) one the opponent has. With one place to go it goes;
// with several its controller chooses; with none the card's Else ops run.
func (g *Game) moveSelf(op card.Op, c effCtx) []Event {
	cr := c.creature
	if cr == nil || !c.hasSrc || g.creatureAt(c.at) != cr {
		return nil
	}
	side, lanes := c.at.P, g.emptyLanes(c.at.P, c.at.Lane)
	if len(lanes) == 0 && op.ElseTo == "empty_opponent" {
		side, lanes = c.at.P.Opp(), g.emptyLanes(c.at.P.Opp(), -1)
	}
	switch len(lanes) {
	case 0:
		return g.runOps(op.Else, c, nil)
	case 1:
		return g.moveCreature(c.at, pos{side, lanes[0]})
	}
	g.ask(Choice{Player: c.p, Kind: ChoiceMove, Card: c.cardID(), Lane: c.at.Lane, SourceUID: cr.UID, Lanes: lanes, MoveSide: side})
	return nil
}
