package engine

import "cardwars/internal/card"

// A Question is an open Choice described in generic terms, so a client needs no code for each kind of choice: who
// asked, what the question says, where it is answered, and the options. Every answer is the one action
// {choose, option: id}. See docs/protocol.md ("Questions") and web/docs/interactions.md for the rules it follows.
//
// The Choice and its handlers are the engine's rules and are unchanged; describeChoice is the one place that turns
// each kind into a Question and says which legacy answer an option stands for.

// Msg is words, not English: a message key with parameters, which the client words from its catalogue (the way an
// error is sent as a code). Text, when set, is an English label owned by a card's data ("Don't freeze a Landscape"),
// which the client looks up in the catalogue and falls back to as written. A parameter whose name ends in "_card" is
// a card id, and the client words it as that card's name.
type Msg struct {
	Key    string         `json:"key,omitempty"`
	Params map[string]any `json:"params,omitempty"`
	Text   string         `json:"text,omitempty"`
}

// Answer says where a question is answered (the rules W1-W4 of the interaction guide).
const (
	AnswerBoard   = "board"   // by clicking board objects: the options' Ref
	AnswerHand    = "hand"    // by clicking cards in your hand: the options' Ref (kind hand_card); other options are buttons
	AnswerCards   = "cards"   // in a pop-up of cards that are not on the table: the options' Card
	AnswerButtons = "buttons" // by pressing a button: the options' Label
)

// Option is one thing the player may answer with. ID is what they send back. An option has a Ref (a board object, or
// a card of the hand) to click, or a Card to pick in a pop-up, or a Label to press; Role is "danger" when it costs
// them something, and Result says what it does where that is known.
type Option struct {
	ID     int        `json:"id"`
	Ref    *TargetRef `json:"ref,omitempty"`
	Card   *Instance  `json:"card,omitempty"`
	Label  *Msg       `json:"label,omitempty"`
	Role   string     `json:"role,omitempty"`
	Result *Msg       `json:"result,omitempty"`
}

// Question is what a player is asked.
type Question struct {
	Player  PlayerID  `json:"player"`
	Card    string    `json:"card"`              // the card that asked, always shown
	Subject *Instance `json:"subject,omitempty"` // a second card it is about (the Creature leaving play, the Building to place)
	Text    Msg       `json:"text"`
	Answer  string    `json:"answer"`
	// Options are the answers; Skip is the one that declines (it has an ID like any other).
	Options []Option `json:"options"`
	Skip    *Option  `json:"skip,omitempty"`
	// Left is how many more picks a multi-card choice still wants (discard 2: 2, then 1).
	Left int `json:"left,omitempty"`
}

func key(k string, kv ...any) *Msg {
	m := &Msg{Key: k}
	for i := 0; i+1 < len(kv); i += 2 {
		if m.Params == nil {
			m.Params = map[string]any{}
		}
		m.Params[kv[i].(string)] = kv[i+1]
	}
	return m
}

// describeChoice turns the open Choice into a Question for its decider, and lists the answers its options stand
// for, in the same order: the option with ID i is answers[i].
func (g *Game) describeChoice(ch *Choice) (Question, []Action) {
	p := ch.Player
	q := Question{Player: p, Card: ch.Card, Answer: AnswerButtons}
	var answers []Action
	add := func(a Action, o Option) *Option {
		o.ID = len(answers)
		answers = append(answers, a)
		return &o
	}
	option := func(a Action, o Option) { q.Options = append(q.Options, *add(a, o)) }
	handCard := func(uid int) Option { return Option{Ref: &TargetRef{Kind: "hand_card", Player: p, Card: uid}} }
	creatureRef := func(lane int) *TargetRef { return &TargetRef{Kind: "creature", Player: p, Lane: lane} }
	pick := func(in Instance) Option { c := in; return Option{Card: &c} }
	switch ch.Kind {
	case ChoiceDiscard:
		q.Text, q.Answer, q.Left = *key("question.discard", "n", ch.Count), AnswerHand, ch.Count
		for _, in := range g.State.Players[p].Hand {
			if ch.OnlyCost == nil || g.handCost(in) == *ch.OnlyCost {
				option(Action{Kind: ActChoose, Card: in.UID}, handCard(in.UID))
			}
		}
	case ChoiceRansom:
		q.Text, q.Answer = *key("question.ransom", "min", *ch.MinCost), AnswerHand
		for _, in := range g.State.Players[p].Hand {
			if g.handCost(in) >= *ch.MinCost {
				option(Action{Kind: ActChoose, Card: in.UID}, handCard(in.UID))
			}
		}
		option(Action{Kind: ActChoose}, Option{Label: key("question.take_it"), Role: "danger", Result: describeOps(ch.Do)}) // take what comes instead
	case ChoiceUpkeep:
		q.Text, q.Answer = *key("question.upkeep", "n", ch.Count), AnswerHand
		for _, in := range g.State.Players[p].Hand {
			option(Action{Kind: ActChoose, Card: in.UID}, handCard(in.UID))
		}
		option(Action{Kind: ActChoose}, Option{Label: key("question.destroy"), Role: "danger"}) // destroy it instead
	case ChoiceRebound:
		q.Text, q.Answer = *key("question.rebound"), AnswerHand
		for _, in := range g.State.Players[p].Hand {
			option(Action{Kind: ActChoose, Card: in.UID}, handCard(in.UID))
		}
		q.Skip = add(Action{Kind: ActChoose}, Option{Label: key("question.leave_it")}) // keep it in the discard pile
	case ChoiceReveal, ChoiceSearch:
		q.Text, q.Answer = *key("question.reveal"), AnswerCards
		if ch.Kind == ChoiceSearch {
			q.Text = *key("question.search")
		}
		for _, in := range ch.Revealed {
			option(Action{Kind: ActChoose, Card: in.UID}, pick(in))
		}
	case ChoicePlace:
		q.Text, q.Answer = *key("question.place"), AnswerBoard
		if len(ch.Revealed) > 0 {
			b := ch.Revealed[0]
			q.Subject = &b
		}
		for _, lane := range ch.Lanes {
			option(Action{Kind: ActChoose, Lane: lane}, Option{Ref: &TargetRef{Kind: "lane", Player: p, Lane: lane}})
		}
	case ChoiceRedirect, ChoiceDefend:
		q.Text, q.Answer = *key("question.redirect"), AnswerBoard
		skip := "question.redirect_skip"
		if ch.Kind == ChoiceDefend {
			q.Text, skip = *key("question.defend"), "question.defend_skip"
			if at := g.creatureAt(pos{ch.MoveSide, ch.Other}); at != nil {
				in := at.Instance
				q.Subject = &in // the Creature that attacks
			}
		}
		for _, in := range ch.Revealed {
			for lane, l := range g.State.Players[p].Lanes {
				if l.Creature != nil && l.Creature.UID == in.UID {
					option(Action{Kind: ActChoose, Card: in.UID}, Option{Ref: creatureRef(lane)})
				}
			}
		}
		q.Skip = add(Action{Kind: ActChoose}, Option{Label: key(skip)})
	case ChoiceMoveDamage:
		from, to := g.creatureAt(pos{p, ch.Lane}), g.creatureAt(pos{p, ch.Other})
		q.Text = *key("question.move_damage", "from_card", cardIDOf(from), "to_card", cardIDOf(to))
		for n := 1; n <= ch.Count; n++ {
			o := Option{Label: key("question.move_n", "n", n)}
			if from != nil && to != nil {
				o.Result = key("question.move_result", "from_card", from.CardID, "a1", from.Damage, "a2", from.Damage-n, "to_card", to.CardID, "b1", to.Damage, "b2", to.Damage+n)
			}
			option(Action{Kind: ActChoose, Card: n}, o)
		}
	case ChoiceMove:
		side := "question.move_mine"
		if ch.MoveSide != p {
			side = "question.move_theirs"
		}
		q.Text, q.Answer = *key(side), AnswerBoard
		for _, lane := range ch.Lanes {
			option(Action{Kind: ActChoose, Lane: lane}, Option{Ref: &TargetRef{Kind: "landscape", Player: ch.MoveSide, Lane: lane}})
		}
	case ChoiceCamel:
		if len(ch.Revealed) > 0 {
			in := ch.Revealed[0]
			q.Subject = &in
		}
		q.Text = *key("question.camel")
		var camels []int
		for _, lane := range ch.Lanes {
			if cm := g.State.Players[p].Lanes[lane].Creature; cm != nil {
				camels = append(camels, lane)
			}
		}
		for _, lane := range camels {
			label := key("question.camel_put")
			if len(camels) > 1 {
				label = key("question.camel_put_lane", "lane", lane+1)
			}
			option(Action{Kind: ActChoose, Card: g.State.Players[p].Lanes[lane].Creature.UID}, Option{Label: label})
		}
		q.Skip = add(Action{Kind: ActChoose}, Option{Label: key("question.camel_skip")}) // let it go
	case ChoiceLandscape:
		q.Text = *key("question.landscape", "lane", ch.Lane+1)
		for i, ls := range card.Landscapes {
			option(Action{Kind: ActChoose, Card: i + 1}, Option{Label: key("landscape." + string(ls))})
		}
	case ChoiceTarget:
		what := "target" // what the first option is, for "Choose a Landscape"; "target" when it is none of those
		if len(ch.Targets) > 0 {
			what = ch.Targets[0].Kind
		}
		params := []any{"optional", ch.Optional, "what", what}
		if ch.Label != "" {
			params = append(params, "label", ch.Label)
		}
		q.Text, q.Answer = *key("question.target", params...), AnswerBoard
		for _, r := range ch.Targets {
			r := r
			option(Action{Kind: ActChoose, Targets: [][]TargetRef{{r}}}, Option{Ref: &r})
		}
		if ch.Optional {
			label := &Msg{Key: "skip.none"}
			if ch.SkipLabel != "" {
				label = &Msg{Text: ch.SkipLabel}
			}
			q.Skip = add(Action{Kind: ActChoose}, Option{Label: label}) // choose nothing
		}
	case ChoiceAmount:
		q.Text = *key("question.amount", "options", ch.Options)
		for _, n := range ch.Options {
			option(Action{Kind: ActChoose, Card: n}, Option{Label: key("question.amount_n", "n", n, "unit", ch.Unit)})
		}
	}
	return q, answers
}

func cardIDOf(c *Creature) string {
	if c == nil {
		return ""
	}
	return c.CardID
}

// DescribeChoice is the open Choice as a Question, or nil when none is open. It has everything: a view must keep it
// from anyone but the decider.
func (g *Game) DescribeChoice() (*Question, []Action) {
	ch := g.State.Choice
	if ch == nil {
		return nil, nil
	}
	q, answers := g.describeChoice(ch)
	return &q, answers
}

// resolveOption turns an answer that names an option ({choose, option: id}) into the answer the engine's handlers take.
// Any other action is returned as it was.
func (g *Game) resolveOption(p PlayerID, a Action) (Action, error) {
	if a.Kind != ActChoose || a.Option == nil {
		return a, nil
	}
	ch := g.State.Choice
	if ch == nil {
		return a, ErrNoChoice
	}
	if p != ch.Player {
		return a, ErrNotYourTurn
	}
	_, answers := g.describeChoice(ch)
	if *a.Option < 0 || *a.Option >= len(answers) {
		return a, ErrBadTarget
	}
	return answers[*a.Option], nil
}

// describeOps says what a list of ops does to the player who is asked, for the ones a card commonly threatens them
// with (damage to their Hero, or to all their Creatures): a message to put on an option as its Result. Any other op, or
// a list of several, is not described (nil) and the option says only its label. It reads the ops as a ransom's
// "else", which happen to the opponent of the card's controller, who is the one asked.
func describeOps(ops []card.Op) *Msg {
	if len(ops) != 1 || ops[0].Amount == nil || ops[0].Amount.Per != nil || ops[0].Amount.N <= 0 {
		return nil
	}
	op := ops[0]
	switch {
	case op.Do == "damage_hero" && op.Who == "opponent":
		return key("result.damage_hero", "n", op.Amount.N)
	case op.Do == "damage_creature" && op.All != nil && op.All.Rel == "opposing":
		return key("result.damage_creatures", "n", op.Amount.N)
	}
	return nil
}
