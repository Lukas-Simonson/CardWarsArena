// Package server hosts matches over WebSocket: rooms, seats, a bot opponent,
// and per-player views so a client never receives hidden information.
package server

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode"

	"cardwars/internal/card"
	"cardwars/internal/decks"
	"cardwars/internal/engine"
)

// Config holds server settings. The zero value is usable once DataDir or
// Catalog is set.
type Config struct {
	DataDir  string        // directory with cards/ and effects/ (ignored if Catalog is set)
	Catalog  card.Catalog  // preloaded catalog (tests)
	BotDelay time.Duration // pause between the bot's moves, so clients can animate
	MaxRooms int           // default 1000
	// Per-connection message limit: Burst messages at once, refilling at
	// MessagesPerSecond. Defaults 40 and 20; far above what a person produces.
	MessagesPerSecond float64
	Burst             float64
	RoomTTL           time.Duration // idle rooms are removed after this long; default 30m
	// AllowSeed lets clients choose the shuffle seed. Development only: a
	// known seed lets a player predict the opponent's deck order.
	AllowSeed bool
	// AllowUnmodelled permits decks with cards whose rules are not modelled
	// (their text is ignored). Development only.
	AllowUnmodelled bool
	// OriginPatterns lists extra browser origins allowed to open a WebSocket
	// (host patterns such as "localhost:*"). Same-origin and non-browser
	// clients are always allowed.
	OriginPatterns []string
	Logger         *slog.Logger
}

// Server is the HTTP handler and room registry.
type Server struct {
	cfg   Config
	cat   card.Catalog
	decks []decks.Info
	log   *slog.Logger

	mu    sync.Mutex
	rooms map[string]*Room
}

// New loads the card data and prepares a server.
func New(cfg Config) (*Server, error) {
	if cfg.Catalog == nil {
		cat, err := card.Load(cfg.DataDir)
		if err != nil {
			return nil, err
		}
		cfg.Catalog = cat
	}
	if cfg.MaxRooms == 0 {
		cfg.MaxRooms = 1000
	}
	if cfg.MessagesPerSecond == 0 {
		cfg.MessagesPerSecond = 20
	}
	if cfg.Burst == 0 {
		cfg.Burst = 40
	}
	if cfg.RoomTTL == 0 {
		cfg.RoomTTL = 30 * time.Minute
	}
	if cfg.Logger == nil {
		cfg.Logger = slog.New(slog.NewTextHandler(discard{}, nil))
	}
	return &Server{cfg: cfg, cat: cfg.Catalog, decks: decks.Find(cfg.Catalog), log: cfg.Logger, rooms: map[string]*Room{}}, nil
}

type discard struct{}

func (discard) Write(p []byte) (int, error) { return len(p), nil }

// Run removes idle rooms until ctx ends.
func (s *Server) Run(ctx context.Context) {
	t := time.NewTicker(time.Minute)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case now := <-t.C:
			s.sweep(now)
		}
	}
}

// sweep closes rooms nobody has touched within the TTL.
func (s *Server) sweep(now time.Time) {
	s.mu.Lock()
	var stale []*Room
	for _, r := range s.rooms {
		r.mu.Lock()
		if now.Sub(r.touched) > s.cfg.RoomTTL {
			stale = append(stale, r)
		}
		r.mu.Unlock()
	}
	s.mu.Unlock()
	for _, r := range stale {
		r.close("room expired", "")
	}
}

func (s *Server) removeRoom(r *Room) {
	s.mu.Lock()
	delete(s.rooms, r.id)
	s.mu.Unlock()
}

// RoomCount reports how many rooms exist (for tests and metrics).
func (s *Server) RoomCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.rooms)
}

func (s *Server) room(code string) *Room {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.rooms[strings.ToUpper(strings.TrimSpace(code))]
}

// deckFor resolves and checks a deck query.
func (s *Server) deckFor(query string) (decks.Info, error) {
	d, err := decks.Pick(s.decks, query)
	if err != nil {
		return d, fail(ErrUnknownDeck, err.Error())
	}
	ok := d.LandOK && len(d.Cards) >= engine.MinDeckSize && (d.Modelled == d.Distinct || s.cfg.AllowUnmodelled)
	if !ok {
		return d, fail(ErrDeckNotReady, d.Key()+" cannot be played yet")
	}
	return d, nil
}

// botDeckFor picks the computer's deck: the one asked for, or the next
// playable deck after the player's.
func (s *Server) botDeckFor(query string, player decks.Info) (decks.Info, error) {
	if query != "" {
		return s.deckFor(query)
	}
	var playable []decks.Info
	for _, d := range s.decks {
		if _, err := s.deckFor(d.Key()); err == nil {
			playable = append(playable, d)
		}
	}
	for i, d := range playable {
		if d.Key() == player.Key() {
			return playable[(i+1)%len(playable)], nil
		}
	}
	return decks.Info{}, fail(ErrUnknownDeck, "no deck available for the bot")
}

func cleanName(n, fallback string) string {
	n = strings.TrimSpace(n)
	var b strings.Builder
	for _, r := range n {
		if unicode.IsControl(r) {
			continue
		}
		b.WriteRune(r)
		if b.Len() >= 24 {
			break
		}
	}
	if b.Len() == 0 {
		return fallback
	}
	return b.String()
}

// create makes a room with the client in seat 0.
func (s *Server) create(sink Sink, m ClientMsg) (*Room, string, error) {
	deck, err := s.deckFor(m.Deck)
	if err != nil {
		return nil, "", err
	}
	var botDeck decks.Info
	if m.VsBot {
		if botDeck, err = s.botDeckFor(m.BotDeck, deck); err != nil {
			return nil, "", err
		}
	}
	seed := randomSeed()
	if m.Seed != nil && s.cfg.AllowSeed {
		seed = *m.Seed
	}

	s.mu.Lock()
	if len(s.rooms) >= s.cfg.MaxRooms {
		s.mu.Unlock()
		return nil, "", fail(ErrServerFull, "too many games in progress; try again later")
	}
	id := newRoomCode()
	for s.rooms[id] != nil {
		id = newRoomCode()
	}
	r := &Room{id: id, srv: s, vsBot: m.VsBot, seed: seed, touched: time.Now(), onClose: s.removeRoom}
	host := &seat{name: cleanName(m.Name, "Player"), token: newToken(), deck: deck, sink: sink}
	r.seats[0] = host
	if m.VsBot {
		r.seats[1] = &seat{name: "Bot", isBot: true, deck: botDeck}
		if err := r.startLocked(); err != nil {
			s.mu.Unlock()
			return nil, "", fail(ErrBadRequest, err.Error())
		}
	}
	s.rooms[id] = r
	s.mu.Unlock()

	r.mu.Lock()
	sink.Send(r.joinedMsg(0))
	if m.VsBot {
		r.broadcastLocked(nil)
		r.kickBotLocked()
	} else {
		sink.Send(ServerMsg{Type: MsgWaiting, Room: r.id})
	}
	r.mu.Unlock()
	return r, host.token, nil
}

// Handler serves the WebSocket endpoint, the JSON API and the debug page.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) { w.Write([]byte("ok\n")) })
	mux.HandleFunc("GET /api/decks", s.handleDecks)
	mux.HandleFunc("GET /api/cards", s.handleCards)
	mux.HandleFunc("GET /ws", s.handleWS)
	mux.HandleFunc("GET /{$}", s.handleDebugPage)
	return mux
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}

func (s *Server) handleDecks(w http.ResponseWriter, _ *http.Request) {
	list := make([]decks.Info, 0, len(s.decks))
	for _, d := range s.decks {
		_, err := s.deckFor(d.Key())
		d.Ready = err == nil // what this server will accept, not just the strict rule
		list = append(list, d)
	}
	writeJSON(w, list)
}

func (s *Server) handleCards(w http.ResponseWriter, _ *http.Request) {
	type apiCard struct {
		*card.Card
		Modeled bool `json:"modeled"`
	}
	list := make([]apiCard, 0, len(s.cat))
	for _, c := range s.cat {
		list = append(list, apiCard{c, c.Implemented()})
	}
	sort.Slice(list, func(i, j int) bool { return list[i].ID < list[j].ID })
	writeJSON(w, list)
}
