package server

import (
	"fmt"
	"math/rand/v2"
	"sync"
	"time"

	"cardwars/internal/bot"
	"cardwars/internal/decks"
	"cardwars/internal/engine"
	"cardwars/internal/view"
)

// Sink receives messages for one connected client. Send must never block.
type Sink interface {
	Send(ServerMsg)
}

type seat struct {
	name  string
	token string
	deck  decks.Info
	isBot bool
	sink  Sink // nil while disconnected
}

// roomError is a failure reported to one client.
type roomError struct{ code, msg string }

func (e *roomError) Error() string { return e.msg }

func fail(code, msg string) error { return &roomError{code, msg} }

// Room is one match: two seats (one possibly a bot) and the game.
type Room struct {
	id  string
	srv *Server

	mu         sync.Mutex
	seats      [2]*seat
	g          *engine.Game
	seq        int
	vsBot      bool
	seed       uint64
	closed     bool
	botRunning bool
	brng       *rand.Rand
	touched    time.Time
	onClose    func(*Room)
}

func (r *Room) names() []string {
	out := make([]string, 2)
	for i, s := range r.seats {
		if s != nil {
			out[i] = s.name
		}
	}
	return out
}

func (r *Room) seatOf(token string) (int, bool) {
	for i, s := range r.seats {
		if s != nil && !s.isBot && token != "" && s.token == token {
			return i, true
		}
	}
	return 0, false
}

// startLocked builds the game once both seats are filled.
func (r *Room) startLocked() error {
	first := engine.PlayerID(r.seed % 2)
	g, err := engine.NewGame(r.srv.cat, engine.Config{Seed: r.seed, First: first, AllowUnimplemented: r.srv.cfg.AllowUnmodelled},
		[2]engine.DeckSpec{r.seats[0].deck.Spec(), r.seats[1].deck.Spec()})
	if err != nil {
		return err
	}
	r.g = g
	r.brng = rand.New(rand.NewPCG(r.seed, 0xB07))
	return nil
}

// broadcastLocked sends each connected seat its own view of the game.
func (r *Room) broadcastLocked(events []engine.Event) {
	r.seq++
	for i, s := range r.seats {
		if s == nil || s.sink == nil || s.isBot {
			continue
		}
		p := engine.PlayerID(i)
		v := view.Build(r.g, p)
		s.sink.Send(ServerMsg{Type: MsgUpdate, Seq: r.seq, View: &v, Events: view.FilterEvents(p, events), Names: r.names()})
	}
}

func (r *Room) joinedMsg(i int) ServerMsg {
	seatNo := i
	return ServerMsg{Type: MsgJoined, Room: r.id, Seat: &seatNo, Token: r.seats[i].token, VsBot: r.vsBot, Names: r.names()}
}

func (r *Room) notifyOpponentLocked(i int, connected bool) {
	o := r.seats[1-i]
	if o == nil || o.sink == nil || o.isBot || r.seats[i] == nil {
		return
	}
	o.sink.Send(ServerMsg{Type: MsgOpponent, Connected: &connected, Name: r.seats[i].name})
}

// join seats a second human. It returns the seat's token.
func (r *Room) join(sink Sink, name string, deck decks.Info) (string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.closed {
		return "", fail(ErrRoomNotFound, "room is closed")
	}
	if r.seats[1] != nil {
		return "", fail(ErrRoomFull, "room is full")
	}
	r.seats[1] = &seat{name: name, token: newToken(), deck: deck, sink: sink}
	r.touched = time.Now()
	if err := r.startLocked(); err != nil {
		r.seats[1] = nil
		return "", fail(ErrBadRequest, err.Error())
	}
	sink.Send(r.joinedMsg(1))
	r.notifyOpponentLocked(1, true)
	if h := r.seats[0]; h.sink != nil {
		h.sink.Send(ServerMsg{Type: MsgOpponent, Connected: boolp(true), Name: name})
	}
	r.broadcastLocked(nil)
	return r.seats[1].token, nil
}

// rejoin reattaches a client to its seat and brings it up to date.
func (r *Room) rejoin(token string, sink Sink) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.closed {
		return fail(ErrRoomNotFound, "room is closed")
	}
	i, ok := r.seatOf(token)
	if !ok {
		return fail(ErrBadToken, "unknown token for this room")
	}
	r.seats[i].sink = sink
	r.touched = time.Now()
	sink.Send(r.joinedMsg(i))
	if r.g == nil {
		sink.Send(ServerMsg{Type: MsgWaiting, Room: r.id})
	} else {
		p := engine.PlayerID(i)
		v := view.Build(r.g, p)
		sink.Send(ServerMsg{Type: MsgUpdate, Seq: r.seq, View: &v, Names: r.names()})
	}
	r.notifyOpponentLocked(i, true)
	if o := r.seats[1-i]; o != nil && !o.isBot && o.sink != nil {
		sink.Send(ServerMsg{Type: MsgOpponent, Connected: boolp(true), Name: o.name})
	}
	return nil
}

// detach marks a client's connection as gone (the seat stays reserved).
func (r *Room) detach(token string, sink Sink) {
	r.mu.Lock()
	defer r.mu.Unlock()
	i, ok := r.seatOf(token)
	if !ok || r.seats[i].sink != sink {
		return
	}
	r.seats[i].sink = nil
	r.touched = time.Now()
	r.notifyOpponentLocked(i, false)
}

// act applies a player's action and tells everyone.
func (r *Room) act(token string, a engine.Action, seq *int) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.closed {
		return fail(ErrRoomNotFound, "room is closed")
	}
	i, ok := r.seatOf(token)
	if !ok {
		return fail(ErrBadToken, "unknown token for this room")
	}
	if r.g == nil {
		return fail(ErrNotStarted, "waiting for an opponent")
	}
	if seq != nil && *seq != r.seq {
		return fail(ErrStale, fmt.Sprintf("the game has moved on (now at seq %d)", r.seq))
	}
	events, err := r.g.Apply(engine.PlayerID(i), a)
	if err != nil {
		return fail(ErrIllegal, err.Error())
	}
	r.touched = time.Now()
	r.broadcastLocked(events)
	r.kickBotLocked()
	return nil
}

// leave removes a player: the room closes and the opponent is told.
func (r *Room) leave(token string) {
	r.mu.Lock()
	_, ok := r.seatOf(token)
	started := r.g != nil
	r.mu.Unlock()
	if !ok {
		return
	}
	reason := "host left"
	if started {
		reason = "opponent left"
	}
	r.close(reason, token)
}

// close shuts the room, telling every connected seat except skip.
func (r *Room) close(reason, skipToken string) {
	r.mu.Lock()
	if r.closed {
		r.mu.Unlock()
		return
	}
	r.closed = true
	for _, s := range r.seats {
		if s != nil && s.sink != nil && !s.isBot && s.token != skipToken {
			s.sink.Send(ServerMsg{Type: MsgRoomClosed, Message: reason})
		}
	}
	cb := r.onClose
	r.mu.Unlock()
	if cb != nil {
		cb(r)
	}
}

// kickBotLocked starts the bot driver if the bot has a decision to make.
func (r *Room) kickBotLocked() {
	if !r.vsBot || r.closed || r.botRunning || r.g == nil || !r.botAwaitedLocked() {
		return
	}
	r.botRunning = true
	go r.runBot()
}

func (r *Room) botAwaitedLocked() bool {
	for _, p := range r.g.Awaiting() {
		if r.seats[p] != nil && r.seats[p].isBot {
			return true
		}
	}
	return false
}

// runBot plays the bot's moves one at a time, pausing between them so a
// client can show what happened.
func (r *Room) runBot() {
	for {
		r.mu.Lock()
		if r.closed || !r.botAwaitedLocked() {
			r.botRunning = false
			r.mu.Unlock()
			return
		}
		var done bool
		for _, p := range r.g.Awaiting() {
			if r.seats[p] == nil || !r.seats[p].isBot {
				continue
			}
			a, ok := bot.Choose(r.g, p, r.brng)
			if !ok {
				continue
			}
			events, err := r.g.Apply(p, a)
			if err == nil {
				r.broadcastLocked(events)
				done = true
			}
			break
		}
		if !done {
			r.botRunning = false
			r.mu.Unlock()
			return
		}
		r.mu.Unlock()
		if d := r.srv.cfg.BotDelay; d > 0 {
			time.Sleep(d)
		}
	}
}

func boolp(b bool) *bool { return &b }
