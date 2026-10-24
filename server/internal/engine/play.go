package engine

import "cardwars/internal/card"

// requireMain checks that p may act in the main phase.
func (g *Game) requireMain(p PlayerID) error {
	if g.State.Phase != PhaseMain {
		return ErrWrongPhase
	}
	if p != g.State.Active {
		return ErrNotYourTurn
	}
	return nil
}

// handIndex finds a card in p's hand by UID.
func (g *Game) handIndex(p PlayerID, uid int) int {
	for i, in := range g.State.Players[p].Hand {
		if in.UID == uid {
			return i
		}
	}
	return -1
}

// faceUp counts p's face-up Landscapes of the given type.
func (g *Game) faceUp(p PlayerID, ls card.Landscape) int {
	n := 0
	for _, l := range g.State.Players[p].Lanes {
		if !l.FaceDown && l.Landscape == ls {
			n++
		}
	}
	return n
}

// canAfford checks the Action cost and the Landscape requirement. The cost is
// both the number of Actions paid and the number of face-up Landscapes of the
// card's type required. Rainbow and 0-cost cards need no Landscape, and 0-cost
// cards need no Actions, so they remain playable after Actions are spent.
func (g *Game) canAfford(p PlayerID, c *card.Card) error {
	if c.Cost == nil {
		return ErrNotPlayable
	}
	cost := *c.Cost
	if cost == 0 {
		return nil
	}
	if g.State.ActionsLeft < cost {
		return ErrNoActions
	}
	if c.Landscape != card.Rainbow && g.faceUp(p, c.Landscape) < cost {
		return ErrLandscapes
	}
	return nil
}

func (g *Game) validatePlay(p PlayerID, a Action) error {
	if err := g.requireMain(p); err != nil {
		return err
	}
	idx := g.handIndex(p, a.Card)
	if idx < 0 {
		return ErrNoSuchCard
	}
	c := g.Card(g.State.Players[p].Hand[idx])
	switch c.Type {
	case card.Creature, card.Building, card.Spell:
	default:
		return ErrNotPlayable
	}
	if c.Type != card.Spell {
		if a.Lane < 0 || a.Lane >= NumLanes {
			return ErrBadLane
		}
	}
	if err := g.canAfford(p, c); err != nil {
		return err
	}
	if c.Type == card.Creature {
		// A Flooped Creature must be Ready before it can be replaced.
		if old := g.State.Players[p].Lanes[a.Lane].Creature; old != nil && old.Status == Flooped {
			return ErrFlooped
		}
	}
	return nil
}

// doPlay pays for and plays a validated card.
func (g *Game) doPlay(p PlayerID, a Action) []Event {
	pl := &g.State.Players[p]
	idx := g.handIndex(p, a.Card)
	in := pl.Hand[idx]
	pl.Hand = append(pl.Hand[:idx:idx], pl.Hand[idx+1:]...)
	c := g.Card(in)
	g.State.ActionsLeft -= *c.Cost

	var ev []Event
	switch c.Type {
	case card.Creature:
		lane := &pl.Lanes[a.Lane]
		if old := lane.Creature; old != nil {
			// Replaced Creatures go to their owner's discard; damage is dropped.
			pl.Discard = append(pl.Discard, old.Instance)
			ev = append(ev, Event{Kind: EvReplace, Player: p, Card: old.UID, Lane: a.Lane})
		}
		lane.Creature = &Creature{Instance: in, Status: Ready}
	case card.Building:
		lane := &pl.Lanes[a.Lane]
		if old := lane.Building; old != nil {
			pl.Discard = append(pl.Discard, old.Instance)
			ev = append(ev, Event{Kind: EvReplace, Player: p, Card: old.UID, Lane: a.Lane})
		}
		lane.Building = &Building{Instance: in, Status: Ready}
	case card.Spell:
		// Spells never enter play. Their effects are resolved by the effect
		// system; a spell the engine cannot resolve is rejected at deck build.
		pl.Discard = append(pl.Discard, in)
	}
	return append(ev, Event{Kind: EvPlay, Player: p, Card: in.UID, Lane: a.Lane})
}

func (g *Game) validateDraw(p PlayerID) error {
	if err := g.requireMain(p); err != nil {
		return err
	}
	if g.State.ActionsLeft < 1 {
		return ErrNoActions
	}
	if len(g.State.Players[p].Deck) == 0 {
		return ErrDeckEmpty
	}
	return nil
}

func (g *Game) doDraw(p PlayerID) []Event {
	g.State.ActionsLeft--
	var ev []Event
	for _, c := range g.draw(p, 1) {
		ev = append(ev, Event{Kind: EvDraw, Player: p, Card: c.UID})
	}
	return ev
}
