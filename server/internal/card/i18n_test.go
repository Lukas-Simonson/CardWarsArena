package card

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func writeFile(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func sampleCatalog() Catalog {
	return Catalog{
		"finn": {ID: "finn", Name: "Finn", Type: Hero, Text: "Your Creatures have +1 ATK.", Printings: []Printing{{Set: "Finn Vs Jake", Deck: "Finn's Deck", Copies: 1}}},
		"pup":  {ID: "pup", Name: "Pup", Type: Creature, Text: "Draw a card.", Notes: "Good boy.", ImageURL: "https://x/pup.webp"},
		"rock": {ID: "rock", Name: "Rock", Type: Creature},
	}
}

func TestChainWalksFromTheSpecificToTheGeneral(t *testing.T) {
	for tag, want := range map[string][]string{
		"fr-CA": {"fr-CA", "fr"}, "fr": {"fr"}, "zh-Hant-TW": {"zh-Hant-TW", "zh-Hant", "zh"},
		"en": nil, "en-GB": nil, "": nil,
	} {
		if got := Chain(tag); !reflect.DeepEqual(got, want) {
			t.Errorf("Chain(%q) = %v, want %v", tag, got, want)
		}
	}
}

func TestATranslationReplacesWhatItHasAndLeavesTheRestEnglish(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "i18n", "fr", "cards.json"), `{"pup": {"name": "Chiot", "text": "Piochez une carte."}, "finn": {"name": "Finn"}}`)
	writeFile(t, filepath.Join(root, "i18n", "fr-CA", "cards.json"), `{"pup": {"name": "Chiot (CA)"}}`)
	writeFile(t, filepath.Join(root, "i18n", "fr", "decks.json"), `{"Finn Vs Jake|Finn's Deck": {"title": "Le deck de Finn", "owner": "Finn"}}`)
	l, err := LoadLocales(root)
	if err != nil {
		t.Fatal(err)
	}
	cat := sampleCatalog()

	fr, en := l.Localized(cat["pup"], "fr")
	if fr.Name != "Chiot" || fr.Text != "Piochez une carte." || fr.Notes != "Good boy." || fr.ImageURL != "https://x/pup.webp" || en != "Pup" {
		t.Errorf("fr: %+v (English name %q): fields it lacks stay English", fr, en)
	}
	ca, _ := l.Localized(cat["pup"], "fr-CA")
	if ca.Name != "Chiot (CA)" || ca.Text != "Piochez une carte." {
		t.Errorf("fr-CA: the most specific wins, the rest comes from fr: %+v", ca)
	}
	if got, en := l.Localized(cat["rock"], "fr"); got.Name != "Rock" || en != "" {
		t.Errorf("an untranslated card is English with no name_en: %+v %q", got, en)
	}
	if got, _ := l.Localized(cat["pup"], "de"); got.Name != "Pup" {
		t.Errorf("a language we lack is English: %+v", got)
	}
	if got, _ := l.Localized(cat["pup"], "en-GB"); got.Name != "Pup" {
		t.Errorf("English is the data itself: %+v", got)
	}
	if cat["pup"].Name != "Pup" {
		t.Error("localizing changed the catalogue")
	}
	if d, ok := l.Deck("Finn Vs Jake", "Finn's Deck", "fr-CA"); !ok || d.Title != "Le deck de Finn" || d.Owner != "Finn" {
		t.Errorf("deck: %+v %v", d, ok)
	}
	if _, ok := l.Deck("Finn Vs Jake", "Finn's Deck", "de"); ok {
		t.Error("a deck of a language we lack")
	}
	var none Locales
	if got, _ := none.Localized(cat["pup"], "fr"); got.Name != "Pup" {
		t.Error("no locales at all means English")
	}
}

func TestLocalesAreStrictAndMayBeAbsent(t *testing.T) {
	if l, err := LoadLocales(t.TempDir()); err != nil || len(l) != 0 {
		t.Errorf("no i18n directory: %v %v", l, err)
	}
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "i18n", "fr", "cards.json"), `{"pup": {"nmae": "Chiot"}}`)
	if _, err := LoadLocales(root); err == nil || !strings.Contains(err.Error(), "nmae") {
		t.Errorf("a typo in a field should be an error: %v", err)
	}
}

func TestCheckFindsMissingStaleAndUnknownTranslations(t *testing.T) {
	cat := sampleCatalog()
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "i18n", "fr", "cards.json"),
		`{"pup": {"name": "Chiot", "source": "`+Fingerprint(cat["pup"])+`"}, "finn": {"name": "Finn", "source": "old"}, "gone": {"name": "Parti"}, "rock": {"source": "x"}}`)
	writeFile(t, filepath.Join(root, "i18n", "fr", "decks.json"), `{"Finn Vs Jake|Finn's Deck": {"title": "Le deck de Finn"}}`)
	l, err := LoadLocales(root)
	if err != nil {
		t.Fatal(err)
	}
	r := l.Check(cat, "fr")
	if r.Translated != 2 || !reflect.DeepEqual(r.Missing, []string{"rock"}) || !reflect.DeepEqual(r.Stale, []string{"finn"}) || !reflect.DeepEqual(r.Unknown, []string{"gone"}) {
		t.Errorf("report %+v", r)
	}
	if r.Decks != 1 || r.DecksDone != 1 {
		t.Errorf("decks %d of %d", r.DecksDone, r.Decks)
	}
	// Editing the English makes a translation out of date.
	before := Fingerprint(cat["pup"])
	cat["pup"].Text = "Draw two cards."
	if Fingerprint(cat["pup"]) == before {
		t.Error("the fingerprint ignored a change to the text")
	}
}

// What ships must match the catalogue: a translation for a card that was removed or renamed is a mistake to fix.
func TestShippedTranslationsMatchTheCatalogue(t *testing.T) {
	cat, err := Load("../../data")
	if err != nil {
		t.Fatal(err)
	}
	l, err := LoadLocales("../../data")
	if err != nil {
		t.Fatal(err)
	}
	for _, tag := range l.Tags() {
		if r := l.Check(cat, tag); len(r.Unknown) > 0 {
			t.Errorf("%s has translations for cards that do not exist: %v", tag, r.Unknown)
		}
	}
}
