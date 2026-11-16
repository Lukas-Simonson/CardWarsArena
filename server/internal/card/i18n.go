package card

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Translations of the printed card data. The catalogue (data/cards) is the English source and is what the importer
// writes; a language lives apart from it, in data/i18n/<tag>/, so re-importing never overwrites a translation:
//
//	data/i18n/fr/cards.json   { "finn": { "name": "…", "text": "…", "source": "ab12cd34ef56" } }
//	data/i18n/fr/decks.json   { "Finn Vs Jake|Finn's Deck": { "title": "…", "owner": "…", "set": "…" } }
//
// A field left out stays English, so a translation can be done a card at a time. The rules themselves (data/effects)
// are language-neutral. A tag such as fr-CA falls back to fr, then to English.

// Translation is one card's words in a language.
type Translation struct {
	Name          string `json:"name,omitempty"`
	Text          string `json:"text,omitempty"`
	Notes         string `json:"notes,omitempty"`         // flavour text
	Clarification string `json:"clarification,omitempty"` // the official ruling printed with the card
	ImageURL      string `json:"image_url,omitempty"`     // a scan of the card in this language
	// EnName and EnText are the English words, for the translator to read; nothing uses them.
	EnName string `json:"en_name,omitempty"`
	EnText string `json:"en_text,omitempty"`
	// Source is Fingerprint of the English card this was written for. When the English changes, the translation
	// is flagged as out of date (it is still used).
	Source string `json:"source,omitempty"`
}

// DeckTranslation is a preconstructed deck's words in a language. Decks are identified by Key ("Set|Deck").
type DeckTranslation struct {
	Title string `json:"title,omitempty"` // the deck's name: "Finn's Deck"
	Owner string `json:"owner,omitempty"` // whose it is: "Finn"
	Set   string `json:"set,omitempty"`   // the product: "Finn Vs Jake"
}

// DeckKey identifies a deck in decks.json.
func DeckKey(set, deck string) string { return set + "|" + deck }

// Language is everything translated into one language.
type Language struct {
	Cards map[string]Translation
	Decks map[string]DeckTranslation
}

// Locales are the languages with translations, by tag.
type Locales map[string]*Language

// LoadLocales reads every language in root/i18n. Unknown fields are errors, so a typo cannot silently lose a
// translation. A missing directory is fine: it means English only.
func LoadLocales(root string) (Locales, error) {
	dirs, err := filepath.Glob(filepath.Join(root, "i18n", "*"))
	if err != nil {
		return nil, err
	}
	out := Locales{}
	for _, dir := range dirs {
		if st, err := os.Stat(dir); err != nil || !st.IsDir() {
			continue
		}
		lang := &Language{Cards: map[string]Translation{}, Decks: map[string]DeckTranslation{}}
		if err := readStrict(filepath.Join(dir, "cards.json"), &lang.Cards); err != nil {
			return nil, err
		}
		if err := readStrict(filepath.Join(dir, "decks.json"), &lang.Decks); err != nil {
			return nil, err
		}
		out[filepath.Base(dir)] = lang
	}
	return out, nil
}

func readStrict(path string, into any) error {
	b, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.DisallowUnknownFields()
	if err := dec.Decode(into); err != nil {
		return fmt.Errorf("card: %s: %w", path, err)
	}
	return nil
}

// Tags lists the languages that have translations, sorted.
func (l Locales) Tags() []string {
	out := make([]string, 0, len(l))
	for tag := range l {
		out = append(out, tag)
	}
	sort.Strings(out)
	return out
}

// Chain is the tags to look through for tag, most specific first: fr-CA, fr. English is the card data itself.
func Chain(tag string) []string {
	var out []string
	for t := strings.TrimSpace(tag); t != "" && t != "en" && !strings.HasPrefix(t, "en-"); {
		out = append(out, t)
		i := strings.LastIndexByte(t, '-')
		if i < 0 {
			break
		}
		t = t[:i]
	}
	return out
}

// Localized is the card as a player of this language reads it: the translation's words where it has them, English
// where it does not. NameEN is the English name when the name differs, so a deck list in either can be read.
func (l Locales) Localized(c *Card, tag string) (card Card, nameEN string) {
	card = *c
	for _, t := range Chain(tag) {
		tr, ok := l[t].tr(c.ID)
		if !ok {
			continue
		}
		if card.Name == c.Name && tr.Name != "" {
			card.Name = tr.Name
		}
		if card.Text == c.Text && tr.Text != "" {
			card.Text = tr.Text
		}
		if card.Notes == c.Notes && tr.Notes != "" {
			card.Notes = tr.Notes
		}
		if card.Clarification == c.Clarification && tr.Clarification != "" {
			card.Clarification = tr.Clarification
		}
		if card.ImageURL == c.ImageURL && tr.ImageURL != "" {
			card.ImageURL = tr.ImageURL
		}
	}
	if card.Name != c.Name {
		nameEN = c.Name
	}
	return card, nameEN
}

func (l *Language) tr(id string) (Translation, bool) {
	if l == nil {
		return Translation{}, false
	}
	t, ok := l.Cards[id]
	return t, ok
}

// Deck is a deck's words in this language: the first tag in the chain that has them.
func (l Locales) Deck(set, deck, tag string) (DeckTranslation, bool) {
	var out DeckTranslation
	found := false
	for _, t := range Chain(tag) {
		if d, ok := l[t].deck(set, deck); ok {
			found = true
			if out.Title == "" {
				out.Title = d.Title
			}
			if out.Owner == "" {
				out.Owner = d.Owner
			}
			if out.Set == "" {
				out.Set = d.Set
			}
		}
	}
	return out, found
}

func (l *Language) deck(set, deck string) (DeckTranslation, bool) {
	if l == nil {
		return DeckTranslation{}, false
	}
	d, ok := l.Decks[DeckKey(set, deck)]
	return d, ok
}

// Fingerprint identifies the English words of a card, to tell when a translation was written for older ones.
func Fingerprint(c *Card) string {
	sum := sha256.Sum256([]byte(strings.Join([]string{c.Name, c.Text, c.Notes, c.Clarification}, "\x00")))
	return hex.EncodeToString(sum[:6])
}

// Report says how complete and how current one language is.
type Report struct {
	Tag        string
	Cards      int      // cards in the catalogue
	Translated int      // cards with a name or text in this language
	Missing    []string // ids of cards with nothing translated
	Stale      []string // ids whose English changed since they were translated
	Unknown    []string // ids that are not in the catalogue (the card was removed or renamed): an error
	Decks      int      // preconstructed decks, and how many are translated
	DecksDone  int
}

// Check compares a language with the catalogue.
func (l Locales) Check(cat Catalog, tag string) Report {
	r := Report{Tag: tag, Cards: len(cat)}
	lang := l[tag]
	if lang == nil {
		return r
	}
	for id := range lang.Cards {
		if cat[id] == nil {
			r.Unknown = append(r.Unknown, id)
		}
	}
	for id, c := range cat {
		tr, ok := lang.Cards[id]
		switch {
		case !ok || (tr.Name == "" && tr.Text == ""):
			r.Missing = append(r.Missing, id)
		default:
			r.Translated++
			if tr.Source != "" && tr.Source != Fingerprint(c) {
				r.Stale = append(r.Stale, id)
			}
		}
	}
	sort.Strings(r.Missing)
	sort.Strings(r.Stale)
	sort.Strings(r.Unknown)
	decks := map[string]bool{}
	for _, c := range cat {
		for _, p := range c.Printings {
			if p.Set != "" && p.Deck != "" && p.Copies > 0 {
				decks[DeckKey(p.Set, p.Deck)] = true
			}
		}
	}
	r.Decks = len(decks)
	for k := range decks {
		if d, ok := lang.Decks[k]; ok && d.Title != "" {
			r.DecksDone++
		}
	}
	return r
}
