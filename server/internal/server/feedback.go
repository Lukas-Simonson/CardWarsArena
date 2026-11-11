package server

import (
	"encoding/json"
	"net/http"
)

// A Link is a labelled address the web client shows.
type Link struct {
	ID    string `json:"id"`
	Label string `json:"label"`
	URL   string `json:"url"`
}

// The project's feedback forms and repository, shown on the home screen when Config.Feedback is on. They are
// fixed here for now; a server that is not this project's turns the whole thing off (the default).
var feedbackLinks = []Link{
	{"bug", "Report a bug", "https://docs.google.com/forms/d/e/1FAIpQLSejBLdWP5nGURVzNBP0mXVUcrIzDCQ76fOtHklw-sSMXlL_0A/viewform"},
	{"card", "Report a card problem", "https://docs.google.com/forms/d/e/1FAIpQLSeWfVUBsiVFOvIjHGHtLLdSxAio0t4Va5t6p4hnWirBkqWHgw/viewform"},
	{"feature", "Suggest a feature", "https://docs.google.com/forms/d/e/1FAIpQLSexnqjzgPTE1rlL_O0uOW9EOl1e2rPEpaq7Ey0L7lg02SB5Fg/viewform"},
	{"repo", "GitHub", "https://github.com/Lukas-Simonson/CardWarsArena"},
}

// handleFeedback says whether to show the feedback links, and which. Off, it offers none.
func (s *Server) handleFeedback(w http.ResponseWriter, _ *http.Request) {
	out := struct {
		Enabled bool   `json:"enabled"`
		Links   []Link `json:"links"`
	}{Links: []Link{}}
	if s.cfg.Feedback {
		out.Enabled, out.Links = true, feedbackLinks
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(out)
}
