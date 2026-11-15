package engine

import "cardwars/internal/card"

// pos is a place on one player's side of the board.
type pos struct {
	P    PlayerID
	Lane int
}

// source is a card in play whose effects may apply to other cards.
type source struct {
	pos
	creature *Creature // nil for Buildings and Heroes
	hero     bool      // the source is a Hero (its pos has Lane -1)
	effects  []card.Effect
}

// forEachSource visits every Creature and Building in play.
func (g *Game) forEachSource(fn func(source)) {
	for p := range g.State.Players {
		if g.State.Players[p].Hero.CardID != "" {
			fn(source{pos: pos{PlayerID(p), heroLane}, hero: true, effects: g.heroEffects(PlayerID(p))})
		}
		for i := range g.State.Players[p].Lanes {
			l := &g.State.Players[p].Lanes[i]
			at := pos{PlayerID(p), i}
			if l.Creature != nil {
				fn(source{pos: at, creature: l.Creature, effects: g.effectsOf(l.Creature)})
			}
			for _, b := range l.buildings() {
				fn(source{pos: at, effects: g.buildingEffects(at, b)})
			}
		}
	}
}

// effectsOf is the rules a Creature currently has. Normally that is its
// printed effects; a Silenced Creature has none, or only the Floop ability it
// was lent. Every read of a Creature's abilities goes through here.
func (g *Game) effectsOf(c *Creature) []card.Effect {
	if !c.Silenced {
		if granted := g.grantedTo(c); len(granted) > 0 {
			return append(append([]card.Effect(nil), g.Card(c.Instance).Effects...), granted...)
		}
		return g.Card(c.Instance).Effects
	}
	var out []card.Effect
	if lent := g.cat[c.Lent]; lent != nil {
		for _, e := range lent.Effects {
			if e.Kind == card.KindFloop {
				out = append(out, e)
			}
		}
	}
	return out
}

func (g *Game) creatureAt(at pos) *Creature { return g.State.Players[at.P].Lanes[at.Lane].Creature }

// atk and def are a Creature's current ATK and DEF: printed value, plus
// "this turn" modifiers, plus continuous effects from everything in play.
// Every read of a stat goes through here.
func (g *Game) atk(at pos) int {
	c := g.creatureAt(at)
	return max(0, baseStat(g.Card(c.Instance).Attack)+c.TempATK+c.PermATK+g.staticMod("atk", at))
}

func (g *Game) def(at pos) int {
	c := g.creatureAt(at)
	return max(0, baseStat(g.Card(c.Instance).Defense)+c.TempDEF+c.PermDEF+g.staticMod("def", at))
}

func baseStat(v *int) int {
	if v == nil {
		return 0
	}
	return *v
}

// staticMod sums every continuous modifier to stat that applies to the
// Creature at target.
func (g *Game) staticMod(stat string, target pos) int {
	t := g.creatureAt(target)
	total := 0
	g.forEachSource(func(s source) {
		for _, e := range s.effects {
			if e.Kind != card.KindStatic {
				continue
			}
			for _, m := range e.Mods {
				if m.Stat != stat || !g.selects(m.To, s, target, t) {
					continue
				}
				if m.If != nil && !g.cond(*m.If, s) {
					continue
				}
				total += g.value(m.Amount, s)
			}
		}
	})
	return total
}

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}

// selects reports whether the Creature t, at target, is picked by sel when
// sel is evaluated from the point of view of source s.
func (g *Game) selects(sel card.Selector, s source, target pos, t *Creature) bool {
	switch sel.Rel {
	case "self":
		if s.creature != t {
			return false
		}
	case "lane_ally":
		if target.P != s.P || target.Lane != s.Lane {
			return false
		}
	case "adjacent_allies":
		if target.P != s.P || abs(target.Lane-s.Lane) != 1 {
			return false
		}
	case "your":
		if target.P != s.P {
			return false
		}
	case "opposing":
		if target.P == s.P {
			return false
		}
	case "lane_foe":
		if target.P == s.P || target.Lane != s.Lane {
			return false
		}
	case "any":
	default:
		return false
	}
	if sel.Damage != nil && !sel.Damage.Has(t.Damage) {
		return false
	}
	if sel.Landscape != "" && !g.isType(target, sel.Landscape) {
		return false
	}
	if sel.Flooped != nil && (t.Status == Flooped) != *sel.Flooped {
		return false
	}
	if sel.WithBuilding && g.State.Players[target.P].Lanes[target.Lane].Building == nil {
		return false
	}
	if sel.FaceDown != nil && g.State.Players[target.P].Lanes[target.Lane].FaceDown != *sel.FaceDown {
		return false
	}
	if sel.ChangedLanes && !t.ChangedLanes {
		return false
	}
	if sel.Frozen != nil && g.State.Players[target.P].Lanes[target.Lane].Frozen != *sel.Frozen {
		return false
	}
	return true
}

// value evaluates a Value from the point of view of source s.
func (g *Game) value(v card.Value, s source) int {
	if v.Per == nil {
		return v.N
	}
	n := g.count(*v.Per, s)
	if v.Per.Div > 0 {
		n /= v.Per.Div
	}
	return v.N * n
}

// count evaluates a Count from the point of view of source s.
func (g *Game) count(c card.Count, s source) int {
	st := &g.State
	players := []PlayerID{s.P}
	if c.Scope == "any" {
		players = []PlayerID{0, 1}
	}
	switch c.Of {
	case "landscapes":
		n := 0
		for _, p := range players {
			for i, l := range st.Players[p].Lanes {
				if c.Adjacent && (p != s.P || abs(i-s.Lane) != 1) {
					continue
				}
				if !l.FaceDown && (c.Type == "" || l.Landscape == c.Type) {
					n++
				}
			}
			if !c.Adjacent {
				n += g.extraLandscapes(p, c.Type)
			}
		}
		return n
	case "landscape_types":
		seen := map[card.Landscape]bool{}
		for _, l := range st.Players[s.P].Lanes {
			if !l.FaceDown {
				seen[l.Landscape] = true
			}
		}
		for _, ls := range card.Landscapes {
			if g.extraLandscapes(s.P, ls) > 0 {
				seen[ls] = true
			}
		}
		return len(seen)
	case "creatures", "flooped_creatures":
		n := 0
		for _, p := range players {
			for i, l := range st.Players[p].Lanes {
				cr := l.Creature
				if cr == nil || (c.Other && cr == s.creature) {
					continue
				}
				if c.Type != "" && !g.isType(pos{p, i}, c.Type) {
					continue
				}
				if c.Card != "" && cr.CardID != c.Card {
					continue
				}
				if c.Of == "flooped_creatures" && cr.Status != Flooped {
					continue
				}
				n++
			}
		}
		return n
	case "buildings":
		n := 0
		for _, p := range players {
			n += g.buildingCount(p, c.Card)
		}
		return n
	case "played_titled":
		if s.P != st.Active {
			return 0
		}
		return g.playedTitled(c.Words)
	case "drawn_this_turn":
		return st.Drawn[s.P]
	case "spells_played":
		if s.P != st.Active {
			return 0
		}
		return st.SpellsThisTurn
	case "opponent_hand":
		return len(st.Players[s.P.Opp()].Hand)
	case "discard":
		return len(st.Players[s.P].Discard)
	case "frozen_landscapes":
		return g.frozenCount(players)
	case "exhausted_creatures":
		n := 0
		for _, p := range players {
			for _, l := range st.Players[p].Lanes {
				if cr := l.Creature; cr != nil && cr.Status != Ready && !(c.Other && cr == s.creature) {
					n++
				}
			}
		}
		return n
	case "destroyed":
		return g.destroyed
	case "face_down_landscapes":
		n := 0
		for _, p := range players {
			for _, l := range st.Players[p].Lanes {
				if l.FaceDown {
					n++
				}
			}
		}
		return n
	case "entered_lane":
		if s.Lane < 0 {
			return 0
		}
		return st.Entered[s.P][s.Lane]
	case "bounced_cost":
		return g.bouncedCost
	case "under":
		if s.creature == nil {
			return 0
		}
		return len(s.creature.Under)
	case "empty_lanes":
		n := 0
		for _, l := range st.Players[s.P].Lanes {
			if l.Creature == nil {
				n++
			}
		}
		return n
	case "entered_this_turn":
		n := 0
		for _, p := range players {
			for _, e := range st.Entered[p] {
				n += e
			}
		}
		return n
	case "entered_adjacent":
		n := 0
		for i, e := range st.Entered[s.P] {
			if abs(i-s.Lane) == 1 {
				n += e
			}
		}
		return n
	case "base_attack":
		if s.creature == nil {
			return 0
		}
		return baseStat(g.Card(s.creature.Instance).Attack)
	case "milled_cost":
		return g.milledCost
	case "milled_spells":
		return g.milledSpells
	case "damage_dealt":
		return g.heroDamage
	case "damage":
		if s.creature == nil {
			return 0
		}
		return s.creature.Damage
	}
	return 0
}

// cond evaluates a Cond from the point of view of source s.
func (g *Game) cond(c card.Cond, s source) bool {
	st := &g.State
	if c.OpposingCreatureInLane != nil &&
		(st.Players[s.P.Opp()].Lanes[s.Lane].Creature != nil) != *c.OpposingCreatureInLane {
		return false
	}
	if c.BuildingInLane != nil &&
		(st.Players[s.P].Lanes[s.Lane].Building != nil) != *c.BuildingInLane {
		return false
	}
	if c.OpponentHandMax != nil && len(st.Players[s.P.Opp()].Hand) > *c.OpponentHandMax {
		return false
	}
	if c.DiscardMin != nil && len(st.Players[s.P].Discard) < *c.DiscardMin {
		return false
	}
	if c.Controls != "" {
		found := false
		for i, l := range st.Players[s.P].Lanes {
			found = found || (l.Creature != nil && g.isType(pos{s.P, i}, c.Controls))
		}
		if !found {
			return false
		}
	}
	if c.OnLandscape != "" {
		if l := st.Players[s.P].Lanes[s.Lane]; l.FaceDown || l.Landscape != c.OnLandscape {
			return false
		}
	}
	if c.LandscapeTypes != nil && !c.LandscapeTypes.Has(g.count(card.Count{Of: "landscape_types"}, s)) {
		return false
	}
	if c.Returned != nil && !c.Returned.Has(st.Returned[s.P]) {
		return false
	}
	if c.EnteredMax != nil {
		n := 0
		for _, e := range st.Entered[s.P] {
			n += e
		}
		if n > *c.EnteredMax {
			return false
		}
	}
	if c.Damage != nil && (s.creature == nil || !c.Damage.Has(s.creature.Damage)) {
		return false
	}
	if c.Replaced != nil && (s.creature == nil || s.creature.Replaced != *c.Replaced) {
		return false
	}
	if c.Flooped != nil && (s.creature == nil || (s.creature.Status == Flooped) != *c.Flooped) {
		return false
	}
	if c.OtherCreatures != nil {
		n := 0
		for _, l := range st.Players[s.P].Lanes {
			if l.Creature != nil && l.Creature != s.creature {
				n++
			}
		}
		if !c.OtherCreatures.Has(n) {
			return false
		}
	}
	if c.EmptyLanes != nil && !c.EmptyLanes.Has(g.count(card.Count{Of: "empty_lanes"}, s)) {
		return false
	}
	if c.OwnedAbroad != nil {
		abroad := false
		for _, l := range st.Players[s.P.Opp()].Lanes {
			abroad = abroad || (l.Creature != nil && l.Creature.Owner == s.P)
		}
		if abroad != *c.OwnedAbroad {
			return false
		}
	}
	switch c.LaneFrozen {
	case "":
	case "own":
		if s.Lane < 0 || !st.Players[s.P].Lanes[s.Lane].Frozen {
			return false
		}
	case "both":
		if s.Lane < 0 || !st.Players[s.P].Lanes[s.Lane].Frozen || !st.Players[s.P.Opp()].Lanes[s.Lane].Frozen {
			return false
		}
	default:
		return false
	}
	if c.FrozenLandscapes != nil && !c.FrozenLandscapes.Has(g.frozenCount([]PlayerID{0, 1})) {
		return false
	}
	if c.YourTurn != nil && (st.Active == s.P) != *c.YourTurn {
		return false
	}
	if c.HP != nil && !c.HP.Has(st.Players[s.P].HP) {
		return false
	}
	if c.LaneAllyExhausted != nil && s.Lane >= 0 {
		cr := st.Players[s.P].Lanes[s.Lane].Creature
		if (cr != nil && cr.Status != Ready) != *c.LaneAllyExhausted {
			return false
		}
	}
	if pc := c.Played; pc != nil {
		n := 0
		if s.P == st.Active {
			n = g.playedOfType(pc.Landscape)
		}
		if pc.Min != nil && n < *pc.Min {
			return false
		}
	}
	if lc := c.Landscapes; lc != nil {
		n := g.landscapeCount(s.P, lc.Type)
		if (lc.Min != nil && n < *lc.Min) || (lc.Max != nil && n > *lc.Max) {
			return false
		}
	}
	return true
}

// attackable reports whether the Creature at target may be Attacked. Cool Dog
// protects the Creatures in the Lanes next to it.
func (g *Game) attackable(target pos) bool {
	if cr := g.creatureAt(target); cr != nil && cr.Shielded {
		return false
	}
	ok := true
	g.forEachSource(func(s source) {
		if s.P != target.P || abs(s.Lane-target.Lane) != 1 {
			return
		}
		for _, e := range s.effects {
			if e.Kind == card.KindStatic && e.Restrict == card.RestrictAdjacentAlliesNotAttackable {
				ok = false
			}
		}
	})
	return ok
}

// blocksTriggers reports whether the Creature entering or leaving play on p's
// side of a Lane is stopped from triggering entering and leaving play effects
// by an opposing Building (Palace of Bone).
func (g *Game) blocksTriggers(p PlayerID, lane int) bool {
	for _, b := range g.State.Players[p.Opp()].Lanes[lane].buildings() {
		if hasRestrict(g.buildingEffects(pos{p.Opp(), lane}, b), card.RestrictOpposingNoEnterLeave) {
			return true
		}
	}
	return false
}

// Attack and Defense are the current ATK and DEF of p's Creature in a Lane
// (0 if there is none), after all modifiers. For display and tooling.
func (g *Game) Attack(p PlayerID, lane int) int {
	if g.creatureAt(pos{p, lane}) == nil {
		return 0
	}
	return g.atk(pos{p, lane})
}

func (g *Game) Defense(p PlayerID, lane int) int {
	if g.creatureAt(pos{p, lane}) == nil {
		return 0
	}
	return g.def(pos{p, lane})
}

// CanBeAttacked reports whether the Creature in p's Lane is currently
// protected from Fights (Cool Dog).
func (g *Game) CanBeAttacked(p PlayerID, lane int) bool { return g.attackable(pos{p, lane}) }
