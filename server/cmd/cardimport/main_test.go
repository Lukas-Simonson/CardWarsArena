package main

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	"cardwars/internal/card"
)

func loadConverted(t *testing.T) []card.Card {
	t.Helper()
	data, err := os.ReadFile("../../data/raw/cards.json")
	if err != nil {
		t.Fatal(err)
	}
	var raws []rawCard
	if err := json.Unmarshal(data, &raws); err != nil {
		t.Fatal(err)
	}
	cards, _ := convert(raws)
	return cards
}

// The rulebook's official deck lists: each starter deck is 40 non-Landscape cards.
func TestStarterDecksMatchRulebook(t *testing.T) {
	cards := loadConverted(t)
	for deck, want := range map[string]int{
		"Finn's Blue Plains Deck": 40,
		"Jake's Cornfield Deck":   40,
	} {
		got := 0
		for _, c := range cards {
			for _, p := range c.Printings {
				if p.Set == "CP1" && p.Deck == deck {
					got += p.Copies
				}
			}
		}
		if got != want {
			t.Errorf("%s: %d cards, want %d", deck, got, want)
		}
	}
}

func TestNoTeamworkAndUniqueIDs(t *testing.T) {
	seen := map[string]bool{}
	for _, c := range loadConverted(t) {
		if seen[c.ID] {
			t.Errorf("duplicate id %s", c.ID)
		}
		seen[c.ID] = true
		if strings.EqualFold(string(c.Type), "teamwork") {
			t.Errorf("teamwork card %s kept", c.Name)
		}
	}
	if len(seen) != 502 {
		t.Errorf("got %d cards, want 502", len(seen))
	}
}
