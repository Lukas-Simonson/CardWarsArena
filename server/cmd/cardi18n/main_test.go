package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"cardwars/internal/card"
)

func TestInitStartsALanguageAndOnlyAddsWhatIsMissing(t *testing.T) {
	cat := card.Catalog{
		"pup":  {ID: "pup", Name: "Pup", Type: card.Creature, Text: "Draw a card.", Printings: []card.Printing{{Set: "Finn Vs Jake", Deck: "Finn's Deck", Copies: 2}}},
		"rock": {ID: "rock", Name: "Rock", Type: card.Creature},
	}
	data := t.TempDir()
	if err := initLanguage(cat, data, "fr"); err != nil {
		t.Fatal(err)
	}
	l, err := card.LoadLocales(data) // what init writes must be readable
	if err != nil {
		t.Fatal(err)
	}
	if got := l["fr"].Cards["pup"]; got.Name != "" || got.EnName != "Pup" || got.EnText != "Draw a card." || got.Source != card.Fingerprint(cat["pup"]) {
		t.Errorf("skeleton: %+v", got)
	}
	if d := l["fr"].Decks[card.DeckKey("Finn Vs Jake", "Finn's Deck")]; d.Owner != "Finn" {
		t.Errorf("deck skeleton: %+v", d)
	}
	if r := l.Check(cat, "fr"); r.Translated != 0 || len(r.Missing) != 2 {
		t.Errorf("a skeleton has nothing translated: %+v", r)
	}

	// A translator fills one in; a new card arrives; init adds only the new one and keeps the work.
	file := filepath.Join(data, "i18n", "fr", "cards.json")
	b, _ := os.ReadFile(file)
	edited := strings.Replace(string(b), `"name": ""`, `"name": "Chiot"`, 1)
	if err := os.WriteFile(file, []byte(edited), 0o644); err != nil {
		t.Fatal(err)
	}
	cat["new"] = &card.Card{ID: "new", Name: "New", Type: card.Spell}
	if err := initLanguage(cat, data, "fr"); err != nil {
		t.Fatal(err)
	}
	l, err = card.LoadLocales(data)
	if err != nil {
		t.Fatal(err)
	}
	names := l["fr"].Cards["new"].EnName + "/" + l["fr"].Cards["pup"].Name + l["fr"].Cards["rock"].Name
	if names != "New/Chiot" && names != "New/" {
		t.Errorf("the second init kept what was there and added the new card: %q", names)
	}
	if l.Check(cat, "fr").Translated != 1 {
		t.Error("the translator's work was lost")
	}
	if status(cat, l, []string{"fr"}) != 0 {
		t.Error("status of a clean language should succeed")
	}
	l["fr"].Cards["ghost"] = card.Translation{Name: "Fantôme"}
	if status(cat, l, []string{"fr"}) != 1 {
		t.Error("a translation for a card that is gone should fail status")
	}
}
