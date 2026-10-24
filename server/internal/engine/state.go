// Package engine is the Card Wars rules engine. It is pure game logic: no I/O,
// no networking, deterministic given a seed. See card-wars-rulebook.md.
package engine

import "cardwars/internal/card"

const (
	NumLanes       = 4
	StartingHP     = 25
	StartingHand   = 5
	ActionsPerTurn = 2
)

// PlayerID is a seat at the table, 0 or 1.
type PlayerID int

func (p PlayerID) Opp() PlayerID { return 1 - p }

// Phase is the part of the game the engine is in.
type Phase string

const (
	PhaseMulligan Phase = "mulligan" // both players decide on their opening hand
	PhaseMain     Phase = "main"     // play cards, Floop, draw for Actions
	PhaseFight    Phase = "fight"    // Ready Creatures must Fight
	PhaseOver     Phase = "over"
)

// Status is the Ready/Exhausted state of a Creature or Building. Flooped and
// Fought are both exhausted, but some cards care about which.
type Status string

const (
	Ready   Status = "ready"
	Flooped Status = "flooped"
	Fought  Status = "fought"
)

// Instance is one physical copy of a card in a game.
type Instance struct {
	UID    int    `json:"uid"`
	CardID string `json:"card"`
}

// Creature is a Creature in play.
type Creature struct {
	Instance
	Damage int    `json:"damage"`
	Status Status `json:"status"`
	// Per-turn state, cleared when the turn ends.
	TempATK      int  `json:"temp_atk,omitempty"`
	TempDEF      int  `json:"temp_def,omitempty"`
	ChangedLanes bool `json:"changed_lanes,omitempty"`
}

// Building is a Building in play.
type Building struct {
	Instance
	Status Status `json:"status"`
}

// Lane is one player's half of a Lane: a Landscape plus what sits on it.
type Lane struct {
	Landscape card.Landscape `json:"landscape"`
	FaceDown  bool           `json:"face_down"`
	Creature  *Creature      `json:"creature,omitempty"`
	Building  *Building      `json:"building,omitempty"`
}

// Player is everything one player owns.
type Player struct {
	HP         int            `json:"hp"`
	Deck       []Instance     `json:"deck"` // index 0 is the top
	Hand       []Instance     `json:"hand"`
	Discard    []Instance     `json:"discard"`
	Lanes      [NumLanes]Lane `json:"lanes"`
	Kept       bool           `json:"kept"`       // finished the mulligan decision
	Mulliganed bool           `json:"mulliganed"` // used their one mulligan
}

// State is the complete, serializable game state.
type State struct {
	Players     [2]Player `json:"players"`
	Active      PlayerID  `json:"active"`
	First       PlayerID  `json:"first"` // who took the very first turn
	Phase       Phase     `json:"phase"`
	Turn        int       `json:"turn"` // 1 for the first turn of the game
	ActionsLeft int       `json:"actions_left"`
	Winner      PlayerID  `json:"winner"` // meaningful only when Phase == PhaseOver
	NextUID     int       `json:"next_uid"`
	// SpellsThisTurn counts Spells the active player has played this turn.
	SpellsThisTurn int `json:"spells_this_turn"`
}
