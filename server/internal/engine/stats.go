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
	creature *Creature // nil for Buildings
	effects  []card.Effect
}

// forEachSource visits every Creature and Building in play.
func (g *Game) forEachSource(fn func(source)) {
	for p := range g.State.Players {
		for i := range g.State.Players[p].Lanes {
			l := &g.State.Players[p].Lanes[i]
			at := pos{PlayerID(p), i}
			if l.Creature != nil {
				fn(source{at, l.Creature, g.Card(l.Creature.Instance).Effects})
			}
			if l.Building != nil {
				fn(source{at, nil, g.Card(l.Building.Instance).Effects})
			}
		}
	}
}

func (g *Game) creatureAt(at pos) *Creature { return g.State.Players[at.P].Lanes[at.Lane].Creature }

// atk and def are a Creature's current ATK and DEF: printed value, plus
// "this turn" modifiers, plus continuous effects from everything in play.
// Every read of a stat goes through here.
func (g *Game) atk(at pos) int {
	c := g.creatureAt(at)
	return max(0, baseStat(g.Card(c.Instance).Attack)+c.TempATK+g.staticMod("atk", at))
}

func (g *Game) def(at pos) int {
	c := g.creatureAt(at)
	return max(0, baseStat(g.Card(c.Instance).Defense)+c.TempDEF+g.staticMod("def", at))
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
	case "any":
	default:
		return false
	}
	if sel.Landscape != "" && g.Card(t.Instance).Landscape != sel.Landscape {
		return false
	}
	if sel.ChangedLanes && !t.ChangedLanes {
		return false
	}
	return true
}

// value evaluates a Value from the point of view of source s.
func (g *Game) value(v card.Value, s source) int {
	if v.Per == nil {
		return v.N
	}
	return v.N * g.count(*v.Per, s)
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
		}
		return n
	case "landscape_types":
		seen := map[card.Landscape]bool{}
		for _, l := range st.Players[s.P].Lanes {
			if !l.FaceDown {
				seen[l.Landscape] = true
			}
		}
		return len(seen)
	case "creatures", "flooped_creatures":
		n := 0
		for _, p := range players {
			for _, l := range st.Players[p].Lanes {
				cr := l.Creature
				if cr == nil || (c.Other && cr == s.creature) {
					continue
				}
				if c.Type != "" && g.Card(cr.Instance).Landscape != c.Type {
					continue
				}
				if c.Of == "flooped_creatures" && cr.Status != Flooped {
					continue
				}
				n++
			}
		}
		return n
	case "spells_played":
		if s.P != st.Active {
			return 0
		}
		return st.SpellsThisTurn
	case "opponent_hand":
		return len(st.Players[s.P.Opp()].Hand)
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
	if lc := c.Landscapes; lc != nil {
		n := g.faceUp(s.P, lc.Type)
		if (lc.Min != nil && n < *lc.Min) || (lc.Max != nil && n > *lc.Max) {
			return false
		}
	}
	return true
}

// attackable reports whether the Creature at target may be Attacked. Cool Dog
// protects the Creatures in the Lanes next to it.
func (g *Game) attackable(target pos) bool {
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
