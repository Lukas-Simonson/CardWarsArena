// Package bot chooses actions for a computer-controlled player. It is a
// weighted-random player: it follows the rulebook's mulligan advice, prefers
// developing its board and Fighting over stalling, and otherwise picks any
// legal action. It only ever returns actions the engine reports as legal.
package bot

import (
	"fmt"
	"math/rand/v2"
	"sort"
	"strings"

	"cardwars/internal/card"
	"cardwars/internal/engine"
)

// Bot chooses actions for a computer-controlled player. Every Bot returns only
// actions the engine reports as legal, and false when the player has none.
type Bot interface {
	Choose(g *engine.Game, p engine.PlayerID, rng *rand.Rand) (engine.Action, bool)
}

// Default is the bot used when none is asked for.
const Default = "random"

var registry = map[string]Bot{
	"random":    Random{},
	"heuristic": Heuristic{},
}

// Names lists the available bots, the default first.
func Names() []string {
	out := []string{Default}
	for name := range registry {
		if name != Default {
			out = append(out, name)
		}
	}
	sort.Strings(out[1:])
	return out
}

// New returns the bot with this name; "" is the default.
func New(name string) (Bot, error) {
	if name == "" {
		name = Default
	}
	b, ok := registry[name]
	if !ok {
		return nil, fmt.Errorf("unknown bot %q (have %s)", name, strings.Join(Names(), ", "))
	}
	return b, nil
}

// Choose plays as the default bot.
func Choose(g *engine.Game, p engine.PlayerID, rng *rand.Rand) (engine.Action, bool) {
	return Random{}.Choose(g, p, rng)
}

// Random is a weighted-random player.
type Random struct{}

// Choose returns the action player p should take, or false if p has none.
func (Random) Choose(g *engine.Game, p engine.PlayerID, rng *rand.Rand) (engine.Action, bool) {
	legal := g.LegalActions(p)
	if len(legal) == 0 {
		return engine.Action{}, false
	}
	if g.State.Phase == engine.PhaseMulligan {
		return chooseMulligan(g, p, legal), true
	}

	// Working actions: anything but ending the phase or drawing.
	busy := 0
	for _, a := range legal {
		if a.Kind != engine.ActEndMain && a.Kind != engine.ActDraw {
			busy++
		}
	}
	weights := make([]float64, len(legal))
	total := 0.0
	for i, a := range legal {
		w := weight(g, p, a, busy > 0)
		weights[i] = w
		total += w
	}
	x := rng.Float64() * total
	for i, w := range weights {
		if x < w {
			return legal[i], true
		}
		x -= w
	}
	return legal[len(legal)-1], true
}

// chooseMulligan follows the rulebook: with fewer than two Creatures in a
// hand of five, consider a mulligan (once).
func chooseMulligan(g *engine.Game, p engine.PlayerID, legal []engine.Action) engine.Action {
	creatures := 0
	for _, in := range g.State.Players[p].Hand {
		if g.Card(in).Type == card.Creature {
			creatures++
		}
	}
	want := engine.ActKeepHand
	if creatures < 2 {
		want = engine.ActMulligan
	}
	for _, a := range legal {
		if a.Kind == want {
			return a
		}
	}
	return legal[0]
}

func weight(g *engine.Game, p engine.PlayerID, a engine.Action, busy bool) float64 {
	switch a.Kind {
	case engine.ActPlayCard:
		if hand := g.State.Players[p].Hand; hand != nil {
			for _, in := range hand {
				if in.UID == a.Card && g.Card(in).Type == card.Spell {
					return 3
				}
			}
		}
		return 5
	case engine.ActFloop:
		return 3
	case engine.ActActivate:
		return 1.5
	case engine.ActAttack, engine.ActPlayPending:
		return 2
	case engine.ActDraw:
		if !busy || len(g.State.Players[p].Hand) < 3 {
			return 1
		}
		return 0.2
	case engine.ActEndMain:
		if busy {
			return 0.15 // do something useful first, but do not loop forever
		}
		return 1
	}
	return 1
}
