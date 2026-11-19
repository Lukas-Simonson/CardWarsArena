package server

import (
	"encoding/json"
	"math/rand/v2"
	"net/http"
	"strings"
	"testing"
	"time"

	"cardwars/internal/stats"
)

// statsHarness is a server that counts: as a real one is, with no development switches.
func statsHarness(t *testing.T) (*harness, *stats.Counters) {
	t.Helper()
	c, err := stats.New(nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	return newHarness(t, func(cfg *Config) { cfg.AllowSeed, cfg.Stats = false, c }), c
}

func count(c *stats.Counters, name string) int64 { return c.Totals()[name] }

func TestAFinishedGameAgainstTheBotIsCounted(t *testing.T) {
	h, c := statsHarness(t)
	a := h.dial()
	h.createVsBot(a, "finn")
	final := a.play(rand.New(rand.NewPCG(1, 1)))
	if !final.View.Over {
		t.Fatal("not finished")
	}
	want := map[string]int64{"games_started": 1, "games_started.bot": 1, "games_completed": 1, "games_completed.bot": 1, "games_abandoned": 0}
	for k, v := range want {
		if got := count(c, k); got != v {
			t.Errorf("%s = %d, want %d", k, got, v)
		}
	}
	if count(c, "bot_wins")+count(c, "player_wins") != 1 {
		t.Errorf("exactly one side won: %v", c.Totals())
	}
	var won int64
	for k, v := range c.Totals() {
		if strings.HasPrefix(k, "deck_won.") {
			won += v
		}
	}
	if count(c, "deck_started.finn") != 1 || won != 1 {
		t.Errorf("decks: %v", c.Totals())
	}
	played := count(c, "cards_played.creature") + count(c, "cards_played.spell") + count(c, "cards_played.building")
	if played == 0 || count(c, "fights") == 0 || count(c, "damage_dealt") == 0 {
		t.Errorf("a whole game plays cards, fights and hurts: %v", c.Totals())
	}
	a.conn.CloseNow()
	time.Sleep(50 * time.Millisecond)
	if count(c, "games_abandoned") != 0 {
		t.Error("closing a finished game's room is not abandoning it")
	}
}

func TestLeavingAGameBeforeTheEndIsAbandonedAndAWaitingRoomIsNotAGame(t *testing.T) {
	h, c := statsHarness(t)
	a := h.dial()
	a.send(ClientMsg{Type: MsgCreate, Deck: "finn"}) // a room waiting for a person
	a.expect(MsgJoined)
	a.send(ClientMsg{Type: MsgLeave})
	deadline := time.Now().Add(2 * time.Second)
	for h.srv.RoomCount() != 0 && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if count(c, "games_started") != 0 || count(c, "games_abandoned") != 0 {
		t.Errorf("nobody played: %v", c.Totals())
	}

	b := h.dial()
	h.createVsBot(b, "finn")
	b.send(ClientMsg{Type: MsgLeave})
	deadline = time.Now().Add(2 * time.Second)
	for count(c, "games_abandoned") == 0 && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if count(c, "games_started") != 1 || count(c, "games_abandoned") != 1 || count(c, "games_completed") != 0 {
		t.Errorf("totals: %v", c.Totals())
	}
}

func TestAnIdleGameThatExpiresIsAbandoned(t *testing.T) {
	c, _ := stats.New(nil, nil)
	h := newHarness(t, func(cfg *Config) { cfg.AllowSeed, cfg.Stats, cfg.RoomTTL = false, c, time.Minute })
	a := h.dial()
	h.createVsBot(a, "finn")
	h.srv.sweep(time.Now().Add(2 * time.Minute))
	if count(c, "games_abandoned") != 1 {
		t.Errorf("totals: %v", c.Totals())
	}
}

func TestAReconnectIsNotANewGame(t *testing.T) {
	h, c := statsHarness(t)
	a, b := h.dial(), h.dial()
	a.send(ClientMsg{Type: MsgCreate, Name: "Ann", Deck: "finn"})
	a.expect(MsgJoined)
	b.send(ClientMsg{Type: MsgJoin, Room: a.room, Name: "Bob", Deck: "jake"})
	b.expect(MsgJoined)
	a.conn.CloseNow()
	a2 := h.dial()
	a2.send(ClientMsg{Type: MsgRejoin, Room: a.room, Token: a.token})
	a2.expect(MsgJoined)
	if count(c, "games_started") != 1 || count(c, "games_started.person") != 1 {
		t.Errorf("a person's game began once: %v", c.Totals())
	}
}

func TestADevelopmentServerCountsNothing(t *testing.T) {
	c, _ := stats.New(nil, nil)
	h := newHarness(t, func(cfg *Config) { cfg.Stats = c }) // the harness allows seeds, like -dev
	a := h.dial()
	h.createVsBot(a, "finn")
	a.play(rand.New(rand.NewPCG(1, 1)))
	if len(c.Totals()) != 0 {
		t.Errorf("sandbox games are not counted: %v", c.Totals())
	}
}

func TestStatsEndpointAndNothingPersonalInIt(t *testing.T) {
	h, c := statsHarness(t)
	a := h.dial()
	a.send(ClientMsg{Type: MsgCreate, Name: "AdaLovelace", Deck: "finn", VsBot: true})
	a.expect(MsgJoined)
	room := a.room
	a.play(rand.New(rand.NewPCG(2, 2)))

	res, err := http.Get(h.ts.URL + "/api/stats")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	var got statsAnswer
	if res.StatusCode != 200 || json.NewDecoder(res.Body).Decode(&got) != nil || got.Counts["games_completed"] != 1 || got.Kept {
		t.Fatalf("GET /api/stats: %d %+v", res.StatusCode, got)
	}
	// every name is a kind of thing counted, never a person, a room or a game
	for k := range c.Totals() {
		if strings.Contains(strings.ToLower(k), "ada") || strings.Contains(strings.ToLower(k), strings.ToLower(room)) {
			t.Errorf("counter %q mentions a player or room", k)
		}
	}
}

func TestStatsOffIs404(t *testing.T) {
	h := newHarness(t)
	res, err := http.Get(h.ts.URL + "/api/stats")
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != http.StatusNotFound {
		t.Errorf("status %d", res.StatusCode)
	}
}
