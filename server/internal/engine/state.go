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
	// Owner is the player whose deck the card came from. Discarded or
	// returned cards always go to their owner, even when someone else
	// controls them (a Creature controlled by the player whose side it is on).
	Owner PlayerID `json:"owner"`
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
	// Silenced means the Creature has lost all its printed abilities this
	// turn. If Lent is set it instead has only the FLOOP ability of that
	// card (Schoolhouse).
	Silenced bool   `json:"silenced,omitempty"`
	Lent     string `json:"lent,omitempty"`
	// PermATK and PermDEF are bonuses that last while the Creature is in play.
	PermATK int `json:"perm_atk,omitempty"`
	PermDEF int `json:"perm_def,omitempty"`
	// Borrowed Creatures go back to their owner's side at the end of the turn
	// (Spirit Tower).
	Borrowed bool `json:"borrowed,omitempty"`
	// Offered holds the optional abilities its triggers offer right now.
	Offered []Offer `json:"offered,omitempty"`
	// Replaced is set while the Creature has just replaced another on entering
	// play, for "if X replaces a Creature" triggers.
	Replaced bool `json:"replaced,omitempty"`
	// Shielded Creatures cannot be targeted by the opponent or attacked until
	// their controller's next turn starts (Cave of Solitude).
	Shielded bool `json:"shielded,omitempty"`
	// Rainbow means the Creature has lost its Landscape type and is a Rainbow
	// Creature until the end of the turn (Puppy Parade).
	Rainbow bool `json:"rainbow,omitempty"`
	// Fired lists the once-per-turn triggers and the uses of limited
	// abilities this turn, by the index of the effect.
	Fired []int `json:"fired,omitempty"`
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
	// Frozen means the Landscape has a frozen token (at most one). It keeps its type, and stays when the
	// Landscape is turned over. No Creature may be played onto it, and a Creature on it cannot be replaced.
	Frozen   bool      `json:"frozen,omitempty"`
	Creature *Creature `json:"creature,omitempty"`
	Building *Building `json:"building,omitempty"`
	// Extra holds the Buildings played here besides Building while a Creature
	// allowed it (Burly Lumberjack). They stay if that Creature leaves.
	Extra []*Building `json:"extra,omitempty"`
	// ReplaceDiscount is how many Actions replacing the Creature here costs
	// less this turn (Pyramidia).
	ReplaceDiscount int `json:"replace_discount,omitempty"`
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
	// Hero is the Hero card that came with the deck; its rules apply all game
	// (zero when a game is made without one). HeroFired are the once-per-turn
	// triggers and limited abilities it has used this turn, by effect index.
	Hero      Instance `json:"hero"`
	HeroFired []int    `json:"hero_fired,omitempty"`
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
	// Pending, when set, is a decision the active player must make before
	// anything else can happen.
	Pending *Pending `json:"pending,omitempty"`
	// SpellsThisTurn counts Spells the active player has played this turn.
	SpellsThisTurn int `json:"spells_this_turn"`
	// Entered counts the Creatures that entered play this turn, per side and Lane.
	Entered [2][NumLanes]int `json:"entered"`
	// CreatureDiscount is how many Actions the next Creature the active
	// player plays costs less (Unempty Coffin).
	CreatureDiscount int `json:"creature_discount,omitempty"`
	// NoCombatDamage[p] means p's Creatures take no damage from opposing
	// Creatures this turn (Falling Star).
	NoCombatDamage [2]bool `json:"no_combat_damage,omitempty"`
	// Played lists the cards the active player has played this turn, for
	// "if you played a Rainbow card" and "for each Dog or Puppy" rules.
	Played []PlayedCard `json:"played,omitempty"`
	// Drawn counts the cards each player has drawn this turn.
	Drawn [2]int `json:"drawn,omitempty"`
	// Returned counts the Creatures each player returned to their hand this
	// turn, from play or from their discard pile (Lady Rainicorn).
	Returned [2]int `json:"returned,omitempty"`
	// RainbowHand holds the UIDs of hand cards that are Rainbow cards until the
	// end of the turn (Woadic Weirdo).
	RainbowHand []int `json:"rainbow_hand,omitempty"`
	// Choice is a decision someone must make before anything else happens;
	// Queue holds those that arose while it was open.
	Choice *Choice  `json:"choice,omitempty"`
	Queue  []Choice `json:"queue,omitempty"`
}

// Pending is a card taken from the opponent that must be played for free
// (Silo of Truth). The card is not in any zone while it waits.
type Pending struct {
	Card Instance `json:"card"`
}

// Choice kinds.
const (
	ChoiceDiscard = "discard" // a player picks cards from their hand to discard
	ChoiceReveal  = "reveal"  // a player picks one of the revealed cards; the rest are discarded
	ChoiceUpkeep  = "upkeep"  // discard cards, or (Card 0) destroy the Creature that asks (Steakchop)
	// search: pick a Building out of your deck to put into play (Deforestation)
	ChoiceSearch = "search"
	// rebound: discard a card to return the Creature that asks to your hand, or (Card 0) not (Impossible Possum)
	ChoiceRebound = "rebound"
	// redirect: pick the Flooped Creature that fights instead, or (Card 0) the one that attacked (Pied Piper)
	ChoiceRedirect = "redirect"
	// move_damage: pick how much damage to move from one of your Creatures to another (Friendship Bracelet)
	ChoiceMoveDamage = "move_damage"
	// move: pick the empty Landscape a Creature moves to at the start of the turn (Drained Cleric)
	ChoiceMove = "move"
)

// PlayedCard is a card played this turn, as it was when it was played.
type PlayedCard struct {
	CardID  string `json:"card"`
	Rainbow bool   `json:"rainbow,omitempty"` // a Rainbow card at the time, however it got to be
}

// Choice is a decision that stops the game until a player makes it. It is
// the general form of Pending: the player who must decide need not be the
// active one (an opponent chooses what to discard).
type Choice struct {
	Player PlayerID `json:"player"` // who decides
	Kind   string   `json:"kind"`
	Card   string   `json:"card"` // the card that asks
	// discard: cards still to discard.
	Count int `json:"count,omitempty"`
	// upkeep: where the Creature that asks is. It becomes a discard Choice
	// once the player starts discarding.
	Lane      int `json:"lane,omitempty"`
	SourceUID int `json:"source_uid,omitempty"`
	// Other is a second Lane: where search puts the Building is Lane; for
	// move_damage Lane is the Creature the damage leaves and Other the one it goes to.
	Other int `json:"other,omitempty"`
	// Bonus, on a discard Choice, is the Actions gained if a Rainbow card is discarded.
	Bonus int `json:"bonus,omitempty"`
	// move: the Lanes (all on MoveSide) the Creature in Lane may move to.
	Lanes    []int    `json:"lanes,omitempty"`
	MoveSide PlayerID `json:"move_side,omitempty"`
	// reveal: the cards shown, out of the deck until the choice is made. The
	// chosen one goes to the bottom of the deck and the rest are discarded.
	Revealed []Instance `json:"revealed,omitempty"`
}

// Offer is an optional ability a trigger has put on a Creature ("you may",
// Ms. Mummy). It appears as an activated ability of that Creature, at no cost,
// until its controller does anything else.
type Offer struct {
	Targets []card.TargetSpec `json:"targets,omitempty"`
	Then    []card.Op         `json:"then,omitempty"`
}
