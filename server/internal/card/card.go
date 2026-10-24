// Package card defines the canonical card data model and loads it from data/cards.
package card

// Type is the kind of a card.
type Type string

const (
	Creature Type = "creature"
	Building Type = "building"
	Spell    Type = "spell"
	Hero     Type = "hero"
)

// Landscape is the landscape type a card requires. Heroes have none.
type Landscape string

const (
	Cornfield    Landscape = "cornfield"
	BluePlains   Landscape = "blue_plains"
	UselessSwamp Landscape = "useless_swamp"
	NiceLands    Landscape = "nice_lands"
	SandyLands   Landscape = "sandy_lands"
	IcyLands     Landscape = "icy_lands"
	Rainbow      Landscape = "rainbow"
)

// AbilityKind is the printed category of a card's ability, as recorded in the
// source data. It is a hint, not authoritative; see Card.Effects.
type AbilityKind string

const (
	AbilityNone         AbilityKind = ""
	AbilityContinuous   AbilityKind = "continuous"
	AbilityFloop        AbilityKind = "floop"
	AbilityEnteringPlay AbilityKind = "entering_play"
	AbilityLeavingPlay  AbilityKind = "leaving_play"
	AbilityTrigger      AbilityKind = "trigger"
	AbilityIgnition     AbilityKind = "ignition"
)

// Printing records one product a card appears in.
type Printing struct {
	Deck   string `json:"deck,omitempty"`   // product/deck name, e.g. "Finn's Blue Plains Deck"
	Set    string `json:"set,omitempty"`    // set code, e.g. "CP1"
	Copies int    `json:"copies,omitempty"` // copies in that starter deck
	Rarity string `json:"rarity,omitempty"` // booster/promo rarity, when printed
	Raw    string `json:"raw,omitempty"`    // original text, kept when it could not be parsed
}

// Card is one unique card.
type Card struct {
	ID        string      `json:"id"`
	Name      string      `json:"name"` // as printed, typos included
	Type      Type        `json:"type"`
	Landscape Landscape   `json:"landscape,omitempty"`
	Cost      *int        `json:"cost,omitempty"`
	Attack    *int        `json:"attack,omitempty"`
	Defense   *int        `json:"defense,omitempty"`
	Ability   AbilityKind `json:"ability,omitempty"`
	Text      string      `json:"text,omitempty"`
	Printings []Printing  `json:"printings,omitempty"`
	ImageURL  string      `json:"image_url,omitempty"`

	// Effects is the machine-readable rules for the card. Empty until the
	// effect model exists; the engine must not guess from Text.
	Effects []map[string]any `json:"effects,omitempty"`
}
