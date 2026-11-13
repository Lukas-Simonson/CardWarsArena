package main

import (
	"encoding/json"
	"fmt"
	"strings"
	"unicode/utf8"

	"cardwars/internal/card"
	"cardwars/internal/engine"
)

const cellWidth = 21

func pad(s string, n int) string {
	if w := utf8.RuneCountInString(s); w > n {
		r := []rune(s)
		return string(r[:n-1]) + "…"
	}
	return s + strings.Repeat(" ", n-utf8.RuneCountInString(s))
}

// creatureCell is the three lines shown for a Creature.
func (u *ui) creatureCell(p engine.PlayerID, lane int) [3]string {
	c := u.g.State.Players[p].Lanes[lane].Creature
	if c == nil {
		return [3]string{"·", "", ""}
	}
	state := map[engine.Status]string{engine.Ready: "ready", engine.Flooped: "FLOOPED", engine.Fought: "tired"}[c.Status]
	line2 := fmt.Sprintf("%d/%d dmg%d %s", u.g.Attack(p, lane), u.g.Defense(p, lane), c.Damage, state)
	var tags []string
	if c.Owner != p {
		tags = append(tags, "stolen")
	}
	if c.Silenced {
		if c.Lent != "" {
			tags = append(tags, "lent:"+u.cardName(c.Lent))
		} else {
			tags = append(tags, "silenced")
		}
	}
	if n := len(c.Under); n > 0 {
		tags = append(tags, fmt.Sprintf("under:%d", n))
	}
	if !u.g.CanBeAttacked(p, lane) {
		tags = append(tags, "protected")
	}
	return [3]string{u.cardName(c.CardID), line2, strings.Join(tags, " ")}
}

func (u *ui) buildingCell(p engine.PlayerID, lane int) string {
	b := u.g.State.Players[p].Lanes[lane].Building
	if b == nil {
		return "·"
	}
	s := u.cardName(b.CardID)
	if b.Status == engine.Flooped {
		s += " (flooped)"
	}
	return s
}

func (u *ui) landscapeCell(p engine.PlayerID, lane int) string {
	l := u.g.State.Players[p].Lanes[lane]
	name := landscapeName(l.Landscape)
	if l.FaceDown {
		name = "▼ face down"
	}
	if l.Frozen {
		name += " ❄"
	}
	return name
}

// row prints one labelled row across the four lanes.
func row(b *strings.Builder, label string, cells [engine.NumLanes]string) {
	fmt.Fprintf(b, "  %-5s", label)
	for _, c := range cells {
		b.WriteString(" │ " + pad(c, cellWidth))
	}
	b.WriteString(" │\n")
}

func (u *ui) playerLine(p engine.PlayerID, me engine.PlayerID) string {
	pl := &u.g.State.Players[p]
	tag := "opp"
	if p == me {
		tag = "you"
	}
	return fmt.Sprintf("%s (%s)  HP %d   hand %d · deck %d · discard %d", u.who(p), tag, pl.HP, len(pl.Hand), len(pl.Deck), len(pl.Discard))
}

// board draws both sides with me at the bottom.
func (u *ui) board(me engine.PlayerID) string {
	st := &u.g.State
	opp := me.Opp()
	var b strings.Builder

	turn := fmt.Sprintf("Turn %d · %s's turn · %s phase", st.Turn, u.who(st.Active), strings.ToUpper(string(st.Phase)))
	if st.Phase == engine.PhaseMain {
		turn += fmt.Sprintf(" · Actions %d", st.ActionsLeft)
	}
	if st.Phase == engine.PhaseMulligan {
		turn = "Opening hands"
	}
	b.WriteString("══ " + turn + " " + strings.Repeat("═", max(0, 78-utf8.RuneCountInString(turn))) + "\n")
	b.WriteString(u.playerLine(opp, me) + "\n")

	var heads, oland, obld, ocr, ocr2, ocr3, mcr, mcr2, mcr3, mbld, mland [engine.NumLanes]string
	for lane := 0; lane < engine.NumLanes; lane++ {
		heads[lane] = fmt.Sprintf("LANE %d", lane+1)
		oland[lane] = u.landscapeCell(opp, lane)
		obld[lane] = u.buildingCell(opp, lane)
		c := u.creatureCell(opp, lane)
		ocr[lane], ocr2[lane], ocr3[lane] = c[0], c[1], c[2]
		m := u.creatureCell(me, lane)
		mcr[lane], mcr2[lane], mcr3[lane] = m[0], m[1], m[2]
		mbld[lane] = u.buildingCell(me, lane)
		mland[lane] = u.landscapeCell(me, lane)
	}
	row(&b, "", heads)
	row(&b, "land", oland)
	row(&b, "bldg", obld)
	row(&b, "crea", ocr)
	row(&b, "", ocr2)
	row(&b, "", ocr3)
	b.WriteString("  " + strings.Repeat("─", 6+4*(cellWidth+3)+1) + "\n")
	row(&b, "crea", mcr)
	row(&b, "", mcr2)
	row(&b, "", mcr3)
	row(&b, "bldg", mbld)
	row(&b, "land", mland)
	b.WriteString(u.playerLine(me, me) + "\n")
	if st.Pending != nil {
		fmt.Fprintf(&b, "!! %s stole %s and must play it for free.\n", u.who(st.Active), u.g.Card(st.Pending.Card).Name)
	}
	if ch := st.Choice; ch != nil {
		fmt.Fprintf(&b, "!! %s must decide (%s, from %s).\n", u.who(ch.Player), ch.Kind, u.g.Catalog()[ch.Card].Name)
	}
	return b.String()
}

func cardStats(c *card.Card) string {
	s := string(c.Type)
	if c.Landscape != "" {
		s = landscapeName(c.Landscape) + " " + s
	}
	if c.Cost != nil {
		s = fmt.Sprintf("cost %d · %s", *c.Cost, s)
	}
	if c.Attack != nil || c.Defense != nil {
		a, d := 0, 0
		if c.Attack != nil {
			a = *c.Attack
		}
		if c.Defense != nil {
			d = *c.Defense
		}
		s += fmt.Sprintf(" · %d/%d", a, d)
	}
	return s
}

// hand lists the cards in p's hand, numbered.
func (u *ui) hand(p engine.PlayerID) string {
	pl := &u.g.State.Players[p]
	if len(pl.Hand) == 0 {
		return "Your hand is empty.\n"
	}
	var b strings.Builder
	fmt.Fprintf(&b, "Your hand (%d):\n", len(pl.Hand))
	for i, in := range pl.Hand {
		c := u.g.Card(in)
		fmt.Fprintf(&b, "  %2d. %-24s %s\n", i+1, c.Name, cardStats(c))
		if c.Text != "" {
			fmt.Fprintf(&b, "      %s\n", c.Text)
		}
	}
	return b.String()
}

func (u *ui) discard(p engine.PlayerID) string {
	pl := &u.g.State.Players[p]
	if len(pl.Discard) == 0 {
		return u.who(p) + "'s discard pile is empty.\n"
	}
	var names []string
	for _, in := range pl.Discard {
		names = append(names, u.g.Card(in).Name)
	}
	return fmt.Sprintf("%s's discard pile (%d): %s\n", u.who(p), len(names), strings.Join(names, ", "))
}

// inspect describes a card by catalog entry, including how the engine models it.
func (u *ui) inspect(c *card.Card) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s  [%s]\n  %s\n", c.Name, c.ID, cardStats(c))
	if c.Text != "" {
		fmt.Fprintf(&b, "  “%s”\n", c.Text)
	}
	switch {
	case c.Modeled && len(c.Effects) > 0:
		js, _ := json.Marshal(c.Effects)
		fmt.Fprintf(&b, "  engine: modelled · %s\n", js)
	case c.Modeled:
		b.WriteString("  engine: modelled · no rules effects\n")
	case c.Text == "":
		b.WriteString("  engine: no rules text\n")
	default:
		b.WriteString("  engine: NOT MODELLED (text is ignored; decks with it need --allow-unmodelled)\n")
	}
	return b.String()
}
