package decks

import (
	"fmt"
	"strings"
	"testing"

	"cardwars/internal/card"
)

func TestLandscapesOf(t *testing.T) {
	c, i, b := card.Cornfield, card.IcyLands, card.BluePlains
	cat := card.Catalog{
		"a": {Landscape: c}, "a2": {Landscape: c}, "i": {Landscape: i}, "b": {Landscape: b},
		"r": {Landscape: card.Rainbow}, "h": {},
	}
	for name, tc := range map[string]struct {
		ids  []string
		want [4]card.Landscape
		ok   bool
	}{
		"one type fills every lane":       {[]string{"a", "a2", "r", "h"}, [4]card.Landscape{c, c, c, c}, true},
		"two types, the common one first": {[]string{"i", "a", "a2"}, [4]card.Landscape{c, c, i, i}, true},
		"three types cannot be told":      {[]string{"a", "i", "b"}, [4]card.Landscape{}, false},
		"only Rainbow cannot be told":     {[]string{"r"}, [4]card.Landscape{}, false},
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
	good := Custom(cat, "Mine", "Ada", "finn", ok)
	if p := good.Problems(cat, false); len(p) != 0 || !good.Ready {
		t.Fatalf("a legal deck has problems %v (ready %v)", p, good.Ready)
	}
	for name, tc := range map[string]struct {
		d    Info
		want string
	}{
		"no hero":         {Custom(cat, "x", "o", "", ok), "choose a Hero"},
		"hero not ready":  {Custom(cat, "x", "o", "vagueh", ok), "Vague Hero's rules"},
		"too few":         {Custom(cat, "x", "o", "finn", ok[:10]), "10 cards: a deck needs at least 40"},
		"three types":     {Custom(cat, "x", "o", "finn", append([]string{"corn", "icy", "blue"}, ok...)), "3 Landscape types"},
		"only rainbow":    {Custom(cat, "x", "o", "finn", []string{"rain"}), "at least one Landscape type"},
		"unmodelled card": {Custom(cat, "x", "o", "finn", append([]string{"vague"}, ok...)), "Vague Spell's rules"},
	} {
		found := false
		for _, p := range tc.d.Problems(cat, false) {
			found = found || strings.Contains(p, tc.want)
		}
		if !found {
			t.Errorf("%s: problems %v lack %q", name, tc.d.Problems(cat, false), tc.want)
		}
	}
	// A development server plays cards whose rules are not modelled.
	withVague := Custom(cat, "x", "o", "vagueh", append([]string{"vague"}, ok...))
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
