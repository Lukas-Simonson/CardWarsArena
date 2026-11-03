package card

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const tinyCards = `[
 {"id":"grunt","name":"Grunt","type":"creature","landscape":"cornfield","cost":1,"attack":1,"defense":2,"text":"Pays off."},
 {"id":"zap","name":"Zap","type":"spell","landscape":"rainbow","cost":0,"text":"Zap."}
]`

func loadWithEffects(t *testing.T, effects string) (Catalog, error) {
	t.Helper()
	root := t.TempDir()
	for dir, body := range map[string]string{"cards": tinyCards, "effects": effects} {
		if err := os.MkdirAll(filepath.Join(root, dir), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, dir, "x.json"), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return Load(root)
}

func TestEffectsMergeAndMarkModeled(t *testing.T) {
	cat, err := loadWithEffects(t, `{
	  "grunt": [{"kind":"static","mods":[{"stat":"atk","to":{"rel":"self"},"amount":{"n":1,"per":{"of":"landscapes","type":"cornfield"}}}]}],
	  "zap":   [{"kind":"spell","do":[{"do":"draw","who":"you","amount":{"n":3}}]}]
	}`)
	if err != nil {
		t.Fatal(err)
	}
	g := cat["grunt"]
	if !g.Modeled || len(g.Effects) != 1 || g.Effects[0].Mods[0].Amount.Per.Type != Cornfield {
		t.Errorf("grunt = %+v", g)
	}
	if !cat["zap"].Implemented() {
		t.Error("modeled spell should be implemented")
	}
}

func TestUnmodeledTextIsNotImplemented(t *testing.T) {
	cat, err := loadWithEffects(t, `{"zap": [{"kind":"spell","do":[{"do":"draw","who":"you","amount":{"n":1}}]}]}`)
	if err != nil {
		t.Fatal(err)
	}
	if cat["grunt"].Implemented() {
		t.Error("a card with rules text and no effects entry must not count as implemented")
	}
}

func TestEmptyEffectsListModelsVanillaCard(t *testing.T) {
	cat, err := loadWithEffects(t, `{"grunt": []}`)
	if err != nil {
		t.Fatal(err)
	}
	if !cat["grunt"].Implemented() {
		t.Error("an explicit empty list means the text is flavor only")
	}
}

func TestBadEffectsAreRejected(t *testing.T) {
	bad := map[string]string{
		"unknown card":      `{"nobody": []}`,
		"unknown field":     `{"grunt": [{"kind":"static","restriction":"x"}]}`,
		"unknown kind":      `{"grunt": [{"kind":"explode"}]}`,
		"unknown op":        `{"zap": [{"kind":"spell","do":[{"do":"explode"}]}]}`,
		"slot out of range": `{"zap": [{"kind":"spell","do":[{"do":"bounce","slot":2}]}]}`,
		"missing slot":      `{"zap": [{"kind":"spell","do":[{"do":"bounce"}]}]}`,
		"bad stat":          `{"grunt": [{"kind":"static","mods":[{"stat":"hp","to":{"rel":"self"},"amount":{"n":1}}]}]}`,
		"bad selector":      `{"grunt": [{"kind":"static","mods":[{"stat":"atk","to":{"rel":"everyone"},"amount":{"n":1}}]}]}`,
		"bad count":         `{"grunt": [{"kind":"static","mods":[{"stat":"atk","to":{"rel":"self"},"amount":{"n":1,"per":{"of":"moons"}}}]}]}`,
		"spell on creature": `{"grunt": [{"kind":"spell"}]}`,
		"static on spell":   `{"zap": [{"kind":"static","restrict":"adjacent_allies_not_attackable"}]}`,
		"bad trigger":       `{"grunt": [{"kind":"trigger","on":"sneeze"}]}`,
		"free activate":     `{"grunt": [{"kind":"activate"}]}`,
		"bad target":        `{"zap": [{"kind":"spell","targets":[{"what":"moon"}]}]}`,
	}
	for name, body := range bad {
		if _, err := loadWithEffects(t, body); err == nil {
			t.Errorf("%s: expected an error", name)
		} else if strings.Contains(err.Error(), "no card files") {
			t.Errorf("%s: wrong error %v", name, err)
		}
	}
}

func TestModelledTwiceIsRejected(t *testing.T) {
	root := t.TempDir()
	os.MkdirAll(filepath.Join(root, "cards"), 0o755)
	os.MkdirAll(filepath.Join(root, "effects"), 0o755)
	os.WriteFile(filepath.Join(root, "cards", "c.json"), []byte(tinyCards), 0o644)
	os.WriteFile(filepath.Join(root, "effects", "a.json"), []byte(`{"grunt": []}`), 0o644)
	os.WriteFile(filepath.Join(root, "effects", "b.json"), []byte(`{"grunt": []}`), 0o644)
	if _, err := Load(root); err == nil {
		t.Error("two effect files for one card should be an error")
	}
}
