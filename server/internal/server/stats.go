package server

import (
	"encoding/json"
	"net/http"
)

// statsAnswer is what GET /api/stats returns.
type statsAnswer struct {
	// Kept is whether the totals survive a restart (the server has a database).
	Kept bool `json:"kept"`
	// Counts are the totals, by name (see internal/stats).
	Counts map[string]int64 `json:"counts"`
}

// handleStats serves the shared totals, or 404 when stats are off. Public and read-only; served from memory.
func (s *Server) handleStats(w http.ResponseWriter, _ *http.Request) {
	c := s.cfg.Stats
	if c == nil {
		writeError(w, http.StatusNotFound, "stats_off", "statistics are turned off on this server")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "public, max-age=5")
	json.NewEncoder(w).Encode(statsAnswer{Kept: c.Kept(), Counts: c.Totals()})
}
