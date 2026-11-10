package main

import (
	"bufio"
	"errors"
	"flag"
	"fmt"
	"io"
	"math/rand/v2"
	"strconv"
	"strings"
	"time"

	"cardwars/internal/bot"
	"cardwars/internal/card"
	"cardwars/internal/decks"
	"cardwars/internal/engine"
)

var errHelp = errors.New("help")

type options struct {
	data       string
	deck0      string
	deck1      string
	seed       uint64
	first      int
	bots       string
	sandbox    bool
	reveal     bool
	allowUnmod bool
	listDecks  bool
}

func parseFlags(args []string, errOut io.Writer) (options, error) {
	var o options
	fs := flag.NewFlagSet("cardwars", flag.ContinueOnError)
	fs.SetOutput(errOut)
	fs.StringVar(&o.data, "data", "data", "directory holding cards/ and effects/")
	fs.StringVar(&o.deck0, "p0", "finn", "player 0's deck (words from --list-decks)")
	fs.StringVar(&o.deck1, "p1", "jake", "player 1's deck")
	fs.Uint64Var(&o.seed, "seed", 0, "random seed (default: time based; printed at start)")
	fs.IntVar(&o.first, "first", -1, "who goes first, 0 or 1 (default: from the seed)")
	fs.StringVar(&o.bots, "bot", "p1", "who the computer plays: p0, p1, both or none (none = hot-seat)")
	fs.BoolVar(&o.sandbox, "sandbox", false, "enable /commands that bypass the rules to set up a situation")
	fs.BoolVar(&o.reveal, "reveal", false, "show hidden information such as the opponent's draws")
	fs.BoolVar(&o.allowUnmod, "allow-unmodelled", false, "allow cards whose rules are not modelled (their text is ignored)")
	fs.BoolVar(&o.listDecks, "list-decks", false, "list the decks and exit")
	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return o, errHelp
		}
		return o, err
	}
	switch o.bots {
	case "p0", "p1", "both", "none":
	default:
		return o, fmt.Errorf("--bot must be p0, p1, both or none")
	}
	if o.first < -1 || o.first > 1 {
		return o, fmt.Errorf("--first must be 0 or 1")
	}
	return o, nil
}

type snapshot struct {
	g      *engine.Game
	logLen int
}

type session struct {
	opts    options
	cat     card.Catalog
	in      *bufio.Reader
	out     io.Writer
	g       *engine.Game
	u       *ui
	bots    [2]bool
	brng    *rand.Rand
	log     []string
	history []snapshot
	dirty   bool // the screen needs redrawing before the next prompt
}

func newSession(o options, in io.Reader, out io.Writer) (*session, error) {
	cat, err := card.Load(o.data)
	if err != nil {
		return nil, err
	}
	s := &session{opts: o, cat: cat, in: bufio.NewReader(in), out: out}
	if o.listDecks {
		return s, nil
	}
	if o.seed == 0 {
		o.seed = uint64(time.Now().UnixNano())
		s.opts.seed = o.seed
	}
	all := decks.Find(cat)
	var picked [2]decks.Info
	for i, q := range []string{o.deck0, o.deck1} {
		d, err := decks.Pick(all, q)
		if err != nil {
			return nil, err
		}
		if !d.LandOK {
			return nil, fmt.Errorf("%s: cannot tell its Landscapes from the deck name", d.Key())
		}
		if !o.allowUnmod && d.Modelled < d.Distinct {
			return nil, fmt.Errorf("%s has cards whose rules are not modelled yet (%d of %d); use --allow-unmodelled to play with their text ignored", d.Key(), d.Modelled, d.Distinct)
		}
		picked[i] = d
	}
	first := o.first
	if first < 0 {
		first = int(o.seed % 2)
	}
	g, err := engine.NewGame(cat, engine.Config{Seed: o.seed, First: engine.PlayerID(first), AllowUnimplemented: o.allowUnmod, SkipDeckRules: o.sandbox},
		[2]engine.DeckSpec{picked[0].Spec(), picked[1].Spec()})
	if err != nil {
		return nil, err
	}
	s.g = g
	s.u = &ui{g: g, reveal: o.reveal}
	for i, d := range picked {
		s.u.labels[i] = d.Owner
		if d.Owner == "" {
			s.u.labels[i] = fmt.Sprintf("Player %d", i)
		}
	}
	if s.u.labels[0] == s.u.labels[1] {
		s.u.labels[0] += " (P0)"
		s.u.labels[1] += " (P1)"
	}
	s.bots = [2]bool{o.bots == "p0" || o.bots == "both", o.bots == "p1" || o.bots == "both"}
	s.brng = rand.New(rand.NewPCG(o.seed, 0xbada55))
	s.dirty = true
	return s, nil
}

func (s *session) printf(format string, a ...any) { fmt.Fprintf(s.out, format, a...) }

// actor is the player who must decide next.
func (s *session) actor() engine.PlayerID {
	if s.g.State.Phase == engine.PhaseMulligan {
		for p := engine.PlayerID(0); p < 2; p++ {
			if !s.g.State.Players[p].Kept {
				return p
			}
		}
	}
	return s.g.State.Active
}

func (s *session) loop() {
	s.printf("Card Wars Arena — %s vs %s   (seed %d, first: %s)\n", s.u.who(0), s.u.who(1), s.opts.seed, s.u.who(s.g.State.First))
	if s.opts.sandbox {
		s.printf("Sandbox mode: type /help for the rule-bypassing commands. Deck rules are relaxed.\n")
	}
	s.printf("Type ? for help.\n\n")
	for {
		if s.g.State.Phase == engine.PhaseOver {
			s.printf("\n%s\nGame over: %s wins. (Type u to undo, q to quit.)\n", s.u.board(s.g.State.Winner), s.u.who(s.g.State.Winner))
			if !s.overPrompt() {
				return
			}
			continue
		}
		p := s.actor()
		if s.bots[p] {
			a, ok := bot.Choose(s.g, p, s.brng)
			if !ok {
				s.printf("(bot has no action; stopping)\n")
				return
			}
			s.printf("%s (bot): %s\n", s.u.who(p), s.u.action(p, a))
			s.apply(p, a)
			s.dirty = true
			continue
		}
		if !s.humanTurn(p) {
			return
		}
	}
}

// overPrompt handles input after the game ends; false means quit.
func (s *session) overPrompt() bool {
	for {
		line, ok := s.readLine("> ")
		if !ok {
			return false
		}
		switch strings.ToLower(strings.TrimSpace(line)) {
		case "q", "quit", "exit":
			return false
		case "u", "undo":
			if s.undo() {
				s.dirty = true
				return true
			}
		}
	}
}

func (s *session) readLine(prompt string) (string, bool) {
	s.printf("%s", prompt)
	line, err := s.in.ReadString('\n')
	if err != nil && line == "" {
		s.printf("\n")
		return "", false
	}
	return strings.TrimRight(line, "\r\n"), true
}

// apply runs an action and prints what happened.
func (s *session) apply(p engine.PlayerID, a engine.Action) bool {
	events, err := s.g.Apply(p, a)
	if err != nil {
		s.printf("  not allowed: %v\n", err)
		return false
	}
	for _, e := range events {
		line := s.u.event(p, e)
		s.log = append(s.log, line)
		s.printf("  %s\n", line)
	}
	return true
}

func (s *session) snapshot() {
	s.history = append(s.history, snapshot{s.g.Clone(), len(s.log)})
}

func (s *session) undo() bool {
	if len(s.history) == 0 {
		s.printf("Nothing to undo.\n")
		return false
	}
	h := s.history[len(s.history)-1]
	s.history = s.history[:len(s.history)-1]
	s.g = h.g
	s.u.g = h.g
	s.log = s.log[:h.logLen]
	s.printf("Undone.\n")
	return true
}

// humanTurn shows the position and handles input until the human acts.
func (s *session) humanTurn(p engine.PlayerID) bool {
	legal := s.g.LegalActions(p)
	refresh := func() {
		legal = s.g.LegalActions(p)
		if s.g.State.Phase != engine.PhaseMulligan {
			s.printf("\n%s\n", s.u.board(p))
		} else {
			s.printf("\n── %s: choose your opening hand ──\n", s.u.who(p))
		}
		s.printf("%s\n", s.u.hand(p))
		s.printf("%s", s.menu(p, legal, ""))
		s.dirty = false
	}
	if s.dirty {
		refresh()
	}
	for {
		line, ok := s.readLine(fmt.Sprintf("%s> ", s.u.who(p)))
		if !ok {
			return false
		}
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if n, err := strconv.Atoi(line); err == nil {
			if n < 1 || n > len(legal) {
				s.printf("Choose a number from 1 to %d (a lists them).\n", len(legal))
				continue
			}
			s.snapshot()
			if !s.apply(p, legal[n-1]) {
				s.history = s.history[:len(s.history)-1]
				continue
			}
			s.dirty = true
			return true
		}
		switch res := s.command(p, line, legal); res {
		case cmdQuit:
			return false
		case cmdChanged:
			s.dirty = true
			return true
		case cmdQuiet:
			// A sandbox edit: keep the screen as is, but recompute whose move
			// it is and what is legal.
			s.dirty = false
			return true
		case cmdRedraw:
			refresh()
		}
	}
}

func (s *session) menu(p engine.PlayerID, legal []engine.Action, filter string) string {
	var b strings.Builder
	if len(legal) == 0 {
		return "No legal actions.\n"
	}
	fmt.Fprintf(&b, "Actions:\n")
	shown := 0
	for i, a := range legal {
		d := s.u.action(p, a)
		if filter != "" && !strings.Contains(strings.ToLower(d), strings.ToLower(filter)) {
			continue
		}
		fmt.Fprintf(&b, "  %3d) %s\n", i+1, d)
		shown++
	}
	if shown == 0 {
		fmt.Fprintf(&b, "  (nothing matches %q)\n", filter)
	}
	return b.String()
}
