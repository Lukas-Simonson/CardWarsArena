package view

import (
	"encoding/json"
	"strings"
	"testing"

	"cardwars/internal/card"
	"cardwars/internal/engine"
)

func newGame(t *testing.T, seed uint64) *engine.Game {
	t.Helper()
	cat, err := card.Load("../../data")
	if err != nil {
		t.Fatal(err)
	}
	b, c := card.BluePlains, card.Cornfield
	g, err := engine.NewGame(cat, engine.Config{Seed: seed, First: 0}, [2]engine.DeckSpec{
		{Landscapes: [4]card.Landscape{b, b, b, b}, Cards: cat.Deck("Finn Vs Jake", "Finn's Deck")},
		{Landscapes: [4]card.Landscape{c, c, c, c}, Cards: cat.Deck("Finn Vs Jake", "Jake's Deck")},
	})
	if err != nil {
		t.Fatal(err)
	}
	return g
}

// uids collects every "uid" value in a JSON document.
func uids(t *testing.T, v any) map[int]bool {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	var doc any
	if err := json.Unmarshal(b, &doc); err != nil {
		t.Fatal(err)
	}
	out := map[int]bool{}
	var walk func(x any)
	walk = func(x any) {
		switch x := x.(type) {
		case map[string]any:
			for k, val := range x {
				if k == "uid" {
					out[int(val.(float64))] = true
				}
				walk(val)
			}
		case []any:
			for _, e := range x {
				walk(e)
			}
		}
	}
	walk(doc)
	return out
}

func TestViewNeverContainsHiddenCards(t *testing.T) {
	for seed := uint64(1); seed <= 20; seed++ {
		g := newGame(t, seed)
		for _, p := range []engine.Action{{Kind: engine.ActKeepHand}} {
			g.Apply(0, p)
			g.Apply(1, p)
		}
		for viewer := engine.PlayerID(0); viewer < 2; viewer++ {
			seen := uids(t, Build(g, viewer))
			opp := &g.State.Players[viewer.Opp()]
			me := &g.State.Players[viewer]
			for _, in := range opp.Hand {
				if seen[in.UID] {
					t.Fatalf("seed %d: opponent's hand card %d leaked to viewer %d", seed, in.UID, viewer)
				}
			}
			for _, in := range append(append([]engine.Instance{}, opp.Deck...), me.Deck...) {
				if seen[in.UID] {
					t.Fatalf("seed %d: deck card %d leaked to viewer %d", seed, in.UID, viewer)
				}
			}
			for _, in := range me.Hand {
				if !seen[in.UID] {
					t.Fatalf("seed %d: viewer %d cannot see own hand card %d", seed, viewer, in.UID)
				}
			}
		}
	}
}

func TestViewShowsCountsButNotContents(t *testing.T) {
	g := newGame(t, 1)
	v := Build(g, 0)
	if v.Players[1].HandCount != 5 || v.Players[1].Hand != nil || v.Players[1].DeckCount != 35 {
		t.Errorf("opponent: %+v", v.Players[1])
	}
	if len(v.Players[0].Hand) != 5 || v.You != 0 {
		t.Errorf("own hand: %+v", v.Players[0])
	}
}

func TestLegalActionsOnlyForTheViewerWhoMustAct(t *testing.T) {
	g := newGame(t, 1)
	g.Apply(0, engine.Action{Kind: engine.ActKeepHand})
	g.Apply(1, engine.Action{Kind: engine.ActKeepHand})
	v0, v1 := Build(g, 0), Build(g, 1)
	if len(v0.Legal) == 0 || len(v1.Legal) != 0 {
		t.Errorf("legal: p0 %d, p1 %d (p0 is active)", len(v0.Legal), len(v1.Legal))
	}
	if len(v0.Awaiting) != 1 || v0.Awaiting[0] != 0 || v0.Phase != engine.PhaseMain {
		t.Errorf("awaiting %v phase %s", v0.Awaiting, v0.Phase)
	}
}

func TestMulliganViewShowsBothPlayersWaiting(t *testing.T) {
	g := newGame(t, 1)
	v := Build(g, 1)
	if len(v.Awaiting) != 2 || len(v.Legal) != 2 {
		t.Errorf("awaiting %v legal %d", v.Awaiting, len(v.Legal))
	}
	g.Apply(0, engine.Action{Kind: engine.ActKeepHand})
	v = Build(g, 1)
	if !v.Players[0].Kept || v.Players[1].Kept {
		t.Errorf("kept flags: %+v %+v", v.Players[0].Kept, v.Players[1].Kept)
	}
}

func TestEventsHideOpponentDrawsAndNameEverythingElse(t *testing.T) {
	g := newGame(t, 1)
	hand0, hand1 := g.State.Players[0].Hand[0], g.State.Players[1].Hand[0]
	evs := []engine.Event{
		{Kind: engine.EvDraw, Player: 1, Card: hand1.UID},
		{Kind: engine.EvDraw, Player: 0, Card: hand0.UID},
		{Kind: engine.EvPlay, Player: 1, Card: hand1.UID},
		{Kind: engine.EvHPLoss, Player: 0, Amount: 3},
	}
	got := FilterEvents(g, 0, evs)
	if got[0].Card != 0 || got[0].CardID != "" {
		t.Errorf("an opponent's draw must not name the card: %+v", got[0])
	}
	if got[1].Card != hand0.UID || got[1].CardID != hand0.CardID {
		t.Errorf("own draw: %+v", got[1])
	}
	if got[2].CardID != hand1.CardID {
		t.Errorf("a card the opponent played is public, even if it later left play: %+v", got[2])
	}
	if got[3].CardID != "" || got[3].Amount != 3 {
		t.Errorf("events without a card: %+v", got[3])
	}
	if evs[0].Card != hand1.UID {
		t.Error("the original events must not be modified")
	}
	b, _ := json.Marshal(got[2])
	if !strings.Contains(string(b), `"card_id":"`+hand1.CardID+`"`) || !strings.Contains(string(b), `"kind":"play"`) {
		t.Errorf("JSON should flatten the engine event and add card_id: %s", b)
	}
}

func TestLegalActionsNeverContainNullSlots(t *testing.T) {
	// Psionic Architect and Legion of Earlings have optional targets, which
	// the engine represents with nil slices.
	g := newGame(t, 1)
	g.Apply(0, engine.Action{Kind: engine.ActKeepHand})
	g.Apply(1, engine.Action{Kind: engine.ActKeepHand})
	who := g.State.Active
	if _, err := g.Give(who, "psionic-architect"); err != nil {
		t.Fatal(err)
	}
	if _, err := g.Give(who, "legion-of-earlings"); err != nil {
		t.Fatal(err)
	}
	b, err := json.Marshal(Build(g, who).Legal)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(b), "null") {
		t.Errorf("legal actions contain null: %s", b)
	}
	if !strings.Contains(string(b), `"targets":[[]]`) {
		t.Errorf("an empty optional slot should be [[]]: %.300s", b)
	}
}

func TestCreatureViewHasComputedStats(t *testing.T) {
	g := newGame(t, 1)
	if err := g.Put(0, 0, "husker-knight"); err != nil {
		t.Fatal(err)
	}
	v := Build(g, 1) // the opponent's view
	c := v.Players[0].Lanes[0].Creature
	if c == nil || c.Attack != 0 || c.Defense != 0 {
		// Finn's side has no Cornfields: the Knight is 0/0 and has already died.
		if c != nil {
			t.Errorf("creature = %+v", c)
		}
	}
	g.Put(1, 0, "husker-knight")
	c = Build(g, 0).Players[1].Lanes[0].Creature
	if c == nil || c.Attack != 4 || c.Defense != 8 {
		t.Errorf("Husker Knight with 4 Cornfields should show 4/8: %+v", c)
	}
}

func TestPendingCardIsVisible(t *testing.T) {
	g := newGame(t, 1)
	g.State.Pending = &engine.Pending{Card: engine.Instance{UID: 99, CardID: "cool-dog", Owner: 1}}
	for viewer := engine.PlayerID(0); viewer < 2; viewer++ {
		if v := Build(g, viewer); v.Pending == nil || v.Pending.CardID != "cool-dog" {
			t.Errorf("viewer %d: pending = %+v", viewer, v.Pending)
		}
	}
}

func TestViewJSONIsStable(t *testing.T) {
	g := newGame(t, 1)
	b, err := json.Marshal(Build(g, 0))
	if err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{`"you":0`, `"phase":"mulligan"`, `"awaiting":[0,1]`, `"players":[`, `"legal":[`, `"deck_count":35`} {
		if !strings.Contains(string(b), key) {
			t.Errorf("JSON lacks %s", key)
		}
	}
}

// A reveal Choice shows the revealed cards to both seats, but only the player
// who must decide is offered actions.
func TestChoiceIsVisibleToBothButOnlyOfferedToTheDecider(t *testing.T) {
	cat, err := card.Load("../../data")
	if err != nil {
		t.Fatal(err)
	}
	s, u := card.UselessSwamp, card.SandyLands
	g, err := engine.NewGame(cat, engine.Config{Seed: 3, First: 0}, [2]engine.DeckSpec{
		{Landscapes: [4]card.Landscape{s, s, s, s}, Cards: cat.Deck("BMO Vs Lady Rainicorn", "BMO's Deck")},
		{Landscapes: [4]card.Landscape{u, u, u, u}, Cards: cat.Deck("BMO Vs Lady Rainicorn", "Lady's Deck")},
	})
	if err != nil {
		t.Fatal(err)
	}
	for p := engine.PlayerID(0); p < 2; p++ {
		if _, err := g.Apply(p, engine.Action{Kind: engine.ActKeepHand}); err != nil {
			t.Fatal(err)
		}
	}
	gas, _ := g.Give(0, "ogre-gas")
	if _, err := g.Apply(0, engine.Action{Kind: engine.ActPlayCard, Card: gas.UID}); err != nil {
		t.Fatal(err)
	}
	mine, theirs := Build(g, 0), Build(g, 1)
	for _, v := range []View{mine, theirs} {
		if v.Choice == nil || v.Choice.Kind != "reveal" || v.Choice.Player != 0 || len(v.Choice.Revealed) != 3 {
			t.Fatalf("choice = %+v", v.Choice)
		}
		if len(v.Awaiting) != 1 || v.Awaiting[0] != 0 {
			t.Errorf("awaiting %v", v.Awaiting)
		}
	}
	if len(mine.Legal) != 3 || len(theirs.Legal) != 0 {
		t.Errorf("legal: %d for the decider, %d for the other", len(mine.Legal), len(theirs.Legal))
	}
	for _, in := range mine.Choice.Revealed {
		if got := FilterEvents(g, 1, []engine.Event{{Kind: engine.EvDiscard, Card: in.UID}})[0].CardID; got != in.CardID {
			t.Errorf("revealed cards are named in events: %q", got)
		}
	}
}
