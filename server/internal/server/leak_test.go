package server

import (
	"encoding/json"
	"math/rand/v2"
	"sync"
	"testing"

	"cardwars/internal/engine"
)

// checkSink inspects every message at the moment the room sends it, against
// the live game state, and fails if it contains a card the seat must not see.
type checkSink struct {
	t    *testing.T
	room *Room
	seat engine.PlayerID

	mu   sync.Mutex
	msgs []ServerMsg
}

func (c *checkSink) Send(m ServerMsg) {
	c.mu.Lock()
	c.msgs = append(c.msgs, m)
	c.mu.Unlock()
	if m.Type != MsgUpdate || c.room.g == nil {
		return
	}
	// Send is called with the room lock held, so reading the game is safe.
	hidden := map[int]bool{}
	for _, in := range c.room.g.State.Players[c.seat.Opp()].Hand {
		hidden[in.UID] = true
	}
	for _, p := range []engine.PlayerID{0, 1} {
		for _, in := range c.room.g.State.Players[p].Deck {
			hidden[in.UID] = true
		}
	}
	b, err := json.Marshal(m)
	if err != nil {
		c.t.Error(err)
		return
	}
	var doc any
	json.Unmarshal(b, &doc)
	var walk func(x any)
	walk = func(x any) {
		switch x := x.(type) {
		case map[string]any:
			for k, v := range x {
				if k == "uid" && hidden[int(v.(float64))] {
					c.t.Errorf("seat %d was sent hidden card uid %d (seq %d)", c.seat, int(v.(float64)), m.Seq)
				}
				walk(v)
			}
		case []any:
			for _, e := range x {
				walk(e)
			}
		}
	}
	walk(doc)
	// Events must not name an opponent's drawn card either.
	for _, e := range m.Events {
		if e.Kind == engine.EvDraw && e.Player != c.seat && e.Card != 0 {
			c.t.Errorf("seat %d was told which card the opponent drew", c.seat)
		}
	}
}

func TestNothingHiddenIsEverSentToEitherSeat(t *testing.T) {
	for seed := uint64(1); seed <= 12; seed++ {
		srv, err := New(Config{Catalog: catalog(t), AllowSeed: true})
		if err != nil {
			t.Fatal(err)
		}
		a := &checkSink{t: t, seat: 0}
		b := &checkSink{t: t, seat: 1}
		room, tokenA, err := srv.create(a, ClientMsg{Deck: "cp1 finn", Seed: &seed})
		if err != nil {
			t.Fatal(err)
		}
		a.room, b.room = room, room
		deck, _ := srv.deckFor("cp1 jake")
		tokenB, err := room.join(b, "B", deck)
		if err != nil {
			t.Fatal(err)
		}
		tokens := [2]string{tokenA, tokenB}
		rng := rand.New(rand.NewPCG(seed, 5))
		for steps := 0; steps < 5000; steps++ {
			room.mu.Lock()
			over := room.g.State.Phase == engine.PhaseOver
			awaiting := room.g.Awaiting()
			var seat engine.PlayerID
			var act engine.Action
			if !over && len(awaiting) > 0 {
				seat = awaiting[rng.IntN(len(awaiting))]
				legal := room.g.LegalActions(seat)
				act = legal[rng.IntN(len(legal))]
			}
			room.mu.Unlock()
			if over {
				break
			}
			if err := room.act(tokens[seat], act, nil); err != nil {
				t.Fatalf("seed %d: %v", seed, err)
			}
		}
		if len(a.msgs) < 10 || len(b.msgs) < 10 {
			t.Fatalf("seed %d: too few messages (%d, %d)", seed, len(a.msgs), len(b.msgs))
		}
	}
}
