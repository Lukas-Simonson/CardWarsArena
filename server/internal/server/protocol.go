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

// Why a room closed (room_closed's reason), with the English a client without its own words shows.
const (
	ReasonHostLeft     = "host_left"
	ReasonOpponentLeft = "opponent_left"
	ReasonExpired      = "room_expired"
)

var closedText = map[string]string{ReasonHostLeft: "host left", ReasonOpponentLeft: "opponent left", ReasonExpired: "room expired"}

// ClientMsg is anything a client may send. Which fields matter depends on Type.
type ClientMsg struct {
	Type    string         `json:"type"`
	Name    string         `json:"name,omitempty"`     // display name (create, join)
	Deck    string         `json:"deck,omitempty"`     // words from GET /api/decks (create, join)
	VsBot   bool           `json:"vs_bot,omitempty"`   // create: play the computer
	Bot     string         `json:"bot,omitempty"`      // create with vs_bot: which bot (see GET /api/bots); default heuristic
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
	// Avatars are the seats' pictures (addresses), in the same order as Names; "" for a guest or the bot.
	Avatars []string `json:"avatars,omitempty"`

	Seq    int          `json:"seq,omitempty"`    // update: increases by one per change
	View   *view.View   `json:"view,omitempty"`   // update
	Events []view.Event `json:"events,omitempty"` // update: what just happened, as this seat may see it

	Connected *bool  `json:"connected,omitempty"` // opponent
	Name      string `json:"name,omitempty"`      // opponent

	// error: Code is the kind of failure; Reason, when there is one, says exactly what (a rule of the game that was
	// broken, a deck problem); Params are the values a message about it needs (cards by id). A client words the
	// failure itself from these. Message is the same in English, for logs and for a client that knows no better.
	// room_closed: Reason says why (host_left, opponent_left, room_expired) and Message says it in English.
	Code    string         `json:"code,omitempty"`
	Reason  string         `json:"reason,omitempty"`
	Params  map[string]any `json:"params,omitempty"`
	Message string         `json:"message,omitempty"`
}
