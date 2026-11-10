package engine

import (
	"errors"
	"fmt"
)

// ActionKind names something a player can do.
type ActionKind string

const (
	ActKeepHand    ActionKind = "keep_hand"    // mulligan phase: keep the opening hand
	ActMulligan    ActionKind = "mulligan"     // mulligan phase: redraw the opening hand (once)
	ActPlayCard    ActionKind = "play_card"    // main phase: Card = UID in hand, Lane for Creatures/Buildings
	ActDraw        ActionKind = "draw"         // main phase: spend 1 Action to draw 1 card
	ActEndMain     ActionKind = "end_main"     // main phase: move on to the Fight phase
	ActAttack      ActionKind = "attack"       // fight phase: Activate the Creature in Lane to Fight
	ActPlayPending ActionKind = "play_pending" // play the card taken by Silo of Truth for free (Lane, Targets)
	ActFloop       ActionKind = "floop"        // main phase: Floop the card in Lane (Source: creature or building)
	ActActivate    ActionKind = "activate"     // main phase: pay Actions for an ability of the card in Lane (Index picks which)
	ActChoose      ActionKind = "choose"       // answer a pending Choice: Card (a hand or revealed card), or Lane
	ActThaw        ActionKind = "thaw"         // main phase: discard the hand card Card to remove the frozen token on your Landscape in Lane
)

// Action is a single player decision. It is the unit sent over the wire.
type Action struct {
	Kind ActionKind `json:"kind"`
	Card int        `json:"card,omitempty"` // Instance.UID
	Lane int        `json:"lane,omitempty"`
	// Source is "creature" (default) or "building": which card in Lane a
	// floop or activate action refers to. Index picks among a card's
	// activated abilities.
	Source string `json:"source,omitempty"`
	Index  int    `json:"index,omitempty"`
	// CostTargets holds the choices for the card's additional cost, such as
	// which card to discard.
	CostTargets [][]TargetRef `json:"cost_targets,omitempty"`
	// Targets holds one list of choices per target slot of the effect being
	// used (the Spell, enter-play trigger, Floop or activated ability).
	Targets [][]TargetRef `json:"targets,omitempty"`
	// Labels is sent with a legal action, one per target slot, saying what
	// each choice is for. It is for display only and is ignored when played.
	Labels []string `json:"labels,omitempty"`
}

// TargetRef names one chosen target. Kind is the TargetSpec "what":
// creature, building, landscape and lane use Player and Lane; discard_card
// uses Player and Card (the UID).
type TargetRef struct {
	Kind   string   `json:"kind"`
	Player PlayerID `json:"player"`
	Lane   int      `json:"lane,omitempty"`
	Card   int      `json:"card,omitempty"`
}

func (a Action) String() string {
	switch a.Kind {
	case ActPlayCard:
		return fmt.Sprintf("play_card(uid=%d, lane=%d)", a.Card, a.Lane)
	case ActAttack:
		return fmt.Sprintf("attack(lane=%d)", a.Lane)
	case ActPlayPending:
		return fmt.Sprintf("play_pending(lane=%d)", a.Lane)
	case ActChoose:
		return fmt.Sprintf("choose(card=%d, lane=%d)", a.Card, a.Lane)
	case ActThaw:
		return fmt.Sprintf("thaw(lane=%d, card=%d)", a.Lane, a.Card)
	case ActFloop, ActActivate:
		return fmt.Sprintf("%s(%s lane=%d idx=%d)", a.Kind, a.Source, a.Lane, a.Index)
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
	EvHPGain    EventKind = "hp_gain" // a player healed Hit Points
	EvHeal      EventKind = "heal"    // a Creature had damage healed
	EvDie       EventKind = "die"
	EvGameOver  EventKind = "game_over"
	EvMove      EventKind = "move"      // a Creature or Building changed Lanes (Lane = destination)
	EvFlip      EventKind = "flip"      // a Landscape was turned over
	EvReady     EventKind = "ready"     // a Creature was readied
	EvBounce    EventKind = "bounce"    // a Creature returned to its owner's hand
	EvDestroy   EventKind = "destroy"   // a Building was destroyed
	EvReturn    EventKind = "return"    // a card came back from a discard pile to hand
	EvDeckTop   EventKind = "deck_top"  // a card was put on top of a deck
	EvActions   EventKind = "actions"   // Actions were gained
	EvFloop     EventKind = "floop"     // a card Flooped
	EvActivate  EventKind = "activate"  // an activated ability was used
	EvDiscard   EventKind = "discard"   // a card was discarded from hand
	EvSteal     EventKind = "steal"     // a card was taken from the opponent's hand
	EvAbilities EventKind = "abilities" // a Creature's abilities changed (Lane, Card)
	EvReveal    EventKind = "reveal"    // a card in hand was shown (Card)
	EvFreeze    EventKind = "freeze"    // a Landscape got a frozen token (Player, Lane)
	EvThaw      EventKind = "thaw"      // a Landscape lost its frozen token (Player, Lane)
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
	ErrNoTarget       = errors.New("no legal target")
	ErrNotReady       = errors.New("that card is not Ready")
	ErrNoAbility      = errors.New("that card has no such ability")
	ErrPending        = errors.New("the stolen card must be played first")
	ErrNoPending      = errors.New("there is no stolen card to play")
	ErrBadTarget      = errors.New("invalid target choice")
	ErrCondition      = errors.New("the card's condition is not met")
	ErrChoice         = errors.New("a decision must be made first")
	ErrNoChoice       = errors.New("there is no decision to make")
	ErrLimit          = errors.New("that ability has been used as often as it can be this turn")
	ErrNoFloop        = errors.New("Creatures in this Lane cannot use Floop abilities")
	ErrFrozen         = errors.New("a Creature cannot be played onto a frozen Landscape, or replace one there")
	ErrNotFrozen      = errors.New("that Landscape has no frozen token")
)
