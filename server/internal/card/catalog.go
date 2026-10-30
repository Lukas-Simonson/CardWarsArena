package card

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

// Catalog is the read-only set of all known cards, keyed by Card.ID.
type Catalog map[string]*Card

// Load reads printed card data from root/cards and, if present, hand-written
// rules from root/effects, which are merged onto the cards by id.
func Load(root string) (Catalog, error) {
	cat, err := loadCards(filepath.Join(root, "cards"))
	if err != nil {
		return nil, err
	}
	if err := loadEffects(cat, filepath.Join(root, "effects")); err != nil {
		return nil, err
	}
	return cat, nil
}

func loadCards(dir string) (Catalog, error) {
	files, err := filepath.Glob(filepath.Join(dir, "*.json"))
	if err != nil {
		return nil, err
	}
	if len(files) == 0 {
		return nil, fmt.Errorf("card: no card files in %s", dir)
	}
	cat := Catalog{}
	for _, f := range files {
		b, err := os.ReadFile(f)
		if err != nil {
			return nil, err
		}
		var cards []Card
		if err := json.Unmarshal(b, &cards); err != nil {
			return nil, fmt.Errorf("card: %s: %w", f, err)
		}
		for i := range cards {
			c := &cards[i]
			if _, dup := cat[c.ID]; dup {
				return nil, fmt.Errorf("card: duplicate id %q in %s", c.ID, f)
			}
			cat[c.ID] = c
		}
	}
	return cat, nil
}

// loadEffects merges every effects file in dir. Unknown JSON fields, unknown
// card ids and malformed effects are errors, so typos cannot silently disable
// a rule.
func loadEffects(cat Catalog, dir string) error {
	files, err := filepath.Glob(filepath.Join(dir, "*.json"))
	if err != nil {
		return err
	}
	for _, f := range files {
		b, err := os.ReadFile(f)
		if err != nil {
			return err
		}
		dec := json.NewDecoder(bytes.NewReader(b))
		dec.DisallowUnknownFields()
		var byID map[string][]Effect
		if err := dec.Decode(&byID); err != nil {
			return fmt.Errorf("card: %s: %w", f, err)
		}
		for id, effects := range byID {
			c, ok := cat[id]
			if !ok {
				return fmt.Errorf("card: %s: effects for unknown card %q", f, id)
			}
			if c.Modeled {
				return fmt.Errorf("card: %s: %q is modelled twice", f, id)
			}
			for i := range effects {
				if err := effects[i].Validate(c.Type); err != nil {
					return fmt.Errorf("card: %s: %s effect %d: %w", f, id, i, err)
				}
			}
			c.Effects, c.Modeled = effects, true
		}
	}
	return nil
}

// Deck returns the card ids (one entry per copy, sorted) printed in the given
// product, e.g. Deck("Finn Vs Jake", "Finn's Deck"). The deck's Hero is not
// one of them; see Hero.
func (c Catalog) Deck(set, deck string) []string {
	var ids []string
	for id, card := range c {
		if card.Type == Hero {
			continue
		}
		for _, p := range card.Printings {
			if p.Set == set && p.Deck == deck {
				for i := 0; i < p.Copies; i++ {
					ids = append(ids, id)
				}
			}
		}
	}
	sort.Strings(ids)
	return ids
}

// Hero returns the id of the Hero that comes with the given deck, or "".
func (c Catalog) Hero(set, deck string) string {
	for id, card := range c {
		for _, p := range card.Printings {
			if card.Type == Hero && p.Set == set && p.Deck == deck {
				return id
			}
		}
	}
	return ""
}
