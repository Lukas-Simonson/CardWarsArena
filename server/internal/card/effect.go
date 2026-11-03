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

	// trigger on creature_enters: which entering Creatures it watches.
	Watch *Watch `json:"watch,omitempty"`
	// trigger: fire at most once each turn (Wall of Sand).
	OncePerTurn bool `json:"once_per_turn,omitempty"`

	// floop, activate: only usable while this holds. play_condition: the card
	// may only be played while this holds.
	If *Cond `json:"if,omitempty"`

	// cost_mod: the card costs Reduce less to play (only the Actions paid, if
	// ActionsOnly, otherwise the Landscapes needed too).
	Reduce      *Value `json:"reduce,omitempty"`
	ActionsOnly bool   `json:"actions_only,omitempty"`

	// activate: how many times each turn it may be used (Dogboy: once).
	Limit int `json:"limit,omitempty"`

	// static with restrict counts_as_landscape: the Landscape type it adds.
	As Landscape `json:"as,omitempty"`

	// static, on a Hero: every Creature matching Grant.To has Grant.Ability as well (Finn).
	Grant *Grant `json:"grant,omitempty"`

	// floop, activate, spell, trigger: what is chosen and what happens.
	Targets []TargetSpec `json:"targets,omitempty"`
	Do      []Op         `json:"do,omitempty"`
}

// Grant gives an ability to the Creatures a Selector picks.
type Grant struct {
	To      Selector `json:"to"`
	Ability Effect   `json:"ability"`
}

// Watch says which Creatures entering play under the watcher's control fire
// a creature_enters trigger.
type Watch struct {
	Landscape Landscape `json:"landscape,omitempty"` // the entering card's own type
	Other     bool      `json:"other,omitempty"`     // not the watcher itself
	YourTurn  bool      `json:"your_turn,omitempty"` // only during the controller's turn
}

// EffectKind says when an Effect applies.
type EffectKind string

const (
	KindStatic   EffectKind = "static"
	KindTrigger  EffectKind = "trigger"
	KindFloop    EffectKind = "floop"
	KindActivate EffectKind = "activate"
	KindSpell    EffectKind = "spell"
	// KindPlayCondition and KindCostMod apply to the card while it is in hand.
	KindPlayCondition EffectKind = "play_condition"
	KindCostMod       EffectKind = "cost_mod"
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
	// OnCreatureEnters fires when a Creature enters play under the card's
	// controller's control (the card's Watch narrows which).
	OnCreatureEnters = "creature_enters"
	// OnHeroDamage fires on a Creature that has just dealt damage to the
	// opposing player; Count "damage_dealt" is how much (Snakemint).
	OnHeroDamage = "deals_hero_damage"
	// OnDealsDamage fires on a Creature that has just dealt damage in a Fight,
	// to a Creature or a player; Count "damage_dealt" is how much (Vampire Lord).
	OnDealsDamage = "deals_damage"
	// OnAttacks fires on a Creature that has just been Activated to Fight,
	// before any damage is dealt (Pied Piper, Emboldened Retriever).
	OnAttacks = "attacks"
	// OnDiscarded fires on a Creature that has just gone from play into a
	// discard pile (not back to a hand): Impossible Possum.
	OnDiscarded = "discarded"
	// OnReplaces fires on a Hero when its controller replaces a Creature on their turn (LSP).
	OnReplaces = "replaces"
	// OnBuildingPlayed fires on a Hero when its controller puts a Building into play (Cake).
	OnBuildingPlayed = "building_played"
	// OnReturns fires on a Hero when its controller returns a Creature to their hand from play
	// or from their discard pile on their turn; cond `returned` counts them (Lady Rainicorn).
	OnReturns = "returns"
)

// Restrictions (Effect.Restrict).
const (
	RestrictAdjacentAlliesNotAttackable = "adjacent_allies_not_attackable"
	// Opposing Creatures in the Lane do not trigger entering or leaving play effects (Palace of Bone).
	RestrictOpposingNoEnterLeave = "opposing_no_enter_leave_triggers"
	// Every Creature in the Lane is Rainbow, without its own Landscape type (Charming City).
	RestrictLaneRainbow = "lane_rainbow"
	// The card is also a Rainbow Creature (Tiny Elephant).
	RestrictAlsoRainbow = "also_rainbow"
	// Creatures in the Lane, on both sides, cannot use Floop abilities (Woadic Enchantress).
	RestrictLaneNoFloop = "lane_no_floop"
	// More Buildings may be played in the Lane without replacing the one there (Burly Lumberjack).
	RestrictExtraBuildings = "extra_buildings"
	// Opposing Buildings in the Lane have no abilities while the Creature is Flooped (The Dog).
	RestrictSilenceOpposingBuildings = "silence_opposing_buildings"
	// The Building counts as one more face-up Landscape, of type As (Farmhouse).
	RestrictCountsAsLandscape = "counts_as_landscape"
	// The Building has the rules of the Buildings you control in adjacent Lanes (Learning Center).
	RestrictCopyAdjacentBuildings = "copy_adjacent_buildings"
)

var validRestricts = map[string]bool{
	RestrictAdjacentAlliesNotAttackable: true, RestrictOpposingNoEnterLeave: true, RestrictLaneRainbow: true,
	RestrictAlsoRainbow: true, RestrictLaneNoFloop: true, RestrictExtraBuildings: true,
	RestrictSilenceOpposingBuildings: true, RestrictCountsAsLandscape: true, RestrictCopyAdjacentBuildings: true,
}

// Mod adjusts the ATK or DEF of every creature matching To. The stat
// "damage_taken" instead raises each amount of damage they take (Blood Castle).
type Mod struct {
	Stat   string   `json:"stat"` // "atk", "def" or "damage_taken"
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
	Of   string `json:"of"`
	Div  int    `json:"div,omitempty"`  // count whole groups of this many (for every 5 cards)
	Card string `json:"card,omitempty"` // creatures: only this card
	// FromTarget counts from the point of view of each chosen Creature
	// instead of the card (Zazo's Magic Seeds).
	FromTarget bool   `json:"from_target,omitempty"`
	Scope      string `json:"scope,omitempty"` // "you" (default) or "any"
	// Words, for played_titled: a played card counts if its name has one of them.
	Words    []string  `json:"words,omitempty"`
	Type     Landscape `json:"type,omitempty"`
	Adjacent bool      `json:"adjacent,omitempty"`
	Other    bool      `json:"other,omitempty"`
}

// Selector picks creatures relative to the effect's source.
type Selector struct {
	Rel          string    `json:"rel"`
	Damage       *Range    `json:"damage,omitempty"`    // only Creatures with this much damage on them
	Landscape    Landscape `json:"landscape,omitempty"` // the creature card's own type
	ChangedLanes bool      `json:"changed_lanes,omitempty"`
	Flooped      *bool     `json:"flooped,omitempty"`       // only Flooped (or only not Flooped) Creatures
	WithBuilding bool      `json:"with_building,omitempty"` // only Creatures in a Lane where their controller has a Building
	FaceDown     *bool     `json:"face_down,omitempty"`     // only Creatures on a face-down (or face-up) Landscape
}

// Range bounds a number; an unset end is open.
type Range struct {
	Min *int `json:"min,omitempty"`
	Max *int `json:"max,omitempty"`
}

// Has reports whether n is in the range.
func (r Range) Has(n int) bool {
	return (r.Min == nil || n >= *r.Min) && (r.Max == nil || n <= *r.Max)
}

// Cond is an extra requirement. All fields that are set must hold.
type Cond struct {
	OpposingCreatureInLane *bool          `json:"opposing_creature_in_lane,omitempty"`
	BuildingInLane         *bool          `json:"building_in_lane,omitempty"`
	Landscapes             *LandscapeCond `json:"landscapes,omitempty"`
	OpponentHandMax        *int           `json:"opponent_hand_max,omitempty"` // the opponent holds at most this many cards
	DiscardMin             *int           `json:"discard_min,omitempty"`       // your discard pile holds at least this many
	Controls               Landscape      `json:"controls,omitempty"`          // you control a Creature of this type
	OnLandscape            Landscape      `json:"on_landscape,omitempty"`      // the card's Lane has this face-up Landscape
	EnteredMax             *int           `json:"entered_max,omitempty"`       // at most this many of your Creatures entered play this turn
	Damage                 *Range         `json:"damage,omitempty"`            // the card (a Creature) has this much damage on it
	Replaced               *bool          `json:"replaced,omitempty"`          // the card replaced a Creature when it entered play
	Flooped                *bool          `json:"flooped,omitempty"`           // the card (a Creature) is Flooped
	OtherCreatures         *Range         `json:"other_creatures,omitempty"`   // you control this many Creatures besides the card
	Played                 *PlayedCond    `json:"played,omitempty"`            // cards of a type you played this turn
	LandscapeTypes         *Range         `json:"landscape_types,omitempty"`   // you control this many face-up Landscape types
	Returned               *Range         `json:"returned,omitempty"`          // Creatures you returned to your hand this turn
}

// PlayedCond bounds how many cards of a Landscape type you played this turn.
type PlayedCond struct {
	Landscape Landscape `json:"landscape"`
	Min       *int      `json:"min,omitempty"`
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
	Cost         *int      `json:"cost,omitempty"`      // discard_card
	Flooped      *bool     `json:"flooped,omitempty"`   // creatures
	Ready        bool      `json:"ready,omitempty"`     // creatures: only Ready ones (not Flooped or Fought)
	Damaged      bool      `json:"damaged,omitempty"`   // creatures: only ones with damage on them
	Optional     bool      `json:"optional,omitempty"`
	Count        *Value    `json:"count,omitempty"` // up to this many distinct targets
	// Other leaves out the card's own Lane. LaneOf limits the choice to the
	// Lane of what was chosen for that earlier slot (Volcano). Specs sharing a
	// Group need exactly one target between them (Green Candy: heal or damage).
	// Label says what the choice is for ("Heal"), shown while choosing.
	// Distinct names a set of slots whose choices may not overlap.
	Label    string `json:"label,omitempty"`
	Distinct string `json:"distinct,omitempty"`
	Other    bool   `json:"other,omitempty"`
	LaneOf   *int   `json:"lane_of,omitempty"`
	Group    string `json:"group,omitempty"`
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
	// Self acts on the card itself; Entering on the Creature that fired a
	// creature_enters trigger.
	Self     bool `json:"self,omitempty"`
	Entering bool `json:"entering,omitempty"`
	// Permanent makes mod_creature last as long as the Creature is in play.
	Permanent bool `json:"permanent,omitempty"`
	// RainbowActions, on discard_choice: Actions gained if a Rainbow card is
	// among those discarded (Fiddling Ferret).
	RainbowActions int `json:"rainbow_actions,omitempty"`
	// may offers the controller a free ability until they do something else:
	// to do Then, after choosing Targets (Ms. Mummy's "you may"). Slots in
	// Then refer to Targets.
	Targets []TargetSpec `json:"targets,omitempty"`
	Then    []Op         `json:"then,omitempty"`
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
	"gain_actions": {0}, "copy_floop": {0}, "steal_play": {0}, "return_from_discard": {0}, "put_on_deck": {1}, "discard": {1},
	"destroy_creature": {0}, "mill": {0}, "discount_next_creature": {0},
	"borrow_creature": {2}, "ogre_gas": {0}, "discard_choice": {0}, "may": {0},
	"heal_creature": {0}, "heal_hero": {0}, "swap_creatures": {1}, "protect": {0},
	"no_combat_damage": {0}, "discount_replace": {0}, "upkeep": {0},
	"exhaust": {1}, "play_building": {2}, "search_building": {1}, "move_damage": {2},
	"make_rainbow": {0}, "rainbow_hand": {1}, "rebound": {0}, "redirect_attack": {0},
}

// Validate checks that an effect is well formed for a card of the given type.
func (e *Effect) Validate(t Type) error {
	switch e.Kind {
	case KindStatic:
		if len(e.Mods) == 0 && e.Restrict == "" && e.Grant == nil {
			return fmt.Errorf("static effect has no mods, restrict or grant")
		}
		if e.Grant != nil {
			if t != Hero {
				return fmt.Errorf("only a Hero can grant an ability")
			}
			if e.Grant.Ability.Kind != KindFloop && e.Grant.Ability.Kind != KindActivate {
				return fmt.Errorf("a grant gives a floop or activate ability")
			}
			if err := e.Grant.Ability.Validate(Creature); err != nil {
				return fmt.Errorf("grant: %w", err)
			}
		}
		if e.Restrict != "" && !validRestricts[e.Restrict] {
			return fmt.Errorf("unknown restrict %q", e.Restrict)
		}
		if (e.Restrict == RestrictCountsAsLandscape) != (e.As != "") {
			return fmt.Errorf("as goes with counts_as_landscape, and only with it")
		}
		for _, m := range e.Mods {
			if m.Stat != "atk" && m.Stat != "def" && m.Stat != "damage_taken" {
				return fmt.Errorf("mod stat %q", m.Stat)
			}
			switch m.To.Rel {
			case "self", "lane_ally", "adjacent_allies", "your", "opposing", "any", "lane_foe":
			default:
				return fmt.Errorf("mod selector rel %q", m.To.Rel)
			}
			if err := m.Amount.validate(); err != nil {
				return err
			}
		}
		if t != Creature && t != Building && t != Hero {
			return fmt.Errorf("static effects belong to creatures, buildings and heroes")
		}
	case KindTrigger:
		switch e.On {
		case OnEnterPlay, OnLeavePlay, OnTurnStart, OnChangeLanes, OnCreatureEnters, OnHeroDamage,
			OnDealsDamage, OnAttacks, OnDiscarded:
		case OnReplaces, OnBuildingPlayed, OnReturns:
			if t != Hero {
				return fmt.Errorf("%s triggers belong to heroes", e.On)
			}
		default:
			return fmt.Errorf("unknown trigger %q", e.On)
		}
		if t != Creature && t != Building && t != Hero {
			return fmt.Errorf("triggers belong to creatures, buildings and heroes")
		}
		if len(e.Targets) > 0 && e.On != OnEnterPlay {
			return fmt.Errorf("only enter_play triggers can have targets")
		}
	case KindFloop:
		if t != Creature && t != Building {
			return fmt.Errorf("floop belongs to creatures and buildings")
		}
	case KindActivate:
		free := len(e.Do) > 0 && ((e.Do[0].Self && (e.Do[0].Do == "destroy_creature" || e.Do[0].Do == "bounce")) || e.Do[0].Do == "discard")
		if e.Cost < 1 && !free && t != Hero {
			return fmt.Errorf("activate needs a cost of at least 1 Action, or to remove or discard something first")
		}
	case KindPlayCondition:
		if e.If == nil {
			return fmt.Errorf("play_condition needs an if")
		}
		if t != Creature && t != Building && t != Spell {
			return fmt.Errorf("play conditions belong to cards that are played")
		}
	case KindCostMod:
		if e.Reduce == nil {
			return fmt.Errorf("cost_mod needs reduce")
		}
		if err := e.Reduce.validate(); err != nil {
			return err
		}
		if t != Creature && t != Building && t != Spell {
			return fmt.Errorf("cost mods belong to cards that are played")
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
			if op.Do != "discard" && op.Do != "exhaust" {
				return fmt.Errorf("additional cost op %d: only discard and exhaust are supported, got %q", i, op.Do)
			}
		}
	default:
		return fmt.Errorf("unknown effect kind %q", e.Kind)
	}
	if err := validateSpecs(e.Targets); err != nil {
		return err
	}
	return validateOps(e.Do, e.Targets)
}

func validateSpecs(specs []TargetSpec) error {
	for i, ts := range specs {
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
		if ts.LaneOf != nil && (*ts.LaneOf < 0 || *ts.LaneOf >= i) {
			return fmt.Errorf("target %d: lane_of must name an earlier slot", i)
		}
	}
	return nil
}

// validateOps checks ops whose slots refer to specs.
func validateOps(ops []Op, specs []TargetSpec) error {
	for i, op := range ops {
		info, ok := validOps[op.Do]
		if !ok {
			return fmt.Errorf("op %d: unknown op %q", i, op.Do)
		}
		for _, s := range []*int{op.Slot, op.Slot2} {
			if s != nil && (*s < 0 || *s >= len(specs)) {
				return fmt.Errorf("op %d (%s): slot %d out of range", i, op.Do, *s)
			}
		}
		if info.slots >= 1 && op.Slot == nil && !op.Self {
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
		if op.Do == "may" {
			if err := validateSpecs(op.Targets); err != nil {
				return fmt.Errorf("op %d: %w", i, err)
			}
			if err := validateOps(op.Then, op.Targets); err != nil {
				return fmt.Errorf("op %d: %w", i, err)
			}
		} else if len(op.Then) > 0 || len(op.Targets) > 0 {
			return fmt.Errorf("op %d (%s): then/targets belong to may", i, op.Do)
		}
	}
	return nil
}

func (v Value) validate() error {
	if v.Per == nil {
		return nil
	}
	switch v.Per.Of {
	case "landscapes", "landscape_types", "creatures", "flooped_creatures", "spells_played", "opponent_hand",
		"discard", "empty_lanes", "entered_this_turn", "entered_adjacent", "base_attack", "milled_cost",
		"milled_spells", "damage", "damage_dealt", "buildings", "played_titled", "drawn_this_turn":
		return nil
	}
	return fmt.Errorf("unknown count %q", v.Per.Of)
}
