package server

import (
	"context"
	"encoding/json"
	"errors"
	"mime"
	"net/http"
	"strings"
	"time"
	"unicode"

	"cardwars/internal/card"
	"cardwars/internal/decks"
	"cardwars/internal/store"
)

const (
	maxDecksPerUser = 50
	maxDeckName     = 40
	maxDeckBody     = 64 << 10
	storeTimeout    = 5 * time.Second
	savedPrefix     = "my:" // a saved deck is asked for as "my:<id>" in create and join
)

// savedDeck is a player's deck as the API shows it: what they saved, and whether it can be played yet.
type savedDeck struct {
	ID    string   `json:"id"`
	Name  string   `json:"name"`
	Hero  string   `json:"hero"`
	Cards []string `json:"cards"` // one id per copy, as the game takes a deck
	// Landscapes are the four it is played with, in Lane order; empty when the cards do not say. Chosen says
	// whether the player picked them (otherwise they follow the cards).
	Landscapes []card.Landscape `json:"landscapes"`
	Chosen     bool             `json:"landscapes_chosen"`
	Created    time.Time        `json:"created"`
	Updated    time.Time        `json:"updated"`
	Playable   bool             `json:"playable"`
	Problems   []string         `json:"problems"` // what stops it being played, in words
}

type deckBody struct {
	Name       string   `json:"name"`
	Hero       string   `json:"hero"`
	Cards      []string `json:"cards"`
	Landscapes []string `json:"landscapes"` // four, one for each Lane, or none to let the cards decide
}

func (s *Server) mountMyDecks(mux *http.ServeMux) {
	if s.cfg.Store == nil {
		return
	}
	mux.HandleFunc("GET /api/my/decks", s.signedIn(s.listMyDecks))
	mux.HandleFunc("POST /api/my/decks", s.signedIn(s.createMyDeck))
	mux.HandleFunc("PUT /api/my/decks/{id}", s.signedIn(s.updateMyDeck))
	mux.HandleFunc("DELETE /api/my/decks/{id}", s.signedIn(s.deleteMyDeck))
}

func writeError(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": msg})
}

// signedIn runs h for a signed-in player. A request that changes something must also come from this
// site or an allowed web client and carry JSON, which a form on another site cannot send.
func (s *Server) signedIn(h func(http.ResponseWriter, *http.Request, store.User)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		u, ok := s.cfg.Auth.Identify(r)
		if !ok {
			writeError(w, http.StatusUnauthorized, "sign in first")
			return
		}
		if r.Method != http.MethodGet {
			if o := r.Header.Get("Origin"); o != "" && !s.originAllowed(o, r.Host) {
				writeError(w, http.StatusForbidden, "wrong origin")
				return
			}
		}
		h(w, r, u)
	}
}

func cleanDeckName(n string) string {
	var b strings.Builder
	runes := 0
	for _, r := range strings.TrimSpace(n) {
		if unicode.IsControl(r) {
			continue
		}
		b.WriteRune(r)
		if runes++; runes >= maxDeckName {
			break
		}
	}
	if name := strings.TrimSpace(b.String()); name != "" {
		return name
	}
	return "Untitled deck"
}

// readDeck reads and checks a request body. It writes the error reply itself when there is one.
func (s *Server) readDeck(w http.ResponseWriter, r *http.Request) (deckBody, bool) {
	if mt, _, err := mime.ParseMediaType(r.Header.Get("Content-Type")); err != nil || mt != "application/json" {
		writeError(w, http.StatusUnsupportedMediaType, "send the deck as application/json")
		return deckBody{}, false
	}
	var b deckBody
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxDeckBody))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&b); err != nil {
		writeError(w, http.StatusBadRequest, "not a deck: "+err.Error())
		return deckBody{}, false
	}
	b.Name, b.Hero = cleanDeckName(b.Name), strings.TrimSpace(b.Hero)
	if b.Cards == nil {
		b.Cards = []string{}
	}
	if err := decks.Check(s.cat, b.Hero, b.Cards); err != nil {
		writeError(w, http.StatusUnprocessableEntity, err.Error())
		return deckBody{}, false
	}
	if err := decks.CheckLandscapes(b.Landscapes); err != nil {
		writeError(w, http.StatusUnprocessableEntity, err.Error())
		return deckBody{}, false
	}
	return b, true
}

// describeSaved says what stops a saved deck being played. The card data can change after a deck was
// saved, so a card it names may be gone.
func (s *Server) describeSaved(d store.Deck, owner string) (decks.Info, []string) {
	if err := decks.Check(s.cat, d.Hero, d.Cards); err != nil {
		return decks.Info{}, []string{err.Error()}
	}
	if err := decks.CheckLandscapes(d.Landscapes); err != nil {
		return decks.Info{}, []string{err.Error()}
	}
	info := decks.Custom(s.cat, d.Name, owner, d.Hero, d.Cards, d.Landscapes)
	return info, info.Problems(s.cat, s.cfg.AllowUnmodelled)
}

func (s *Server) savedView(d store.Deck, owner string) savedDeck {
	info, problems := s.describeSaved(d, owner)
	if problems == nil {
		problems = []string{}
	}
	lands := []card.Landscape{}
	if info.LandOK {
		lands = info.Landscapes[:]
	}
	return savedDeck{ID: d.ID, Name: d.Name, Hero: d.Hero, Cards: d.Cards, Landscapes: lands, Chosen: info.Chosen,
		Created: d.Created, Updated: d.Updated, Playable: len(problems) == 0, Problems: problems}
}

func storeCtx(r *http.Request) (context.Context, context.CancelFunc) {
	return context.WithTimeout(r.Context(), storeTimeout)
}

func (s *Server) storeFailed(w http.ResponseWriter, err error) {
	s.log.Error("deck store", "err", err)
	writeError(w, http.StatusInternalServerError, "could not reach the deck store; try again")
}

func (s *Server) listMyDecks(w http.ResponseWriter, r *http.Request, u store.User) {
	ctx, cancel := storeCtx(r)
	defer cancel()
	list, err := s.cfg.Store.Decks(ctx, u.ID)
	if err != nil {
		s.storeFailed(w, err)
		return
	}
	out := make([]savedDeck, 0, len(list))
	for _, d := range list {
		out = append(out, s.savedView(d, u.DisplayName))
	}
	writeJSON(w, out)
}

func (s *Server) createMyDeck(w http.ResponseWriter, r *http.Request, u store.User) {
	b, ok := s.readDeck(w, r)
	if !ok {
		return
	}
	ctx, cancel := storeCtx(r)
	defer cancel()
	have, err := s.cfg.Store.Decks(ctx, u.ID)
	if err != nil {
		s.storeFailed(w, err)
		return
	}
	if len(have) >= maxDecksPerUser {
		writeError(w, http.StatusConflict, "you can keep at most 50 decks: delete one first")
		return
	}
	d, err := s.cfg.Store.SaveDeck(ctx, store.Deck{Owner: u.ID, Name: b.Name, Hero: b.Hero, Cards: b.Cards, Landscapes: b.Landscapes})
	if err != nil {
		s.storeFailed(w, err)
		return
	}
	w.WriteHeader(http.StatusCreated)
	writeJSON(w, s.savedView(d, u.DisplayName))
}

func (s *Server) updateMyDeck(w http.ResponseWriter, r *http.Request, u store.User) {
	b, ok := s.readDeck(w, r)
	if !ok {
		return
	}
	ctx, cancel := storeCtx(r)
	defer cancel()
	d, err := s.cfg.Store.SaveDeck(ctx, store.Deck{ID: r.PathValue("id"), Owner: u.ID, Name: b.Name, Hero: b.Hero, Cards: b.Cards, Landscapes: b.Landscapes})
	switch {
	case errors.Is(err, store.ErrNotFound):
		writeError(w, http.StatusNotFound, "no such deck")
	case err != nil:
		s.storeFailed(w, err)
	default:
		writeJSON(w, s.savedView(d, u.DisplayName))
	}
}

func (s *Server) deleteMyDeck(w http.ResponseWriter, r *http.Request, u store.User) {
	ctx, cancel := storeCtx(r)
	defer cancel()
	switch err := s.cfg.Store.DeleteDeck(ctx, u.ID, r.PathValue("id")); {
	case errors.Is(err, store.ErrNotFound):
		writeError(w, http.StatusNotFound, "no such deck")
	case err != nil:
		s.storeFailed(w, err)
	default:
		w.WriteHeader(http.StatusNoContent)
	}
}

// resolveDeck finds the deck a client asks for: a preset by words, or one of the player's own as
// "my:<id>". Either way it must be playable.
func (s *Server) resolveDeck(user *store.User, query string) (decks.Info, error) {
	id, own := strings.CutPrefix(query, savedPrefix)
	if !own {
		return s.deckFor(query)
	}
	if user == nil || s.cfg.Store == nil {
		return decks.Info{}, fail(ErrUnknownDeck, "sign in to play your own decks")
	}
	ctx, cancel := context.WithTimeout(context.Background(), storeTimeout)
	defer cancel()
	d, err := s.cfg.Store.Deck(ctx, user.ID, id)
	if errors.Is(err, store.ErrNotFound) {
		return decks.Info{}, fail(ErrUnknownDeck, "no such deck")
	}
	if err != nil {
		s.log.Error("deck store", "err", err)
		return decks.Info{}, fail(ErrBadRequest, "could not load that deck; try again")
	}
	info, problems := s.describeSaved(d, user.DisplayName)
	if len(problems) > 0 {
		return decks.Info{}, fail(ErrDeckNotReady, d.Name+" cannot be played yet: "+strings.Join(problems, "; "))
	}
	return info, nil
}
