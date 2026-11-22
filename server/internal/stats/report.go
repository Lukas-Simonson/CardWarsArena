package stats

import "strings"

// Version is the version of the report's shape, sent as "v". It changes when a field is renamed or moved.
const Version = 2

// Report is what GET /api/stats answers: the counters, grouped by topic so the shape explains them. The totals have a
// fixed shape (a counter that has not happened yet is 0), while what grows with the game (per bot, per deck, per
// card) is sparse. Adding a stat is one more counter name in events.go and one place here.
type Report struct {
	V    int  `json:"v"`
	Kept bool `json:"kept"` // whether the totals survive a restart (the server has a database)

	Games struct {
		Started   Split `json:"started"`
		Completed Split `json:"completed"` // reached a winner
		Abandoned int64 `json:"abandoned"` // a player left, or the room closed, before a winner
	} `json:"games"`

	Results struct {
		PlayerWins int64 `json:"player_wins"` // games against the bot that the player won
		BotWins    struct {
			Total int64            `json:"total"`
			ByBot map[string]int64 `json:"by_bot"`
		} `json:"bot_wins"`
	} `json:"results"`

	Plays struct {
		Cards struct {
			Total  int64            `json:"total"`
			ByType map[string]int64 `json:"by_type"` // creature, spell, building
		} `json:"cards"`
		Floops        int64 `json:"floops"`
		Fights        int64 `json:"fights"`
		HeroAbilities int64 `json:"hero_abilities"`
	} `json:"plays"`

	Combat struct {
		DamageDealt        int64 `json:"damage_dealt"` // to Creatures and to players
		CreaturesDestroyed int64 `json:"creatures_destroyed"`
	} `json:"combat"`

	// Decks are by the starter deck's Hero, or "custom" for everything players built.
	Decks map[string]DeckResult `json:"decks"`

	// Cards is only there when asked for (?detail=cards): it grows with the card pool.
	Cards *CardDetail `json:"cards,omitempty"`
}

// Split is a count in all and by kind of game.
type Split struct {
	Total  int64 `json:"total"`
	Bot    int64 `json:"bot"`
	Person int64 `json:"person"`
}

// DeckResult is how often a deck was played and won.
type DeckResult struct {
	Started int64 `json:"started"`
	Won     int64 `json:"won"`
}

// CardDetail is the per-card counts.
type CardDetail struct {
	Played map[string]int64 `json:"played"` // by card id
}

// BuildReport groups the counters into a Report. withCards adds the per-card counts.
func BuildReport(totals map[string]int64, kept, withCards bool) Report {
	var r Report
	r.V, r.Kept = Version, kept
	r.Results.BotWins.ByBot = map[string]int64{}
	r.Plays.Cards.ByType = map[string]int64{"creature": 0, "spell": 0, "building": 0}
	r.Decks = map[string]DeckResult{}
	if withCards {
		r.Cards = &CardDetail{Played: map[string]int64{}}
	}
	for name, n := range totals {
		base, rest, _ := strings.Cut(name, ".")
		switch base {
		case GamesStarted:
			r.Games.Started.add(rest, n)
		case GamesCompleted:
			r.Games.Completed.add(rest, n)
		case GamesAbandoned:
			r.Games.Abandoned = n
		case PlayerWins:
			r.Results.PlayerWins = n
		case BotWins:
			if rest == "" {
				r.Results.BotWins.Total = n
			} else {
				r.Results.BotWins.ByBot[rest] = n
			}
		case CardsPlayed:
			r.Plays.Cards.ByType[rest] = n
			r.Plays.Cards.Total += n
		case CardPlayed:
			if r.Cards != nil {
				r.Cards.Played[rest] = n
			}
		case Floops:
			r.Plays.Floops = n
		case Fights:
			r.Plays.Fights = n
		case HeroAbilities:
			r.Plays.HeroAbilities = n
		case DamageDealt:
			r.Combat.DamageDealt = n
		case CreaturesDestroyed:
			r.Combat.CreaturesDestroyed = n
		case DeckStarted:
			d := r.Decks[rest]
			d.Started = n
			r.Decks[rest] = d
		case DeckWon:
			d := r.Decks[rest]
			d.Won = n
			r.Decks[rest] = d
		}
	}
	return r
}

// add puts n in the total, or in the kind of game the name ends with.
func (s *Split) add(kind string, n int64) {
	switch kind {
	case "":
		s.Total = n
	case "bot":
		s.Bot = n
	case "person":
		s.Person = n
	}
}
