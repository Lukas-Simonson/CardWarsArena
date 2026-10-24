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
	Owner      string                          `json:"owner"` // "Finn" from "Finn's Blue Plains Deck"
	Cards      []string                        `json:"-"`
	Size       int                             `json:"size"`
	Landscapes [engine.NumLanes]card.Landscape `json:"landscapes"`
	LandOK     bool                            `json:"-"`
	Modelled   int                             `json:"-"` // distinct cards with their rules modelled
	Distinct   int                             `json:"-"`
	Ready      bool                            `json:"playable"`
}

// Spec turns the deck into what the engine needs to start a game.
func (d Info) Spec() engine.DeckSpec {
	return engine.DeckSpec{Landscapes: d.Landscapes, Cards: d.Cards}
}

// Key is the "SET Name" identifier used to look a deck up.
func (d Info) Key() string { return d.Set + " " + d.Name }

// Playable says whether the deck can be used with the engine's normal rules.
func (d Info) Playable() bool {
	return d.LandOK && len(d.Cards) >= engine.MinDeckSize && d.Modelled == d.Distinct
}

// LandscapesFromName reads the Landscape types out of a deck name, such as
// "Cornfield/Nice Lands". The data does not store them separately.
func LandscapesFromName(name string) ([engine.NumLanes]card.Landscape, bool) {
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
		d := Info{Set: key.set, Name: key.name, Owner: OwnerFromName(key.name), Cards: cat.Deck(key.set, key.name)}
		d.Landscapes, d.LandOK = LandscapesFromName(key.name)
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
		d.Size = len(d.Cards)
		d.Ready = d.Playable()
		out = append(out, d)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Key() < out[j].Key() })
	return out
}

// Pick finds the single deck whose "SET Name" contains every word of query.
func Pick(decks []Info, query string) (Info, error) {
	words := strings.Fields(strings.ToLower(query))
	var hits []Info
	for _, d := range decks {
		k := strings.ToLower(d.Key())
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
