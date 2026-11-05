package server

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"path/filepath"
	"strings"
	"testing"

	"cardwars/internal/decks"
	"cardwars/internal/store"
)

// storeAuth signs in whoever sends X-Test-User, making their account on first sight.
type storeAuth struct{ db store.Store }

func (a storeAuth) Identify(r *http.Request) (store.User, bool) {
	name := r.Header.Get("X-Test-User")
	if name == "" {
		return store.User{}, false
	}
	u, err := a.db.UserByIdentity(context.Background(), "test", name)
	if err != nil {
		if u, err = a.db.CreateUser(context.Background(), "test", name, name); err != nil {
			return store.User{}, false
		}
	}
	return u, true
}

func (storeAuth) Routes(*http.ServeMux) {}

func decksHarness(t *testing.T, mutate ...func(*Config)) *harness {
	t.Helper()
	db, err := store.OpenSQLite(filepath.Join(t.TempDir(), "cardwars.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	return newHarness(t, append([]func(*Config){func(c *Config) { c.Store, c.Auth = db, storeAuth{db} }}, mutate...)...)
}

type apiReply struct {
	status int
	body   []byte
}

func (a apiReply) json(t *testing.T, v any) {
	t.Helper()
	if err := json.Unmarshal(a.body, v); err != nil {
		t.Fatalf("reply %d %s: %v", a.status, a.body, err)
	}
}

func (a apiReply) errText() string {
	var e struct{ Error string }
	json.Unmarshal(a.body, &e)
	return e.Error
}

// call makes an API request as user ("" for a guest). body is sent as JSON unless it is already a string.
func (h *harness) call(user, method, path string, body any, hdr ...string) apiReply {
	h.t.Helper()
	var rd *bytes.Reader
	switch b := body.(type) {
	case nil:
		rd = bytes.NewReader(nil)
	case string:
		rd = bytes.NewReader([]byte(b))
	default:
		raw, _ := json.Marshal(b)
		rd = bytes.NewReader(raw)
	}
	req, err := http.NewRequest(method, h.ts.URL+path, rd)
	if err != nil {
		h.t.Fatal(err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if user != "" {
		req.Header.Set("X-Test-User", user)
	}
	for i := 0; i+1 < len(hdr); i += 2 {
		req.Header.Set(hdr[i], hdr[i+1])
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		h.t.Fatal(err)
	}
	defer resp.Body.Close()
	var buf bytes.Buffer
	buf.ReadFrom(resp.Body)
	return apiReply{resp.StatusCode, buf.Bytes()}
}

// finnDeck is the Finn starter deck's own cards and Hero, a deck that is playable as it is.
func finnDeck(t *testing.T, h *harness) deckBody {
	t.Helper()
	d, err := decks.Pick(h.srv.decks, "finn")
	if err != nil {
		t.Fatal(err)
	}
	return deckBody{Name: "My Finn", Hero: d.Hero, Cards: d.Cards}
}

func TestSavedDecksNeedSigningIn(t *testing.T) {
	h := decksHarness(t)
	for _, c := range [][2]string{{"GET", "/api/my/decks"}, {"POST", "/api/my/decks"}, {"PUT", "/api/my/decks/x"}, {"DELETE", "/api/my/decks/x"}} {
		if r := h.call("", c[0], c[1], deckBody{Name: "x"}); r.status != http.StatusUnauthorized {
			t.Errorf("%s %s as a guest: %d", c[0], c[1], r.status)
		}
	}
	off := newHarness(t) // no store: the API does not exist
	if r := off.call("Ada", "GET", "/api/my/decks", nil); r.status != http.StatusNotFound {
		t.Errorf("without a store: %d", r.status)
	}
}

func TestSavingListingChangingAndDeletingADeck(t *testing.T) {
	h := decksHarness(t)
	full := finnDeck(t, h)

	var made savedDeck
	r := h.call("Ada", "POST", "/api/my/decks", full)
	if r.status != http.StatusCreated {
		t.Fatalf("create: %d %s", r.status, r.body)
	}
	r.json(t, &made)
	if made.ID == "" || made.Name != "My Finn" || made.Hero != full.Hero || len(made.Cards) != len(full.Cards) || !made.Playable || len(made.Problems) != 0 {
		t.Errorf("created %+v", made)
	}

	draft := h.call("Ada", "POST", "/api/my/decks", deckBody{Name: "  Draft\x00  ", Cards: full.Cards[:5]})
	var d2 savedDeck
	draft.json(t, &d2)
	if draft.status != http.StatusCreated || d2.Name != "Draft" || d2.Playable || len(d2.Problems) < 2 {
		t.Errorf("a draft is saved but not playable: %d %+v", draft.status, d2)
	}

	var list []savedDeck
	h.call("Ada", "GET", "/api/my/decks", nil).json(t, &list)
	if len(list) != 2 {
		t.Fatalf("listed %d decks, want 2", len(list))
	}
	var other []savedDeck
	h.call("Grace", "GET", "/api/my/decks", nil).json(t, &other)
	if len(other) != 0 {
		t.Errorf("another player sees %d of Ada's decks", len(other))
	}

	// Grace can neither change nor delete Ada's deck.
	if r := h.call("Grace", "PUT", "/api/my/decks/"+made.ID, full); r.status != http.StatusNotFound {
		t.Errorf("Grace changed Ada's deck: %d", r.status)
	}
	if r := h.call("Grace", "DELETE", "/api/my/decks/"+made.ID, nil); r.status != http.StatusNotFound {
		t.Errorf("Grace deleted Ada's deck: %d", r.status)
	}

	renamed := full
	renamed.Name = "Renamed"
	var upd savedDeck
	ru := h.call("Ada", "PUT", "/api/my/decks/"+made.ID, renamed)
	ru.json(t, &upd)
	if ru.status != http.StatusOK || upd.Name != "Renamed" || upd.ID != made.ID || !upd.Playable {
		t.Errorf("update: %d %+v", ru.status, upd)
	}
	if r := h.call("Ada", "PUT", "/api/my/decks/nope", full); r.status != http.StatusNotFound {
		t.Errorf("updating a deck that does not exist: %d", r.status)
	}
	if r := h.call("Ada", "DELETE", "/api/my/decks/"+made.ID, nil); r.status != http.StatusNoContent {
		t.Errorf("delete: %d", r.status)
	}
	if r := h.call("Ada", "DELETE", "/api/my/decks/"+made.ID, nil); r.status != http.StatusNotFound {
		t.Errorf("deleting twice: %d", r.status)
	}
}

func TestBadDecksAreRefused(t *testing.T) {
	h := decksHarness(t)
	full := finnDeck(t, h)
	four := deckBody{Name: "x", Cards: []string{full.Cards[0], full.Cards[0], full.Cards[0], full.Cards[0]}}
	for name, tc := range map[string]struct {
		body   any
		status int
		hdr    []string
		text   string
	}{
		"an unknown card":       {deckBody{Cards: []string{"no-such-card"}}, 422, nil, "unknown card"},
		"four copies":           {four, 422, nil, "more than 3"},
		"a Hero as a card":      {deckBody{Cards: []string{full.Hero}}, 422, nil, "Hero"},
		"not a Hero":            {deckBody{Hero: full.Cards[0]}, 422, nil, "not a Hero"},
		"a field we don't have": {`{"name":"x","cards":[],"owner":"someone else"}`, 400, nil, "not a deck"},
		"not JSON":              {`{`, 400, nil, "not a deck"},
		"too big":               {`{"name":"` + strings.Repeat("a", maxDeckBody) + `"}`, 400, nil, "not a deck"},
		"from another site":     {full, 403, []string{"Origin", "https://evil.example"}, "origin"},
	} {
		r := h.call("Ada", "POST", "/api/my/decks", tc.body, tc.hdr...)
		if r.status != tc.status || !strings.Contains(strings.ToLower(r.errText()), strings.ToLower(tc.text)) {
			t.Errorf("%s: %d %q, want %d containing %q", name, r.status, r.errText(), tc.status, tc.text)
		}
	}
	req, _ := http.NewRequest("POST", h.ts.URL+"/api/my/decks", strings.NewReader(`{"name":"x"}`))
	req.Header.Set("X-Test-User", "Ada")
	req.Header.Set("Content-Type", "text/plain") // what a cross-site form can send
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusUnsupportedMediaType {
		t.Errorf("a non-JSON body: %d", resp.StatusCode)
	}
	var list []savedDeck
	h.call("Ada", "GET", "/api/my/decks", nil).json(t, &list)
	if len(list) != 0 {
		t.Errorf("%d decks were saved from refused requests", len(list))
	}
}

func TestADeckIsKeptWithinLimits(t *testing.T) {
	h := decksHarness(t)
	for i := 0; i < maxDecksPerUser; i++ {
		if r := h.call("Ada", "POST", "/api/my/decks", deckBody{Name: fmt.Sprint("deck ", i)}); r.status != http.StatusCreated {
			t.Fatalf("deck %d: %d %s", i, r.status, r.body)
		}
	}
	if r := h.call("Ada", "POST", "/api/my/decks", deckBody{Name: "one too many"}); r.status != http.StatusConflict {
		t.Errorf("the 51st deck: %d", r.status)
	}
	if r := h.call("Grace", "POST", "/api/my/decks", deckBody{Name: "hers"}); r.status != http.StatusCreated {
		t.Errorf("the limit is per player: %d", r.status)
	}
	long := h.call("Grace", "POST", "/api/my/decks", deckBody{Name: strings.Repeat("é", 100)})
	var d savedDeck
	long.json(t, &d)
	if len([]rune(d.Name)) != maxDeckName {
		t.Errorf("name has %d characters, want %d", len([]rune(d.Name)), maxDeckName)
	}
	if r := h.call("Grace", "POST", "/api/my/decks", deckBody{}); true {
		var e savedDeck
		r.json(t, &e)
		if e.Name != "Untitled deck" {
			t.Errorf("an empty name became %q", e.Name)
		}
	}
}

func TestPlayingASavedDeck(t *testing.T) {
	h := decksHarness(t)
	var good, draft savedDeck
	h.call("Ada", "POST", "/api/my/decks", finnDeck(t, h)).json(t, &good)
	h.call("Ada", "POST", "/api/my/decks", deckBody{Name: "Draft", Cards: finnDeck(t, h).Cards[:5]}).json(t, &draft)

	// The bot's deck defaults to a starter even though the player's deck is not one.
	ada := h.dialAs("Ada")
	ada.send(ClientMsg{Type: MsgCreate, Deck: "my:" + good.ID, VsBot: true})
	joined := ada.expect(MsgJoined)
	up := ada.expect(MsgUpdate)
	if joined.Names[0] != "Ada" || len(up.View.Players[0].Hand) == 0 || up.View.Players[0].Hero == nil || up.View.Players[0].Hero.CardID != finnDeck(t, h).Hero {
		t.Errorf("did not start with the saved deck and its Hero: %+v", up.View.Players[0].Hero)
	}

	ada2 := h.dialAs("Ada")
	ada2.send(ClientMsg{Type: MsgCreate, Deck: "my:" + draft.ID, VsBot: true})
	if m := ada2.expect(MsgError); m.Code != ErrDeckNotReady || !strings.Contains(m.Message, "Draft cannot be played yet") {
		t.Errorf("an unfinished deck: %s %q", m.Code, m.Message)
	}

	grace := h.dialAs("Grace")
	grace.send(ClientMsg{Type: MsgCreate, Deck: "my:" + good.ID, VsBot: true})
	grace.expectError(ErrUnknownDeck) // not hers

	guest := h.dial()
	guest.send(ClientMsg{Type: MsgCreate, Deck: "my:" + good.ID, VsBot: true})
	guest.expectError(ErrUnknownDeck)

	// Joining works the same way as creating.
	host := h.dialAs("Ada")
	host.send(ClientMsg{Type: MsgCreate, Deck: "jake"})
	room := host.expect(MsgJoined).Room
	joiner := h.dialAs("Ada")
	joiner.send(ClientMsg{Type: MsgJoin, Room: room, Deck: "my:" + good.ID})
	joiner.expect(MsgJoined)
}

func TestPreflightForTheSavedDecksAPI(t *testing.T) {
	h := decksHarness(t, func(c *Config) { c.OriginPatterns = []string{"localhost:*"} })
	req, _ := http.NewRequest("OPTIONS", h.ts.URL+"/api/my/decks/abc", nil)
	req.Header.Set("Origin", "http://localhost:5173")
	req.Header.Set("Access-Control-Request-Method", "PUT")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	got := resp.Header
	if resp.StatusCode != http.StatusNoContent || got.Get("Access-Control-Allow-Origin") != "http://localhost:5173" ||
		!strings.Contains(got.Get("Access-Control-Allow-Methods"), "PUT") || !strings.Contains(got.Get("Access-Control-Allow-Headers"), "Content-Type") ||
		got.Get("Access-Control-Allow-Credentials") != "true" {
		t.Errorf("preflight: %d %v", resp.StatusCode, got)
	}
	req.Header.Set("Origin", "https://evil.example")
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.Header.Get("Access-Control-Allow-Origin") != "" || resp.Header.Get("Access-Control-Allow-Methods") != "" {
		t.Errorf("an unlisted origin got CORS headers: %v", resp.Header)
	}
}
