// Package auth says who is making a request. The server asks an Authenticator
// and only ever sees a store.User, so how people sign in can change without
// touching the rest of it. Nothing here stores a password: people prove who
// they are to a provider (Discord, GitHub) and we keep only that provider's id
// for them and a session.
package auth

import (
	"encoding/json"
	"net/http"

	"cardwars/internal/store"
)

// Authenticator identifies the signed-in user of a request and serves the
// routes its sign-in flow needs (all under /api/auth/).
type Authenticator interface {
	Identify(r *http.Request) (store.User, bool)
	Routes(mux *http.ServeMux)
}

// Provider is one way to sign in, as the web client lists it.
type Provider struct {
	Name  string `json:"name"`
	Label string `json:"label"`
}

// Config is what the web client reads, from GET /api/auth/config, to know
// what to offer.
type Config struct {
	Mode      string     `json:"mode"` // "none" or "oauth"
	Providers []Provider `json:"providers"`
}

type none struct{}

// None is the anonymous server: nobody is ever signed in and no one can sign in.
func None() Authenticator { return none{} }

func (none) Identify(*http.Request) (store.User, bool) { return store.User{}, false }

func (none) Routes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/auth/config", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, Config{Mode: "none", Providers: []Provider{}})
	})
	mux.HandleFunc("GET /api/auth/me", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, map[string]any{"user": nil})
	})
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	_ = json.NewEncoder(w).Encode(v)
}
