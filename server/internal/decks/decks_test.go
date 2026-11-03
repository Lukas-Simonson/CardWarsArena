package decks

import (
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
			t.Errorf("%s: ready=%v landscapes=%v hero=%q", d.Key(), d.Ready, d.LandOK, d.Hero)
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
