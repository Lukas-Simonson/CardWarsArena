package main

import (
	"bytes"
	"strings"
	"testing"

	"cardwars/internal/card"
	"cardwars/internal/engine"
)

// cli runs the command with the given arguments and input lines.
func cli(t *testing.T, input string, args ...string) (out string, code int) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	code = run(append([]string{"--data", "../../data"}, args...), strings.NewReader(input), &stdout, &stderr)
	return stdout.String() + stderr.String(), code
}

func TestListDecks(t *testing.T) {
	out, code := cli(t, "", "--list-decks")
	if code != 0 || !strings.Contains(out, "Finn's Blue Plains Deck") {
		t.Fatalf("code %d:\n%s", code, out)
	}
	for _, line := range strings.Split(out, "\n") {
		if strings.Contains(line, "Finn's Blue Plains Deck") && !strings.Contains(line, "ready") {
			t.Errorf("Finn's deck should be ready: %q", line)
		}
		if strings.Contains(line, "Jake's Cornfield/Nice Lands") && strings.Contains(line, "ready") {
			t.Errorf("a 2v2 deck with missing cards must not be ready: %q", line)
		}
	}
}

func TestBadArguments(t *testing.T) {
	for name, args := range map[string][]string{
		"unknown deck":   {"--p0", "nope"},
		"bad bot":        {"--bot", "everyone"},
		"bad first":      {"--first", "5"},
		"unmodelled":     {"--p0", "bmo"},
		"unknown flag":   {"--wat"},
		"ambiguous deck": {"--p0", "deck"},
	} {
		if out, code := cli(t, "", args...); code == 0 {
			t.Errorf("%s: expected failure, got:\n%s", name, out)
		}
	}
	if out, code := cli(t, "", "--p0", "bmo", "--allow-unmodelled", "--bot", "both"); code != 0 {
		t.Errorf("--allow-unmodelled should let BMO's deck play: %s", out)
	}
}

func TestBotVersusBotRunsToTheEnd(t *testing.T) {
	out, code := cli(t, "", "--bot", "both", "--seed", "7")
	if code != 0 {
		t.Fatalf("code %d", code)
	}
	if !strings.Contains(out, "wins!") || !strings.Contains(out, "Game over") {
		t.Errorf("no winner announced:\n%s", tail(out))
	}
	again, _ := cli(t, "", "--bot", "both", "--seed", "7")
	if out != again {
		t.Error("the same seed should replay the same game")
	}
}

func tail(s string) string {
	if len(s) > 1500 {
		return s[len(s)-1500:]
	}
	return s
}

func TestHumanCanPlayByNumber(t *testing.T) {
	// Always take option 1: draw a card or keep the hand. Must never panic or
	// reject, and shows the board and menu.
	out, code := cli(t, strings.Repeat("1\n", 60)+"q\n", "--seed", "3")
	if code != 0 {
		t.Fatalf("code %d:\n%s", code, tail(out))
	}
	for _, want := range []string{"LANE 1", "Your hand", "Actions:", "Keep your opening hand"} {
		if !strings.Contains(out, want) {
			t.Errorf("output lacks %q", want)
		}
	}
	if strings.Contains(out, "not allowed") {
		t.Errorf("a listed action was rejected:\n%s", tail(out))
	}
}

func TestInvalidChoicesAreExplained(t *testing.T) {
	out, _ := cli(t, "99\nbogus\n0\nq\n", "--seed", "3")
	if !strings.Contains(out, "Choose a number from 1 to 2") || !strings.Contains(out, "Unknown command") {
		t.Errorf("expected guidance:\n%s", tail(out))
	}
}

func TestInspectShowsHowTheEngineModelsACard(t *testing.T) {
	out, _ := cli(t, "i the pig\ni 1\nq\n", "--seed", "3", "--first", "0")
	if !strings.Contains(out, "The Pig  [the-pig]") || !strings.Contains(out, "engine: modelled") || !strings.Contains(out, `"kind":"floop"`) {
		t.Errorf("inspect output:\n%s", out)
	}
}

func TestSandboxEditsAndUndo(t *testing.T) {
	script := "1\n/hp opp 3\nb\nu\nb\nq\n"
	out, _ := cli(t, script, "--sandbox", "--first", "0", "--seed", "3")
	i := strings.Index(out, "Jake (opp)  HP 3")
	j := strings.LastIndex(out, "Jake (opp)  HP 25")
	if i < 0 || j < i {
		t.Errorf("HP should drop to 3 and come back after undo (i=%d j=%d):\n%s", i, j, tail(out))
	}
}

func TestSandboxPutAndGiveAreUsableImmediately(t *testing.T) {
	script := "1\n/give corn scepter\n/put opp 2 cool dog\n/land me 1 corn\na scepter\nq\n"
	out, _ := cli(t, script, "--sandbox", "--first", "0", "--seed", "3")
	if !strings.Contains(out, "Cast Corn Scepter (cost 1)  [targets: Cool Dog (opp's, lane 2)]") {
		t.Errorf("the given spell should be castable on the placed Creature:\n%s", tail(out))
	}
}

func TestSandboxCommandsNeedTheFlag(t *testing.T) {
	out, _ := cli(t, "/give pig\nq\n", "--seed", "3")
	if !strings.Contains(out, "Sandbox commands are off") {
		t.Errorf("output:\n%s", out)
	}
}

func TestSandboxErrorsAreReported(t *testing.T) {
	out, _ := cli(t, "/give nonsuch\n/put me 9 pig\n/put me 1 teleport\n/hp me x\nq\n", "--sandbox", "--seed", "3", "--first", "0")
	for _, want := range []string{`No card matches "nonsuch"`, "Lane must be 1-4", "only Creatures and Buildings", "usage: /hp"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in:\n%s", want, out)
		}
	}
}

func TestHotSeatShowsOnlyTheMovingPlayersHand(t *testing.T) {
	out, _ := cli(t, "1\n", "--bot", "none", "--seed", "3", "--first", "0")
	// Both players are asked to keep or mulligan, each seeing their own hand.
	if strings.Count(out, "choose your opening hand") != 2 {
		t.Errorf("expected two mulligan screens:\n%s", out)
	}
}

func TestBotCommandHandsControlOver(t *testing.T) {
	out, _ := cli(t, "bot both\n", "--seed", "3")
	if !strings.Contains(out, "Computer controls: both") || !strings.Contains(out, "wins!") {
		t.Errorf("handing both sides to the computer should play the game out:\n%s", tail(out))
	}
}

// ---- unit tests

func TestLandscapesFromName(t *testing.T) {
	c, n, b, s, i := card.Cornfield, card.NiceLands, card.BluePlains, card.SandyLands, card.UselessSwamp
	for name, want := range map[string][4]card.Landscape{
		"Finn's Blue Plains Deck":                                                 {b, b, b, b},
		"Jake's Cornfield/Nice Lands Deck":                                        {c, c, n, n},
		"Lady Rainicorn's SandyLand Deck":                                         {s, s, s, s},
		"Lumpy Space Princess' Blue Plains/Useless Swamp Deck (Totally Rad Deck)": {b, b, i, i},
		"Moniker's Useless Swamp/Icy Lands Deck":                                  {i, i, card.IcyLands, card.IcyLands},
	} {
		got, ok := landscapesFromName(name)
		if !ok || got != want {
			t.Errorf("%q: %v %v, want %v", name, got, ok, want)
		}
	}
	if _, ok := landscapesFromName("Doubles Tournament"); ok {
		t.Error("no Landscape in the name: cannot tell")
	}
}

func TestOwnerFromName(t *testing.T) {
	for in, want := range map[string]string{
		"Finn's Blue Plains Deck":                "Finn",
		"Lumpy Space Princess' Blue Plains Deck": "Lumpy Space Princess",
		"Doubles Tournament":                     "",
	} {
		if got := ownerFromName(in); got != want {
			t.Errorf("%q: %q, want %q", in, got, want)
		}
	}
}

func newUI(t *testing.T) (*ui, *engine.Game) {
	t.Helper()
	s, err := newSession(options{data: "../../data", deck0: "cp1 finn", deck1: "cp1 jake", seed: 3, first: 0, bots: "none"}, strings.NewReader(""), &bytes.Buffer{})
	if err != nil {
		t.Fatal(err)
	}
	return s.u, s.g
}

func TestActionDescriptions(t *testing.T) {
	u, g := newUI(t)
	g.Put(0, 0, "the-pig")
	g.Put(1, 0, "cool-dog")
	got := u.action(0, engine.Action{Kind: engine.ActFloop, Lane: 0,
		Targets: [][]engine.TargetRef{{{Kind: "landscape", Player: 1, Lane: 0}}}})
	want := "Floop The Pig (lane 1)  [targets: Cornfield landscape (opp's, lane 1)]"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
	if got := u.action(0, engine.Action{Kind: engine.ActAttack, Lane: 0}); got != "Fight in lane 1: The Pig fights Cool Dog" {
		t.Errorf("attack: %q", got)
	}
	g.Clear(1, 0)
	if got := u.action(0, engine.Action{Kind: engine.ActAttack, Lane: 0}); !strings.Contains(got, "directly") {
		t.Errorf("direct attack: %q", got)
	}
}

func TestEventDescriptionsHideTheOpponentsDraws(t *testing.T) {
	u, g := newUI(t)
	uid := g.State.Players[1].Hand[0].UID
	e := engine.Event{Kind: engine.EvDraw, Player: 1, Card: uid}
	if got := u.event(0, e); got != "Jake draws a card" {
		t.Errorf("hidden: %q", got)
	}
	if got := u.event(1, e); !strings.HasPrefix(got, "Jake draws ") || strings.Contains(got, "a card") {
		t.Errorf("own draw should name the card: %q", got)
	}
	u.reveal = true
	if got := u.event(0, e); strings.Contains(got, "a card") {
		t.Errorf("reveal mode should name it: %q", got)
	}
}

func TestBoardMarksStolenSilencedAndProtected(t *testing.T) {
	u, g := newUI(t)
	g.Put(1, 1, "cool-dog")
	g.Put(1, 0, "evil-eye")
	g.State.Players[1].Lanes[1].Creature.Owner = 0
	g.State.Players[1].Lanes[0].Creature.Silenced = true
	out := u.board(0)
	for _, want := range []string{"stolen", "silenced", "protected", "Cool Dog", "2/7 dmg0 ready"} {
		if !strings.Contains(out, want) {
			t.Errorf("board lacks %q:\n%s", want, out)
		}
	}
}
