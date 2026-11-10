package decks

import (
	"fmt"
	"sort"

	"cardwars/internal/card"
	"cardwars/internal/engine"
)

// MaxCards is the most cards a saved deck may hold. The game needs 40; this only stops absurd saves.
const MaxCards = 120

// Check refuses a deck that makes no sense even as work in progress: a card nobody has heard of, a Hero in
// the card list, a Hero that is not one, or more copies of a card than the rules allow. Anything else wrong
// with it (too few cards, too many Landscape types, no Hero yet) only stops it being played, not saved: see Problems.
func Check(cat card.Catalog, hero string, cards []string) error {
	if len(cards) > MaxCards {
		return fmt.Errorf("a deck holds at most %d cards", MaxCards)
	}
	if hero != "" {
		if h := cat[hero]; h == nil || h.Type != card.Hero {
			return fmt.Errorf("%q is not a Hero", hero)
		}
	}
	copies := map[string]int{}
	for _, id := range cards {
		c := cat[id]
		switch {
		case c == nil:
			return fmt.Errorf("unknown card %q", id)
		case c.Type == card.Hero:
			return fmt.Errorf("%s is a Hero: choose it as the deck's Hero, not as a card", c.Name)
		}
		if copies[id]++; copies[id] > engine.MaxCopies {
			return fmt.Errorf("more than %d copies of %s", engine.MaxCopies, c.Name)
		}
	}
	return nil
}

// Custom describes a deck a player made. The cards must have passed Check.
func Custom(cat card.Catalog, name, owner, hero string, cards []string) Info {
	return newInfo(cat, "", name, owner, hero, cards)
}

// Problems says, in words, what stops the deck being played; none means it can be. unmodelled
// allows cards whose rules the engine does not play (development servers). The cards must have passed Check.
func (d Info) Problems(cat card.Catalog, unmodelled bool) []string {
	var out []string
	if d.Hero == "" {
		out = append(out, "choose a Hero")
	} else if h := cat[d.Hero]; !unmodelled && !h.Implemented() {
		out = append(out, h.Name+"'s rules are not implemented yet")
	}
	if len(d.Cards) < engine.MinDeckSize {
		out = append(out, fmt.Sprintf("%d cards: a deck needs at least %d", len(d.Cards), engine.MinDeckSize))
	}
	if !d.LandOK {
		out = append(out, landscapeProblem(cat, d.Cards))
	}
	if !unmodelled {
		seen := map[string]bool{}
		var ids []string
		for _, id := range d.Cards {
			if !seen[id] && !cat[id].Implemented() {
				seen[id] = true
				ids = append(ids, id)
			}
		}
		sort.Strings(ids)
		for _, id := range ids {
			out = append(out, cat[id].Name+"'s rules are not implemented yet")
		}
	}
	return out
}

func landscapeProblem(cat card.Catalog, ids []string) string {
	kinds := map[card.Landscape]bool{}
	for _, id := range ids {
		if ls := cat[id].Landscape; ls != "" && ls != card.Rainbow {
			kinds[ls] = true
		}
	}
	if len(kinds) == 0 {
		return "needs at least one Landscape type (some card that is not Rainbow)"
	}
	return fmt.Sprintf("uses %d Landscape types: a deck can use at most %d", len(kinds), engine.NumLanes)
}
