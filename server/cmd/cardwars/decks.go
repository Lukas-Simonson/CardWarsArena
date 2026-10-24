package main

import (
	"fmt"
	"sort"
	"strings"

	"cardwars/internal/card"
	"cardwars/internal/engine"
)

// deckInfo is a preconstructed deck found in the card data's printings.
type deckInfo struct {
	Set, Name  string
	Owner      string // "Finn" from "Finn's Blue Plains Deck"
	Cards      []string
	Landscapes [engine.NumLanes]card.Landscape
	LandOK     bool
	Modelled   int // distinct cards with their rules modelled
	Distinct   int
}

func (d deckInfo) key() string { return d.Set + " " + d.Name }

// playable says whether the deck can be used with the engine's normal rules.
func (d deckInfo) playable() bool {
	return d.LandOK && len(d.Cards) >= engine.MinDeckSize && d.Modelled == d.Distinct
}

// landscapesFromName reads the Landscape types out of a deck name, such as
// "Cornfield/Nice Lands". The data does not store them separately.
func landscapesFromName(name string) ([engine.NumLanes]card.Landscape, bool) {
	var out [engine.NumLanes]card.Landscape
	lower := strings.ToLower(name)
	type found struct {
		at int
		ls card.Landscape
	}
	var fs []found
	for _, c := range []struct {
		text string
		ls   card.Landscape
	}{
		{"cornfield", card.Cornfield}, {"blue plains", card.BluePlains}, {"useless swamp", card.UselessSwamp},
		{"nice lands", card.NiceLands}, {"sandy lands", card.SandyLands}, {"sandyland", card.SandyLands}, {"icy lands", card.IcyLands},
	} {
		if i := strings.Index(lower, c.text); i >= 0 {
			dup := false
			for _, f := range fs {
				dup = dup || f.ls == c.ls
			}
			if !dup {
				fs = append(fs, found{i, c.ls})
			}
		}
	}
	sort.Slice(fs, func(i, j int) bool { return fs[i].at < fs[j].at })
	switch len(fs) {
	case 1:
		for i := range out {
			out[i] = fs[0].ls
		}
	case 2:
		for i := range out {
			out[i] = fs[i/2].ls
		}
	default:
		return out, false
	}
	return out, true
}

func ownerFromName(name string) string {
	if i := strings.Index(name, "'s "); i > 0 {
		return name[:i]
	}
	if i := strings.Index(name, "' "); i > 0 {
		return name[:i]
	}
	return ""
}

// findDecks lists every preconstructed deck in the catalog, sorted by set and name.
func findDecks(cat card.Catalog) []deckInfo {
	type k struct{ set, name string }
	seen := map[k]bool{}
	for _, c := range cat {
		for _, p := range c.Printings {
			if p.Set != "" && p.Deck != "" && p.Copies > 0 {
				seen[k{p.Set, p.Deck}] = true
			}
		}
	}
	var out []deckInfo
	for key := range seen {
		d := deckInfo{Set: key.set, Name: key.name, Owner: ownerFromName(key.name), Cards: cat.Deck(key.set, key.name)}
		d.Landscapes, d.LandOK = landscapesFromName(key.name)
		distinct := map[string]bool{}
		for _, id := range d.Cards {
			if !distinct[id] {
				distinct[id] = true
				d.Distinct++
				if cat[id].Implemented() {
					d.Modelled++
				}
			}
		}
		out = append(out, d)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].key() < out[j].key() })
	return out
}

// pickDeck finds the single deck whose "SET Name" contains every word of query.
func pickDeck(decks []deckInfo, query string) (deckInfo, error) {
	words := strings.Fields(strings.ToLower(query))
	var hits []deckInfo
	for _, d := range decks {
		k := strings.ToLower(d.key())
		ok := len(words) > 0
		for _, w := range words {
			ok = ok && strings.Contains(k, w)
		}
		if ok {
			hits = append(hits, d)
		}
	}
	if len(hits) > 1 {
		// Several decks share a name (2v2 versions of Jake's deck): prefer
		// the one that can actually be played.
		var ready []deckInfo
		for _, h := range hits {
			if h.playable() {
				ready = append(ready, h)
			}
		}
		if len(ready) == 1 {
			hits = ready
		}
	}
	switch len(hits) {
	case 0:
		return deckInfo{}, fmt.Errorf("no deck matches %q (try --list-decks)", query)
	case 1:
		return hits[0], nil
	}
	var names []string
	for _, h := range hits {
		names = append(names, h.key())
	}
	return deckInfo{}, fmt.Errorf("%q is ambiguous: %s", query, strings.Join(names, "; "))
}

func listDecks(decks []deckInfo) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%-6s %-62s %5s  %s\n", "SET", "DECK", "CARDS", "STATUS")
	for _, d := range decks {
		status := "ready"
		switch {
		case !d.LandOK:
			status = "needs Landscapes (not in data)"
		case len(d.Cards) < engine.MinDeckSize:
			status = fmt.Sprintf("incomplete (%d cards)", len(d.Cards))
		case d.Modelled < d.Distinct:
			status = fmt.Sprintf("%d of %d cards modelled", d.Modelled, d.Distinct)
		}
		fmt.Fprintf(&b, "%-6s %-62s %5d  %s\n", d.Set, d.Name, len(d.Cards), status)
	}
	return b.String()
}
