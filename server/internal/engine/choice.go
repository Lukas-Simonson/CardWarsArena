package engine

// ask opens a Choice, or queues it behind the one that is open.
func (g *Game) ask(c Choice) {
	if g.State.Choice == nil {
		g.State.Choice = &c
		return
	}
	g.State.Queue = append(g.State.Queue, c)
}

// closeChoice ends the open Choice and opens the next queued one.
func (g *Game) closeChoice() {
	g.State.Choice = nil
	if len(g.State.Queue) > 0 {
		next := g.State.Queue[0]
		g.State.Queue = g.State.Queue[1:]
		g.State.Choice = &next
	}
}

func (g *Game) validateChoice(p PlayerID, a Action) error {
	ch := g.State.Choice
	if p != ch.Player {
		return ErrNotYourTurn
	}
	switch ch.Kind {
	case ChoiceDiscard:
		if a.Kind != ActChoose {
			return ErrChoice
		}
		if g.handIndex(p, a.Card) < 0 {
			return ErrNoSuchCard
		}
	case ChoiceUpkeep:
		if a.Kind != ActChoose {
			return ErrChoice
		}
		if a.Card == 0 {
			return nil // destroy the Creature that asks
		}
		if g.handIndex(p, a.Card) < 0 {
			return ErrNoSuchCard
		}
	case ChoiceReveal:
		if a.Kind != ActChoose {
			return ErrChoice
		}
		for _, in := range ch.Revealed {
			if in.UID == a.Card {
				return nil
			}
		}
		return ErrNoSuchCard
	}
	return nil
}

func (g *Game) doChoice(p PlayerID, a Action) []Event {
	ch := g.State.Choice
	var ev []Event
	switch ch.Kind {
	case ChoiceUpkeep:
		if a.Card == 0 {
			if cr := g.creatureAt(pos{p, ch.Lane}); cr != nil && cr.UID == ch.SourceUID {
				ev = g.destroyAt(pos{p, ch.Lane})
			}
			break
		}
		ev = g.discardFromHand(p, a.Card)
		ch.Count--
		if ch.Count > 0 {
			ch.Kind = ChoiceDiscard // having started, the rest must be discarded
			return ev
		}
	case ChoiceDiscard:
		ev = g.discardFromHand(p, a.Card)
		ch.Count--
		if ch.Count > 0 && len(g.State.Players[p].Hand) > 0 {
			return ev // more to discard: the same Choice stays open
		}
	case ChoiceReveal:
		pl := &g.State.Players[p]
		for _, in := range ch.Revealed {
			if in.UID == a.Card {
				pl.Deck = append(pl.Deck, in)
				continue
			}
			g.State.Players[in.Owner].Discard = append(g.State.Players[in.Owner].Discard, in)
			ev = append(ev, Event{Kind: EvDiscard, Player: p, Card: in.UID})
		}
	}
	g.closeChoice()
	return ev
}

// choiceActions lists every way p can answer the open Choice.
func (g *Game) choiceActions(p PlayerID) []Action {
	ch := g.State.Choice
	if ch == nil || ch.Player != p {
		return nil
	}
	var out []Action
	switch ch.Kind {
	case ChoiceDiscard:
		for _, in := range g.State.Players[p].Hand {
			out = append(out, Action{Kind: ActChoose, Card: in.UID})
		}
	case ChoiceUpkeep:
		for _, in := range g.State.Players[p].Hand {
			out = append(out, Action{Kind: ActChoose, Card: in.UID})
		}
		out = append(out, Action{Kind: ActChoose}) // destroy it instead
	case ChoiceReveal:
		for _, in := range ch.Revealed {
			out = append(out, Action{Kind: ActChoose, Card: in.UID})
		}
	}
	return out
}
