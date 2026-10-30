package main

import (
	"encoding/json"
	"os"
	"testing"

	"cardwars/internal/card"
)

func loadConverted(t *testing.T) ([]card.Card, *Report) {
	t.Helper()
	data, err := os.ReadFile("../../data/CardWarsCatalogue.json")
	if err != nil {
		t.Fatal(err)
	}
	var raws []rawCard
	if err := json.Unmarshal(data, &raws); err != nil {
		t.Fatal(err)
	}
	return convert(raws, "https://img.example/")
}

func TestCatalogueConvertsCleanly(t *testing.T) {
	cards, rep := loadConverted(t)
	if len(rep.Problems) != 0 {
		t.Errorf("problems: %v", rep.Problems)
	}
	seen := map[string]bool{}
	for _, c := range cards {
		if seen[c.ID] {
			t.Errorf("duplicate id %s", c.ID)
		}
		seen[c.ID] = true
	}
	if len(seen) != 91 {
		t.Errorf("got %d cards, want 91", len(seen))
	}
}

// Each deck is 40 non-Hero cards (BMO's has one more) and has its own Hero.
func TestDecksAreWholeAndHaveTheirHero(t *testing.T) {
	cards, _ := loadConverted(t)
	size := map[string]int{}
	hero := map[string]string{}
	for _, c := range cards {
		for _, p := range c.Printings {
			if c.Type == card.Hero {
				hero[p.Deck] = c.Name
			} else {
				size[p.Deck] += p.Copies
			}
		}
	}
	for deck, want := range map[string]int{"Finn's Deck": 40, "Jake's Deck": 40, "BMO's Deck": 41, "Lady's Deck": 40} {
		if size[deck] != want {
			t.Errorf("%s: %d cards, want %d", deck, size[deck], want)
		}
	}
	if hero["Finn's Deck"] != "Finn" || hero["Lady's Deck"] != "Lady Rainicorn" || len(hero) != 4 {
		t.Errorf("heroes: %v", hero)
	}
}

func TestImageURLAndIDs(t *testing.T) {
	cards, _ := loadConverted(t)
	byName := map[string]card.Card{}
	for _, c := range cards {
		byName[c.Name] = c
	}
	if got := byName["Zazo's Magic Seeds"]; got.ID != "zazo-s-magic-seeds" || got.ImageURL != "https://img.example/Zazo%27s%20Magic%20Seeds.webp" && got.ImageURL != "https://img.example/Zazo's%20Magic%20Seeds.webp" {
		t.Errorf("Zazo: %q %q", got.ID, got.ImageURL)
	}
	if byName["Dr. Death"].ID != "dr-death" {
		t.Errorf("ids keep the old slug rule: %q", byName["Dr. Death"].ID)
	}
}
