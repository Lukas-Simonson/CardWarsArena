package card

import "testing"

func TestLoadDirAndStarterDeck(t *testing.T) {
	cat, err := Load("../../data")
	if err != nil {
		t.Fatal(err)
	}
	if len(cat) != 190 {
		t.Errorf("catalog has %d cards, want 190", len(cat))
	}
	pig, ok := cat["the-pig"]
	if !ok || pig.Landscape != Rainbow || pig.Cost == nil || *pig.Cost != 1 {
		t.Fatalf("the-pig not loaded as expected: %+v", pig)
	}
	if got := len(cat.Deck("Finn Vs Jake", "Finn's Deck")); got != 40 {
		t.Errorf("Finn deck has %d cards, want 40", got)
	}
}

func TestLoadDirMissing(t *testing.T) {
	if _, err := Load(t.TempDir()); err == nil {
		t.Error("expected error for empty directory")
	}
}
