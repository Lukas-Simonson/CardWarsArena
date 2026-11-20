package engine

import "cardwars/internal/card"

// Fern's deck: Gobble and Whump, attack tokens, and the rest of what the Armourdillos and Cobnoblins do.

// whumpDiscount is how many Actions and Landscapes less each Creature with Whump p plays costs, for the
// Polished Armourdillos p controls.
func (g *Game) whumpDiscount(p PlayerID) int {
	n := 0
	for i := range g.State.Players[p].Lanes {
		if cr := g.State.Players[p].Lanes[i].Creature; cr != nil && hasRestrict(g.effectsOf(cr), card.RestrictWhumpDiscount) {
			n++
		}
	}
	return n
}

// addTokens puts n attack tokens on the Creature at at.
func (g *Game) addTokens(at pos, n int) {
	if cr := g.creatureAt(at); cr != nil && n > 0 {
		cr.Tokens += n
	}
}

// removeTokens takes up to n attack tokens off the Creature at at and returns how many came off.
func (g *Game) removeTokens(at pos, n int) int {
	cr := g.creatureAt(at)
	if cr == nil || n <= 0 {
		return 0
	}
	n = min(n, cr.Tokens)
	cr.Tokens -= n
	return n
}

// defeated reports whether the damage on the Creature at at has destroyed it, though it has not been removed yet.
func (g *Game) defeated(at pos) bool {
	cr := g.creatureAt(at)
	return cr != nil && cr.Damage >= g.def(at)
}

// defeats runs what happens when the Creature of dealer, at from, has destroyed the Creature it fought: its
// defeats triggers, and the card Ragepop Totem promised.
func (g *Game) defeats(from pos) []Event {
	cr := g.creatureAt(from)
	if cr == nil {
		return nil
	}
	var ev []Event
	ctx := effCtx{p: from.P, at: from, hasSrc: true, creature: cr}
	ev = append(ev, g.fire(card.OnDefeats, ctx, g.effectsOf(cr), nil)...)
	if cr.Rage {
		cr.Rage = false
		for _, in := range g.draw(from.P, 1) {
			ev = append(ev, Event{Kind: EvDraw, Player: from.P, Card: in.UID})
		}
	}
	return ev
}

// lifesteal heals a Creature that has just dealt dmg damage when its controller has played Herbal Hurlers.
func (g *Game) lifesteal(at pos, dmg int) []Event {
	cr := g.creatureAt(at)
	if cr == nil || dmg <= 0 || !g.State.Lifesteal[at.P] {
		return nil
	}
	if healed := min(dmg, cr.Damage); healed > 0 {
		cr.Damage -= healed
		return []Event{{Kind: EvHeal, Player: at.P, Card: cr.UID, Lane: at.Lane, Amount: healed}}
	}
	return nil
}

// askDefend lets the opponent of the Creature of p in lane that has just attacked take the Fight with a Creature
// that says so (Genteel Armourdillo). It reports whether a Choice was opened; the Fight goes on once it is answered.
func (g *Game) askDefend(p PlayerID, lane, fromLane int) bool {
	opp := p.Opp()
	var cands []Instance
	for l := range g.State.Players[opp].Lanes {
		cr := g.State.Players[opp].Lanes[l].Creature
		if cr == nil || l == lane || !g.attackable(pos{opp, l}) || !hasRestrict(g.effectsOf(cr), card.RestrictRedirectAttacks) {
			continue
		}
		cands = append(cands, cr.Instance)
	}
	if len(cands) == 0 {
		return false
	}
	g.ask(Choice{Player: opp, Kind: ChoiceDefend, Card: cands[0].CardID, Lane: lane, Other: fromLane, MoveSide: p, Revealed: cands})
	return true
}

// startFight is where a Fight begins once everything that happens on attacking is done: the defending player may
// have a Creature take it instead, and otherwise the Creature of p in lane fights the one across from it.
func (g *Game) startFight(p PlayerID, lane, fromLane int) []Event {
	if g.askDefend(p, lane, fromLane) {
		return nil
	}
	return g.fight(p, lane, fromLane)
}

// answerDefend carries out a defend Choice: the Creature picked takes the Fight, or the Fight is where it was going.
func (g *Game) answerDefend(p PlayerID, ch *Choice, uid int) []Event {
	lane := ch.Lane
	if uid != 0 {
		for l, ln := range g.State.Players[p].Lanes {
			if ln.Creature != nil && ln.Creature.UID == uid {
				lane = l
			}
		}
	}
	return g.fight(ch.MoveSide, lane, ch.Other)
}

// gobbledOut sends the cards underneath a Creature that is leaving play where it is going.
func (g *Game) gobbledOut(cr *Creature, to destination) {
	for _, in := range cr.Gobbled {
		if to == toHand {
			g.State.Players[in.Owner].Hand = append(g.State.Players[in.Owner].Hand, in)
		} else {
			g.State.Players[in.Owner].Discard = append(g.State.Players[in.Owner].Discard, in)
		}
	}
}

// fireLaneLeave runs the lane_creature_leaves triggers of the Buildings in the Lane the Creature of p has left.
func (g *Game) fireLaneLeave(at pos) []Event {
	var ev []Event
	for _, b := range g.State.Players[at.P].Lanes[at.Lane].buildings() {
		ev = append(ev, g.fire(card.OnLaneCreatureLeaves, effCtx{p: at.P, at: at, hasSrc: true, card: b.CardID, building: b}, g.buildingEffects(at, b), nil)...)
	}
	return ev
}

// playTop plays the top card of p's deck for free (Gallant Armourdillo). The card is played by a pending choice
// of Lane, like Silo of Truth's, and a card that cannot be played anywhere stays on the deck.
func (g *Game) playTop(p PlayerID) []Event {
	pl := &g.State.Players[p]
	if len(pl.Deck) == 0 || g.State.Pending != nil {
		return nil
	}
	in := pl.Deck[0]
	pl.Deck = pl.Deck[1:]
	g.State.Pending = &Pending{Card: in}
	if len(g.pendingActions(p)) == 0 {
		g.State.Pending = nil
		pl.Deck = append([]Instance{in}, pl.Deck...)
		return nil
	}
	return []Event{{Kind: EvReveal, Player: p, Card: in.UID}}
}
