package server

import (
	"cardwars/internal/engine"
	"cardwars/internal/view"
)

// ProtocolVersion is bumped on incompatible message changes.
const ProtocolVersion = 1

// Client message types.
const (
	MsgCreate = "create" // create a room (optionally against a bot)
	MsgJoin   = "join"   // join a waiting room by code
	MsgRejoin = "rejoin" // reattach to a room after a disconnect
	MsgAction = "action" // perform a game action
	MsgLeave  = "leave"  // leave (forfeits a game in progress)
	MsgPing   = "ping"
)

// Server message types.
const (
	MsgHello      = "hello"       // sent on connect
	MsgJoined     = "joined"      // you are seated in a room
	MsgWaiting    = "waiting"     // room created, waiting for an opponent
	MsgUpdate     = "update"      // the game changed: view plus events
	MsgOpponent   = "opponent"    // opponent connected or disconnected
	MsgError      = "error"       // a request failed
	MsgRoomClosed = "room_closed" // the room is gone
	MsgPong       = "pong"
)

// Error codes.
const (
	ErrBadRequest    = "bad_request"
	ErrUnknownDeck   = "unknown_deck"
	ErrDeckNotReady  = "deck_not_playable"
	ErrRoomNotFound  = "room_not_found"
	ErrRoomFull      = "room_full"
	ErrNotInRoom     = "not_in_room"
	ErrAlreadyInRoom = "already_in_room"
	ErrBadToken      = "bad_token"
	ErrIllegal       = "illegal_action"
	ErrNotStarted    = "game_not_started"
	ErrServerFull    = "server_full"
	ErrRateLimited   = "rate_limited"
	ErrStale         = "stale_state"
)

// ClientMsg is anything a client may send. Which fields matter depends on Type.
type ClientMsg struct {
	Type    string         `json:"type"`
	Name    string         `json:"name,omitempty"`     // display name (create, join)
	Deck    string         `json:"deck,omitempty"`     // words from GET /api/decks (create, join)
	VsBot   bool           `json:"vs_bot,omitempty"`   // create: play the computer
	BotDeck string         `json:"bot_deck,omitempty"` // create: the computer's deck (default: any other playable deck)
	Room    string         `json:"room,omitempty"`     // join, rejoin
	Token   string         `json:"token,omitempty"`    // rejoin
	Seed    *uint64        `json:"seed,omitempty"`     // create: fixed seed (only if the server allows it)
	Action  *engine.Action `json:"action,omitempty"`   // action
	// Seq is the update seq the action was chosen from. If the game has moved
	// on since, the server rejects the action as stale instead of applying it
	// to a different position. Optional but strongly recommended.
	Seq *int `json:"seq,omitempty"`
}

// ServerMsg is anything the server sends. Which fields are set depends on Type.
type ServerMsg struct {
	Type string `json:"type"`

	Protocol int `json:"protocol,omitempty"` // hello

	Room  string   `json:"room,omitempty"`  // joined, waiting
	Seat  *int     `json:"seat,omitempty"`  // joined: 0 or 1
	Token string   `json:"token,omitempty"` // joined: keep it to rejoin
	VsBot bool     `json:"vs_bot,omitempty"`
	Names []string `json:"names,omitempty"` // joined/update: seat display names

	Seq    int            `json:"seq,omitempty"`    // update: increases by one per change
	View   *view.View     `json:"view,omitempty"`   // update
	Events []engine.Event `json:"events,omitempty"` // update: what just happened, as this seat may see it

	Connected *bool  `json:"connected,omitempty"` // opponent
	Name      string `json:"name,omitempty"`      // opponent

	Code    string `json:"code,omitempty"`    // error
	Message string `json:"message,omitempty"` // error, room_closed (reason)
}
