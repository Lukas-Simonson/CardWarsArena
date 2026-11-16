// Package problem is the one shape of "something is wrong, and why" that reaches a player: a stable code, the values
// the message needs, and an English text for logs and for clients that do not know the code. A client writes the
// words itself, in the player's language, from the code and the params. Cards are named by id (params "card"), so
// the client can use the card's name in its own language.
package problem

import (
	"encoding/json"
	"fmt"
)

// Problem is a rule broken, a deck that cannot be played, or a request that failed.
type Problem struct {
	Code   string         `json:"code"`
	Params map[string]any `json:"params,omitempty"`
	Text   string         `json:"text"` // in English: what a log, a test or an old client shows
}

// New makes a Problem. params are name, value pairs: New("deck_too_small", "3 cards, minimum is 40", "n", 3, "min", 40).
// It returns a pointer, so a Problem kept in a variable can be compared with == like any sentinel error.
func New(code, text string, params ...any) *Problem {
	p := &Problem{Code: code, Text: text}
	for i := 0; i+1 < len(params); i += 2 {
		if p.Params == nil {
			p.Params = map[string]any{}
		}
		p.Params[fmt.Sprint(params[i])] = params[i+1]
	}
	return p
}

// Newf is New with the text formatted from format and args.
func Newf(code, format string, args []any, params ...any) *Problem {
	return New(code, fmt.Sprintf(format, args...), params...)
}

func (p *Problem) Error() string { return p.Text }

// MarshalJSON leaves out nothing a client needs and keeps params an object, never null.
func (p Problem) MarshalJSON() ([]byte, error) {
	type plain Problem
	return json.Marshal(plain(p))
}
