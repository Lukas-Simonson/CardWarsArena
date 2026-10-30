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
	Deck   string `json:"deck,omitempty"`   // deck within the set, e.g. "Finn's Deck"
	Set    string `json:"set,omitempty"`    // product, e.g. "Finn Vs Jake"
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
	Notes     string      `json:"notes,omitempty"` // flavour text
	Printings []Printing  `json:"printings,omitempty"`
	ImageURL  string      `json:"image_url,omitempty"`

	// Effects and Modeled come from data/effects, not from the card files.
	// Modeled means someone has written down this card's rules (possibly none).
	// The engine must never guess rules from Text.
	Effects []Effect `json:"-"`
	Modeled bool     `json:"-"`
}

// Implemented reports whether the engine can play this card correctly: it has
// no rules text beyond its stats, or its rules have been modelled.
func (c *Card) Implemented() bool {
	return c.Text == "" || c.Modeled
}
