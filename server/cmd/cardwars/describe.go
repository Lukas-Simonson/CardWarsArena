package main

import (
	"fmt"
	"strings"

	"cardwars/internal/card"
	"cardwars/internal/engine"
)

// ui turns game state into text. Lanes are shown 1-4 to people.
type ui struct {
	g      *engine.Game
	labels [2]string // display names, e.g. "Finn"
	reveal bool      // show hidden information (opponent's draws)
}

func (u *ui) cardName(id string) string {
	if c := u.g.Catalog()[id]; c != nil {
		return c.Name
	}
	return id
}

func (u *ui) instName(uid int) string {
	if id, ok := u.g.Instances()[uid]; ok {
		return u.cardName(id)
	}
	return fmt.Sprintf("card #%d", uid)
}

func (u *ui) who(p engine.PlayerID) string { return u.labels[p] }

// side names a player's side relative to the viewer.
func (u *ui) side(me, p engine.PlayerID) string {
	if me == p {
		return "your"
	}
	return "opp's"
}

var landscapeNames = map[card.Landscape]string{
	card.Cornfield: "Cornfield", card.BluePlains: "Blue Plains", card.UselessSwamp: "Useless Swamp",
	card.NiceLands: "Nice Lands", card.SandyLands: "Sandy Lands", card.IcyLands: "Icy Lands", card.Rainbow: "Rainbow",
}

func landscapeName(l card.Landscape) string {
	if n, ok := landscapeNames[l]; ok {
		return n
	}
	return string(l)
}

// ref describes one chosen target.
func (u *ui) ref(me engine.PlayerID, r engine.TargetRef) string {
	st := &u.g.State.Players[r.Player]
	switch r.Kind {
	case "creature":
		if c := st.Lanes[r.Lane].Creature; c != nil {
			return fmt.Sprintf("%s (%s, lane %d)", u.cardName(c.CardID), u.side(me, r.Player), r.Lane+1)
		}
	case "building":
		if b := st.Lanes[r.Lane].Building; b != nil {
			return fmt.Sprintf("%s (%s, lane %d)", u.cardName(b.CardID), u.side(me, r.Player), r.Lane+1)
		}
	case "landscape":
		l := st.Lanes[r.Lane]
		down := ""
		if l.FaceDown {
			down = " face-down"
		}
		return fmt.Sprintf("%s%s landscape (%s, lane %d)", landscapeName(l.Landscape), down, u.side(me, r.Player), r.Lane+1)
	case "lane":
		return fmt.Sprintf("lane %d (%s side)", r.Lane+1, u.side(me, r.Player))
	case "discard_card", "hand_card":
		return u.instName(r.Card)
	}
	return fmt.Sprintf("%s?", r.Kind)
}

func (u *ui) targets(me engine.PlayerID, slots [][]engine.TargetRef) string {
	var parts []string
	for _, slot := range slots {
		if len(slot) == 0 {
			parts = append(parts, "none")
			continue
		}
		var rs []string
		for _, r := range slot {
			rs = append(rs, u.ref(me, r))
		}
		parts = append(parts, strings.Join(rs, " + "))
	}
	return strings.Join(parts, "; ")
}

func withTargets(s, label, t string) string {
	if t == "" {
		return s
	}
	return s + "  [" + label + ": " + t + "]"
}

// action describes an action by player me in plain words.
func (u *ui) action(me engine.PlayerID, a engine.Action) string {
	st := &u.g.State
	switch a.Kind {
	case engine.ActKeepHand:
		return "Keep your opening hand"
	case engine.ActMulligan:
		return "Mulligan: shuffle your hand back and draw 5 new cards (only once)"
	case engine.ActDraw:
		return "Spend 1 Action to draw a card"
	case engine.ActEndMain:
		if st.Turn == 1 {
			return "End your turn (the first player does not Fight)"
		}
		return "Finish: go to the Fight phase"
	case engine.ActPlayCard:
		name, verb, cost, where := "?", "Play", "", ""
		for _, in := range st.Players[me].Hand {
			if in.UID == a.Card {
				c := u.g.Card(in)
				name = c.Name
				if c.Type == card.Spell {
					verb = "Cast"
				} else {
					where = fmt.Sprintf(" into lane %d", a.Lane+1)
				}
				if c.Cost != nil {
					cost = fmt.Sprintf(" (cost %d)", *c.Cost)
				}
			}
		}
		s := fmt.Sprintf("%s %s%s%s", verb, name, cost, where)
		s = withTargets(s, "discard", u.targets(me, a.CostTargets))
		return withTargets(s, "targets", u.targets(me, a.Targets))
	case engine.ActChoose:
		ch := st.Choice
		if ch == nil {
			return "Choose"
		}
		switch ch.Kind {
		case engine.ChoiceUpkeep:
			if a.Card == 0 {
				return "Destroy " + u.g.Catalog()[ch.Card].Name
			}
			return "Discard " + u.instName(a.Card)
		case engine.ChoiceDiscard:
			return "Discard " + u.instName(a.Card)
		case engine.ChoiceReveal:
			return "Put " + u.instName(a.Card) + " on the bottom of the deck"
		case engine.ChoiceSearch:
			return "Put " + u.instName(a.Card) + " into play from the deck"
		case engine.ChoiceRebound:
			if a.Card == 0 {
				return "Leave " + u.g.Catalog()[ch.Card].Name + " in the discard pile"
			}
			return "Discard " + u.instName(a.Card) + " to return " + u.g.Catalog()[ch.Card].Name + " to your hand"
		case engine.ChoiceRedirect:
			if a.Card == 0 {
				return u.g.Catalog()[ch.Card].Name + " fights"
			}
			return u.instName(a.Card) + " fights instead"
		case engine.ChoiceMoveDamage:
			return fmt.Sprintf("Move %d damage", a.Card)
		case engine.ChoiceMove:
			return fmt.Sprintf("Move %s to lane %d (%s side)", u.g.Catalog()[ch.Card].Name, a.Lane+1, u.side(me, ch.MoveSide))
		}
		return "Choose"
	case engine.ActThaw:
		return fmt.Sprintf("Discard %s to remove the frozen token on lane %d", u.instName(a.Card), a.Lane+1)
	case engine.ActPlayPending:
		name := "the stolen card"
		if st.Pending != nil {
			name = u.g.Card(st.Pending.Card).Name
		}
		where := ""
		if st.Pending != nil && u.g.Card(st.Pending.Card).Type != card.Spell {
			where = fmt.Sprintf(" into lane %d", a.Lane+1)
		}
		return withTargets(fmt.Sprintf("Play %s for free%s", name, where), "targets", u.targets(me, a.Targets))
	case engine.ActAttack:
		c := st.Players[me].Lanes[a.Lane].Creature
		name := "?"
		if c != nil {
			name = u.cardName(c.CardID)
		}
		target := "attacks the opposing Hero directly"
		if o := st.Players[me.Opp()].Lanes[a.Lane].Creature; o != nil {
			target = "fights " + u.cardName(o.CardID)
		}
		return fmt.Sprintf("Fight in lane %d: %s %s", a.Lane+1, name, target)
	case engine.ActFloop, engine.ActActivate:
		if a.Source == "hero" {
			return withTargets(fmt.Sprintf("Use %s's ability", u.cardName(st.Players[me].Hero.CardID)), "targets", u.targets(me, a.Targets))
		}
		l := st.Players[me].Lanes[a.Lane]
		name := "?"
		if a.Source == "building" && l.Building != nil {
			name = u.cardName(l.Building.CardID)
		} else if l.Creature != nil {
			name = u.cardName(l.Creature.CardID)
		}
		verb := fmt.Sprintf("Floop %s (lane %d)", name, a.Lane+1)
		if a.Kind == engine.ActFloop && a.Via == "hero" {
			verb = fmt.Sprintf("Floop %s (lane %d) with your Hero's Floop", name, a.Lane+1)
		}
		if a.Kind == engine.ActActivate {
			verb = fmt.Sprintf("Use %s's ability (lane %d)", name, a.Lane+1)
		}
		return withTargets(verb, "targets", u.targets(me, a.Targets))
	}
	return string(a.Kind)
}

// event describes something that happened, as seen by player me.
func (u *ui) event(me engine.PlayerID, e engine.Event) string {
	name := func() string { return u.instName(e.Card) }
	lane := e.Lane + 1
	switch e.Kind {
	case engine.EvMulligan:
		return fmt.Sprintf("%s takes a mulligan", u.who(e.Player))
	case engine.EvKeepHand:
		return fmt.Sprintf("%s keeps their hand", u.who(e.Player))
	case engine.EvTurnStart:
		return fmt.Sprintf("── %s's turn ──", u.who(e.Player))
	case engine.EvDraw:
		if e.Player == me || u.reveal {
			return fmt.Sprintf("%s draws %s", u.who(e.Player), name())
		}
		return fmt.Sprintf("%s draws a card", u.who(e.Player))
	case engine.EvPlay:
		return fmt.Sprintf("%s plays %s", u.who(e.Player), name())
	case engine.EvReplace:
		return fmt.Sprintf("%s is replaced in lane %d", name(), lane)
	case engine.EvAttack:
		return fmt.Sprintf("%s fights with %s (lane %d)", u.who(e.Player), name(), lane)
	case engine.EvDamage:
		return fmt.Sprintf("%s takes %d damage", name(), e.Amount)
	case engine.EvHPLoss:
		return fmt.Sprintf("%s loses %d HP", u.who(e.Player), e.Amount)
	case engine.EvHPGain:
		return fmt.Sprintf("%s heals %d HP", u.who(e.Player), e.Amount)
	case engine.EvHeal:
		return fmt.Sprintf("%s heals %d damage", name(), e.Amount)
	case engine.EvDie:
		return fmt.Sprintf("%s is destroyed (lane %d)", name(), lane)
	case engine.EvGameOver:
		return fmt.Sprintf("*** %s wins! ***", u.who(e.Player))
	case engine.EvMove:
		return fmt.Sprintf("%s moves to lane %d", name(), lane)
	case engine.EvFreeze:
		return fmt.Sprintf("%s's lane %d Landscape is frozen", u.who(e.Player), lane)
	case engine.EvThaw:
		return fmt.Sprintf("%s's lane %d Landscape thaws", u.who(e.Player), lane)
	case engine.EvFlip:
		return fmt.Sprintf("%s's lane %d Landscape is flipped", u.who(e.Player), lane)
	case engine.EvReady:
		return fmt.Sprintf("%s is readied", name())
	case engine.EvBounce:
		return fmt.Sprintf("%s returns to %s's hand", name(), u.who(e.Player))
	case engine.EvDestroy:
		return fmt.Sprintf("%s is destroyed (lane %d)", name(), lane)
	case engine.EvReturn:
		return fmt.Sprintf("%s returns %s from the discard pile to hand", u.who(e.Player), name())
	case engine.EvDeckTop:
		return fmt.Sprintf("%s puts %s on top of their deck", u.who(e.Player), name())
	case engine.EvActions:
		return fmt.Sprintf("%s gains %d Action(s)", u.who(e.Player), e.Amount)
	case engine.EvFloop:
		return fmt.Sprintf("%s Floops %s (lane %d)", u.who(e.Player), name(), lane)
	case engine.EvActivate:
		return fmt.Sprintf("%s uses %s's ability (lane %d)", u.who(e.Player), name(), lane)
	case engine.EvDiscard:
		return fmt.Sprintf("%s discards %s", u.who(e.Player), name())
	case engine.EvSteal:
		return fmt.Sprintf("%s steals %s from the opponent's hand", u.who(e.Player), name())
	case engine.EvAbilities:
		return fmt.Sprintf("%s's abilities change (lane %d)", name(), lane)
	}
	return string(e.Kind)
}
