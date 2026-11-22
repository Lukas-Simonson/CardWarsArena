package engine

import (
	"encoding/json"
	"strings"
	"testing"
)

func optionIDs(q *Question) []int {
	var ids []int
	for _, o := range q.Options {
		ids = append(ids, o.ID)
	}
	if q.Skip != nil {
		ids = append(ids, q.Skip.ID)
	}
	return ids
}

// every option names an answer, in order, and the ids are 0, 1, 2...: a client sends one of them back.
func checkOptions(t *testing.T, g *Game) *Question {
	t.Helper()
	q, answers := g.DescribeChoice()
	if q == nil {
		t.Fatal("no question")
	}
	ids := optionIDs(q)
	if len(ids) != len(answers) {
		t.Fatalf("%d options for %d answers", len(ids), len(answers))
	}
	seen := map[int]bool{}
	for _, id := range ids {
		if id < 0 || id >= len(answers) || seen[id] {
			t.Fatalf("option ids %v", ids)
		}
		seen[id] = true
	}
	// the legal answers are the same list, each naming its option
	legal := g.LegalActions(g.State.Choice.Player)
	if len(legal) != len(answers) {
		t.Fatalf("%d legal actions for %d answers", len(legal), len(answers))
	}
	for i, a := range legal {
		if a.Option == nil || *a.Option != i {
			t.Fatalf("legal action %d does not name its option: %+v", i, a)
		}
	}
	return q
}

func TestAnOptionAnswersAsTheOldAnswerDid(t *testing.T) {
	g := fcGame(t)
	g.State.ActionsLeft = 2
	g.put(1, 0, "pony")
	pl := &g.State.Players[1]
	pl.Deck = append(pl.Deck, g.newInstance("oil-refinery", 1))
	g.must(t, 1, Action{Kind: ActPlayCard, Card: g.give(1, "deforestation"), CostTargets: [][]TargetRef{{crRef(1, 0)}}})
	q := checkOptions(t, g)
	if q.Answer != AnswerCards || q.Text.Key != "question.search" || len(q.Options) == 0 || q.Options[0].Card == nil {
		t.Fatalf("search %+v", q)
	}
	// answer by option, with nothing else
	id := q.Options[0].ID
	g.must(t, 1, Action{Kind: ActChoose, Option: &id})
	q = checkOptions(t, g)
	if q.Answer != AnswerBoard || q.Subject == nil || len(q.Options) != NumLanes || q.Options[2].Ref.Kind != "lane" || q.Options[2].Ref.Lane != 2 {
		t.Fatalf("place %+v", q)
	}
	lane := q.Options[2].ID
	g.must(t, 1, Action{Kind: ActChoose, Option: &lane})
	if pl.Lanes[2].Building == nil || g.State.Choice != nil {
		t.Errorf("the Building is not in Lane 2: %+v", pl.Lanes[2].Building)
	}
}

func TestABadOptionIsRefused(t *testing.T) {
	g := fcGame(t)
	g.State.Choice = &Choice{Player: 0, Kind: ChoiceAmount, Card: "the-big-hen-house", Options: []int{1, 2}, Unit: "damage"}
	for _, n := range []int{-1, 2, 99} {
		n := n
		g.mustFail(t, 0, Action{Kind: ActChoose, Option: &n}, ErrBadTarget)
	}
	one := 0
	g.mustFail(t, 1, Action{Kind: ActChoose, Option: &one}, ErrNotYourTurn) // not their question
	g.State.Choice = nil
	g.mustFail(t, 0, Action{Kind: ActChoose, Option: &one}, ErrNoChoice)
}

// The shape of every kind: what it says, where it is answered, and what its options are.
func TestEveryKindOfChoiceIsDescribed(t *testing.T) {
	g := fcGame(t)
	g.State.Players[0].Hand = nil
	pig := g.put(0, 0, "the-pig")
	dog := g.put(0, 2, "the-pig")
	pig.Damage, dog.Damage = 4, 1
	foe := g.put(1, 1, "wall-of-ears")
	g.give(0, "the-pig")
	g.give(0, "the-pig")
	h := g.State.Players[0].Hand
	cases := []struct {
		name   string
		ch     Choice
		answer string
		key    string
		opts   int // options, not counting a skip
		skip   bool
		check  func(*testing.T, *Question)
	}{
		{"discard", Choice{Kind: ChoiceDiscard, Count: 2}, AnswerHand, "question.discard", 2, false, func(t *testing.T, q *Question) {
			if q.Left != 2 || q.Options[0].Ref.Kind != "hand_card" || q.Options[0].Ref.Card != h[0].UID {
				t.Errorf("%+v", q)
			}
		}},
		{"ransom", Choice{Kind: ChoiceRansom, MinCost: ip(0)}, AnswerHand, "question.ransom", 3, false, func(t *testing.T, q *Question) {
			if last := q.Options[2]; last.Role != "danger" || last.Label.Key != "question.take_it" {
				t.Errorf("%+v", last)
			}
		}},
		{"upkeep", Choice{Kind: ChoiceUpkeep, Count: 2}, AnswerHand, "question.upkeep", 3, false, func(t *testing.T, q *Question) {
			if q.Options[2].Role != "danger" {
				t.Errorf("%+v", q.Options[2])
			}
		}},
		{"rebound", Choice{Kind: ChoiceRebound}, AnswerHand, "question.rebound", 2, true, func(t *testing.T, q *Question) {
			if q.Skip.Label.Key != "question.leave_it" {
				t.Errorf("%+v", q.Skip)
			}
		}},
		{"reveal", Choice{Kind: ChoiceReveal, Revealed: []Instance{h[0], h[1]}}, AnswerCards, "question.reveal", 2, false, func(t *testing.T, q *Question) {
			if q.Options[0].Card == nil || q.Options[0].Card.UID != h[0].UID {
				t.Errorf("%+v", q.Options[0])
			}
		}},
		{"search", Choice{Kind: ChoiceSearch, Revealed: []Instance{h[0]}}, AnswerCards, "question.search", 1, false, nil},
		{"place", Choice{Kind: ChoicePlace, Revealed: []Instance{h[0]}, Lanes: []int{0, 1, 2, 3}}, AnswerBoard, "question.place", 4, false, func(t *testing.T, q *Question) {
			if q.Subject == nil || q.Subject.UID != h[0].UID {
				t.Errorf("the Building being placed should be the subject: %+v", q.Subject)
			}
		}},
		{"redirect", Choice{Kind: ChoiceRedirect, Lane: 0, Revealed: []Instance{dog.Instance}}, AnswerBoard, "question.redirect", 1, true, func(t *testing.T, q *Question) {
			if r := q.Options[0].Ref; r.Kind != "creature" || r.Player != 0 || r.Lane != 2 {
				t.Errorf("%+v", r)
			}
		}},
		{"defend", Choice{Kind: ChoiceDefend, Lane: 1, Other: 1, MoveSide: 1, Revealed: []Instance{pig.Instance}}, AnswerBoard, "question.defend", 1, true, func(t *testing.T, q *Question) {
			if q.Subject == nil || q.Subject.UID != foe.UID {
				t.Errorf("the attacker should be the subject: %+v", q.Subject)
			}
		}},
		{"move_damage", Choice{Kind: ChoiceMoveDamage, Lane: 0, Other: 2, Count: 3}, AnswerButtons, "question.move_damage", 3, false, func(t *testing.T, q *Question) {
			r := q.Options[1].Result
			if r == nil || r.Params["a1"] != 4 || r.Params["a2"] != 2 || r.Params["b1"] != 1 || r.Params["b2"] != 3 || q.Text.Params["from_card"] != "the-pig" {
				t.Errorf("the result of moving 2 should say 4 to 2 and 1 to 3: %+v", q.Options[1])
			}
		}},
		{"move own", Choice{Kind: ChoiceMove, Lanes: []int{1, 3}, MoveSide: 0}, AnswerBoard, "question.move_mine", 2, false, func(t *testing.T, q *Question) {
			if r := q.Options[0].Ref; r.Kind != "landscape" || r.Player != 0 || r.Lane != 1 {
				t.Errorf("%+v", r)
			}
		}},
		{"move theirs", Choice{Kind: ChoiceMove, Lanes: []int{0, 2}, MoveSide: 1}, AnswerBoard, "question.move_theirs", 2, false, func(t *testing.T, q *Question) {
			if r := q.Options[0].Ref; r.Player != 1 || r.Lane != 0 {
				t.Errorf("lane 0 on the opponent's side is an option too: %+v", r)
			}
		}},
		{"camel", Choice{Kind: ChoiceCamel, Lanes: []int{2}, Revealed: []Instance{h[0]}}, AnswerButtons, "question.camel", 1, true, func(t *testing.T, q *Question) {
			if q.Subject == nil || q.Options[0].Label.Key != "question.camel_put" || q.Skip.Label.Key != "question.camel_skip" {
				t.Errorf("%+v", q)
			}
		}},
		{"landscape", Choice{Kind: ChoiceLandscape, Lane: 1}, AnswerButtons, "question.landscape", 7, false, func(t *testing.T, q *Question) {
			if q.Text.Params["lane"] != 2 || q.Options[0].Label.Key != "landscape.cornfield" {
				t.Errorf("%+v", q)
			}
		}},
		{"target", Choice{Kind: ChoiceTarget, Optional: true, Label: "Freeze", SkipLabel: "Don't freeze a Landscape", Targets: []TargetRef{lsRef(1, 0), lsRef(0, 2)}}, AnswerBoard, "question.target", 2, true, func(t *testing.T, q *Question) {
			if q.Text.Params["label"] != "Freeze" || q.Text.Params["what"] != "landscape" || q.Skip.Label.Text != "Don't freeze a Landscape" {
				t.Errorf("%+v", q)
			}
		}},
		{"target required", Choice{Kind: ChoiceTarget, Targets: []TargetRef{lsRef(1, 0)}}, AnswerBoard, "question.target", 1, false, nil},
		{"amount", Choice{Kind: ChoiceAmount, Options: []int{1, 2}, Unit: "damage"}, AnswerButtons, "question.amount", 2, false, func(t *testing.T, q *Question) {
			if q.Options[1].Label.Params["unit"] != "damage" || q.Options[1].Label.Params["n"] != 2 {
				t.Errorf("%+v", q.Options[1])
			}
		}},
	}
	for _, c := range cases {
		g2 := g.Clone()
		ch := c.ch
		ch.Player, ch.Card = 0, "the-pig"
		g2.State.Choice = &ch
		q := checkOptions(t, g2)
		if q.Answer != c.answer || q.Text.Key != c.key || len(q.Options) != c.opts || (q.Skip != nil) != c.skip {
			t.Errorf("%s: answer %q key %q options %d skip %v", c.name, q.Answer, q.Text.Key, len(q.Options), q.Skip != nil)
		}
		if q.Card != "the-pig" || q.Player != 0 {
			t.Errorf("%s: asked by %q for %d", c.name, q.Card, q.Player)
		}
		if c.check != nil {
			c.check(t, q)
		}
		if b, err := json.Marshal(q); err != nil || strings.Contains(string(b), "null") {
			t.Errorf("%s: the question should marshal with no nulls: %s %v", c.name, b, err)
		}
	}
}
