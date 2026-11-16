// Command cardi18n helps translate the card data into a language. The English catalogue (data/cards) is written by
// cardimport; a language lives in data/i18n/<tag>/ and is never touched by the importer.
//
//	cardi18n status [tag...]   how complete each language is, which translations are out of date, and any for cards that are gone
//	cardi18n init <tag>        start a language, or add the cards it lacks (cards.json and decks.json), for a translator to fill in
//
// Both take -data (default "data"). status exits 1 when a translation is for a card that no longer exists.
package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"cardwars/internal/card"
)

func main() {
	data := flag.String("data", "data", "directory with cards/, effects/ and i18n/")
	flag.Usage = func() {
		fmt.Fprintln(os.Stderr, "usage: cardi18n [-data dir] status [tag...] | init <tag>")
		flag.PrintDefaults()
	}
	flag.Parse()
	args := flag.Args()
	if len(args) == 0 {
		flag.Usage()
		os.Exit(2)
	}
	cat, err := card.Load(*data)
	if err != nil {
		fatal(err)
	}
	locales, err := card.LoadLocales(*data)
	if err != nil {
		fatal(err)
	}
	switch args[0] {
	case "status":
		os.Exit(status(cat, locales, args[1:]))
	case "init":
		if len(args) != 2 || !tagRE.MatchString(args[1]) {
			fatal(fmt.Errorf("init needs one language tag, such as fr or pt-BR"))
		}
		if err := initLanguage(cat, *data, args[1]); err != nil {
			fatal(err)
		}
	default:
		flag.Usage()
		os.Exit(2)
	}
}

var tagRE = regexp.MustCompile(`^[a-z]{2,3}(-[A-Za-z0-9]{2,8})*$`)

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "cardi18n:", err)
	os.Exit(1)
}

func status(cat card.Catalog, l card.Locales, tags []string) int {
	if len(tags) == 0 {
		tags = l.Tags()
	}
	if len(tags) == 0 {
		fmt.Println("no languages yet: cardi18n init <tag> starts one")
		return 0
	}
	bad := 0
	for _, tag := range tags {
		if l[tag] == nil {
			fmt.Printf("%s: no translations (cardi18n init %s)\n", tag, tag)
			continue
		}
		r := l.Check(cat, tag)
		fmt.Printf("%s: %d of %d cards translated, %d of %d decks", tag, r.Translated, r.Cards, r.DecksDone, r.Decks)
		fmt.Printf(", %d out of date, %d for cards that no longer exist\n", len(r.Stale), len(r.Unknown))
		list := func(title string, ids []string) {
			if len(ids) > 0 {
				fmt.Printf("  %s: %s\n", title, strings.Join(ids, ", "))
			}
		}
		list("out of date (the English changed)", r.Stale)
		list("not in the catalogue", r.Unknown)
		bad += len(r.Unknown)
	}
	if bad > 0 {
		return 1
	}
	return 0
}

// skeleton is what a translator is given for a card: empty name and text to fill in, the English to read, and the
// fingerprint that will say when the English changes.
type skeleton struct {
	Name   string `json:"name"`
	Text   string `json:"text"`
	EnName string `json:"en_name"`
	EnText string `json:"en_text,omitempty"`
	Source string `json:"source"`
}

type deckSkeleton struct {
	Title string `json:"title"`
	Owner string `json:"owner,omitempty"`
	Set   string `json:"set"`
}

func initLanguage(cat card.Catalog, data, tag string) error {
	dir := filepath.Join(data, "i18n", tag)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	ids := make([]string, 0, len(cat))
	for id := range cat {
		ids = append(ids, id)
	}
	sort.Strings(ids)

	cardsFile := filepath.Join(dir, "cards.json")
	have := map[string]json.RawMessage{}
	if err := readRaw(cardsFile, &have); err != nil {
		return err
	}
	added := 0
	for _, id := range ids {
		if _, ok := have[id]; ok {
			continue
		}
		c := cat[id]
		b, _ := json.Marshal(skeleton{EnName: c.Name, EnText: c.Text, Source: card.Fingerprint(c)})
		have[id] = b
		added++
	}
	if err := writeSorted(cardsFile, have); err != nil {
		return err
	}

	decksFile := filepath.Join(dir, "decks.json")
	haveDecks := map[string]json.RawMessage{}
	if err := readRaw(decksFile, &haveDecks); err != nil {
		return err
	}
	addedDecks := 0
	for _, id := range ids {
		for _, p := range cat[id].Printings {
			key := card.DeckKey(p.Set, p.Deck)
			if p.Set == "" || p.Deck == "" || p.Copies == 0 {
				continue
			}
			if _, ok := haveDecks[key]; !ok {
				b, _ := json.Marshal(deckSkeleton{Owner: ownerOf(p.Deck)})
				haveDecks[key] = b
				addedDecks++
			}
		}
	}
	if err := writeSorted(decksFile, haveDecks); err != nil {
		return err
	}
	fmt.Printf("%s: added %d cards and %d decks to %s (fill in name and text; delete a card's entry to leave it in English)\n", tag, added, addedDecks, dir)
	return nil
}

// ownerOf is "Finn" from "Finn's Deck", the way the deck finder reads it.
func ownerOf(deck string) string {
	if i := strings.Index(deck, "'s "); i > 0 {
		return deck[:i]
	}
	return ""
}

func readRaw(path string, into *map[string]json.RawMessage) error {
	b, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	return json.Unmarshal(b, into)
}

func writeSorted(path string, m map[string]json.RawMessage) error {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var out bytes.Buffer
	out.WriteString("{\n")
	for i, k := range keys {
		kb, _ := json.Marshal(k)
		var pretty bytes.Buffer
		if err := json.Indent(&pretty, m[k], " ", " "); err != nil {
			return err
		}
		fmt.Fprintf(&out, " %s: %s", kb, pretty.String())
		if i < len(keys)-1 {
			out.WriteString(",")
		}
		out.WriteString("\n")
	}
	out.WriteString("}\n")
	return os.WriteFile(path, out.Bytes(), 0o644)
}
