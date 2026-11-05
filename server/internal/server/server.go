// Package server hosts matches over WebSocket: rooms, seats, a bot opponent,
// and per-player views so a client never receives hidden information.
package server

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/url"
	"path"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode"

	"cardwars/internal/auth"
	"cardwars/internal/bot"
	"cardwars/internal/card"
	"cardwars/internal/decks"
	"cardwars/internal/engine"
	"cardwars/internal/store"
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
	// and to read the JSON API (host patterns such as "localhost:*").
	// Same-origin and non-browser clients are always allowed.
	OriginPatterns []string
	// Auth says who is signed in; nil means nobody ever is (auth.None).
	Auth auth.Authenticator
	// Store keeps accounts and decks; nil means nothing is kept and the saved-deck API is off.
	Store  store.Store
	Logger *slog.Logger
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
	if cfg.Auth == nil {
		cfg.Auth = auth.None()
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
	if len(playable) > 0 { // the player has a deck of their own: any starter will do
		return playable[0], nil
	}
	return decks.Info{}, fail(ErrUnknownDeck, "no deck available for the bot")
}

// avatarOf is a signed-in player's picture, or "" for a guest.
func avatarOf(u *store.User) string {
	if u == nil {
		return ""
	}
	return u.AvatarURL
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
func (s *Server) create(sink Sink, user *store.User, m ClientMsg) (*Room, string, error) {
	deck, err := s.resolveDeck(user, m.Deck)
	if err != nil {
		return nil, "", err
	}
	var botDeck decks.Info
	var opponent bot.Bot
	if m.VsBot {
		if opponent, err = bot.New(m.Bot); err != nil {
			return nil, "", fail(ErrBadRequest, err.Error())
		}
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
	r := &Room{id: id, srv: s, vsBot: m.VsBot, bot: opponent, seed: seed, touched: time.Now(), onClose: s.removeRoom}
	host := &seat{name: cleanName(m.Name, "Player"), avatar: avatarOf(user), token: newToken(), deck: deck, sink: sink}
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
	mux.HandleFunc("GET /api/bots", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"default": bot.Default, "bots": bot.Names()})
	})
	s.cfg.Auth.Routes(mux)
	s.mountMyDecks(mux)
	mux.HandleFunc("GET /ws", s.handleWS)
	mux.HandleFunc("GET /{$}", s.handleDebugPage)
	return s.withCORS(mux)
}

// originAllowed reports whether a browser origin may read our API: the same
// host, or one matching OriginPatterns ("localhost:*").
func (s *Server) originAllowed(origin, host string) bool {
	u, err := url.Parse(origin)
	if err != nil || u.Host == "" {
		return false
	}
	if strings.EqualFold(u.Host, host) {
		return true
	}
	for _, p := range s.cfg.OriginPatterns {
		if ok, _ := path.Match(strings.ToLower(p), strings.ToLower(u.Host)); ok {
			return true
		}
	}
	return false
}

// withCORS lets pages served from an allowed origin (a separate web client)
// read the JSON API, with its cookies, so a signed-in player is recognised.
// WebSocket origins are checked by the upgrade itself.
func (s *Server) withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Vary", "Origin")
		if o := r.Header.Get("Origin"); o != "" && s.originAllowed(o, r.Host) {
			w.Header().Set("Access-Control-Allow-Origin", o)
			w.Header().Set("Access-Control-Allow-Credentials", "true")
			if r.Method == http.MethodOptions { // a browser asking before it sends JSON or a PUT/DELETE
				w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE")
				w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
				w.Header().Set("Access-Control-Max-Age", "600")
				w.WriteHeader(http.StatusNoContent)
				return
			}
		}
		next.ServeHTTP(w, r)
	})
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
