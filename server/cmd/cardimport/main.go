// Command cardimport converts the raw card dump (data/raw/cards.json) into the
// canonical per-type files under data/cards, and prints a report of every
// normalization it applied or problem it found.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"cardwars/internal/card"
)

type rawCard struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	CardType    string `json:"card_type"`
	Landscape   string `json:"landscape"`
	Cost        *int   `json:"cost"`
	Attack      *int   `json:"attack"`
	Defence     *int   `json:"defence"`
	Ability     string `json:"ability"`
	DeckInfo    string `json:"deck_info"`
	ImageURL    string `json:"image_url"`
}

func main() {
	in := flag.String("in", "data/raw/cards.json", "raw card dump")
	out := flag.String("out", "data/cards", "output directory")
	flag.Parse()

	data, err := os.ReadFile(*in)
	if err != nil {
		fatal(err)
	}
	var raws []rawCard
	if err := json.Unmarshal(data, &raws); err != nil {
		fatal(err)
	}

	cards, report := convert(raws)

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
	Dropped  []string // cards removed from scope
	Notes    []string // normalizations that lost nothing
	Problems []string // data inconsistencies left for a human
	Unparsed []string // printings kept as raw text
}

func (r *Report) String() string {
	var b strings.Builder
	section := func(title string, lines []string) {
		fmt.Fprintf(&b, "== %s (%d)\n", title, len(lines))
		for _, l := range lines {
			fmt.Fprintf(&b, "  - %s\n", l)
		}
	}
	section("Dropped (out of scope)", r.Dropped)
	section("Problems needing a human", r.Problems)
	section("Unparsed printings", r.Unparsed)
	section("Normalizations", r.Notes)
	return b.String()
}

var landscapes = map[string]card.Landscape{
	"cornfield":     card.Cornfield,
	"blue plains":   card.BluePlains,
	"useless swamp": card.UselessSwamp,
	"nice lands":    card.NiceLands,
	"sandy lands":   card.SandyLands,
	"icy lands":     card.IcyLands,
	"rainbow":       card.Rainbow,
}

var abilities = map[string]card.AbilityKind{
	"":              card.AbilityNone,
	"none":          card.AbilityNone,
	"continuous":    card.AbilityContinuous,
	"floop":         card.AbilityFloop,
	"entering play": card.AbilityEnteringPlay,
	"leaving play":  card.AbilityLeavingPlay,
	"trigger":       card.AbilityTrigger,
	"ignition":      card.AbilityIgnition,
}

var types = map[string]card.Type{
	"creature": card.Creature,
	"building": card.Building,
	"spell":    card.Spell,
	"hero":     card.Hero,
}

func convert(raws []rawCard) ([]card.Card, *Report) {
	rep := &Report{}
	var cards []card.Card
	seen := map[string]string{}
	noneAbility := 0

	for _, r := range raws {
		// 1v1 only: Teamwork cards belong to the 2v2 ruleset.
		if strings.EqualFold(r.CardType, "Teamwork") {
			rep.Dropped = append(rep.Dropped, r.Name+" (Teamwork, 2v2)")
			continue
		}
		t, ok := types[strings.ToLower(r.CardType)]
		if !ok {
			rep.Problems = append(rep.Problems, fmt.Sprintf("%s: unknown card_type %q", r.Name, r.CardType))
			continue
		}

		c := card.Card{
			ID:       slug(r.Name),
			Name:     r.Name,
			Type:     t,
			Cost:     r.Cost,
			Attack:   r.Attack,
			Defense:  r.Defence,
			Text:     strings.TrimSpace(r.Description),
			ImageURL: r.ImageURL,
		}
		if prev, dup := seen[c.ID]; dup {
			rep.Problems = append(rep.Problems, fmt.Sprintf("%s: id %q collides with %q", r.Name, c.ID, prev))
		}
		seen[c.ID] = r.Name

		if r.Landscape != "" {
			ls, ok := landscapes[strings.ToLower(r.Landscape)]
			if !ok {
				rep.Problems = append(rep.Problems, fmt.Sprintf("%s: unknown landscape %q", r.Name, r.Landscape))
			}
			c.Landscape = ls
		}

		ak, ok := abilities[strings.ToLower(r.Ability)]
		if !ok {
			rep.Problems = append(rep.Problems, fmt.Sprintf("%s: unknown ability %q", r.Name, r.Ability))
		}
		if r.Ability == "None" {
			noneAbility++
		}
		c.Ability = ak

		validate(&c, r, rep)

		for _, part := range strings.Split(r.DeckInfo, " : ") {
			p, ok := parsePrinting(part)
			if !ok {
				rep.Unparsed = append(rep.Unparsed, fmt.Sprintf("%s: %q", r.Name, strings.TrimSpace(part)))
			}
			c.Printings = append(c.Printings, p)
		}
		cards = append(cards, c)
	}

	if noneAbility > 0 {
		rep.Notes = append(rep.Notes, fmt.Sprintf(`ability "None" (string) treated as no ability on %d cards`, noneAbility))
	}
	rep.Notes = append(rep.Notes,
		"landscape and ability names normalized to snake_case enums; card names kept exactly as printed",
		"deck_info split on ' : ' into printings; trailing number = copies in that deck, trailing text = rarity")

	sort.Slice(cards, func(i, j int) bool { return cards[i].ID < cards[j].ID })
	return cards, rep
}

// validate flags inconsistencies without changing the data.
func validate(c *card.Card, r rawCard, rep *Report) {
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
		rep.Problems = append(rep.Problems, fmt.Sprintf("%s: missing cost or landscape", c.Name))
	}
	hasFloopText := strings.Contains(strings.ToUpper(c.Text), "FLOOP")
	if c.Type == card.Creature || c.Type == card.Building {
		if c.Ability == card.AbilityFloop && !hasFloopText {
			rep.Problems = append(rep.Problems, fmt.Sprintf("%s: ability is floop but text has no FLOOP", c.Name))
		}
		if c.Ability != card.AbilityFloop && hasFloopText {
			rep.Problems = append(rep.Problems, fmt.Sprintf("%s: text has FLOOP but ability is %q", c.Name, c.Ability))
		}
	}
	if c.Text == "" {
		rep.Problems = append(rep.Problems, fmt.Sprintf("%s: no description", c.Name))
	}
}

var printingRe = regexp.MustCompile(`^(.*?) \((\w+)\)(?: - (.+))?$`)

func parsePrinting(s string) (card.Printing, bool) {
	s = strings.Join(strings.Fields(s), " ") // collapses stray newlines/spaces
	m := printingRe.FindStringSubmatch(s)
	if m == nil {
		return card.Printing{Raw: s}, false
	}
	p := card.Printing{Deck: m[1], Set: m[2]}
	if m[3] != "" {
		if n, err := strconv.Atoi(m[3]); err == nil {
			p.Copies = n
		} else {
			p.Rarity = m[3]
		}
	}
	return p, true
}

var nonAlnum = regexp.MustCompile(`[^a-z0-9]+`)

func slug(name string) string {
	return strings.Trim(nonAlnum.ReplaceAllString(strings.ToLower(name), "-"), "-")
}
