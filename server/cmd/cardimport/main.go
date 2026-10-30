// Command cardimport converts the card catalogue (data/CardWarsCatalogue.json)
// into the canonical per-type files under data/cards, and prints a report of
// every normalization it applied or problem it found.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"cardwars/internal/card"
)

// rawCard is one entry of the catalogue.
type rawCard struct {
	Name     string         `json:"name"`
	Type     string         `json:"type"`
	Faction  string         `json:"faction"`
	Cost     *int           `json:"cost"`
	Attack   *int           `json:"attack"`
	Defense  *int           `json:"defense"`
	Ability  string         `json:"ability"` // the rules text
	Notes    string         `json:"notes"`   // flavour text
	Image    string         `json:"image"`   // file name of the scan
	Quantity int            `json:"quantity"`
	Sets     map[string]int `json:"sets"` // "Finn Vs Jake (Finn)" -> copies
	Extras   map[string]any `json:"extras"`
}

func main() {
	in := flag.String("in", "data/CardWarsCatalogue.json", "card catalogue")
	out := flag.String("out", "data/cards", "output directory")
	images := flag.String("image-base", "https://lukas-simonson.github.io/image-repo/card_wars/", "URL the scans are published under; each card's image file name is appended")
	flag.Parse()

	data, err := os.ReadFile(*in)
	if err != nil {
		fatal(err)
	}
	var raws []rawCard
	if err := json.Unmarshal(data, &raws); err != nil {
		fatal(err)
	}

	cards, report := convert(raws, *images)

	byType := map[card.Type][]card.Card{}
	for _, c := range cards {
		byType[c.Type] = append(byType[c.Type], c)
	}
	if err := os.MkdirAll(*out, 0o755); err != nil {
		fatal(err)
	}
	for t, list := range byType {
		file := filepath.Join(*out, fileName(t))
		b, err := json.MarshalIndent(list, "", "  ")
		if err != nil {
			fatal(err)
		}
		if err := os.WriteFile(file, append(b, '\n'), 0o644); err != nil {
			fatal(err)
		}
		fmt.Printf("wrote %s (%d cards)\n", file, len(list))
	}
	fmt.Println()
	fmt.Print(report.String())
}

func fileName(t card.Type) string {
	if t == card.Hero {
		return "heroes.json"
	}
	return string(t) + "s.json"
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "cardimport:", err)
	os.Exit(1)
}

// Report collects what the converter changed or could not resolve.
type Report struct {
	Notes    []string // normalizations that lost nothing
	Problems []string // data inconsistencies left for a human
}

func (r *Report) String() string {
	var b strings.Builder
	section := func(title string, lines []string) {
		fmt.Fprintf(&b, "== %s (%d)\n", title, len(lines))
		for _, l := range lines {
			fmt.Fprintf(&b, "  - %s\n", l)
		}
	}
	section("Problems needing a human", r.Problems)
	section("Normalizations", r.Notes)
	return b.String()
}

// factions maps the catalogue's faction names (spaces and case ignored) to Landscapes.
var factions = map[string]card.Landscape{
	"cornfield":    card.Cornfield,
	"blueplains":   card.BluePlains,
	"uselessswamp": card.UselessSwamp,
	"nicelands":    card.NiceLands,
	"sandylands":   card.SandyLands,
	"sandyland":    card.SandyLands,
	"icylands":     card.IcyLands,
	"rainbow":      card.Rainbow,
}

var types = map[string]card.Type{
	"creature": card.Creature,
	"building": card.Building,
	"spell":    card.Spell,
	"hero":     card.Hero,
}

// setKeyRe splits "Finn Vs Jake (Finn)" into the set and the deck's owner.
var setKeyRe = regexp.MustCompile(`^(.*?) \((.+)\)$`)

// deckName is how a deck is called: its owner's name, "Finn's Deck".
func deckName(owner string) string { return owner + "'s Deck" }

type deckKey struct{ set, owner string }

func convert(raws []rawCard, imageBase string) ([]card.Card, *Report) {
	rep := &Report{}
	var cards []card.Card
	seen := map[string]string{}
	decks := map[deckKey]bool{}
	var heroes []int // indexes into cards

	for _, r := range raws {
		t, ok := types[strings.ToLower(r.Type)]
		if !ok {
			rep.Problems = append(rep.Problems, fmt.Sprintf("%s: unknown type %q", r.Name, r.Type))
			continue
		}
		c := card.Card{
			ID:      slug(r.Name),
			Name:    r.Name,
			Type:    t,
			Cost:    r.Cost,
			Attack:  r.Attack,
			Defense: r.Defense,
			Text:    strings.TrimSpace(r.Ability),
			Notes:   strings.TrimSpace(r.Notes),
		}
		if r.Image != "" {
			c.ImageURL = imageBase + url.PathEscape(r.Image)
		}
		if prev, dup := seen[c.ID]; dup {
			rep.Problems = append(rep.Problems, fmt.Sprintf("%s: id %q collides with %q", r.Name, c.ID, prev))
		}
		seen[c.ID] = r.Name
		if len(r.Extras) > 0 {
			rep.Problems = append(rep.Problems, fmt.Sprintf("%s: has extras, which the importer does not read", r.Name))
		}

		if r.Faction != "" {
			ls, ok := factions[strings.ToLower(strings.ReplaceAll(r.Faction, " ", ""))]
			if !ok {
				rep.Problems = append(rep.Problems, fmt.Sprintf("%s: unknown faction %q", r.Name, r.Faction))
			}
			c.Landscape = ls
		}

		// A Hero belongs to its owner's deck, whatever sets the catalogue lists; see below.
		if t != card.Hero {
			total := 0
			for key, copies := range r.Sets {
				m := setKeyRe.FindStringSubmatch(key)
				if m == nil {
					rep.Problems = append(rep.Problems, fmt.Sprintf("%s: cannot read set %q (want \"Set (Deck)\")", r.Name, key))
					continue
				}
				c.Printings = append(c.Printings, card.Printing{Set: m[1], Deck: deckName(m[2]), Copies: copies})
				decks[deckKey{m[1], m[2]}] = true
				total += copies
			}
			if r.Quantity != total {
				rep.Problems = append(rep.Problems, fmt.Sprintf("%s: quantity %d but the sets add up to %d", r.Name, r.Quantity, total))
			}
			sort.Slice(c.Printings, func(i, j int) bool {
				return c.Printings[i].Set+c.Printings[i].Deck < c.Printings[j].Set+c.Printings[j].Deck
			})
		} else {
			heroes = append(heroes, len(cards))
		}

		validate(&c, rep)
		cards = append(cards, c)
	}

	// Heroes: each goes with the deck whose owner it is named for ("Lady Rainicorn" is Lady's).
	for _, i := range heroes {
		c := &cards[i]
		var owned []deckKey
		for d := range decks {
			if c.Name == d.owner || strings.HasPrefix(c.Name, d.owner+" ") {
				owned = append(owned, d)
			}
		}
		if len(owned) != 1 {
			rep.Problems = append(rep.Problems, fmt.Sprintf("%s: could not tell whose Hero this is (%d decks match)", c.Name, len(owned)))
			continue
		}
		c.Printings = []card.Printing{{Set: owned[0].set, Deck: deckName(owned[0].owner), Copies: 1}}
		rep.Notes = append(rep.Notes, fmt.Sprintf("%s: Hero placed in %s (%s), not the sets the catalogue lists", c.Name, deckName(owned[0].owner), owned[0].set))
	}

	rep.Notes = append(rep.Notes,
		"faction names normalized to snake_case Landscapes; card names and rules text kept exactly as given",
		"set \"Finn Vs Jake (Finn)\" read as set \"Finn Vs Jake\", deck \"Finn's Deck\"",
		"image file names turned into URLs under -image-base")

	sort.Slice(cards, func(i, j int) bool { return cards[i].ID < cards[j].ID })
	return cards, rep
}

// validate flags inconsistencies without changing the data.
func validate(c *card.Card, rep *Report) {
	switch c.Type {
	case card.Creature:
		if c.Attack == nil || c.Defense == nil {
			rep.Problems = append(rep.Problems, fmt.Sprintf("%s: creature missing ATK/DEF (variable stats?): %q", c.Name, c.Text))
		}
	case card.Hero:
		if c.Landscape != "" || c.Cost != nil {
			rep.Problems = append(rep.Problems, fmt.Sprintf("%s: hero has landscape/cost", c.Name))
		}
	default:
		if c.Attack != nil || c.Defense != nil {
			rep.Problems = append(rep.Problems, fmt.Sprintf("%s: %s has ATK/DEF", c.Name, c.Type))
		}
	}
	if c.Type != card.Hero && (c.Cost == nil || c.Landscape == "") {
		rep.Problems = append(rep.Problems, fmt.Sprintf("%s: missing cost or faction", c.Name))
	}
	if c.Text == "" && c.Type != card.Creature { // a vanilla Creature has no rules text
		rep.Problems = append(rep.Problems, fmt.Sprintf("%s: no rules text", c.Name))
	}
}

var nonAlnum = regexp.MustCompile(`[^a-z0-9]+`)

func slug(name string) string {
	return strings.Trim(nonAlnum.ReplaceAllString(strings.ToLower(name), "-"), "-")
}
