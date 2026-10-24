package decks

import (
	"testing"

	"cardwars/internal/card"
)

func TestLandscapesFromName(t *testing.T) {
	c, n, b, s, i := card.Cornfield, card.NiceLands, card.BluePlains, card.SandyLands, card.UselessSwamp
	for name, want := range map[string][4]card.Landscape{
		"Finn's Blue Plains Deck":                                                 {b, b, b, b},
		"Jake's Cornfield/Nice Lands Deck":                                        {c, c, n, n},
		"Lady Rainicorn's SandyLand Deck":                                         {s, s, s, s},
		"Lumpy Space Princess' Blue Plains/Useless Swamp Deck (Totally Rad Deck)": {b, b, i, i},
		"Moniker's Useless Swamp/Icy Lands Deck":                                  {i, i, card.IcyLands, card.IcyLands},
	} {
		got, ok := LandscapesFromName(name)
		if !ok || got != want {
			t.Errorf("%q: %v %v, want %v", name, got, ok, want)
		}
	}
	if _, ok := LandscapesFromName("Doubles Tournament"); ok {
		t.Error("no Landscape in the name: cannot tell")
	}
}

func TestOwnerFromName(t *testing.T) {
	for in, want := range map[string]string{
		"Finn's Blue Plains Deck":                "Finn",
		"Lumpy Space Princess' Blue Plains Deck": "Lumpy Space Princess",
		"Doubles Tournament":                     "",
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
	finn, err := Pick(all, "cp1 finn")
	if err != nil || !finn.Playable() || finn.Size != 40 || finn.Owner != "Finn" || !finn.Ready {
		t.Fatalf("Finn: %+v %v", finn, err)
	}
	if _, err := Pick(all, "jake"); err != nil {
		t.Errorf("\"jake\" should prefer the playable CP1 deck: %v", err)
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
