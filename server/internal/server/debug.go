package server

import (
	_ "embed"
	"net/http"
)

//go:embed web/debug.html
var debugPage []byte

// handleDebugPage serves a tiny browser client for trying the protocol by hand.
func (s *Server) handleDebugPage(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write(debugPage)
}
