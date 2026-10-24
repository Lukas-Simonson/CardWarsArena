package engine

import (
	"errors"
	"fmt"
)

// ActionKind names something a player can do.
type ActionKind string

const (
	ActKeepHand ActionKind = "keep_hand" // mulligan phase: keep the opening hand
	ActMulligan ActionKind = "mulligan"  // mulligan phase: redraw the opening hand (once)
	ActPlayCard ActionKind = "play_card" // main phase: Card = UID in hand, Lane for Creatures/Buildings
	ActDraw     ActionKind = "draw"      // main phase: spend 1 Action to draw 1 card
	ActEndMain  ActionKind = "end_main"  // main phase: move on to the Fight phase
	ActAttack   ActionKind = "attack"    // fight phase: Activate the Creature in Lane to Fight
)

// Action is a single player decision. It is the unit sent over the wire.
type Action struct {
	Kind ActionKind `json:"kind"`
	Card int        `json:"card,omitempty"` // Instance.UID
	Lane int        `json:"lane,omitempty"`
}

func (a Action) String() string {
	switch a.Kind {
	case ActPlayCard:
		return fmt.Sprintf("play_card(uid=%d, lane=%d)", a.Card, a.Lane)
	case ActAttack:
		return fmt.Sprintf("attack(lane=%d)", a.Lane)
	}
	return string(a.Kind)
}

// EventKind names something that happened, for logs and client animation.
type EventKind string

const (
	EvMulligan  EventKind = "mulligan"
	EvKeepHand  EventKind = "keep_hand"
	EvTurnStart EventKind = "turn_start"
	EvDraw      EventKind = "draw"
	EvPlay      EventKind = "play"
	EvReplace   EventKind = "replace"
	EvAttack    EventKind = "attack"
	EvDamage    EventKind = "damage"  // Creature took damage
	EvHPLoss    EventKind = "hp_loss" // a player lost Hit Points
	EvDie       EventKind = "die"
	EvGameOver  EventKind = "game_over"
)

// Event describes one state change. Fields are used as relevant to the kind.
type Event struct {
	Kind   EventKind `json:"kind"`
	Player PlayerID  `json:"player"`
	Card   int       `json:"card,omitempty"` // UID
	Lane   int       `json:"lane,omitempty"`
	Amount int       `json:"amount,omitempty"`
}

// Rule violations returned by Apply.
var (
	ErrGameOver       = errors.New("game is over")
	ErrWrongPhase     = errors.New("action not allowed in this phase")
	ErrNotYourTurn    = errors.New("not your turn")
	ErrAlreadyDecided = errors.New("mulligan decision already made")
	ErrNoMulligan     = errors.New("mulligan already used")
	ErrUnknownAction  = errors.New("unknown action")
	ErrNoSuchCard     = errors.New("card not in hand")
	ErrBadLane        = errors.New("lane out of range")
	ErrNoActions      = errors.New("not enough Actions")
	ErrLandscapes     = errors.New("not enough Landscapes of the required type")
	ErrFlooped        = errors.New("cannot replace a Flooped card")
	ErrDeckEmpty      = errors.New("deck is empty")
	ErrNoAttacker     = errors.New("no Ready Creature in that Lane")
	ErrFirstTurn      = errors.New("the first player may not Floop or Fight on the first turn")
	ErrNotPlayable    = errors.New("card cannot be played")
	ErrCannotAttack   = errors.New("that Creature cannot be attacked")
)
