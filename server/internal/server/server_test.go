package server

import (
	"context"
	"encoding/json"
	"math/rand/v2"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/coder/websocket"

	"cardwars/internal/card"
	"cardwars/internal/engine"
)

var (
	catOnce sync.Once
	testCat card.Catalog
)

func catalog(t testing.TB) card.Catalog {
	t.Helper()
	catOnce.Do(func() {
		var err error
		if testCat, err = card.Load("../../data"); err != nil {
			t.Fatal(err)
		}
	})
	return testCat
}

type harness struct {
	t   *testing.T
	srv *Server
	ts  *httptest.Server
}

func newHarness(t *testing.T, mutate ...func(*Config)) *harness {
	t.Helper()
	// Scripted clients answer instantly, far faster than a person: relax the limiter.
	cfg := Config{Catalog: catalog(t), AllowSeed: true, MessagesPerSecond: 5000, Burst: 5000}
	for _, m := range mutate {
		m(&cfg)
	}
	srv, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	ts := httptest.NewServer(srv.Handler())
	t.Cleanup(ts.Close)
	return &harness{t, srv, ts}
}

// tc is a scripted client.
type tc struct {
	t    *testing.T
	conn *websocket.Conn
	in   chan ServerMsg
	raw  chan []byte
	done chan struct{}

	room, token string
	seat        int
	last        *ServerMsg // most recent update
}

func (h *harness) dial() *tc {
	h.t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	conn, _, err := websocket.Dial(ctx, "ws"+strings.TrimPrefix(h.ts.URL, "http")+"/ws", nil)
	if err != nil {
		h.t.Fatal(err)
	}
	c := &tc{t: h.t, conn: conn, in: make(chan ServerMsg, 4096), raw: make(chan []byte, 4096), done: make(chan struct{})}
	go func() {
		defer close(c.done)
		for {
			_, data, err := conn.Read(context.Background())
			if err != nil {
				return
			}
			var m ServerMsg
			if json.Unmarshal(data, &m) == nil {
				c.raw <- data
				c.in <- m
			}
		}
	}()
	h.t.Cleanup(func() { conn.CloseNow() })
	c.expect(MsgHello)
	return c
}

func (c *tc) send(m ClientMsg) {
	c.t.Helper()
	b, _ := json.Marshal(m)
	c.sendRaw(b)
}

func (c *tc) sendRaw(b []byte) {
	c.t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := c.conn.Write(ctx, websocket.MessageText, b); err != nil {
		c.t.Fatalf("write: %v", err)
	}
}

// next returns the next message, or fails after a timeout.
func (c *tc) next() ServerMsg {
	c.t.Helper()
	select {
	case m := <-c.in:
		<-c.raw
		if m.Type == MsgUpdate {
			mm := m
			c.last = &mm
		}
		return m
	case <-time.After(5 * time.Second):
		if c.last != nil && c.last.View != nil {
			v := c.last.View
			c.t.Fatalf("timed out waiting for a message; last update seq=%d phase=%s turn=%d active=%d awaiting=%v legal=%d over=%v you=%d",
				c.last.Seq, v.Phase, v.Turn, v.Active, v.Awaiting, len(v.Legal), v.Over, v.You)
		}
		c.t.Fatal("timed out waiting for a message (no update seen)")
	}
	return ServerMsg{}
}

// expect reads until it finds a message of the given type, skipping others.
func (c *tc) expect(typ string) ServerMsg {
	c.t.Helper()
	for {
		m := c.next()
		if m.Type == typ {
			if typ == MsgJoined {
				c.room, c.token, c.seat = m.Room, m.Token, *m.Seat
			}
			return m
		}
		if m.Type == MsgError && typ != MsgError {
			c.t.Fatalf("unexpected error while waiting for %s: %s: %s", typ, m.Code, m.Message)
		}
	}
}

func (c *tc) expectError(code string) {
	c.t.Helper()
	if m := c.expect(MsgError); m.Code != code {
		c.t.Fatalf("error code = %s (%s), want %s", m.Code, m.Message, code)
	}
}

// play drives a client: whenever it is awaited it takes a random legal action,
// until the game is over. It returns the final update.
func (c *tc) play(rng *rand.Rand) ServerMsg {
	c.t.Helper()
	for {
		m := c.next()
		if m.Type == MsgError {
			c.t.Logf("seat %d got error %s: %s", c.seat, m.Code, m.Message)
		}
		if m.Type != MsgUpdate || m.View == nil {
			continue
		}
		v := m.View
		if v.Over {
			return m
		}
		awaited := false
		for _, p := range v.Awaiting {
			awaited = awaited || p == c.seat
		}
		if awaited && len(v.Legal) > 0 {
			a := v.Legal[rng.IntN(len(v.Legal))]
			seq := m.Seq
			c.send(ClientMsg{Type: MsgAction, Action: &a, Seq: &seq})
		}
	}
}

func (h *harness) createVsBot(c *tc, deck string, extra ...func(*ClientMsg)) {
	h.t.Helper()
	m := ClientMsg{Type: MsgCreate, Name: "Tester", Deck: deck, VsBot: true}
	for _, e := range extra {
		e(&m)
	}
	c.send(m)
	c.expect(MsgJoined)
}

func seedOf(n uint64) func(*ClientMsg) { return func(m *ClientMsg) { m.Seed = &n } }

func TestHelloAndHTTPEndpoints(t *testing.T) {
	h := newHarness(t)
	c := h.dial()
	_ = c
	for path, want := range map[string]string{
		"/healthz":   "ok",
		"/api/decks": "Finn's Deck",
		"/api/cards": "Cool Dog",
		"/":          "<title>",
	} {
		resp, err := http.Get(h.ts.URL + path)
		if err != nil {
			t.Fatal(err)
		}
		var b strings.Builder
		buf := make([]byte, 4096)
		for {
			n, err := resp.Body.Read(buf)
			b.Write(buf[:n])
			if err != nil {
				break
			}
		}
		resp.Body.Close()
		if resp.StatusCode != 200 || !strings.Contains(b.String(), want) {
			t.Errorf("%s: status %d, missing %q", path, resp.StatusCode, want)
		}
	}
}

func TestDecksEndpointMarksPlayability(t *testing.T) {
	h := newHarness(t)
	resp, err := http.Get(h.ts.URL + "/api/decks")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var list []struct {
		Set, Name string
		Playable  bool
	}
	if err := json.NewDecoder(resp.Body).Decode(&list); err != nil {
		t.Fatal(err)
	}
	ready := map[string]bool{}
	for _, d := range list {
		ready[d.Name] = d.Playable
	}
	if !ready["Finn's Deck"] || !ready["Jake's Deck"] || ready["BMO's Deck"] || ready["Lady's Deck"] {
		t.Errorf("playable = %v", ready)
	}
}

func TestVersusBotRunsToTheEnd(t *testing.T) {
	h := newHarness(t)
	for seed := uint64(1); seed <= 5; seed++ {
		c := h.dial()
		h.createVsBot(c, "finn", seedOf(seed))
		final := c.play(rand.New(rand.NewPCG(seed, 1)))
		if !final.View.Over || final.View.Winner == nil {
			t.Fatalf("seed %d: not finished: %+v", seed, final.View)
		}
		if c.seat != 0 {
			t.Errorf("the host sits in seat 0")
		}
	}
}

func TestUpdatesAreSequentialAndPersonal(t *testing.T) {
	h := newHarness(t)
	c := h.dial()
	h.createVsBot(c, "finn", seedOf(4))
	prev, seen := 0, 0
	for seen < 15 {
		m := c.next()
		if m.Type != MsgUpdate {
			continue // a stale_state error for an action that lost a race is fine
		}
		seen++
		if prev != 0 && m.Seq != prev+1 {
			t.Fatalf("seq jumped %d -> %d", prev, m.Seq)
		}
		prev = m.Seq
		if m.View.You != 0 || len(m.View.Players[0].Hand) == 0 || m.View.Players[1].Hand != nil {
			t.Fatalf("view is not this seat's: you=%d", m.View.You)
		}
		if len(m.Names) != 2 || m.Names[1] != "Bot" {
			t.Fatalf("names = %v", m.Names)
		}
		if m.View.Over {
			break
		}
		if len(m.View.Legal) > 0 {
			act, seq := m.View.Legal[0], m.Seq
			c.send(ClientMsg{Type: MsgAction, Action: &act, Seq: &seq})
		}
	}
}

func TestTwoPlayersPlayAFullGame(t *testing.T) {
	h := newHarness(t)
	a, b := h.dial(), h.dial()
	a.send(ClientMsg{Type: MsgCreate, Name: "Ann", Deck: "finn"})
	a.expect(MsgJoined)
	a.expect(MsgWaiting)
	b.send(ClientMsg{Type: MsgJoin, Room: strings.ToLower(a.room), Name: "Bob", Deck: "jake"})
	b.expect(MsgJoined)
	if b.seat != 1 || a.seat != 0 {
		t.Fatalf("seats %d %d", a.seat, b.seat)
	}
	if m := a.expect(MsgOpponent); m.Connected == nil || !*m.Connected || m.Name != "Bob" {
		t.Errorf("opponent msg = %+v", m)
	}

	var wg sync.WaitGroup
	results := make([]ServerMsg, 2)
	for i, c := range []*tc{a, b} {
		wg.Add(1)
		go func() {
			defer wg.Done()
			results[i] = c.play(rand.New(rand.NewPCG(uint64(i), 9)))
		}()
	}
	wg.Wait()
	if results[0].View.Winner == nil || *results[0].View.Winner != *results[1].View.Winner {
		t.Errorf("players disagree about the winner: %v %v", results[0].View.Winner, results[1].View.Winner)
	}
	if results[0].Seq != results[1].Seq {
		t.Errorf("final seq differs: %d vs %d", results[0].Seq, results[1].Seq)
	}
}

func ptr[T any](v T) *T { return &v }

func stateJSON(r *Room) string {
	r.mu.Lock()
	defer r.mu.Unlock()
	b, _ := json.Marshal(r.g.State)
	return string(b)
}

func TestIllegalActionIsRejectedAndChangesNothing(t *testing.T) {
	h := newHarness(t)
	a, b := h.dial(), h.dial()
	a.send(ClientMsg{Type: MsgCreate, Deck: "finn", Seed: ptr(uint64(2))})
	a.expect(MsgJoined)
	b.send(ClientMsg{Type: MsgJoin, Room: a.room, Deck: "jake"})
	b.expect(MsgJoined)
	a.expect(MsgUpdate)
	b.expect(MsgUpdate)

	before := stateJSON(h.srv.room(a.room))
	// Not in the mulligan phase's vocabulary and not this seat's turn.
	b.send(ClientMsg{Type: MsgAction, Action: &engine.Action{Kind: engine.ActEndMain}})
	b.expectError(ErrIllegal)
	b.send(ClientMsg{Type: MsgAction, Action: &engine.Action{Kind: "teleport_me_to_victory"}})
	b.expectError(ErrIllegal)
	if after := stateJSON(h.srv.room(a.room)); after != before {
		t.Error("a rejected action changed the game")
	}
	// The seat is still usable afterwards.
	b.send(ClientMsg{Type: MsgAction, Action: &engine.Action{Kind: engine.ActKeepHand}})
	if m := b.expect(MsgUpdate); !m.View.Players[1].Kept {
		t.Error("keep should have registered")
	}
}

func TestRequestErrors(t *testing.T) {
	h := newHarness(t)
	c := h.dial()
	c.send(ClientMsg{Type: MsgAction, Action: &engine.Action{Kind: engine.ActDraw}})
	c.expectError(ErrNotInRoom)
	c.send(ClientMsg{Type: MsgLeave})
	c.expectError(ErrNotInRoom)
	c.send(ClientMsg{Type: MsgCreate, Deck: "nonsense"})
	c.expectError(ErrUnknownDeck)
	c.send(ClientMsg{Type: MsgCreate, Deck: "bmo"})
	c.expectError(ErrDeckNotReady)
	c.send(ClientMsg{Type: MsgJoin, Room: "ZZZZ", Deck: "finn"})
	c.expectError(ErrRoomNotFound)
	c.send(ClientMsg{Type: MsgRejoin, Room: "ZZZZ", Token: "x"})
	c.expectError(ErrRoomNotFound)
	c.send(ClientMsg{Type: "fly"})
	c.expectError(ErrBadRequest)
	c.sendRaw([]byte("{not json"))
	c.expectError(ErrBadRequest)
	c.send(ClientMsg{Type: MsgCreate, Deck: "finn"})
	c.expect(MsgJoined)
	c.send(ClientMsg{Type: MsgAction})
	c.expectError(ErrBadRequest)
	c.send(ClientMsg{Type: MsgAction, Action: &engine.Action{Kind: engine.ActDraw}})
	c.expectError(ErrNotStarted)
	c.send(ClientMsg{Type: MsgCreate, Deck: "finn"})
	c.expectError(ErrAlreadyInRoom)
	c.send(ClientMsg{Type: MsgJoin, Room: c.room, Deck: "jake"})
	c.expectError(ErrAlreadyInRoom)
}

func TestRoomFullAndOwnRoom(t *testing.T) {
	h := newHarness(t)
	a, b, c := h.dial(), h.dial(), h.dial()
	a.send(ClientMsg{Type: MsgCreate, Deck: "finn"})
	a.expect(MsgJoined)
	b.send(ClientMsg{Type: MsgJoin, Room: a.room, Deck: "jake"})
	b.expect(MsgJoined)
	c.send(ClientMsg{Type: MsgJoin, Room: a.room, Deck: "jake"})
	c.expectError(ErrRoomFull)
}

func TestServerFull(t *testing.T) {
	h := newHarness(t, func(c *Config) { c.MaxRooms = 1 })
	a, b := h.dial(), h.dial()
	a.send(ClientMsg{Type: MsgCreate, Deck: "finn"})
	a.expect(MsgJoined)
	b.send(ClientMsg{Type: MsgCreate, Deck: "finn"})
	b.expectError(ErrServerFull)
}

func TestRejoinAfterDisconnect(t *testing.T) {
	h := newHarness(t)
	a, b := h.dial(), h.dial()
	a.send(ClientMsg{Type: MsgCreate, Name: "Ann", Deck: "finn", Seed: ptr(uint64(3))})
	a.expect(MsgJoined)
	b.send(ClientMsg{Type: MsgJoin, Room: a.room, Name: "Bob", Deck: "jake"})
	b.expect(MsgJoined)
	a.expect(MsgUpdate)
	b.expect(MsgUpdate)
	keep := engine.Action{Kind: engine.ActKeepHand}
	a.send(ClientMsg{Type: MsgAction, Action: &keep})
	last := a.expect(MsgUpdate)

	a.conn.CloseNow()
	if m := drainFor(b, MsgOpponent, func(m ServerMsg) bool { return m.Connected != nil && !*m.Connected }); m == nil {
		t.Fatal("Bob should be told Ann disconnected")
	}

	a2 := h.dial()
	a2.send(ClientMsg{Type: MsgRejoin, Room: a.room, Token: "wrong"})
	a2.expectError(ErrBadToken)
	a2.send(ClientMsg{Type: MsgRejoin, Room: a.room, Token: a.token})
	a2.expect(MsgJoined)
	up := a2.expect(MsgUpdate)
	if up.Seq != last.Seq || !up.View.Players[0].Kept || up.View.You != 0 {
		t.Errorf("rejoin update: seq %d (want %d), view %+v", up.Seq, last.Seq, up.View.Players[0])
	}
	if drainFor(b, MsgOpponent, func(m ServerMsg) bool { return m.Connected != nil && *m.Connected }) == nil {
		t.Error("Bob should be told Ann is back")
	}
	// The seat works again; Bob can finish the mulligan and the game starts.
	b.send(ClientMsg{Type: MsgAction, Action: &keep})
	if m := a2.expect(MsgUpdate); m.View.Phase != engine.PhaseMain {
		t.Errorf("phase = %s", m.View.Phase)
	}
}

// drainFor reads messages until one of the type satisfies ok (or 2s pass).
func drainFor(c *tc, typ string, ok func(ServerMsg) bool) *ServerMsg {
	deadline := time.After(2 * time.Second)
	for {
		select {
		case m := <-c.in:
			<-c.raw
			if m.Type == typ && ok(m) {
				return &m
			}
		case <-deadline:
			return nil
		}
	}
}

func TestLeaveClosesTheRoom(t *testing.T) {
	h := newHarness(t)
	a, b := h.dial(), h.dial()
	a.send(ClientMsg{Type: MsgCreate, Deck: "finn"})
	a.expect(MsgJoined)
	b.send(ClientMsg{Type: MsgJoin, Room: a.room, Deck: "jake"})
	b.expect(MsgJoined)
	a.send(ClientMsg{Type: MsgLeave})
	if m := b.expect(MsgRoomClosed); m.Message != "opponent left" {
		t.Errorf("reason = %q", m.Message)
	}
	if n := h.srv.RoomCount(); n != 0 {
		t.Errorf("rooms = %d", n)
	}
	// Both clients are free to start over.
	b.send(ClientMsg{Type: MsgCreate, Deck: "jake"})
	b.expect(MsgJoined)
}

func TestHostLeavingAWaitingRoom(t *testing.T) {
	h := newHarness(t)
	a, b := h.dial(), h.dial()
	a.send(ClientMsg{Type: MsgCreate, Deck: "finn"})
	a.expect(MsgJoined)
	a.send(ClientMsg{Type: MsgLeave})
	deadline := time.Now().Add(2 * time.Second)
	for h.srv.RoomCount() != 0 && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	b.send(ClientMsg{Type: MsgJoin, Room: a.room, Deck: "jake"})
	b.expectError(ErrRoomNotFound)
}

func TestDroppedConnectionKeepsTheRoomForRejoin(t *testing.T) {
	h := newHarness(t)
	a := h.dial()
	h.createVsBot(a, "finn", seedOf(6))
	room, token := a.room, a.token
	a.expect(MsgUpdate)
	a.conn.CloseNow()
	time.Sleep(50 * time.Millisecond)
	if h.srv.RoomCount() != 1 {
		t.Fatal("room should survive a dropped connection")
	}
	a2 := h.dial()
	a2.send(ClientMsg{Type: MsgRejoin, Room: room, Token: token})
	a2.expect(MsgJoined)
	if m := a2.expect(MsgUpdate); m.View == nil || m.View.You != 0 {
		t.Error("rejoin should deliver the current view")
	}
}

func TestIdleRoomsExpire(t *testing.T) {
	h := newHarness(t, func(c *Config) { c.RoomTTL = time.Minute })
	a := h.dial()
	h.createVsBot(a, "finn")
	h.srv.sweep(time.Now().Add(30 * time.Second))
	if h.srv.RoomCount() != 1 {
		t.Fatal("a fresh room must not expire")
	}
	h.srv.sweep(time.Now().Add(2 * time.Minute))
	a.expect(MsgRoomClosed)
	if h.srv.RoomCount() != 0 {
		t.Error("the idle room should be gone")
	}
}

func TestSeedControl(t *testing.T) {
	handOf := func(h *harness, seed uint64) string {
		c := h.dial()
		h.createVsBot(c, "finn", seedOf(seed))
		m := c.expect(MsgUpdate)
		var cards []string
		for _, in := range m.View.Players[0].Hand {
			cards = append(cards, in.CardID)
		}
		return strings.Join(cards, ",")
	}
	h := newHarness(t)
	if a, b := handOf(h, 5), handOf(h, 5); a != b || a == "" {
		t.Errorf("same seed, different hands: %q %q", a, b)
	}
	// Without AllowSeed the request is ignored, so identical "seeds" give
	// different games (compare a few: a collision is possible but not likely).
	locked := newHarness(t, func(c *Config) { c.AllowSeed = false })
	distinct := map[string]bool{}
	for i := 0; i < 6; i++ {
		distinct[handOf(locked, 5)] = true
	}
	if len(distinct) < 2 {
		t.Errorf("the client's seed must be ignored when AllowSeed is off, but all hands were identical")
	}
}

func TestRateLimit(t *testing.T) {
	h := newHarness(t, func(c *Config) { c.MessagesPerSecond, c.Burst = 20, 40 })
	c := h.dial()
	// A modest overshoot: the first 40 messages pass, a few more are refused.
	for i := 0; i < 55; i++ {
		c.send(ClientMsg{Type: MsgPing})
	}
	pongs, limited := 0, 0
	for pongs+limited < 55 {
		switch m := c.next(); {
		case m.Type == MsgPong:
			pongs++
		case m.Type == MsgError && m.Code == ErrRateLimited:
			limited++
		}
	}
	if limited == 0 || pongs < 40 {
		t.Errorf("pongs %d, rate-limited %d: the burst of 40 should pass and the rest be refused", pongs, limited)
	}
	// The connection survives, and recovers once the bucket refills.
	time.Sleep(300 * time.Millisecond)
	c.send(ClientMsg{Type: MsgPing})
	c.expect(MsgPong)
}

func TestSustainedFloodingIsDisconnected(t *testing.T) {
	h := newHarness(t, func(c *Config) { c.MessagesPerSecond, c.Burst = 20, 40 })
	c := h.dial()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	for i := 0; i < 400; i++ {
		if err := c.conn.Write(ctx, websocket.MessageText, []byte(`{"type":"ping"}`)); err != nil {
			break // the server hung up part way through: expected
		}
	}
	select {
	case <-c.done:
	case <-time.After(3 * time.Second):
		t.Error("a client that keeps flooding should be disconnected")
	}
}

func TestOversizedMessageClosesTheConnection(t *testing.T) {
	h := newHarness(t)
	c := h.dial()
	// The server may hang up mid-write, so a write error is fine here.
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	_ = c.conn.Write(ctx, websocket.MessageText, []byte(`{"type":"ping","name":"`+strings.Repeat("x", 64<<10)+`"}`))
	select {
	case <-c.done:
	case <-time.After(3 * time.Second):
		t.Error("an oversized message should close the connection")
	}
}

func TestNamesAreSanitised(t *testing.T) {
	if got := cleanName("  Lukas\x00\x07  ", "x"); got != "Lukas" {
		t.Errorf("%q", got)
	}
	if got := cleanName("", "Player"); got != "Player" {
		t.Errorf("%q", got)
	}
	if got := cleanName(strings.Repeat("a", 100), "x"); len(got) != 24 {
		t.Errorf("len %d", len(got))
	}
}

func TestBotDeckDefaultsToTheOtherStarter(t *testing.T) {
	h := newHarness(t)
	c := h.dial()
	h.createVsBot(c, "finn")
	m := c.expect(MsgUpdate)
	if m.View.Players[1].Lanes[0].Landscape != card.Cornfield {
		t.Errorf("the bot should play Jake's Cornfield deck, got %s", m.View.Players[1].Lanes[0].Landscape)
	}
}

func TestCORSForSeparateWebClients(t *testing.T) {
	get := func(h *harness, origin string) http.Header {
		req, _ := http.NewRequest("GET", h.ts.URL+"/api/decks", nil)
		if origin != "" {
			req.Header.Set("Origin", origin)
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		return resp.Header
	}
	open := newHarness(t, func(c *Config) { c.OriginPatterns = []string{"localhost:*", "game.example.com"} })
	if got := get(open, "http://localhost:5173").Get("Access-Control-Allow-Origin"); got != "http://localhost:5173" {
		t.Errorf("allowed origin: %q", got)
	}
	if got := get(open, "https://game.example.com").Get("Access-Control-Allow-Origin"); got != "https://game.example.com" {
		t.Errorf("exact host pattern: %q", got)
	}
	if got := get(open, "https://evil.example").Get("Access-Control-Allow-Origin"); got != "" {
		t.Errorf("unlisted origin must not be allowed: %q", got)
	}
	if got := get(open, "").Get("Access-Control-Allow-Origin"); got != "" {
		t.Errorf("no Origin header: %q", got)
	}
	if got := get(open, "").Values("Vary"); len(got) == 0 {
		t.Error("responses must declare Vary: Origin")
	}
	closed := newHarness(t)
	if got := get(closed, "http://localhost:5173").Get("Access-Control-Allow-Origin"); got != "" {
		t.Errorf("no patterns configured, but %q was allowed", got)
	}
	if got := get(closed, closed.ts.URL).Get("Access-Control-Allow-Origin"); got != closed.ts.URL {
		t.Errorf("same-origin: %q", got)
	}
}
