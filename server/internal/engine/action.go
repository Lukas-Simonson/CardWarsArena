package engine

import (
	"fmt"

	"cardwars/internal/problem"
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
	// Via is sent with a legal Floop that was granted to the card, not printed on it ("hero"). Display only.
	Via string `json:"via,omitempty"`
	// CostTargets holds the choices for the card's additional cost, such as
	// which card to discard.
	CostTargets [][]TargetRef `json:"cost_targets,omitempty"`
	// Targets holds one list of choices per target slot of the effect being
	// used (the Spell, enter-play trigger, Floop or activated ability).
	Targets [][]TargetRef `json:"targets,omitempty"`
	// Labels is sent with a legal action, one per target slot, saying what
	// each choice is for. It is for display only and is ignored when played.
	Labels []string `json:"labels,omitempty"`
	// SkipLabels is sent with a legal action too, one per target slot: what leaving that optional slot empty does
	// ("Don't move a Building"), or "" for a slot that has no such wording. Display only.
	SkipLabels []string `json:"skip_labels,omitempty"`
	// Groups is sent with a legal action too, one per target slot: the Group of slots that share one target between
	// them ("mode": heal one Creature or damage one), or "" for a slot of its own. A client may ask for the target first
	// and then which of the group's slots it goes in. Display only.
	Groups []string `json:"groups,omitempty"`
	// Piles is sent with a legal action that takes a card from a discard pile of either player's choosing: the seats
	// whose piles it may take from, so a client can show both even when one is empty. Display only.
	Piles []int `json:"piles,omitempty"`
}

// TargetRef names one chosen target. Kind is the TargetSpec "what":
// creature, building, landscape and lane use Player and Lane; discard_card
// uses Player and Card (the UID); player uses Player alone.
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
	EvBurn      EventKind = "burn"      // a burn token went onto the Creature Card in Lane, or onto Player when Card is 0
	EvFloop     EventKind = "floop"     // a card Flooped
	EvActivate  EventKind = "activate"  // an activated ability was used
	EvDiscard   EventKind = "discard"   // a card was discarded from hand
	EvSteal     EventKind = "steal"     // a card was taken from the opponent's hand
	EvAbilities EventKind = "abilities" // a Creature's abilities changed (Lane, Card)
	EvReveal    EventKind = "reveal"    // a card in hand was shown (Card)
	EvFreeze    EventKind = "freeze"    // a Landscape got a frozen token (Player, Lane)
	EvThaw      EventKind = "thaw"      // a Landscape lost its frozen token (Player, Lane)
	EvUnder     EventKind = "under"     // a Creature (Card) went underneath the Camel in Lane (Player is its owner)
	EvRemove    EventKind = "remove"    // a card (Card, owned by Player) was removed from the game
	EvRetype    EventKind = "retype"    // a Landscape changed type (Player, Lane; Amount is 1 + its index in card.Landscapes)
)

// Event describes one state change. Fields are used as relevant to the kind.
type Event struct {
	Kind   EventKind `json:"kind"`
	Player PlayerID  `json:"player"`
	Card   int       `json:"card,omitempty"` // UID
	Lane   int       `json:"lane,omitempty"`
	Amount int       `json:"amount,omitempty"`
	// Cause says what a damage or HP loss came from when that is not a Fight or a card's effect: "burn" for the
	// damage burning does at the start of a turn.
	Cause string `json:"cause,omitempty"`
}

// Rule violations returned by Apply. Each is a *problem.Problem: its code is stable and the client words it.
var (
	ErrGameOver       = problem.New("game_over", "game is over")
	ErrWrongPhase     = problem.New("wrong_phase", "action not allowed in this phase")
	ErrNotYourTurn    = problem.New("not_your_turn", "not your turn")
	ErrAlreadyDecided = problem.New("already_decided", "mulligan decision already made")
	ErrNoMulligan     = problem.New("no_mulligan", "mulligan already used")
	ErrUnknownAction  = problem.New("unknown_action", "unknown action")
	ErrNoSuchCard     = problem.New("no_such_card", "card not in hand")
	ErrBadLane        = problem.New("bad_lane", "lane out of range")
	ErrNoActions      = problem.New("no_actions", "not enough Actions")
	ErrLandscapes     = problem.New("landscapes", "not enough Landscapes of the required type")
	ErrFlooped        = problem.New("flooped", "cannot replace a Flooped card")
	ErrDeckEmpty      = problem.New("deck_empty", "deck is empty")
	ErrNoAttacker     = problem.New("no_attacker", "no Ready Creature in that Lane")
	ErrFirstTurn      = problem.New("first_turn", "the first player may not Floop or Fight on the first turn")
	ErrNotPlayable    = problem.New("not_playable", "card cannot be played")
	ErrCannotAttack   = problem.New("cannot_attack", "that Creature cannot be attacked")
	ErrNoTarget       = problem.New("no_target", "no legal target")
	ErrNotReady       = problem.New("not_ready", "that card is not Ready")
	ErrNoAbility      = problem.New("no_ability", "that card has no such ability")
	ErrPending        = problem.New("pending", "the stolen card must be played first")
	ErrNoPending      = problem.New("no_pending", "there is no stolen card to play")
	ErrBadTarget      = problem.New("bad_target", "invalid target choice")
	ErrCondition      = problem.New("condition", "the card's condition is not met")
	ErrChoice         = problem.New("choice", "a decision must be made first")
	ErrNoChoice       = problem.New("no_choice", "there is no decision to make")
	ErrLimit          = problem.New("limit", "that ability has been used as often as it can be this turn")
	ErrNoFloop        = problem.New("no_floop", "Creatures in this Lane cannot use Floop abilities")
	ErrFrozen         = problem.New("frozen", "a Creature cannot be played onto a frozen Landscape, or replace one there")
	ErrNotFrozen      = problem.New("not_frozen", "that Landscape has no frozen token")
	ErrNoReplace      = problem.New("no_replace", "that Creature cannot be replaced")
	ErrInert          = problem.New("inert", "a Creature on a frozen Landscape here cannot Fight or use abilities")
)
