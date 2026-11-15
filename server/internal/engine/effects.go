package engine

import (
	"math/rand/v2"
	"slices"

	"cardwars/internal/card"
)

// effCtx is who is using an effect and from where.
type effCtx struct {
	p        PlayerID  // controller
	at       pos       // where the source card sits (only if hasSrc)
	hasSrc   bool      // false for Spells
	creature *Creature // the source, when it is a Creature
	playing  int       // UID of the card being played (not a legal hand_card choice)
	entering *pos      // for creature_enters triggers: the Creature that entered
	card     string    // the source card's id when it is not a Creature (a Spell, Building or Hero)
	hero     bool      // the source is its controller's Hero
}

// cardID is the id of the card using the effect.
func (c effCtx) cardID() string {
	if c.creature != nil {
		return c.creature.CardID
	}
	return c.card
}

func (c effCtx) source() source { return source{pos: c.at, creature: c.creature, hero: c.hero} }

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
func (g *Game) candidates(spec card.TargetSpec, c effCtx, given [][]TargetRef) []TargetRef {
	var out []TargetRef
	if spec.What == "hand_card" {
		// Only ever the controller's own hand (discard costs).
		for _, in := range g.State.Players[c.p].Hand {
			if in.UID != c.playing {
				out = append(out, TargetRef{Kind: spec.What, Player: c.p, Card: in.UID})
			}
		}
		return out
	}
	if spec.What == "discard_card" {
		for _, p := range sidesFor(spec.Side, c.p) {
			for _, in := range g.State.Players[p].Discard {
				cd := g.Card(in)
				if (spec.CardType != "" && cd.Type != spec.CardType) ||
					(spec.Landscape != "" && cd.Landscape != spec.Landscape) ||
					(spec.Cost != nil && (cd.Cost == nil || *cd.Cost != *spec.Cost)) {
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
			if spec.Other && c.hasSrc && lane == c.at.Lane {
				continue
			}
			if spec.LaneOf != nil {
				// Only the Lane of what was chosen for that earlier slot.
				refs := slotRefs(spec.LaneOf, given)
				if len(refs) != 1 || refs[0].Lane != lane {
					continue
				}
			}
			if spec.AdjacentLane && (!c.hasSrc || abs(lane-c.at.Lane) != 1) {
				continue
			}
			l := &g.State.Players[p].Lanes[lane]
			ok := false
			switch spec.What {
			case "creature":
				if cr := l.Creature; cr != nil && !(cr.Shielded && p != c.p) && (!spec.Owned || cr.Owner == c.p) {
					ok = (spec.Landscape == "" || g.isType(pos{p, lane}, spec.Landscape)) &&
						(spec.Flooped == nil || (cr.Status == Flooped) == *spec.Flooped) &&
						(!spec.Ready || cr.Status == Ready) && (!spec.Exhausted || cr.Status != Ready) && (!spec.Damaged || cr.Damage > 0)
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
				if spec.Landscape != "" && (l.FaceDown || l.Landscape != spec.Landscape) {
					ok = false
				}
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

// withoutTaken drops the choices already made for earlier slots of the same
// Distinct set as spec i.
func withoutTaken(cands []TargetRef, specs []card.TargetSpec, i int, given [][]TargetRef) []TargetRef {
	if specs[i].Distinct == "" {
		return cands
	}
	var out []TargetRef
	for _, r := range cands {
		taken := false
		for j := 0; j < i && j < len(given); j++ {
			taken = taken || (specs[j].Distinct == specs[i].Distinct && contains(given[j], r))
		}
		if !taken {
			out = append(out, r)
		}
	}
	return out
}

// labelsOf is the Labels of an action choosing for specs, or nil when none is set.
func labelsOf(specs []card.TargetSpec) []string {
	var out []string
	any := false
	for _, s := range specs {
		out = append(out, s.Label)
		any = any || s.Label != ""
	}
	if !any {
		return nil
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
	if len(given) > len(specs) {
		return true, ErrBadTarget
	}
	// A slot's choices can depend on the slots before it (LaneOf), so judge
	// each against what was given for those.
	cands := make([][]TargetRef, len(specs))
	for i, spec := range specs {
		cands[i] = withoutTaken(g.candidates(spec, c, given), specs, i, given)
		if len(cands[i]) == 0 && !spec.Optional {
			return false, nil
		}
	}
	groups := map[string]int{}
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
		if spec.Group != "" {
			groups[spec.Group] += len(chosen)
		}
	}
	for _, n := range groups {
		if n != 1 {
			return true, ErrBadTarget // exactly one choice among the group's slots
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
	out := [][][]TargetRef{nil}
	for i, spec := range specs {
		var next [][][]TargetRef
		for _, partial := range out {
			cands := withoutTaken(g.candidates(spec, c, partial), specs, i, partial)
			max := g.slotMax(spec, c, len(cands))
			lo := max
			if spec.Optional {
				lo = 0
			}
			for size := lo; size <= max; size++ {
				for _, ch := range subsets(cands, size) {
					if len(next) >= limit {
						break
					}
					next = append(next, append(append([][]TargetRef(nil), partial...), ch))
				}
			}
		}
		if len(next) == 0 {
			return nil
		}
		out = next
	}
	// Specs sharing a group need exactly one choice between them.
	var kept [][][]TargetRef
	for _, combo := range out {
		groups := map[string]int{}
		for i, spec := range specs {
			if spec.Group != "" {
				groups[spec.Group] += len(combo[i])
			}
		}
		ok := true
		for _, n := range groups {
			ok = ok && n == 1
		}
		if ok {
			kept = append(kept, combo)
		}
	}
	return kept
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
	if op.Self {
		if c.creature != nil && g.creatureAt(c.at) == c.creature {
			out = append(out, c.at)
		}
		return out
	}
	if op.Entering {
		if c.entering != nil && g.creatureAt(*c.entering) != nil {
			out = append(out, *c.entering)
		}
		return out
	}
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
				if dealt := g.hurt(at, amt); dealt > 0 {
					ev = append(ev, Event{Kind: EvDamage, Player: at.P, Card: cr.UID, Lane: at.Lane, Amount: dealt})
				}
			}
		case "heal_creature":
			// Heals up to amt; "all" is a large amount.
			for _, at := range g.creaturesFor(op, c, given) {
				cr := g.creatureAt(at)
				if healed := min(amt, cr.Damage); healed > 0 {
					cr.Damage -= healed
					ev = append(ev, Event{Kind: EvHeal, Player: at.P, Card: cr.UID, Lane: at.Lane, Amount: healed})
				}
			}
		case "heal_hero":
			for _, p := range g.whoList(op.Who, c.p) {
				pl := &g.State.Players[p]
				if healed := min(amt, StartingHP-pl.HP); healed > 0 {
					pl.HP += healed
					ev = append(ev, Event{Kind: EvHPGain, Player: p, Amount: healed})
				}
			}
		case "swap_creatures":
			if refs := slotRefs(op.Slot, given); op.Self && len(refs) == 1 && c.hasSrc {
				ev = append(ev, g.swapCreatures(c.at, pos{refs[0].Player, refs[0].Lane})...)
			}
		case "protect":
			for _, at := range g.creaturesFor(op, c, given) {
				g.creatureAt(at).Shielded = true
			}
		case "no_combat_damage":
			g.State.NoCombatDamage[c.p] = true
		case "discount_replace":
			if c.hasSrc {
				g.State.Players[c.p].Lanes[c.at.Lane].ReplaceDiscount += amt
			}
		case "upkeep":
			ev = append(ev, g.upkeep(amt, c)...)
		case "mod_creature":
			for _, at := range g.creaturesFor(op, c, given) {
				cr := g.creatureAt(at)
				n := amt
				if op.Amount != nil && op.Amount.Per != nil && op.Amount.Per.FromTarget {
					n = g.value(*op.Amount, source{pos: at, creature: cr}) // counted from the chosen Creature
				}
				switch {
				case op.Permanent && op.Stat == "atk":
					cr.PermATK += n
				case op.Permanent:
					cr.PermDEF += n
				case op.Stat == "atk":
					cr.TempATK += n
				default:
					cr.TempDEF += n
				}
			}
		case "ready_creature":
			for _, at := range g.creaturesFor(op, c, given) {
				if cr := g.creatureAt(at); cr.Status == Flooped {
					cr.Status = Ready
					ev = append(ev, Event{Kind: EvReady, Player: at.P, Card: cr.UID, Lane: at.Lane})
				}
			}
		case "destroy_creature":
			g.destroyed = 0
			for _, at := range g.creaturesFor(op, c, given) {
				if g.creatureAt(at) != nil {
					g.destroyed++
				}
				ev = append(ev, g.destroyAt(at)...)
			}
		case "freeze":
			for _, at := range g.freezeTargets(op, c, given) {
				ev = append(ev, g.freeze(at.P, at.Lane)...)
			}
		case "thaw":
			if op.Self && c.hasSrc {
				ev = append(ev, g.thaw(c.at.P, c.at.Lane)...)
			}
		case "move_self":
			ev = append(ev, g.moveSelf(op, c)...)
		case "mill":
			g.milledCost, g.milledSpells = 0, 0
			for _, p := range g.whoList(op.Who, c.p) {
				ev = append(ev, g.mill(p, amt)...)
			}
		case "discard_choice":
			for _, p := range g.whoList(op.Who, c.p) {
				ev = append(ev, g.discardChoice(p, amt, c, op.RainbowActions)...)
			}
		case "exhaust":
			for _, at := range g.creaturesFor(op, c, given) {
				if cr := g.creatureAt(at); cr.Status == Ready {
					cr.Status = Flooped
				}
			}
		case "play_building":
			from, to := slotRefs(op.Slot, given), slotRefs(op.Slot2, given)
			if len(from) == 1 && len(to) == 1 && to[0].Player == c.p {
				if in, ok := g.inDiscard(from[0].Player, from[0].Card); ok && g.Card(in).Type == card.Building {
					g.takeFromDiscard(from[0].Player, in.UID)
					ev = append(ev, g.putBuilding(c.p, in, to[0].Lane)...)
				}
			}
		case "search_building":
			if refs := slotRefs(op.Slot, given); len(refs) == 1 {
				g.searchBuilding(c.p, refs[0].Lane, c)
			}
		case "move_damage":
			from, to := slotRefs(op.Slot, given), slotRefs(op.Slot2, given)
			if len(from) == 1 && len(to) == 1 && from[0].Player == c.p && to[0].Player == c.p {
				ev = append(ev, g.startMoveDamage(c.p, from[0].Lane, to[0].Lane, c)...)
			}
		case "make_rainbow":
			for _, at := range g.creaturesFor(op, c, given) {
				g.creatureAt(at).Rainbow = true
			}
		case "rainbow_hand":
			for _, r := range slotRefs(op.Slot, given) {
				if !slices.Contains(g.State.RainbowHand, r.Card) {
					g.State.RainbowHand = append(g.State.RainbowHand, r.Card)
				}
				ev = append(ev, Event{Kind: EvReveal, Player: r.Player, Card: r.Card})
			}
		case "rebound":
			if cr := c.creature; cr != nil && len(g.State.Players[cr.Owner].Hand) > 0 {
				g.ask(Choice{Player: cr.Owner, Kind: ChoiceRebound, Card: c.cardID(), SourceUID: cr.UID})
			}
		case "redirect_attack":
			ev = append(ev, g.askRedirect(c)...)
		case "may":
			if c.creature != nil {
				c.creature.Offered = append(c.creature.Offered, Offer{Targets: op.Targets, Then: op.Then})
			}
		case "discount_next_creature":
			g.State.CreatureDiscount += amt
		case "borrow_creature":
			from, to := slotRefs(op.Slot, given), slotRefs(op.Slot2, given)
			if len(from) == 1 && len(to) == 1 && from[0].Player != c.p && to[0].Player == c.p {
				at := pos{c.p, to[0].Lane}
				ev = append(ev, g.moveCreature(pos{from[0].Player, from[0].Lane}, at)...)
				if cr := g.creatureAt(at); cr != nil {
					cr.Status, cr.Borrowed = Ready, true
					ev = append(ev, Event{Kind: EvReady, Player: c.p, Card: cr.UID, Lane: at.Lane})
				}
			}
		case "ogre_gas":
			g.ogreGas(c.p, 3, c)
		case "bounce":
			g.bouncedCost = 0
			for _, at := range g.creaturesFor(op, c, given) {
				g.bouncedCost += baseStat(g.Card(g.creatureAt(at).Instance).Cost)
				ev = append(ev, g.removeCreature(at, toHand)...)
			}
		case "move_creature":
			from, to := slotRefs(op.Slot, given), slotRefs(op.Slot2, given)
			if op.Self {
				// The Creature moves itself; what comes after acts on it where it is now (Husky Parrotrooper).
				if c.creature != nil && c.hasSrc && g.creatureAt(c.at) == c.creature && len(to) == 1 {
					dest := pos{to[0].Player, to[0].Lane}
					ev = append(ev, g.moveCreature(c.at, dest)...)
					if g.creatureAt(dest) == c.creature {
						c.at, c.p = dest, dest.P
						s = c.source()
					}
				}
			} else if len(from) == 1 && len(to) == 1 {
				ev = append(ev, g.moveCreature(pos{from[0].Player, from[0].Lane}, pos{to[0].Player, to[0].Lane})...)
			}
		case "discard":
			for _, r := range slotRefs(op.Slot, given) {
				pl := &g.State.Players[r.Player]
				for i, in := range pl.Hand {
					if in.UID == r.Card {
						pl.Hand = append(pl.Hand[:i:i], pl.Hand[i+1:]...)
						g.State.Players[in.Owner].Discard = append(g.State.Players[in.Owner].Discard, in)
						ev = append(ev, Event{Kind: EvDiscard, Player: r.Player, Card: in.UID})
						break
					}
				}
			}
		case "move_building":
			from, to := slotRefs(op.Slot, given), slotRefs(op.Slot2, given)
			if len(from) == 1 && len(to) == 1 {
				ev = append(ev, g.moveBuilding(pos{from[0].Player, from[0].Lane}, to[0].Lane)...)
			}
		case "flip_landscape":
			if op.Tracked {
				ev = append(ev, g.unflipTracked(c)...)
			}
			for _, r := range slotRefs(op.Slot, given) {
				if op.Who == "you" {
					r.Player = c.p // the controller's Landscape in the chosen Lane
				}
				ev = append(ev, g.flipLandscape(op, c, r.Player, r.Lane)...)
			}
		case "retype_landscape":
			if c.hasSrc && c.at.Lane >= 0 {
				g.askLandscape(c.p, c.at.Lane, c)
			}
		case "destroy_building":
			for _, r := range slotRefs(op.Slot, given) {
				l := &g.State.Players[r.Player].Lanes[r.Lane]
				if b := l.takeBuilding(); b != nil {
					g.State.Players[b.Owner].Discard = append(g.State.Players[b.Owner].Discard, b.Instance)
					ev = append(ev, Event{Kind: EvDestroy, Player: r.Player, Card: b.UID, Lane: r.Lane})
				}
			}
		case "steal_play":
			ev = append(ev, g.stealPlay(c)...)
		case "copy_floop":
			ev = append(ev, g.copyFloop(c)...)
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

// mill puts the top n cards of p's deck into their owner's discard pile.
func (g *Game) mill(p PlayerID, n int) []Event {
	pl := &g.State.Players[p]
	var ev []Event
	for ; n > 0 && len(pl.Deck) > 0; n-- {
		in := pl.Deck[0]
		pl.Deck = pl.Deck[1:]
		g.State.Players[in.Owner].Discard = append(g.State.Players[in.Owner].Discard, in)
		if g.Card(in).Type == card.Spell {
			g.milledSpells++
		}
		if cost := g.Card(in).Cost; cost != nil {
			g.milledCost = *cost
		} else {
			g.milledCost = 0
		}
		ev = append(ev, Event{Kind: EvDiscard, Player: p, Card: in.UID})
	}
	return ev
}

// discardChoice makes p discard n cards from their hand, by their own choice.
// When they would have to discard their whole hand there is nothing to choose.
func (g *Game) discardChoice(p PlayerID, n int, c effCtx, bonus int) []Event {
	pl := &g.State.Players[p]
	n = min(n, len(pl.Hand))
	if n <= 0 {
		return nil
	}
	if n == len(pl.Hand) {
		var ev []Event
		for len(pl.Hand) > 0 {
			ev = append(ev, g.discardForBonus(p, pl.Hand[0].UID, bonus)...)
		}
		return ev
	}
	g.ask(Choice{Player: p, Kind: ChoiceDiscard, Card: c.cardID(), Count: n, Bonus: bonus})
	return nil
}

// discardForBonus discards a card from hand; if bonus is set and the card is
// a Rainbow card, p gains that many Actions (Fiddling Ferret).
func (g *Game) discardForBonus(p PlayerID, uid, bonus int) []Event {
	rainbow := false
	for _, in := range g.State.Players[p].Hand {
		if in.UID == uid {
			rainbow = g.rainbowCard(in)
		}
	}
	ev := g.discardFromHand(p, uid)
	if bonus > 0 && rainbow && len(ev) > 0 {
		g.State.ActionsLeft += bonus
		ev = append(ev, Event{Kind: EvActions, Player: p, Amount: bonus})
	}
	return ev
}

// putBuilding puts a Building that is not coming from the hand into play.
func (g *Game) putBuilding(p PlayerID, in Instance, lane int) []Event {
	ev := g.place(p, in, lane)
	ev = append(ev, Event{Kind: EvPlay, Player: p, Card: in.UID, Lane: lane})
	return append(ev, g.fire(card.OnEnterPlay, g.placedCtx(p, lane), g.Card(in).Effects, nil)...)
}

// inDiscard finds a card in p's discard pile.
func (g *Game) inDiscard(p PlayerID, uid int) (Instance, bool) {
	for _, in := range g.State.Players[p].Discard {
		if in.UID == uid {
			return in, true
		}
	}
	return Instance{}, false
}

// searchBuilding (Deforestation) lets p pick a Building from their deck to put
// into play in a Lane. The deck is shuffled afterwards, even if there was none.
func (g *Game) searchBuilding(p PlayerID, lane int, c effCtx) {
	pl := &g.State.Players[p]
	var found []Instance
	for _, in := range pl.Deck {
		if g.Card(in).Type == card.Building {
			found = append(found, in)
		}
	}
	if len(found) == 0 {
		g.shuffle(pl.Deck)
		return
	}
	g.ask(Choice{Player: p, Kind: ChoiceSearch, Card: c.cardID(), Lane: lane, Revealed: found})
}

// startMoveDamage (Friendship Bracelet) moves damage from p's Creature in
// one Lane to their Creature in another. With a choice of amounts, p is asked.
func (g *Game) startMoveDamage(p PlayerID, from, to int, c effCtx) []Event {
	a, b := g.creatureAt(pos{p, from}), g.creatureAt(pos{p, to})
	if a == nil || b == nil || a == b || a.Damage == 0 {
		return nil
	}
	if a.Damage == 1 {
		return g.moveDamage(p, from, to, 1)
	}
	g.ask(Choice{Player: p, Kind: ChoiceMoveDamage, Card: c.cardID(), Lane: from, Other: to, Count: a.Damage})
	return nil
}

// moveDamage takes n damage off p's Creature in from and puts it on the one in to.
func (g *Game) moveDamage(p PlayerID, from, to, n int) []Event {
	a, b := g.creatureAt(pos{p, from}), g.creatureAt(pos{p, to})
	if a == nil || b == nil || n < 1 || n > a.Damage {
		return nil
	}
	a.Damage -= n
	b.Damage += n
	return []Event{
		{Kind: EvHeal, Player: p, Card: a.UID, Lane: from, Amount: n},
		{Kind: EvDamage, Player: p, Card: b.UID, Lane: to, Amount: n},
	}
}

// askRedirect (Pied Piper): when the Creature that attacks fights a Creature,
// its controller may pick a Flooped Creature of theirs to fight it instead.
func (g *Game) askRedirect(c effCtx) []Event {
	if !c.hasSrc || g.creatureAt(pos{c.p.Opp(), c.at.Lane}) == nil {
		return nil
	}
	var cands []Instance
	for lane, l := range g.State.Players[c.p].Lanes {
		if cr := l.Creature; cr != nil && cr.Status == Flooped && lane != c.at.Lane {
			cands = append(cands, cr.Instance)
		}
	}
	if len(cands) > 0 {
		g.ask(Choice{Player: c.p, Kind: ChoiceRedirect, Card: c.cardID(), Lane: c.at.Lane, Revealed: cands})
	}
	return nil
}

// upkeep (Steakchop): the controller discards n cards or destroys the card
// that asks. With too few cards in hand to discard there is nothing to choose.
func (g *Game) upkeep(n int, c effCtx) []Event {
	if c.creature == nil || g.creatureAt(c.at) != c.creature {
		return nil
	}
	if len(g.State.Players[c.p].Hand) < n {
		return g.destroyAt(c.at)
	}
	g.ask(Choice{Player: c.p, Kind: ChoiceUpkeep, Card: c.cardID(), Count: n, Lane: c.at.Lane, SourceUID: c.creature.UID})
	return nil
}

// destroyAt destroys the Creature at at, as the destroy_creature op does.
func (g *Game) destroyAt(at pos) []Event {
	cr := g.creatureAt(at)
	if cr == nil {
		return nil
	}
	ev := []Event{{Kind: EvDie, Player: at.P, Card: cr.UID, Lane: at.Lane}}
	return append(ev, g.removeCreature(at, toDiscard)...)
}

// swapCreatures exchanges the Creatures in two Lanes of one side. Both count
// as having changed Lanes.
func (g *Game) swapCreatures(a, b pos) []Event {
	if a.P != b.P || a.Lane == b.Lane {
		return nil
	}
	la, lb := &g.State.Players[a.P].Lanes[a.Lane], &g.State.Players[b.P].Lanes[b.Lane]
	if la.Creature == nil || lb.Creature == nil {
		return nil
	}
	la.Creature, lb.Creature = lb.Creature, la.Creature
	la.Creature.ChangedLanes, lb.Creature.ChangedLanes = true, true
	ev := []Event{
		{Kind: EvMove, Player: a.P, Card: la.Creature.UID, Lane: a.Lane},
		{Kind: EvMove, Player: b.P, Card: lb.Creature.UID, Lane: b.Lane},
	}
	for _, at := range []pos{a, b} {
		cr := g.creatureAt(at)
		ev = append(ev, g.fire(card.OnChangeLanes, effCtx{p: at.P, at: at, hasSrc: true, creature: cr}, g.effectsOf(cr), nil)...)
	}
	return ev
}

// discardFromHand moves one of p's hand cards to its owner's discard pile.
func (g *Game) discardFromHand(p PlayerID, uid int) []Event {
	pl := &g.State.Players[p]
	for i, in := range pl.Hand {
		if in.UID == uid {
			pl.Hand = append(pl.Hand[:i:i], pl.Hand[i+1:]...)
			g.State.Players[in.Owner].Discard = append(g.State.Players[in.Owner].Discard, in)
			return []Event{{Kind: EvDiscard, Player: p, Card: in.UID}}
		}
	}
	return nil
}

// ogreGas reveals the top n cards of p's deck: p chooses one to put on the
// bottom and the rest are discarded.
func (g *Game) ogreGas(p PlayerID, n int, c effCtx) {
	pl := &g.State.Players[p]
	n = min(n, len(pl.Deck))
	if n == 0 {
		return
	}
	top := append([]Instance(nil), pl.Deck[:n]...)
	pl.Deck = pl.Deck[n:]
	if n == 1 { // nothing to choose
		pl.Deck = append(pl.Deck, top[0])
		return
	}
	g.ask(Choice{Player: p, Kind: ChoiceReveal, Card: c.cardID(), Revealed: top})
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
	ev := []Event{{Kind: EvReturn, Player: c.p, Card: in.UID}}
	if g.Card(in).Type == card.Creature {
		ev = append(ev, g.noteReturn(c.p)...)
	}
	return ev
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
	// Whatever was put underneath it is discarded.
	for _, in := range cr.Under {
		g.State.Players[in.Owner].Discard = append(g.State.Players[in.Owner].Discard, in)
	}
	// A Camel of its owner's may take it instead; the owner is asked once it has gone.
	camels := g.camelLanes(cr)
	var ev []Event
	if len(camels) == 0 {
		ev = g.settleLeaver(cr, at.Lane, to)
	}
	ctx := effCtx{p: at.P, at: at, hasSrc: true, creature: cr}
	ev = append(ev, g.fire(card.OnLeavePlay, ctx, g.effectsOf(cr), nil)...)
	if len(camels) > 0 {
		hand := 0
		if to == toHand {
			hand = 1
		}
		g.ask(Choice{Player: cr.Owner, Kind: ChoiceCamel, Card: g.effectsCamel(camels, cr.Owner), Count: hand, Lane: at.Lane,
			MoveSide: at.P, Lanes: camels, Revealed: []Instance{cr.Instance}})
		return ev
	}
	if to == toDiscard {
		ev = append(ev, g.fire(card.OnDiscarded, ctx, g.effectsOf(cr), nil)...)
	}
	return ev
}

// moveCreature moves a Creature to an empty Lane. Within one side that is a
// plain Lane change; onto the other player's side it changes control: the
// Creature stays owned by its owner but is now exhausted until its new
// controller's next turn (Field Reaper).
func (g *Game) moveCreature(from, to pos) []Event {
	src := &g.State.Players[from.P].Lanes[from.Lane]
	dst := &g.State.Players[to.P].Lanes[to.Lane]
	cr := src.Creature
	if cr == nil || dst.Creature != nil || src == dst {
		return nil
	}
	src.Creature = nil
	dst.Creature = cr
	cr.ChangedLanes = true
	if from.P != to.P {
		cr.Status = Fought
	}
	ev := []Event{{Kind: EvMove, Player: to.P, Card: cr.UID, Lane: to.Lane}}
	ctx := effCtx{p: to.P, at: to, hasSrc: true, creature: cr}
	return append(ev, g.fire(card.OnChangeLanes, ctx, g.effectsOf(cr), nil)...)
}

// moveBuilding moves a Building to a Lane on its controller's side that has none.
func (g *Game) moveBuilding(from pos, lane int) []Event {
	pl := &g.State.Players[from.P]
	b := pl.Lanes[from.Lane].Building
	if b == nil || lane == from.Lane || pl.Lanes[lane].Building != nil {
		return nil
	}
	pl.Lanes[from.Lane].takeBuilding()
	pl.Lanes[lane].Building = b
	return []Event{{Kind: EvMove, Player: from.P, Card: b.UID, Lane: lane}}
}

// copyFloop (Schoolhouse): the controller's Creature in the source's Lane
// loses all abilities and gains the FLOOP ability of a random Creature with
// one from the controller's discard pile, until end of turn. With no such
// Creature in the discard pile nothing happens.
func (g *Game) copyFloop(c effCtx) []Event {
	if !c.hasSrc {
		return nil
	}
	at := pos{c.p, c.at.Lane}
	cr := g.creatureAt(at)
	if cr == nil {
		return nil
	}
	var donors []string
	for _, in := range g.State.Players[c.p].Discard {
		cd := g.Card(in)
		if cd.Type != card.Creature {
			continue
		}
		for _, e := range cd.Effects {
			if e.Kind == card.KindFloop {
				donors = append(donors, in.CardID)
				break
			}
		}
	}
	if len(donors) == 0 {
		return nil
	}
	cr.Silenced = true
	cr.Lent = donors[rand.New(&g.rng).IntN(len(donors))]
	return []Event{{Kind: EvAbilities, Player: c.p, Card: cr.UID, Lane: at.Lane}}
}

// stealPlay (Silo of Truth) takes a random card from the opponent's hand. The
// controller must then play it for free. A card that cannot legally be played
// anywhere goes to its owner's discard pile instead.
func (g *Game) stealPlay(c effCtx) []Event {
	opp := &g.State.Players[c.p.Opp()]
	if len(opp.Hand) == 0 {
		return nil
	}
	i := rand.New(&g.rng).IntN(len(opp.Hand))
	in := opp.Hand[i]
	opp.Hand = append(opp.Hand[:i:i], opp.Hand[i+1:]...)
	ev := []Event{{Kind: EvSteal, Player: c.p, Card: in.UID}}
	g.State.Pending = &Pending{Card: in}
	if len(g.pendingActions(c.p)) == 0 {
		g.State.Pending = nil
		g.State.Players[in.Owner].Discard = append(g.State.Players[in.Owner].Discard, in)
		ev = append(ev, Event{Kind: EvDiscard, Player: in.Owner, Card: in.UID})
	}
	return ev
}
