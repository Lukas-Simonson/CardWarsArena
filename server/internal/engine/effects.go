package engine

import (
	"math/rand/v2"

	"cardwars/internal/card"
)

// effCtx is who is using an effect and from where.
type effCtx struct {
	p        PlayerID  // controller
	at       pos       // where the source card sits (only if hasSrc)
	hasSrc   bool      // false for Spells
	creature *Creature // the source, when it is a Creature
}

func (c effCtx) source() source { return source{pos: c.at, creature: c.creature} }

func sidesFor(side string, p PlayerID) []PlayerID {
	switch side {
	case "you":
		return []PlayerID{p}
	case "opponent":
		return []PlayerID{p.Opp()}
	}
	return []PlayerID{p, p.Opp()}
}

// candidates lists every legal choice for one target slot.
func (g *Game) candidates(spec card.TargetSpec, c effCtx) []TargetRef {
	var out []TargetRef
	if spec.What == "discard_card" {
		for _, p := range sidesFor(spec.Side, c.p) {
			for _, in := range g.State.Players[p].Discard {
				cd := g.Card(in)
				if (spec.CardType != "" && cd.Type != spec.CardType) ||
					(spec.Landscape != "" && cd.Landscape != spec.Landscape) {
					continue
				}
				out = append(out, TargetRef{Kind: spec.What, Player: p, Card: in.UID})
			}
		}
		return out
	}
	for _, p := range sidesFor(spec.Side, c.p) {
		for lane := 0; lane < NumLanes; lane++ {
			if spec.InLane && (!c.hasSrc || lane != c.at.Lane) {
				continue
			}
			if spec.AdjacentLane && (!c.hasSrc || abs(lane-c.at.Lane) != 1) {
				continue
			}
			l := &g.State.Players[p].Lanes[lane]
			ok := false
			switch spec.What {
			case "creature":
				if cr := l.Creature; cr != nil {
					ok = (spec.Landscape == "" || g.Card(cr.Instance).Landscape == spec.Landscape) &&
						(spec.Flooped == nil || (cr.Status == Flooped) == *spec.Flooped)
				}
			case "building":
				ok = l.Building != nil
			case "landscape":
				ok = true
				if spec.Landscape != "" && (l.FaceDown || l.Landscape != spec.Landscape) {
					ok = false // a face-down Landscape has no type
				}
				if spec.FaceDown != nil && l.FaceDown != *spec.FaceDown {
					ok = false
				}
			case "lane":
				ok = true
				if spec.Without == "creature" && l.Creature != nil {
					ok = false
				}
				if spec.Without == "building" && l.Building != nil {
					ok = false
				}
			}
			if ok {
				out = append(out, TargetRef{Kind: spec.What, Player: p, Lane: lane})
			}
		}
	}
	return out
}

func (g *Game) slotMax(spec card.TargetSpec, c effCtx, n int) int {
	max := 1
	if spec.Count != nil {
		max = g.value(*spec.Count, c.source())
	}
	return min(max, n)
}

// checkTargets validates the chosen targets for an effect. usable is false
// when a mandatory slot has no legal choice at all, in which case a Spell or
// ability cannot be used and a trigger does nothing; err is only set for
// choices that are wrong while the effect is usable.
func (g *Game) checkTargets(specs []card.TargetSpec, given [][]TargetRef, c effCtx) (usable bool, err error) {
	cands := make([][]TargetRef, len(specs))
	for i, spec := range specs {
		cands[i] = g.candidates(spec, c)
		if len(cands[i]) == 0 && !spec.Optional {
			return false, nil
		}
	}
	if len(given) > len(specs) {
		return true, ErrBadTarget
	}
	for i, spec := range specs {
		var chosen []TargetRef
		if i < len(given) {
			chosen = given[i]
		}
		max := g.slotMax(spec, c, len(cands[i]))
		need := max
		if spec.Optional {
			need = 0
		}
		if len(chosen) < need || len(chosen) > max {
			return true, ErrBadTarget
		}
		seen := map[TargetRef]bool{}
		for _, ref := range chosen {
			if seen[ref] || !contains(cands[i], ref) {
				return true, ErrBadTarget
			}
			seen[ref] = true
		}
	}
	return true, nil
}

func contains(list []TargetRef, r TargetRef) bool {
	for _, x := range list {
		if x == r {
			return true
		}
	}
	return false
}

// targetCombos enumerates complete target choices for an effect, capped so a
// card with a wide slot cannot flood the legal-action list.
func (g *Game) targetCombos(specs []card.TargetSpec, c effCtx, limit int) [][][]TargetRef {
	var perSlot [][][]TargetRef // slot -> choice -> refs
	for _, spec := range specs {
		cands := g.candidates(spec, c)
		max := g.slotMax(spec, c, len(cands))
		lo := max
		if spec.Optional {
			lo = 0
		}
		var choices [][]TargetRef
		for size := lo; size <= max; size++ {
			choices = append(choices, subsets(cands, size)...)
		}
		if len(choices) == 0 {
			return nil
		}
		perSlot = append(perSlot, choices)
	}
	out := [][][]TargetRef{nil}
	for _, choices := range perSlot {
		var next [][][]TargetRef
		for _, partial := range out {
			for _, ch := range choices {
				if len(next) >= limit {
					break
				}
				next = append(next, append(append([][]TargetRef(nil), partial...), ch))
			}
		}
		out = next
	}
	return out
}

func subsets(items []TargetRef, size int) [][]TargetRef {
	if size == 0 {
		return [][]TargetRef{nil}
	}
	var out [][]TargetRef
	for i := 0; i+size <= len(items); i++ {
		for _, rest := range subsets(items[i+1:], size-1) {
			out = append(out, append([]TargetRef{items[i]}, rest...))
		}
	}
	return out
}

func (g *Game) whoList(who string, p PlayerID) []PlayerID {
	switch who {
	case "opponent":
		return []PlayerID{p.Opp()}
	case "each":
		return []PlayerID{p, p.Opp()}
	}
	return []PlayerID{p}
}

// creaturesFor returns the Creatures an op acts on: its chosen slot, or every
// Creature matching op.All.
func (g *Game) creaturesFor(op card.Op, c effCtx, given [][]TargetRef) []pos {
	var out []pos
	if op.All != nil {
		for p := range g.State.Players {
			for lane := range g.State.Players[p].Lanes {
				at := pos{PlayerID(p), lane}
				if t := g.creatureAt(at); t != nil && g.selects(*op.All, c.source(), at, t) {
					out = append(out, at)
				}
			}
		}
		return out
	}
	if op.Slot != nil && *op.Slot < len(given) {
		for _, r := range given[*op.Slot] {
			if g.creatureAt(pos{r.Player, r.Lane}) != nil {
				out = append(out, pos{r.Player, r.Lane})
			}
		}
	}
	return out
}

func slotRefs(i *int, given [][]TargetRef) []TargetRef {
	if i == nil || *i >= len(given) {
		return nil
	}
	return given[*i]
}

// runOps performs an effect's operations in order.
func (g *Game) runOps(ops []card.Op, c effCtx, given [][]TargetRef) []Event {
	var ev []Event
	s := c.source()
	for _, op := range ops {
		if op.If != nil && !g.cond(*op.If, s) {
			continue
		}
		amt := 0
		if op.Amount != nil {
			amt = g.value(*op.Amount, s)
		}
		switch op.Do {
		case "draw":
			for _, p := range g.whoList(op.Who, c.p) {
				for _, in := range g.draw(p, amt) {
					ev = append(ev, Event{Kind: EvDraw, Player: p, Card: in.UID})
				}
			}
		case "damage_hero":
			for _, p := range g.whoList(op.Who, c.p) {
				if amt > 0 {
					g.State.Players[p].HP -= amt
					ev = append(ev, Event{Kind: EvHPLoss, Player: p, Amount: amt})
				}
			}
		case "damage_creature":
			for _, at := range g.creaturesFor(op, c, given) {
				cr := g.creatureAt(at)
				cr.Damage += amt
				if amt > 0 {
					ev = append(ev, Event{Kind: EvDamage, Player: at.P, Card: cr.UID, Lane: at.Lane, Amount: amt})
				}
			}
		case "mod_creature":
			for _, at := range g.creaturesFor(op, c, given) {
				cr := g.creatureAt(at)
				if op.Stat == "atk" {
					cr.TempATK += amt
				} else {
					cr.TempDEF += amt
				}
			}
		case "ready_creature":
			for _, at := range g.creaturesFor(op, c, given) {
				if cr := g.creatureAt(at); cr.Status == Flooped {
					cr.Status = Ready
					ev = append(ev, Event{Kind: EvReady, Player: at.P, Card: cr.UID, Lane: at.Lane})
				}
			}
		case "bounce":
			for _, at := range g.creaturesFor(op, c, given) {
				ev = append(ev, g.removeCreature(at, toHand)...)
			}
		case "move_creature":
			from, to := slotRefs(op.Slot, given), slotRefs(op.Slot2, given)
			if len(from) == 1 && len(to) == 1 {
				ev = append(ev, g.moveCreature(pos{from[0].Player, from[0].Lane}, to[0].Lane)...)
			}
		case "move_building":
			from, to := slotRefs(op.Slot, given), slotRefs(op.Slot2, given)
			if len(from) == 1 && len(to) == 1 {
				ev = append(ev, g.moveBuilding(pos{from[0].Player, from[0].Lane}, to[0].Lane)...)
			}
		case "flip_landscape":
			for _, r := range slotRefs(op.Slot, given) {
				l := &g.State.Players[r.Player].Lanes[r.Lane]
				l.FaceDown = op.Face == "down"
				ev = append(ev, Event{Kind: EvFlip, Player: r.Player, Lane: r.Lane})
			}
		case "destroy_building":
			for _, r := range slotRefs(op.Slot, given) {
				l := &g.State.Players[r.Player].Lanes[r.Lane]
				if b := l.Building; b != nil {
					g.State.Players[r.Player].Discard = append(g.State.Players[r.Player].Discard, b.Instance)
					l.Building = nil
					ev = append(ev, Event{Kind: EvDestroy, Player: r.Player, Card: b.UID, Lane: r.Lane})
				}
			}
		case "gain_actions":
			g.State.ActionsLeft += amt
			ev = append(ev, Event{Kind: EvActions, Player: c.p, Amount: amt})
		case "return_from_discard":
			ev = append(ev, g.returnFromDiscard(op, c, given)...)
		case "put_on_deck":
			for _, r := range slotRefs(op.Slot, given) {
				if in, ok := g.takeFromDiscard(r.Player, r.Card); ok {
					pl := &g.State.Players[r.Player]
					pl.Deck = append([]Instance{in}, pl.Deck...)
					ev = append(ev, Event{Kind: EvDeckTop, Player: r.Player, Card: in.UID})
				}
			}
		}
	}
	return ev
}

func (g *Game) takeFromDiscard(p PlayerID, uid int) (Instance, bool) {
	pl := &g.State.Players[p]
	for i, in := range pl.Discard {
		if in.UID == uid {
			pl.Discard = append(pl.Discard[:i:i], pl.Discard[i+1:]...)
			return in, true
		}
	}
	return Instance{}, false
}

// returnFromDiscard moves a chosen or random matching card from the
// controller's discard pile to their hand.
func (g *Game) returnFromDiscard(op card.Op, c effCtx, given [][]TargetRef) []Event {
	pl := &g.State.Players[c.p]
	var uid int
	if refs := slotRefs(op.Slot, given); len(refs) > 0 {
		uid = refs[0].Card
	} else if op.Random {
		var matches []int
		for _, in := range pl.Discard {
			cd := g.Card(in)
			if op.Filter != nil && ((op.Filter.Landscape != "" && cd.Landscape != op.Filter.Landscape) ||
				(op.Filter.Type != "" && cd.Type != op.Filter.Type)) {
				continue
			}
			matches = append(matches, in.UID)
		}
		if len(matches) == 0 {
			return nil
		}
		uid = matches[rand.New(&g.rng).IntN(len(matches))]
	} else {
		return nil
	}
	in, ok := g.takeFromDiscard(c.p, uid)
	if !ok {
		return nil
	}
	pl.Hand = append(pl.Hand, in)
	return []Event{{Kind: EvReturn, Player: c.p, Card: in.UID}}
}

type destination int

const (
	toDiscard destination = iota
	toHand
)

// removeCreature takes the Creature at at out of play, to its owner's discard
// pile or hand. Damage is dropped. Every way a Creature leaves play goes
// through here.
func (g *Game) removeCreature(at pos, to destination) []Event {
	l := &g.State.Players[at.P].Lanes[at.Lane]
	cr := l.Creature
	if cr == nil {
		return nil
	}
	l.Creature = nil
	pl := &g.State.Players[at.P]
	var ev []Event
	switch to {
	case toHand:
		pl.Hand = append(pl.Hand, cr.Instance)
		ev = append(ev, Event{Kind: EvBounce, Player: at.P, Card: cr.UID, Lane: at.Lane})
	default:
		pl.Discard = append(pl.Discard, cr.Instance)
	}
	ctx := effCtx{p: at.P, at: at, hasSrc: true, creature: cr}
	return append(ev, g.fire(card.OnLeavePlay, ctx, cr.Instance, nil)...)
}

// moveCreature moves a Creature to an empty Lane on its controller's side.
func (g *Game) moveCreature(from pos, lane int) []Event {
	pl := &g.State.Players[from.P]
	cr := pl.Lanes[from.Lane].Creature
	if cr == nil || lane == from.Lane || pl.Lanes[lane].Creature != nil {
		return nil
	}
	pl.Lanes[from.Lane].Creature = nil
	pl.Lanes[lane].Creature = cr
	cr.ChangedLanes = true
	ev := []Event{{Kind: EvMove, Player: from.P, Card: cr.UID, Lane: lane}}
	ctx := effCtx{p: from.P, at: pos{from.P, lane}, hasSrc: true, creature: cr}
	return append(ev, g.fire(card.OnChangeLanes, ctx, cr.Instance, nil)...)
}

// moveBuilding moves a Building to a Lane on its controller's side that has none.
func (g *Game) moveBuilding(from pos, lane int) []Event {
	pl := &g.State.Players[from.P]
	b := pl.Lanes[from.Lane].Building
	if b == nil || lane == from.Lane || pl.Lanes[lane].Building != nil {
		return nil
	}
	pl.Lanes[from.Lane].Building = nil
	pl.Lanes[lane].Building = b
	return []Event{{Kind: EvMove, Player: from.P, Card: b.UID, Lane: lane}}
}
