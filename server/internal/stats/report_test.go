package stats

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestReportGroupsTheCountersAndKeepsTheTotalsFixed(t *testing.T) {
	r := BuildReport(map[string]int64{
		"games_started": 5, "games_started.bot": 3, "games_started.person": 2,
		"games_completed": 4, "games_completed.bot": 3, "games_completed.person": 1,
		"games_abandoned": 1, "bot_wins": 2, "bot_wins.heuristic": 1, "bot_wins.random": 1, "player_wins": 1,
		"cards_played.creature": 10, "cards_played.spell": 4, "card_played.corn-dog": 3, "card_played.hot-shot": 1,
		"floops": 7, "fights": 8, "damage_dealt": 90, "creatures_destroyed": 12, "hero_abilities": 2,
		"deck_started.finn": 3, "deck_won.finn": 2, "deck_started.custom": 1,
	}, true, false)
	if r.V != Version || !r.Kept {
		t.Errorf("v %d kept %v", r.V, r.Kept)
	}
	if r.Games.Started != (Split{5, 3, 2}) || r.Games.Completed != (Split{4, 3, 1}) || r.Games.Abandoned != 1 {
		t.Errorf("games %+v", r.Games)
	}
	if r.Results.PlayerWins != 1 || r.Results.BotWins.Total != 2 || r.Results.BotWins.ByBot["random"] != 1 {
		t.Errorf("results %+v", r.Results)
	}
	if r.Plays.Cards.Total != 14 || r.Plays.Cards.ByType["building"] != 0 || len(r.Plays.Cards.ByType) != 3 || r.Plays.Floops != 7 || r.Plays.Fights != 8 || r.Plays.HeroAbilities != 2 {
		t.Errorf("plays %+v", r.Plays)
	}
	if r.Combat.DamageDealt != 90 || r.Combat.CreaturesDestroyed != 12 {
		t.Errorf("combat %+v", r.Combat)
	}
	if r.Decks["finn"] != (DeckResult{3, 2}) || r.Decks["custom"] != (DeckResult{1, 0}) {
		t.Errorf("decks %+v", r.Decks)
	}
	if r.Cards != nil {
		t.Error("the per-card counts came without being asked for")
	}
}

func TestAnEmptyReportHasItsWholeShape(t *testing.T) {
	b, _ := json.Marshal(BuildReport(nil, false, false))
	for _, want := range []string{`"v":2`, `"started":{"total":0,"bot":0,"person":0}`, `"by_bot":{}`, `"by_type":{"building":0,"creature":0,"spell":0}`, `"decks":{}`, `"damage_dealt":0`} {
		if !strings.Contains(string(b), want) {
			t.Errorf("%s is missing from %s", want, b)
		}
	}
	if strings.Contains(string(b), `"cards":{"played"`) {
		t.Errorf("per-card counts without being asked: %s", b)
	}
}

func TestPerCardCountsComeWithDetail(t *testing.T) {
	r := BuildReport(map[string]int64{"card_played.corn-dog": 3, "cards_played.creature": 3}, false, true)
	if r.Cards == nil || r.Cards.Played["corn-dog"] != 3 {
		t.Errorf("cards %+v", r.Cards)
	}
	if empty := BuildReport(nil, false, true); empty.Cards == nil || empty.Cards.Played == nil {
		t.Error("asked for the per-card counts, got none even empty")
	}
}
