// Command cardwars is an interactive terminal front end for the rules engine:
// play a match (against a bot or yourself), inspect cards, and, in sandbox
// mode, set up any situation to test a card.
package main

import (
	"fmt"
	"io"
	"os"
)

func main() { os.Exit(run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr)) }

func run(args []string, in io.Reader, out, errOut io.Writer) int {
	opts, err := parseFlags(args, errOut)
	if err != nil {
		if err != errHelp {
			fmt.Fprintln(errOut, "cardwars:", err)
			return 2
		}
		return 0
	}
	s, err := newSession(opts, in, out)
	if err != nil {
		fmt.Fprintln(errOut, "cardwars:", err)
		return 1
	}
	if opts.listDecks {
		fmt.Fprint(out, listDecks(findDecks(s.cat)))
		return 0
	}
	s.loop()
	return 0
}
