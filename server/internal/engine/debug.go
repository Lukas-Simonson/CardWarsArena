package engine

import (
	"fmt"

	"cardwars/internal/card"
)

// The helpers in this file bypass the rules. They exist for the CLI sandbox
// and for tests, never for real matches.

// Give puts a new copy of a card into p's hand.
func (g *Game) Give(p PlayerID, cardID string) (Instance, error) {
	if g.cat[cardID] == nil {
		return Instance{}, fmt.Errorf("unknown card %q", cardID)
	}
	in := g.newInstance(cardID, p)
	g.State.Players[p].Hand = append(g.State.Players[p].Hand, in)
	return in, nil
}

// Put places a new Creature or Building directly into a Lane, replacing what
// is there, without paying costs or firing triggers.
func (g *Game) Put(p PlayerID, lane int, cardID string) error {
	c := g.cat[cardID]
	if c == nil {
		return fmt.Errorf("unknown card %q", cardID)
	}
	if lane < 0 || lane >= NumLanes {
		return ErrBadLane
	}
	in := g.newInstance(cardID, p)
	l := &g.State.Players[p].Lanes[lane]
	switch c.Type {
	case card.Creature:
		l.Creature = &Creature{Instance: in, Status: Ready}
	case card.Building:
		l.Building = &Building{Instance: in, Status: Ready}
	default:
		return fmt.Errorf("%s is a %s: only Creatures and Buildings can be put in a Lane", c.Name, c.Type)
	}
	g.settle()
	return nil
}

// SetHP sets a player's Hit Points.
func (g *Game) SetHP(p PlayerID, hp int) { g.State.Players[p].HP = hp; g.settle() }

// SetActions sets the active player's remaining Actions.
func (g *Game) SetActions(n int) { g.State.ActionsLeft = n }

// SetLandscape changes the Landscape in a Lane and whether it is face down.
func (g *Game) SetLandscape(p PlayerID, lane int, ls card.Landscape, faceDown bool) error {
	if lane < 0 || lane >= NumLanes {
		return ErrBadLane
	}
	l := &g.State.Players[p].Lanes[lane]
	l.Landscape, l.FaceDown = ls, faceDown
	return nil
}

// Clear removes whatever Creature and Building are in a Lane (to the discard).
func (g *Game) Clear(p PlayerID, lane int) error {
	if lane < 0 || lane >= NumLanes {
		return ErrBadLane
	}
	g.removeCreature(pos{p, lane}, toDiscard)
	for _, b := range g.State.Players[p].Lanes[lane].dropBuildings() {
		g.State.Players[b.Owner].Discard = append(g.State.Players[b.Owner].Discard, b.Instance)
	}
	g.settle()
	return nil
}

// Instances lists every card instance in the game with its card id, by UID,
// so a front end can name cards that appear in events.
func (g *Game) Instances() map[int]string {
	out := map[int]string{}
	add := func(in Instance) { out[in.UID] = in.CardID }
	for _, pl := range g.State.Players {
		for _, list := range [][]Instance{pl.Deck, pl.Hand, pl.Discard} {
			for _, in := range list {
				add(in)
			}
		}
		for _, l := range pl.Lanes {
			if l.Creature != nil {
				add(l.Creature.Instance)
			}
			for _, b := range l.buildings() {
				add(b.Instance)
			}
		}
	}
	if g.State.Pending != nil {
		add(g.State.Pending.Card)
	}
	for _, ch := range append([]Choice{}, append(g.State.Queue, ptrVal(g.State.Choice)...)...) {
		for _, in := range ch.Revealed {
			add(in)
		}
	}
	return out
}

// Catalog returns the card catalog the game was created with.
func (g *Game) Catalog() card.Catalog { return g.cat }

func ptrVal(c *Choice) []Choice {
	if c == nil {
		return nil
	}
	return []Choice{*c}
}
