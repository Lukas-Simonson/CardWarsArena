package stats

import (
	"cardwars/internal/card"
	"cardwars/internal/engine"
)

// The names of the counters, which is also the keys of /api/stats. Adding a stat is one more name here and one more
// line where it is counted.
const (
	GamesStarted       = "games_started"   // + ".bot" or ".person"
	GamesCompleted     = "games_completed" // + ".bot" or ".person": reached a winner
	GamesAbandoned     = "games_abandoned" // a player left, or the room closed, before a winner
	BotWins            = "bot_wins"        // games against the bot that the bot won; + "." and the bot's name
	PlayerWins         = "player_wins"     // games against the bot that the player won
	CardsPlayed        = "cards_played"    // + ".creature", ".spell" or ".building"
	CardPlayed         = "card_played"     // + "." and a card id: the times that card was played
	Floops             = "floops"
	Fights             = "fights"
	DamageDealt        = "damage_dealt" // to Creatures and to players
	CreaturesDestroyed = "creatures_destroyed"
	HeroAbilities      = "hero_abilities"
	DeckStarted        = "deck_started" // + "." and the starter deck's Hero, or ".custom"
	DeckWon            = "deck_won"
)

// Events counts what happened in one move, from the engine's own events (so a client cannot inflate them).
func Events(c *Counters, g *engine.Game, cat card.Catalog, events []engine.Event) {
	if c == nil {
		return
	}
	ids := g.Instances()
	typeOf := func(uid int) card.Type {
		if cd := cat[ids[uid]]; cd != nil {
			return cd.Type
		}
		return ""
	}
	for _, e := range events {
		switch e.Kind {
		case engine.EvPlay:
			switch t := typeOf(e.Card); t {
			case card.Creature, card.Spell, card.Building:
				c.Add(CardsPlayed+"."+string(t), 1)
				c.Add(CardPlayed+"."+ids[e.Card], 1)
			}
		case engine.EvFloop:
			c.Add(Floops, 1)
			if typeOf(e.Card) == card.Hero {
				c.Add(HeroAbilities, 1)
			}
		case engine.EvActivate:
			if typeOf(e.Card) == card.Hero {
				c.Add(HeroAbilities, 1)
			}
		case engine.EvAttack:
			c.Add(Fights, 1)
		case engine.EvDamage, engine.EvHPLoss:
			c.Add(DamageDealt, int64(e.Amount))
		case engine.EvDie:
			c.Add(CreaturesDestroyed, 1)
		}
	}
}

// DeckBucket names a deck for the per-deck counts: a starter deck by its Hero, anything a player built "custom".
// Nothing about a particular player's deck is recorded.
func DeckBucket(set, hero, key string) string {
	if set == "" {
		return "custom"
	}
	if hero != "" {
		return hero
	}
	return key
}
