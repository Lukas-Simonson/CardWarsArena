package server

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"sync"
	"time"

	"github.com/coder/websocket"

	"cardwars/internal/store"
)

const (
	maxMessageBytes = 16 << 10
	sendBuffer      = 256
	maxViolations   = 25 // consecutive rate-limited messages before we hang up
	writeTimeout    = 10 * time.Second
	pingInterval    = 30 * time.Second
)

// client is one WebSocket connection. It implements Sink.
type client struct {
	srv  *Server
	conn *websocket.Conn
	send chan ServerMsg
	stop context.CancelFunc
	// user is who the connection signed in as when it opened, if anyone.
	user *store.User

	mu    sync.Mutex
	room  *Room
	token string

	bucket     tokenBucket
	violations int // consecutive rate-limited messages
}

// Send queues a message without blocking. A client too slow to keep up is
// disconnected rather than allowed to stall the room.
func (c *client) Send(m ServerMsg) {
	if m.Type == MsgRoomClosed {
		c.mu.Lock()
		c.room, c.token = nil, ""
		c.mu.Unlock()
	}
	select {
	case c.send <- m:
	default:
		c.stop()
	}
}

func (c *client) membership() (*Room, string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.room, c.token
}

func (c *client) setMembership(r *Room, token string) {
	c.mu.Lock()
	c.room, c.token = r, token
	c.mu.Unlock()
}

func (c *client) fail(err error) {
	var re *roomError
	if errors.As(err, &re) {
		c.Send(ServerMsg{Type: MsgError, Code: re.code, Message: re.msg})
		return
	}
	c.Send(ServerMsg{Type: MsgError, Code: ErrBadRequest, Message: err.Error()})
}

func (c *client) writer(ctx context.Context) {
	ping := time.NewTicker(pingInterval)
	defer ping.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case m := <-c.send:
			b, err := json.Marshal(m)
			if err != nil {
				continue
			}
			wctx, cancel := context.WithTimeout(ctx, writeTimeout)
			err = c.conn.Write(wctx, websocket.MessageText, b)
			cancel()
			if err != nil {
				c.stop()
				return
			}
		case <-ping.C:
			pctx, cancel := context.WithTimeout(ctx, writeTimeout)
			err := c.conn.Ping(pctx)
			cancel()
			if err != nil {
				c.stop()
				return
			}
		}
	}
}

func (s *Server) handleWS(w http.ResponseWriter, r *http.Request) {
	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{OriginPatterns: s.cfg.OriginPatterns})
	if err != nil {
		return // Accept has already replied
	}
	conn.SetReadLimit(maxMessageBytes)
	ctx, cancel := context.WithCancel(context.Background())
	var user *store.User
	if u, ok := s.cfg.Auth.Identify(r); ok {
		user = &u
	}
	c := &client{srv: s, conn: conn, user: user, send: make(chan ServerMsg, sendBuffer), stop: cancel, bucket: newTokenBucket(s.cfg.Burst, s.cfg.MessagesPerSecond)}
	defer func() {
		cancel()
		if room, token := c.membership(); room != nil {
			room.detach(token, c)
		}
		conn.CloseNow()
	}()
	go c.writer(ctx)
	c.Send(ServerMsg{Type: MsgHello, Protocol: ProtocolVersion})

	for {
		typ, data, err := conn.Read(ctx)
		if err != nil {
			return
		}
		if typ != websocket.MessageText {
			c.fail(fail(ErrBadRequest, "messages must be JSON text"))
			continue
		}
		if !c.bucket.allow() {
			// A small overshoot gets an error; a client that keeps flooding is
			// disconnected rather than answered forever.
			if c.violations++; c.violations >= maxViolations {
				conn.Close(websocket.StatusPolicyViolation, "rate limit exceeded")
				return
			}
			c.fail(fail(ErrRateLimited, "slow down"))
			continue
		}
		c.violations = 0
		var m ClientMsg
		if err := json.Unmarshal(data, &m); err != nil {
			c.fail(fail(ErrBadRequest, "invalid JSON: "+err.Error()))
			continue
		}
		c.dispatch(m)
	}
}

func (c *client) dispatch(m ClientMsg) {
	s := c.srv
	if c.user != nil {
		m.Name = c.user.DisplayName // a signed-in player cannot play under someone else's name
	}
	switch m.Type {
	case MsgPing:
		c.Send(ServerMsg{Type: MsgPong})
	case MsgCreate:
		if r, _ := c.membership(); r != nil {
			c.fail(fail(ErrAlreadyInRoom, "leave your current room first"))
			return
		}
		room, token, err := s.create(c, c.user, m)
		if err != nil {
			c.fail(err)
			return
		}
		c.setMembership(room, token)
	case MsgJoin:
		if r, _ := c.membership(); r != nil {
			c.fail(fail(ErrAlreadyInRoom, "leave your current room first"))
			return
		}
		room := s.room(m.Room)
		if room == nil {
			c.fail(fail(ErrRoomNotFound, "no such room"))
			return
		}
		deck, err := s.resolveDeck(c.user, m.Deck)
		if err != nil {
			c.fail(err)
			return
		}
		token, err := room.join(c, cleanName(m.Name, "Player"), avatarOf(c.user), deck)
		if err != nil {
			c.fail(err)
			return
		}
		c.setMembership(room, token)
	case MsgRejoin:
		if r, _ := c.membership(); r != nil {
			c.fail(fail(ErrAlreadyInRoom, "leave your current room first"))
			return
		}
		room := s.room(m.Room)
		if room == nil {
			c.fail(fail(ErrRoomNotFound, "no such room"))
			return
		}
		if err := room.rejoin(m.Token, c); err != nil {
			c.fail(err)
			return
		}
		c.setMembership(room, m.Token)
	case MsgAction:
		room, token := c.membership()
		if room == nil {
			c.fail(fail(ErrNotInRoom, "join or create a room first"))
			return
		}
		if m.Action == nil {
			c.fail(fail(ErrBadRequest, "action is required"))
			return
		}
		if err := room.act(token, *m.Action, m.Seq); err != nil {
			c.fail(err)
		}
	case MsgLeave:
		room, token := c.membership()
		if room == nil {
			c.fail(fail(ErrNotInRoom, "not in a room"))
			return
		}
		c.setMembership(nil, "")
		room.leave(token)
	default:
		c.fail(fail(ErrBadRequest, "unknown message type "+m.Type))
	}
}

// tokenBucket is a small rate limiter: burst capacity, steady refill per second.
type tokenBucket struct {
	cap, rate float64
	level     float64
	last      time.Time
}

func newTokenBucket(capacity, perSecond float64) tokenBucket {
	return tokenBucket{cap: capacity, rate: perSecond, level: capacity, last: time.Now()}
}

func (b *tokenBucket) allow() bool {
	now := time.Now()
	b.level = min(b.cap, b.level+now.Sub(b.last).Seconds()*b.rate)
	b.last = now
	if b.level < 1 {
		return false
	}
	b.level--
	return true
}
