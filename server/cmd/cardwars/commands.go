package main

import (
	"encoding/json"
	"sort"
	"strconv"
	"strings"

	"cardwars/internal/card"
	"cardwars/internal/engine"
)

type cmdResult int

const (
	cmdNone    cmdResult = iota // printed something; keep prompting
	cmdRedraw                   // redraw the screen
	cmdChanged                  // the game changed: re-evaluate whose move it is, then redraw
	cmdQuiet                    // the game changed but do not redraw
	cmdQuit
)

const helpText = `Choose an action by typing its number. Other commands:
  b            show the board            h            show your hand
  a [text]     list actions (optionally only those containing text)
  d [me|opp]   show a discard pile       i <name|#>   inspect a card (name, or hand number)
  log [n]      show the last n events    json         dump the full game state
  u            undo your last action     r            toggle revealing hidden information
  bot <p0|p1|both|none>   change who the computer plays
  q            quit
Sandbox commands start with / (need --sandbox); try /help.
`

const sandboxHelp = `Sandbox commands bypass the rules. Players are 0, 1, me or opp; lanes are 1-4.
  /give <card> [player]          put a card in a hand (default: yours)
  /put <player> <lane> <card>    put a Creature or Building straight into a lane
  /clear <player> <lane>         remove the Creature and Building in a lane
  /hp <player> <n>               set Hit Points
  /actions <n>                   set remaining Actions this turn
  /land <player> <lane> <type> [down]   set a lane's Landscape (and face-down)
Card names match by case-insensitive substring ("scholar" finds Ancient Scholar).
`

func (s *session) command(p engine.PlayerID, line string, legal []engine.Action) cmdResult {
	f := strings.Fields(line)
	cmd := strings.ToLower(f[0])
	args := f[1:]
	if strings.HasPrefix(cmd, "/") {
		return s.sandboxCommand(p, cmd, args)
	}
	switch cmd {
	case "?", "help":
		s.printf("%s", helpText)
	case "b", "board":
		s.printf("%s", s.u.board(p))
	case "h", "hand":
		s.printf("%s", s.u.hand(p))
	case "a", "actions":
		s.printf("%s", s.menu(p, legal, strings.Join(args, " ")))
	case "d", "discard":
		who := p
		if len(args) > 0 {
			if q, ok := s.player(p, args[0]); ok {
				who = q
			}
		}
		s.printf("%s", s.u.discard(who))
	case "i", "inspect":
		s.inspect(p, strings.Join(args, " "))
	case "log":
		n := 15
		if len(args) > 0 {
			if v, err := strconv.Atoi(args[0]); err == nil && v > 0 {
				n = v
			}
		}
		start := max(0, len(s.log)-n)
		for _, l := range s.log[start:] {
			s.printf("  %s\n", l)
		}
	case "json":
		b, _ := json.MarshalIndent(s.g.State, "", "  ")
		s.printf("%s\n", b)
	case "u", "undo":
		if s.undo() {
			return cmdChanged
		}
	case "r", "reveal":
		s.u.reveal = !s.u.reveal
		s.printf("Revealing hidden information: %v\n", s.u.reveal)
	case "bot":
		if len(args) != 1 || !map[string]bool{"p0": true, "p1": true, "both": true, "none": true}[args[0]] {
			s.printf("usage: bot p0|p1|both|none\n")
			break
		}
		s.bots = [2]bool{args[0] == "p0" || args[0] == "both", args[0] == "p1" || args[0] == "both"}
		s.printf("Computer controls: %s\n", args[0])
		return cmdChanged
	case "q", "quit", "exit":
		return cmdQuit
	default:
		s.printf("Unknown command %q. Type ? for help.\n", cmd)
	}
	return cmdNone
}

// player parses 0, 1, me or opp.
func (s *session) player(me engine.PlayerID, tok string) (engine.PlayerID, bool) {
	switch strings.ToLower(tok) {
	case "0":
		return 0, true
	case "1":
		return 1, true
	case "me", "you":
		return me, true
	case "opp", "opponent":
		return me.Opp(), true
	}
	return 0, false
}

// findCards returns catalog cards matching q: an exact id or name wins,
// otherwise every name containing q.
func (s *session) findCards(q string) []*card.Card {
	q = strings.ToLower(strings.TrimSpace(q))
	var hits []*card.Card
	for _, c := range s.cat {
		if strings.ToLower(c.Name) == q || c.ID == q {
			return []*card.Card{c}
		}
		if strings.Contains(strings.ToLower(c.Name), q) || strings.Contains(c.ID, q) {
			hits = append(hits, c)
		}
	}
	sort.Slice(hits, func(i, j int) bool { return hits[i].Name < hits[j].Name })
	return hits
}

func (s *session) oneCard(q string) (*card.Card, bool) {
	if strings.TrimSpace(q) == "" {
		s.printf("Which card?\n")
		return nil, false
	}
	hits := s.findCards(q)
	switch len(hits) {
	case 0:
		s.printf("No card matches %q.\n", q)
	case 1:
		return hits[0], true
	default:
		var names []string
		for i, c := range hits {
			if i == 12 {
				names = append(names, "…")
				break
			}
			names = append(names, c.Name)
		}
		s.printf("%q matches several cards: %s\n", q, strings.Join(names, ", "))
	}
	return nil, false
}

func (s *session) inspect(p engine.PlayerID, q string) {
	if n, err := strconv.Atoi(q); err == nil {
		hand := s.g.State.Players[p].Hand
		if n < 1 || n > len(hand) {
			s.printf("You have %d cards in hand.\n", len(hand))
			return
		}
		s.printf("%s", s.u.inspect(s.g.Card(hand[n-1])))
		return
	}
	if c, ok := s.oneCard(q); ok {
		s.printf("%s", s.u.inspect(c))
	}
}

func (s *session) laneArg(tok string) (int, bool) {
	n, err := strconv.Atoi(tok)
	if err != nil || n < 1 || n > engine.NumLanes {
		s.printf("Lane must be 1-%d.\n", engine.NumLanes)
		return 0, false
	}
	return n - 1, true
}

func (s *session) sandboxCommand(me engine.PlayerID, cmd string, args []string) cmdResult {
	if cmd == "/help" {
		s.printf("%s", sandboxHelp)
		return cmdNone
	}
	if !s.opts.sandbox {
		s.printf("Sandbox commands are off. Restart with --sandbox.\n")
		return cmdNone
	}
	usage := func(u string) cmdResult { s.printf("usage: %s\n", u); return cmdNone }
	player := func(i int) (engine.PlayerID, bool) {
		if i >= len(args) {
			return 0, false
		}
		return s.player(me, args[i])
	}
	// Run the change on a snapshot so u undoes it.
	do := func(fn func() error) cmdResult {
		s.snapshot()
		if err := fn(); err != nil {
			s.history = s.history[:len(s.history)-1]
			s.printf("  %v\n", err)
			return cmdNone
		}
		s.printf("  done (sandbox)\n")
		return cmdQuiet
	}
	switch cmd {
	case "/give":
		if len(args) < 1 {
			return usage("/give <card> [player]")
		}
		who := me
		q := strings.Join(args, " ")
		if len(args) > 1 {
			if pl, ok := s.player(me, args[len(args)-1]); ok {
				who, q = pl, strings.Join(args[:len(args)-1], " ")
			}
		}
		c, ok := s.oneCard(q)
		if !ok {
			return cmdNone
		}
		return do(func() error { _, err := s.g.Give(who, c.ID); return err })
	case "/put":
		pl, ok := player(0)
		if !ok || len(args) < 3 {
			return usage("/put <player> <lane> <card>")
		}
		lane, ok := s.laneArg(args[1])
		if !ok {
			return cmdNone
		}
		c, ok := s.oneCard(strings.Join(args[2:], " "))
		if !ok {
			return cmdNone
		}
		return do(func() error { return s.g.Put(pl, lane, c.ID) })
	case "/clear":
		pl, ok := player(0)
		if !ok || len(args) != 2 {
			return usage("/clear <player> <lane>")
		}
		lane, ok := s.laneArg(args[1])
		if !ok {
			return cmdNone
		}
		return do(func() error { return s.g.Clear(pl, lane) })
	case "/hp":
		pl, ok := player(0)
		n, err := 0, error(nil)
		if len(args) == 2 {
			n, err = strconv.Atoi(args[1])
		}
		if !ok || len(args) != 2 || err != nil {
			return usage("/hp <player> <n>")
		}
		return do(func() error { s.g.SetHP(pl, n); return nil })
	case "/actions":
		n, err := 0, error(nil)
		if len(args) == 1 {
			n, err = strconv.Atoi(args[0])
		}
		if len(args) != 1 || err != nil || n < 0 {
			return usage("/actions <n>")
		}
		return do(func() error { s.g.SetActions(n); return nil })
	case "/land":
		pl, ok := player(0)
		if !ok || len(args) < 3 {
			return usage("/land <player> <lane> <type> [down]")
		}
		lane, ok := s.laneArg(args[1])
		if !ok {
			return cmdNone
		}
		ls, ok := parseLandscape(args[2])
		if !ok {
			s.printf("Landscape types: cornfield, blue, swamp, nice, sandy, icy.\n")
			return cmdNone
		}
		down := len(args) > 3 && strings.EqualFold(args[3], "down")
		return do(func() error { return s.g.SetLandscape(pl, lane, ls, down) })
	}
	s.printf("Unknown sandbox command %q. Try /help.\n", cmd)
	return cmdNone
}

func parseLandscape(tok string) (card.Landscape, bool) {
	t := strings.ToLower(tok)
	for _, c := range []struct {
		prefix string
		ls     card.Landscape
	}{
		{"corn", card.Cornfield}, {"blue", card.BluePlains}, {"swamp", card.UselessSwamp}, {"useless", card.UselessSwamp},
		{"nice", card.NiceLands}, {"sandy", card.SandyLands}, {"icy", card.IcyLands},
	} {
		if strings.HasPrefix(t, c.prefix) {
			return c.ls, true
		}
	}
	return "", false
}
