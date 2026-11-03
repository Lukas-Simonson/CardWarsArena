// Package decks finds the preconstructed decks in the card data and works out
// the Landscapes each one is played with.
package decks

import (
	"fmt"
	"sort"
	"strings"

	"cardwars/internal/card"
	"cardwars/internal/engine"
)

// Info is a preconstructed deck found in the card data's printings.
type Info struct {
	Set        string                          `json:"set"`
	Name       string                          `json:"name"`
	Owner      string                          `json:"owner"`          // "Finn" from "Finn's Blue Plains Deck"
	Cards      []string                        `json:"-"`              // the playable cards; the Hero is kept apart
	Hero       string                          `json:"hero,omitempty"` // card id of the deck's Hero
	Size       int                             `json:"size"`
	Landscapes [engine.NumLanes]card.Landscape `json:"landscapes"`
	LandOK     bool                            `json:"-"`
	Modelled   int                             `json:"-"` // distinct cards with their rules modelled
	Distinct   int                             `json:"-"`
	Ready      bool                            `json:"playable"`
}

// Spec turns the deck into what the engine needs to start a game.
func (d Info) Spec() engine.DeckSpec {
	return engine.DeckSpec{Landscapes: d.Landscapes, Cards: d.Cards, Hero: d.Hero}
}

// Key is the "SET Name" identifier used to look a deck up.
func (d Info) Key() string { return d.Set + " " + d.Name }

// Playable says whether the deck can be used with the engine's normal rules.
func (d Info) Playable() bool {
	return d.LandOK && len(d.Cards) >= engine.MinDeckSize && d.Modelled == d.Distinct
}

// LandscapesOf works out the four Landscapes a deck is played with from its
// cards: the Landscape types they need (Rainbow cards fit any). One type fills
// all four lanes, two take two lanes each (the more common one first). The
// data does not store them separately.
func LandscapesOf(cat card.Catalog, ids []string) ([engine.NumLanes]card.Landscape, bool) {
	var out [engine.NumLanes]card.Landscape
	count := map[card.Landscape]int{}
	for _, id := range ids {
		if ls := cat[id].Landscape; ls != "" && ls != card.Rainbow {
			count[ls]++
		}
	}
	var kinds []card.Landscape
	for ls := range count {
		kinds = append(kinds, ls)
	}
	sort.Slice(kinds, func(i, j int) bool {
		if count[kinds[i]] != count[kinds[j]] {
			return count[kinds[i]] > count[kinds[j]]
		}
		return kinds[i] < kinds[j]
	})
	switch len(kinds) {
	case 1:
		for i := range out {
			out[i] = kinds[0]
		}
	case 2:
		for i := range out {
			out[i] = kinds[i/2]
		}
	default:
		return out, false
	}
	return out, true
}

func OwnerFromName(name string) string {
	if i := strings.Index(name, "'s "); i > 0 {
		return name[:i]
	}
	if i := strings.Index(name, "' "); i > 0 {
		return name[:i]
	}
	return ""
}

// Find lists every preconstructed deck in the catalog, sorted by set and name.
func Find(cat card.Catalog) []Info {
	type k struct{ set, name string }
	seen := map[k]bool{}
	for _, c := range cat {
		for _, p := range c.Printings {
			if p.Set != "" && p.Deck != "" && p.Copies > 0 {
				seen[k{p.Set, p.Deck}] = true
			}
		}
	}
	var out []Info
	for key := range seen {
		d := Info{Set: key.set, Name: key.name, Owner: OwnerFromName(key.name), Cards: cat.Deck(key.set, key.name), Hero: cat.Hero(key.set, key.name)}
		d.Landscapes, d.LandOK = LandscapesOf(cat, d.Cards)
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
		if h := cat[d.Hero]; h != nil { // a Hero whose rules are missing makes the deck unplayable too
			d.Distinct++
			if h.Implemented() {
				d.Modelled++
			}
		}
		d.Size = len(d.Cards)
		d.Ready = d.Playable()
		out = append(out, d)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Key() < out[j].Key() })
	return out
}

// Pick finds the single deck whose "SET Name" contains every word of query.
// A query that names exactly one deck on its own ("finn") wins over matches on
// the set's name too ("Finn Vs Jake").
func Pick(decks []Info, query string) (Info, error) {
	words := strings.Fields(strings.ToLower(query))
	match := func(text func(Info) string) []Info {
		var hits []Info
		for _, d := range decks {
			k := strings.ToLower(text(d))
			ok := len(words) > 0
			for _, w := range words {
				ok = ok && strings.Contains(k, w)
			}
			if ok {
				hits = append(hits, d)
			}
		}
		return hits
	}
	hits := match(func(d Info) string { return d.Name })
	if len(hits) != 1 {
		hits = match(Info.Key)
	}
	if len(hits) > 1 {
		// Several decks match: prefer
		// the one that can actually be played.
		var ready []Info
		for _, h := range hits {
			if h.Playable() {
				ready = append(ready, h)
			}
		}
		if len(ready) == 1 {
			hits = ready
		}
	}
	switch len(hits) {
	case 0:
		return Info{}, fmt.Errorf("no deck matches %q (try --list-decks)", query)
	case 1:
		return hits[0], nil
	}
	var names []string
	for _, h := range hits {
		names = append(names, h.Key())
	}
	return Info{}, fmt.Errorf("%q is ambiguous: %s", query, strings.Join(names, "; "))
}

// Table formats a deck list for people.
func Table(decks []Info) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%-24s %-22s %5s  %s\n", "SET", "DECK", "CARDS", "STATUS")
	for _, d := range decks {
		status := "ready"
		switch {
		case !d.LandOK:
			status = "cannot tell its Landscapes from its cards"
		case len(d.Cards) < engine.MinDeckSize:
			status = fmt.Sprintf("incomplete (%d cards)", len(d.Cards))
		case d.Modelled < d.Distinct:
			status = fmt.Sprintf("%d of %d cards modelled", d.Modelled, d.Distinct)
		}
		fmt.Fprintf(&b, "%-24s %-22s %5d  %s\n", d.Set, d.Name, len(d.Cards), status)
	}
	return b.String()
}
