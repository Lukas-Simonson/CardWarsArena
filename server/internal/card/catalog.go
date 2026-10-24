package card

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

// Catalog is the read-only set of all known cards, keyed by Card.ID.
type Catalog map[string]*Card

// LoadDir reads every *.json card file in dir into a Catalog.
func LoadDir(dir string) (Catalog, error) {
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

// Deck returns the card ids (one entry per copy, sorted) printed in the given
// product, e.g. Deck("CP1", "Finn's Blue Plains Deck").
func (c Catalog) Deck(set, deck string) []string {
	var ids []string
	for id, card := range c {
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
