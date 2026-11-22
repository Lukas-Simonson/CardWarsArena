package engine

import (
	"slices"

	"cardwars/internal/card"
)

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
		i := g.handIndex(p, a.Card)
		if i < 0 {
			return ErrNoSuchCard
		}
		if ch.OnlyCost != nil && g.handCost(g.State.Players[p].Hand[i]) != *ch.OnlyCost {
			return ErrBadTarget
		}
	case ChoiceRansom:
		if a.Card == 0 {
			return nil // take what comes instead
		}
		i := g.handIndex(p, a.Card)
		if i < 0 {
			return ErrNoSuchCard
		}
		if g.handCost(g.State.Players[p].Hand[i]) < *ch.MinCost {
			return ErrBadTarget
		}
	case ChoiceAmount:
		if !slices.Contains(ch.Options, a.Card) {
			return ErrBadTarget
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
	case ChoiceSearch:
		for _, in := range ch.Revealed {
			if in.UID == a.Card {
				return nil
			}
		}
		return ErrNoSuchCard
	case ChoicePlace:
		if !slices.Contains(ch.Lanes, a.Lane) {
			return ErrBadLane
		}
	case ChoiceRebound:
		if a.Card != 0 && g.handIndex(p, a.Card) < 0 {
			return ErrNoSuchCard
		}
	case ChoiceDefend:
		if a.Card == 0 {
			return nil
		}
		for _, in := range ch.Revealed {
			if in.UID == a.Card {
				return nil
			}
		}
		return ErrNoSuchCard
	case ChoiceRedirect:
		if a.Card == 0 {
			return nil
		}
		for _, in := range ch.Revealed {
			if in.UID == a.Card {
				return nil
			}
		}
		return ErrNoSuchCard
	case ChoiceMoveDamage:
		if a.Card < 1 || a.Card > ch.Count {
			return ErrBadTarget
		}
	case ChoiceMove:
		if !slices.Contains(ch.Lanes, a.Lane) {
			return ErrBadLane
		}
	case ChoiceCamel:
		if a.Card == 0 {
			return nil
		}
		for _, lane := range ch.Lanes {
			if cm := g.State.Players[p].Lanes[lane].Creature; cm != nil && cm.UID == a.Card {
				return nil
			}
		}
		return ErrNoSuchCard
	case ChoiceLandscape:
		if a.Card < 1 || a.Card > len(card.Landscapes) {
			return ErrBadTarget
		}
	case ChoiceTarget:
		if len(a.Targets) == 0 || (len(a.Targets) == 1 && len(a.Targets[0]) == 0) {
			if ch.Optional {
				return nil
			}
			return ErrBadTarget
		}
		if len(a.Targets) != 1 || len(a.Targets[0]) != 1 || !slices.Contains(ch.Targets, a.Targets[0][0]) {
			return ErrBadTarget
		}
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
		ev = g.discardForBonus(p, a.Card, ch.Bonus)
		ch.Count--
		if ch.Count > 0 && len(g.State.Players[p].Hand) > 0 {
			return ev // more to discard: the same Choice stays open
		}
	case ChoiceSearch:
		// The Building is taken out of the deck, then its Lane is chosen (ChoicePlace), and the deck is shuffled.
		pl := &g.State.Players[p]
		for i, in := range pl.Deck {
			if in.UID == a.Card {
				pl.Deck = append(pl.Deck[:i:i], pl.Deck[i+1:]...)
				g.ask(Choice{Player: p, Kind: ChoicePlace, Card: ch.Card, Revealed: []Instance{in}, Lanes: []int{0, 1, 2, 3}})
				break
			}
		}
		g.shuffle(pl.Deck)
	case ChoicePlace:
		ev = g.putBuilding(p, ch.Revealed[0], a.Lane)
	case ChoiceRebound:
		if a.Card == 0 {
			break
		}
		ev = g.discardFromHand(p, a.Card)
		if in, ok := g.takeFromDiscard(p, ch.SourceUID); ok {
			g.State.Players[p].Hand = append(g.State.Players[p].Hand, in)
			ev = append(ev, Event{Kind: EvReturn, Player: p, Card: in.UID})
			if g.Card(in).Type == card.Creature {
				ev = append(ev, g.noteReturn(p)...)
			}
		}
	case ChoiceRedirect:
		lane := ch.Lane
		for l, ln := range g.State.Players[p].Lanes {
			if ln.Creature != nil && ln.Creature.UID == a.Card && a.Card != 0 {
				lane = l
			}
		}
		if at := g.State.Attacking; at != nil {
			at.From = lane // the Fight goes on, once nothing else is being asked, with the Creature picked
		}
	case ChoiceDefend:
		ev = g.answerDefend(p, ch, a.Card)
	case ChoiceMoveDamage:
		ev = g.moveDamage(p, ch.Lane, ch.Other, a.Card)
	case ChoiceMove:
		if cr := g.creatureAt(pos{p, ch.Lane}); cr != nil && cr.UID == ch.SourceUID {
			ev = g.moveCreature(pos{p, ch.Lane}, pos{ch.MoveSide, a.Lane})
			ev = append(ev, g.afterMove(ch.Do, p, pos{ch.MoveSide, a.Lane}, g.creatureAt(pos{ch.MoveSide, a.Lane}))...)
		}
	case ChoiceCamel:
		ev = g.camelAnswer(p, ch, a.Card)
	case ChoiceLandscape:
		ev = g.retype(p, ch.Lane, card.Landscapes[a.Card-1])
	case ChoiceTarget:
		ev = g.answerTarget(ch, a)
	case ChoiceRansom:
		ev = g.answerRansom(ch, a)
	case ChoiceAmount:
		ev = g.answerAmount(ch, a.Card)
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
	return append(ev, g.continueAttack()...)
}

// choiceActions lists every way p can answer the open Choice. They are the answers behind the Question's options, so
// each carries the Option that names it.
func (g *Game) choiceActions(p PlayerID) []Action {
	ch := g.State.Choice
	if ch == nil || ch.Player != p {
		return nil
	}
	_, answers := g.describeChoice(ch)
	out := make([]Action, len(answers))
	for i, a := range answers {
		id := i
		a.Option = &id
		out[i] = a
	}
	return out
}
