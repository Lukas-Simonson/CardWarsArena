package card

import "fmt"

// Effect is one rule of a card. See docs/effects.md for the model.
type Effect struct {
	Kind EffectKind `json:"kind"`

	// static
	Mods     []Mod  `json:"mods,omitempty"`
	Restrict string `json:"restrict,omitempty"`

	// trigger
	On string `json:"on,omitempty"`

	// activate: Pay Cost Actions >>> ...
	Cost int `json:"cost,omitempty"`

	// floop, activate, spell, trigger: what is chosen and what happens.
	Targets []TargetSpec `json:"targets,omitempty"`
	Do      []Op         `json:"do,omitempty"`
}

// EffectKind says when an Effect applies.
type EffectKind string

const (
	KindStatic   EffectKind = "static"
	KindTrigger  EffectKind = "trigger"
	KindFloop    EffectKind = "floop"
	KindActivate EffectKind = "activate"
	KindSpell    EffectKind = "spell"
	// KindAdditionalCost is paid when the card is played: its targets are
	// chosen with the action and its ops (discard) run before the card does.
	KindAdditionalCost EffectKind = "additional_cost"
)

// Trigger events (Effect.On).
const (
	OnEnterPlay   = "enter_play"
	OnLeavePlay   = "leave_play"
	OnTurnStart   = "turn_start"
	OnChangeLanes = "change_lanes"
)

// Restrictions (Effect.Restrict).
const RestrictAdjacentAlliesNotAttackable = "adjacent_allies_not_attackable"

// Mod adjusts the ATK or DEF of every creature matching To.
type Mod struct {
	Stat   string   `json:"stat"` // "atk" or "def"
	To     Selector `json:"to"`
	Amount Value    `json:"amount"`
	If     *Cond    `json:"if,omitempty"`
}

// Value is a constant, or N times a Count.
type Value struct {
	N   int    `json:"n"`
	Per *Count `json:"per,omitempty"`
}

// Count things, always from the point of view of the effect's controller.
type Count struct {
	Of       string    `json:"of"`
	Scope    string    `json:"scope,omitempty"` // "you" (default) or "any"
	Type     Landscape `json:"type,omitempty"`
	Adjacent bool      `json:"adjacent,omitempty"`
	Other    bool      `json:"other,omitempty"`
}

// Selector picks creatures relative to the effect's source.
type Selector struct {
	Rel          string    `json:"rel"`
	Landscape    Landscape `json:"landscape,omitempty"` // the creature card's own type
	ChangedLanes bool      `json:"changed_lanes,omitempty"`
}

// Cond is an extra requirement. All fields that are set must hold.
type Cond struct {
	OpposingCreatureInLane *bool          `json:"opposing_creature_in_lane,omitempty"`
	BuildingInLane         *bool          `json:"building_in_lane,omitempty"`
	Landscapes             *LandscapeCond `json:"landscapes,omitempty"`
}

// LandscapeCond bounds how many face-up Landscapes of a type you control.
type LandscapeCond struct {
	Type Landscape `json:"type"`
	Min  *int      `json:"min,omitempty"`
	Max  *int      `json:"max,omitempty"`
}

// TargetSpec describes one target slot.
type TargetSpec struct {
	What         string    `json:"what"`           // creature, building, landscape, lane, discard_card
	Side         string    `json:"side,omitempty"` // you, opponent, any (default)
	InLane       bool      `json:"in_lane,omitempty"`
	AdjacentLane bool      `json:"adjacent_lane,omitempty"`
	Landscape    Landscape `json:"landscape,omitempty"`
	FaceDown     *bool     `json:"face_down,omitempty"`
	Without      string    `json:"without,omitempty"`   // lanes: "creature" or "building"
	CardType     Type      `json:"card_type,omitempty"` // discard_card
	Flooped      *bool     `json:"flooped,omitempty"`   // creatures
	Optional     bool      `json:"optional,omitempty"`
	Count        *Value    `json:"count,omitempty"` // up to this many distinct targets
}

// Op is one step of an effect.
type Op struct {
	Do     string      `json:"do"`
	If     *Cond       `json:"if,omitempty"`
	Slot   *int        `json:"slot,omitempty"`  // target slot acted on
	Slot2  *int        `json:"slot2,omitempty"` // second slot (destination lane etc.)
	All    *Selector   `json:"all,omitempty"`   // act on every matching creature instead
	Who    string      `json:"who,omitempty"`   // you, opponent, each
	Stat   string      `json:"stat,omitempty"`
	Face   string      `json:"face,omitempty"` // up, down
	Amount *Value      `json:"amount,omitempty"`
	Random bool        `json:"random,omitempty"`
	Filter *CardFilter `json:"filter,omitempty"` // return_from_discard
}

// CardFilter narrows cards in a pile.
type CardFilter struct {
	Landscape Landscape `json:"landscape,omitempty"`
	Type      Type      `json:"type,omitempty"`
}

var validOps = map[string]struct{ slots int }{
	"draw": {0}, "damage_creature": {0}, "damage_hero": {0}, "mod_creature": {0},
	"move_creature": {2}, "move_building": {2}, "flip_landscape": {1},
	"ready_creature": {1}, "bounce": {1}, "destroy_building": {1},
	"gain_actions": {0}, "copy_floop": {0}, "return_from_discard": {0}, "put_on_deck": {1}, "discard": {1},
}

// Validate checks that an effect is well formed for a card of the given type.
func (e *Effect) Validate(t Type) error {
	switch e.Kind {
	case KindStatic:
		if len(e.Mods) == 0 && e.Restrict == "" {
			return fmt.Errorf("static effect has no mods or restrict")
		}
		if e.Restrict != "" && e.Restrict != RestrictAdjacentAlliesNotAttackable {
			return fmt.Errorf("unknown restrict %q", e.Restrict)
		}
		for _, m := range e.Mods {
			if m.Stat != "atk" && m.Stat != "def" {
				return fmt.Errorf("mod stat %q", m.Stat)
			}
			switch m.To.Rel {
			case "self", "lane_ally", "adjacent_allies", "your", "opposing", "any":
			default:
				return fmt.Errorf("mod selector rel %q", m.To.Rel)
			}
			if err := m.Amount.validate(); err != nil {
				return err
			}
		}
		if t != Creature && t != Building {
			return fmt.Errorf("static effects belong to creatures and buildings")
		}
	case KindTrigger:
		switch e.On {
		case OnEnterPlay, OnLeavePlay, OnTurnStart, OnChangeLanes:
		default:
			return fmt.Errorf("unknown trigger %q", e.On)
		}
		if t != Creature && t != Building {
			return fmt.Errorf("triggers belong to creatures and buildings")
		}
		if len(e.Targets) > 0 && e.On != OnEnterPlay {
			return fmt.Errorf("only enter_play triggers can have targets")
		}
	case KindFloop:
		if t != Creature && t != Building {
			return fmt.Errorf("floop belongs to creatures and buildings")
		}
	case KindActivate:
		if e.Cost < 1 {
			return fmt.Errorf("activate needs a cost of at least 1 Action")
		}
	case KindSpell:
		if t != Spell {
			return fmt.Errorf("spell effect on a %s", t)
		}
	case KindAdditionalCost:
		if t != Creature && t != Building && t != Spell {
			return fmt.Errorf("additional costs belong to cards that are played")
		}
		for i, op := range e.Do {
			if op.Do != "discard" {
				return fmt.Errorf("additional cost op %d: only discard is supported, got %q", i, op.Do)
			}
		}
	default:
		return fmt.Errorf("unknown effect kind %q", e.Kind)
	}
	for i, ts := range e.Targets {
		switch ts.What {
		case "creature", "building", "landscape", "lane", "discard_card", "hand_card":
		default:
			return fmt.Errorf("target %d: unknown what %q", i, ts.What)
		}
		switch ts.Side {
		case "", "you", "opponent", "any":
		default:
			return fmt.Errorf("target %d: unknown side %q", i, ts.Side)
		}
	}
	for i, op := range e.Do {
		info, ok := validOps[op.Do]
		if !ok {
			return fmt.Errorf("op %d: unknown op %q", i, op.Do)
		}
		for _, s := range []*int{op.Slot, op.Slot2} {
			if s != nil && (*s < 0 || *s >= len(e.Targets)) {
				return fmt.Errorf("op %d (%s): slot %d out of range", i, op.Do, *s)
			}
		}
		if info.slots >= 1 && op.Slot == nil {
			return fmt.Errorf("op %d (%s): needs slot", i, op.Do)
		}
		if info.slots == 2 && op.Slot2 == nil {
			return fmt.Errorf("op %d (%s): needs slot2", i, op.Do)
		}
		if op.Amount != nil {
			if err := op.Amount.validate(); err != nil {
				return fmt.Errorf("op %d: %w", i, err)
			}
		}
	}
	return nil
}

func (v Value) validate() error {
	if v.Per == nil {
		return nil
	}
	switch v.Per.Of {
	case "landscapes", "landscape_types", "creatures", "flooped_creatures", "spells_played", "opponent_hand":
		return nil
	}
	return fmt.Errorf("unknown count %q", v.Per.Of)
}
