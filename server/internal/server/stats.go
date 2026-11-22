package server

import (
	"encoding/json"
	"net/http"

	"cardwars/internal/stats"
)

// handleStats serves the shared totals, grouped by topic (see stats.Report), or 404 when stats are off. Public and
// read-only; served from memory. ?detail=cards adds how often each card was played.
func (s *Server) handleStats(w http.ResponseWriter, r *http.Request) {
	c := s.cfg.Stats
	if c == nil {
		writeError(w, http.StatusNotFound, "stats_off", "statistics are turned off on this server")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "public, max-age=5")
	json.NewEncoder(w).Encode(stats.BuildReport(c.Totals(), c.Kept(), r.URL.Query().Get("detail") == "cards"))
}
