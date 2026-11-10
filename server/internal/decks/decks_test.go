package decks

import (
	"fmt"
	"strings"
	"testing"

	"cardwars/internal/card"
)

func TestLandscapesOf(t *testing.T) {
	c, i, b, s := card.Cornfield, card.IcyLands, card.BluePlains, card.SandyLands
	cat := card.Catalog{
		"a": {Landscape: c}, "a2": {Landscape: c}, "i": {Landscape: i}, "b": {Landscape: b},
		"s": {Landscape: card.SandyLands}, "n": {Landscape: card.NiceLands},
		"r": {Landscape: card.Rainbow}, "h": {},
	}
	for name, tc := range map[string]struct {
		ids  []string
		want [4]card.Landscape
		ok   bool
	}{
		"one type fills every lane":         {[]string{"a", "a2", "r", "h"}, [4]card.Landscape{c, c, c, c}, true},
		"two types, the common one first":   {[]string{"i", "a", "a2"}, [4]card.Landscape{c, c, i, i}, true},
		"three types, the common one twice": {[]string{"a", "a2", "i", "b"}, [4]card.Landscape{c, c, b, i}, true},
		"four types, one lane each":         {[]string{"a", "a2", "i", "b", "s"}, [4]card.Landscape{c, b, i, s}, true},
		"five types cannot be told":         {[]string{"a", "i", "b", "s", "n"}, [4]card.Landscape{}, false},
		"only Rainbow cannot be told":       {[]string{"r"}, [4]card.Landscape{}, false},
	} {
		got, ok := LandscapesOf(cat, tc.ids)
		if ok != tc.ok || (ok && got != tc.want) {
			t.Errorf("%s: %v %v, want %v %v", name, got, ok, tc.want, tc.ok)
		}
	}
}

func TestOwnerFromName(t *testing.T) {
	for in, want := range map[string]string{
		"Finn's Deck":                "Finn",
		"Lumpy Space Princess' Deck": "Lumpy Space Princess",
		"Doubles Tournament":         "",
	} {
		if got := OwnerFromName(in); got != want {
			t.Errorf("%q: %q, want %q", in, got, want)
		}
	}
}

func TestFindAndPick(t *testing.T) {
	cat, err := card.Load("../../data")
	if err != nil {
		t.Fatal(err)
	}
	all := Find(cat)
	finn, err := Pick(all, "finn")
	if err != nil || !finn.Playable() || finn.Size != 40 || finn.Owner != "Finn" || !finn.Ready || finn.Hero != "finn" {
		t.Fatalf("Finn: %+v %v", finn, err)
	}
	if finn.Landscapes[0] != card.BluePlains {
		t.Errorf("Finn plays Blue Plains, got %v", finn.Landscapes)
	}
	if _, err := Pick(all, "jake"); err != nil {
		t.Errorf("\"jake\" should find Jake's deck, not both decks of the set: %v", err)
	}
	if lady, err := Pick(all, "lady"); err != nil || lady.Hero != "lady-rainicorn" || lady.Landscapes[0] != card.SandyLands {
		t.Errorf("Lady: %+v %v", lady, err)
	}
	if byKey, err := Pick(all, "vs jake finn's"); err != nil || byKey.Owner != "Finn" {
		t.Errorf("the full key should work too: %+v %v", byKey, err)
	}
	for _, d := range all {
		if !d.Ready || !d.LandOK || d.Hero == "" {
			t.Errorf("%s: ready=%v landscapes=%v hero=%q modelled=%d/%d", d.Key(), d.Ready, d.LandOK, d.Hero, d.Modelled, d.Distinct)
		}
	}
	if _, err := Pick(all, "deck"); err == nil {
		t.Error("\"deck\" is ambiguous")
	}
	if _, err := Pick(all, "zzz"); err == nil {
		t.Error("no match should be an error")
	}
	if got := len(finn.Spec().Cards); got != 40 {
		t.Errorf("spec cards = %d", got)
	}
}

func customCatalog() card.Catalog {
	return card.Catalog{
		"corn":    {ID: "corn", Name: "Corn Dog", Type: card.Creature, Landscape: card.Cornfield},
		"icy":     {ID: "icy", Name: "Icicle", Type: card.Creature, Landscape: card.IcyLands},
		"blue":    {ID: "blue", Name: "Blue Thing", Type: card.Creature, Landscape: card.BluePlains},
		"sand":    {ID: "sand", Name: "Sandy Thing", Type: card.Creature, Landscape: card.SandyLands},
		"nice":    {ID: "nice", Name: "Nice Thing", Type: card.Creature, Landscape: card.NiceLands},
		"swamp":   {ID: "swamp", Name: "Swamp Thing", Type: card.Creature, Landscape: card.UselessSwamp},
		"rain":    {ID: "rain", Name: "Rainbow Pup", Type: card.Creature, Landscape: card.Rainbow},
		"vague":   {ID: "vague", Name: "Vague Spell", Type: card.Spell, Landscape: card.Cornfield, Text: "Does something."},
		"finn":    {ID: "finn", Name: "Finn", Type: card.Hero},
		"vagueh":  {ID: "vagueh", Name: "Vague Hero", Type: card.Hero, Text: "Does something."},
		"herocrd": {ID: "herocrd", Name: "Hero as card", Type: card.Hero},
	}
}

func repeat(id string, n int) []string {
	out := make([]string, n)
	for i := range out {
		out[i] = id
	}
	return out
}

func TestCheckRefusesDecksThatMakeNoSense(t *testing.T) {
	cat := customCatalog()
	for name, tc := range map[string]struct {
		hero  string
		cards []string
		ok    bool
	}{
		"an empty draft is fine":      {"", nil, true},
		"a short deck is fine":        {"finn", []string{"corn", "icy"}, true},
		"three copies are fine":       {"finn", repeat("corn", 3), true},
		"four copies are not":         {"finn", repeat("corn", 4), false},
		"an unknown card":             {"finn", []string{"nope"}, false},
		"a Hero among the cards":      {"finn", []string{"herocrd"}, false},
		"a card that is not a Hero":   {"corn", nil, false},
		"an unknown Hero":             {"nope", nil, false},
		"far too many cards":          {"", append(repeat("corn", 3), repeat("icy", MaxCards)...), false},
		"a deck of exactly the limit": {"", append(append(repeat("corn", 3), repeat("icy", 3)...), repeat("blue", 3)...), true},
	} {
		if err := Check(cat, tc.hero, tc.cards); (err == nil) != tc.ok {
			t.Errorf("%s: %v", name, err)
		}
	}
}

func TestProblemsSayWhatStopsADeckBeingPlayed(t *testing.T) {
	cat := customCatalog()
	// A legal deck: three Cornfield cards and thirteen Rainbow ones, three copies each (42 cards).
	ok := repeat("corn", 3)
	for i := 0; i < 13; i++ {
		id := fmt.Sprintf("rainbow%d", i)
		cat[id] = &card.Card{ID: id, Name: "Rainbow " + id, Type: card.Creature, Landscape: card.Rainbow}
		ok = append(ok, repeat(id, 3)...)
	}
	good := Custom(cat, "Mine", "Ada", "finn", ok, nil)
	if p := good.Problems(cat, false); len(p) != 0 || !good.Ready {
		t.Fatalf("a legal deck has problems %v (ready %v)", p, good.Ready)
	}
	for name, tc := range map[string]struct {
		d    Info
		want string
	}{
		"no hero":         {Custom(cat, "x", "o", "", ok, nil), "choose a Hero"},
		"hero not ready":  {Custom(cat, "x", "o", "vagueh", ok, nil), "Vague Hero's rules"},
		"too few":         {Custom(cat, "x", "o", "finn", ok[:10], nil), "10 cards: a deck needs at least 40"},
		"five types":      {Custom(cat, "x", "o", "finn", append([]string{"corn", "icy", "blue", "sand", "nice"}, ok...), nil), "5 Landscape types: choose your 4 Landscapes"},
		"only rainbow":    {Custom(cat, "x", "o", "finn", []string{"rain"}, nil), "choose your Landscapes"},
		"unmodelled card": {Custom(cat, "x", "o", "finn", append([]string{"vague"}, ok...), nil), "Vague Spell's rules"},
	} {
		found := false
		for _, p := range tc.d.Problems(cat, false) {
			found = found || strings.Contains(p, tc.want)
		}
		if !found {
			t.Errorf("%s: problems %v lack %q", name, tc.d.Problems(cat, false), tc.want)
		}
	}
	// Up to four types are fine: one for each Lane.
	four := Custom(cat, "x", "o", "finn", append([]string{"corn", "icy", "blue", "sand"}, ok...), nil)
	for _, p := range four.Problems(cat, false) {
		if strings.Contains(p, "Landscape") {
			t.Errorf("a deck of four Landscape types was refused: %q", p)
		}
	}
	// Landscapes the player chose are used whatever the cards say, so a deck of five types, or of Rainbow
	// cards alone, can be played once its Landscapes are picked.
	picked := []string{"cornfield", "cornfield", "icy_lands", "blue_plains"}
	for name, cards := range map[string][]string{"five types": append([]string{"corn", "icy", "blue", "sand", "nice"}, ok...), "only rainbow": append([]string{}, ok[3:]...)} {
		d := Custom(cat, "x", "o", "finn", cards, picked)
		if !d.LandOK || !d.Chosen || d.Landscapes != [4]card.Landscape{card.Cornfield, card.Cornfield, card.IcyLands, card.BluePlains} {
			t.Errorf("%s: chosen Landscapes not used: %v ok=%v chosen=%v", name, d.Landscapes, d.LandOK, d.Chosen)
		}
		for _, p := range d.Problems(cat, false) {
			if strings.Contains(p, "Landscape") {
				t.Errorf("%s: a deck with chosen Landscapes was refused: %q", name, p)
			}
		}
	}
	if d := Custom(cat, "x", "o", "finn", ok, nil); d.Chosen {
		t.Error("a deck with no chosen Landscapes says it chose them")
	}
	// A development server plays cards whose rules are not modelled.
	withVague := Custom(cat, "x", "o", "vagueh", append([]string{"vague"}, ok...), nil)
	for _, p := range withVague.Problems(cat, true) {
		if strings.Contains(p, "rules") {
			t.Errorf("unmodelled cards were reported on a server that allows them: %q", p)
		}
	}
}

func TestEveryPresetIsWithoutProblems(t *testing.T) {
	cat, err := card.Load("../../data")
	if err != nil {
		t.Fatal(err)
	}
	for _, d := range Find(cat) {
		if p := d.Problems(cat, false); len(p) != 0 {
			t.Errorf("%s: %v", d.Key(), p)
		}
	}
}

func TestCheckLandscapes(t *testing.T) {
	for name, tc := range map[string]struct {
		in []string
		ok bool
	}{
		"none: the cards decide":     {nil, true},
		"four, repeats allowed":      {[]string{"cornfield", "cornfield", "icy_lands", "nice_lands"}, true},
		"three":                      {[]string{"cornfield", "cornfield", "icy_lands"}, false},
		"five":                       {[]string{"cornfield", "cornfield", "cornfield", "cornfield", "cornfield"}, false},
		"rainbow is not a Landscape": {[]string{"cornfield", "cornfield", "cornfield", "rainbow"}, false},
		"an unknown one":             {[]string{"cornfield", "cornfield", "cornfield", "moon"}, false},
		"an empty name":              {[]string{"cornfield", "cornfield", "cornfield", ""}, false},
	} {
		if err := CheckLandscapes(tc.in); (err == nil) != tc.ok {
			t.Errorf("%s: %v", name, err)
		}
	}
}
