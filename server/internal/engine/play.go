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

// checkPlayable runs every check on a play except target choices.
func (g *Game) checkPlayable(p PlayerID, a Action) (Instance, *card.Card, error) {
	if err := g.requireMain(p); err != nil {
		return Instance{}, nil, err
	}
	idx := g.handIndex(p, a.Card)
	if idx < 0 {
		return Instance{}, nil, ErrNoSuchCard
	}
	in := g.State.Players[p].Hand[idx]
	c := g.Card(in)
	switch c.Type {
	case card.Creature, card.Building, card.Spell:
	default:
		return in, c, ErrNotPlayable
	}
	if c.Type != card.Spell && (a.Lane < 0 || a.Lane >= NumLanes) {
		return in, c, ErrBadLane
	}
	if err := g.canAfford(p, c); err != nil {
		return in, c, err
	}
	if c.Type == card.Creature {
		// A Flooped Creature must be Ready before it can be replaced.
		if old := g.State.Players[p].Lanes[a.Lane].Creature; old != nil && old.Status == Flooped {
			return in, c, ErrFlooped
		}
	}
	return in, c, nil
}

func (g *Game) validatePlay(p PlayerID, a Action) error {
	in, c, err := g.checkPlayable(p, a)
	if err != nil {
		return err
	}
	// An additional cost (Field Reaper discards a card) is chosen with the play.
	if ce := costEffect(c); ce != nil {
		usable, err := g.checkTargets(ce.Targets, a.CostTargets, effCtx{p: p, playing: a.Card})
		if !usable {
			return ErrNoTarget
		}
		if err != nil {
			return err
		}
	} else if len(a.CostTargets) > 0 {
		return ErrBadTarget
	}
	return g.validatePlayTargets(p, in, c, a)
}

// validatePlayTargets checks the lane-dependent target choices of a play: a
// Spell's targets or an enter-play trigger's.
func (g *Game) validatePlayTargets(p PlayerID, in Instance, c *card.Card, a Action) error {
	switch c.Type {
	case card.Spell:
		if e := spellEffect(c); e != nil {
			usable, err := g.checkTargets(e.Targets, a.Targets, effCtx{p: p})
			if !usable {
				return ErrNoTarget
			}
			return err
		}
	default:
		e := enterEffect(c)
		if e == nil || len(e.Targets) == 0 {
			if len(a.Targets) > 0 {
				return ErrBadTarget
			}
			return nil
		}
		// Targets are judged against the board as it will be once the card
		// is placed (a replaced Creature is already gone).
		h := g.Clone()
		h.place(p, in, a.Lane)
		usable, err := h.checkTargets(e.Targets, a.Targets, h.placedCtx(p, a.Lane))
		if !usable {
			if len(a.Targets) > 0 {
				return ErrBadTarget
			}
			return nil // the trigger will simply do nothing
		}
		return err
	}
	if len(a.Targets) > 0 {
		return ErrBadTarget
	}
	return nil
}

// place puts a Creature or Building into a Lane of p, replacing what is there.
func (g *Game) place(p PlayerID, in Instance, laneIdx int) []Event {
	pl := &g.State.Players[p]
	lane := &pl.Lanes[laneIdx]
	var ev []Event
	if g.Card(in).Type == card.Building {
		if old := lane.Building; old != nil {
			g.State.Players[old.Owner].Discard = append(g.State.Players[old.Owner].Discard, old.Instance)
			ev = append(ev, Event{Kind: EvReplace, Player: p, Card: old.UID, Lane: laneIdx})
		}
		lane.Building = &Building{Instance: in, Status: Ready}
		return ev
	}
	if old := lane.Creature; old != nil {
		// Replaced Creatures go to their owner's discard; damage is dropped.
		ev = append(ev, Event{Kind: EvReplace, Player: p, Card: old.UID, Lane: laneIdx})
		ev = append(ev, g.removeCreature(pos{p, laneIdx}, toDiscard)...)
	}
	lane.Creature = &Creature{Instance: in, Status: Ready}
	return ev
}

// placedCtx is the effect context for the card most recently placed in a Lane.
func (g *Game) placedCtx(p PlayerID, lane int) effCtx {
	return effCtx{p: p, at: pos{p, lane}, hasSrc: true, creature: g.creatureAt(pos{p, lane})}
}

// doPlay pays for and plays a validated card.
func (g *Game) doPlay(p PlayerID, a Action) []Event {
	pl := &g.State.Players[p]
	idx := g.handIndex(p, a.Card)
	in := pl.Hand[idx]
	pl.Hand = append(pl.Hand[:idx:idx], pl.Hand[idx+1:]...)
	c := g.Card(in)
	g.State.ActionsLeft -= *c.Cost
	var costEv []Event
	if ce := costEffect(c); ce != nil {
		costEv = g.runOps(ce.Do, effCtx{p: p, playing: a.Card}, a.CostTargets)
	}
	return append(costEv, g.resolvePlay(p, in, c, a)...)
}

// resolvePlay puts a paid-for card into play, or resolves a Spell.
func (g *Game) resolvePlay(p PlayerID, in Instance, c *card.Card, a Action) []Event {
	if c.Type == card.Spell {
		// Spells never enter play: resolve, then discard. The Spell counts as
		// played before it resolves (Woadic Chief counts it) but is not in the
		// discard pile while it does.
		ev := []Event{{Kind: EvPlay, Player: p, Card: in.UID}}
		g.State.SpellsThisTurn++
		if e := spellEffect(c); e != nil {
			ev = append(ev, g.runOps(e.Do, effCtx{p: p, at: pos{p, 0}}, a.Targets)...)
		}
		g.State.Players[in.Owner].Discard = append(g.State.Players[in.Owner].Discard, in)
		return ev
	}
	ev := g.place(p, in, a.Lane)
	ev = append(ev, Event{Kind: EvPlay, Player: p, Card: in.UID, Lane: a.Lane})
	return append(ev, g.fire(card.OnEnterPlay, g.placedCtx(p, a.Lane), c.Effects, a.Targets)...)
}

// spellEffect returns a Spell's resolution effect, if it has one.
func spellEffect(c *card.Card) *card.Effect {
	for i := range c.Effects {
		if c.Effects[i].Kind == card.KindSpell {
			return &c.Effects[i]
		}
	}
	return nil
}

// costEffect returns a card's additional cost, if it has one.
func costEffect(c *card.Card) *card.Effect {
	for i := range c.Effects {
		if c.Effects[i].Kind == card.KindAdditionalCost {
			return &c.Effects[i]
		}
	}
	return nil
}

// enterEffect returns a card's enter-play trigger, if it has one.
func enterEffect(c *card.Card) *card.Effect {
	for i := range c.Effects {
		if c.Effects[i].Kind == card.KindTrigger && c.Effects[i].On == card.OnEnterPlay {
			return &c.Effects[i]
		}
	}
	return nil
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

// validatePending checks playing the card taken by Silo of Truth. It is
// played for free: no Action cost, no Landscape requirement, no additional cost.
func (g *Game) validatePending(p PlayerID, a Action) error {
	if g.State.Pending == nil {
		return ErrNoPending
	}
	if p != g.State.Active {
		return ErrNotYourTurn
	}
	in := g.State.Pending.Card
	c := g.Card(in)
	if len(a.CostTargets) > 0 {
		return ErrBadTarget
	}
	if c.Type != card.Spell {
		if a.Lane < 0 || a.Lane >= NumLanes {
			return ErrBadLane
		}
		if old := g.State.Players[p].Lanes[a.Lane].Creature; c.Type == card.Creature && old != nil && old.Status == Flooped {
			return ErrFlooped
		}
	}
	return g.validatePlayTargets(p, in, c, a)
}

func (g *Game) doPending(p PlayerID, a Action) []Event {
	in := g.State.Pending.Card
	g.State.Pending = nil
	return g.resolvePlay(p, in, g.Card(in), a)
}

// pendingActions lists every legal way to play the pending card.
func (g *Game) pendingActions(p PlayerID) []Action {
	if g.State.Pending == nil {
		return nil
	}
	in := g.State.Pending.Card
	c := g.Card(in)
	var cands []Action
	if c.Type == card.Spell {
		choices := [][][]TargetRef{nil}
		if e := spellEffect(c); e != nil && len(e.Targets) > 0 {
			choices = g.targetCombos(e.Targets, effCtx{p: p}, maxTargetCombos)
		}
		for _, t := range choices {
			cands = append(cands, Action{Kind: ActPlayPending, Targets: t})
		}
	} else {
		for lane := 0; lane < NumLanes; lane++ {
			choices := [][][]TargetRef{nil}
			if e := enterEffect(c); e != nil && len(e.Targets) > 0 {
				h := g.Clone()
				h.place(p, in, lane)
				if combos := h.targetCombos(e.Targets, h.placedCtx(p, lane), maxTargetCombos); len(combos) > 0 {
					choices = combos
				}
			}
			for _, t := range choices {
				cands = append(cands, Action{Kind: ActPlayPending, Lane: lane, Targets: t})
			}
		}
	}
	var legal []Action
	for _, a := range cands {
		if g.validatePending(p, a) == nil {
			legal = append(legal, a)
		}
	}
	return legal
}
