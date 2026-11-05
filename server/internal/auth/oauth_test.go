package auth

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"golang.org/x/oauth2"

	"cardwars/internal/store"
)

// fakeProvider plays the part of Discord or GitHub: a token endpoint that
// checks the PKCE verifier against the challenge it was sent, and a user endpoint.
type fakeProvider struct {
	*httptest.Server
	challenge string // the last code_challenge it was sent
	subject   string
	name      string
	userFails bool
	tokenSeen int
}

func newFakeProvider(t *testing.T) *fakeProvider {
	f := &fakeProvider{subject: "u-1", name: "Fay"}
	mux := http.NewServeMux()
	mux.HandleFunc("POST /token", func(w http.ResponseWriter, r *http.Request) {
		f.tokenSeen++
		r.ParseForm()
		sum := sha256.Sum256([]byte(r.Form.Get("code_verifier")))
		if r.Form.Get("code") != "good-code" || base64.RawURLEncoding.EncodeToString(sum[:]) != f.challenge {
			http.Error(w, `{"error":"invalid_grant"}`, http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"access_token":"tok","token_type":"bearer"}`))
	})
	mux.HandleFunc("GET /user", func(w http.ResponseWriter, r *http.Request) {
		if f.userFails || r.Header.Get("Authorization") != "Bearer tok" {
			http.Error(w, "no", http.StatusInternalServerError)
			return
		}
		json.NewEncoder(w).Encode(map[string]string{"id": f.subject, "name": f.name})
	})
	f.Server = httptest.NewServer(mux)
	t.Cleanup(f.Close)
	return f
}

func (f *fakeProvider) provider() provider {
	return provider{
		name: "fake", label: "Fake",
		endpoint: oauth2.Endpoint{AuthURL: f.URL + "/authorize", TokenURL: f.URL + "/token"},
		userURL:  f.URL + "/user",
		parse: func(b []byte) (string, string, error) {
			var u struct{ ID, Name string }
			err := json.Unmarshal(b, &u)
			return u.ID, u.Name, err
		},
	}
}

type env struct {
	t     *testing.T
	a     *OAuth
	db    *store.SQLite
	fake  *fakeProvider
	mux   *http.ServeMux
	clock time.Time
}

func newEnv(t *testing.T, publicURL string) *env {
	t.Helper()
	db, err := store.OpenSQLite(filepath.Join(t.TempDir(), "cardwars.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	e := &env{t: t, db: db, fake: newFakeProvider(t), mux: http.NewServeMux(), clock: time.Date(2026, 11, 5, 12, 0, 0, 0, time.UTC)}
	e.a, err = newOAuth(OAuthOptions{
		Store: db, PublicURL: publicURL, SessionTTL: time.Hour, HTTPClient: e.fake.Client(), Now: func() time.Time { return e.clock },
		Credentials: map[string]Credentials{"fake": {ClientID: "id", ClientSecret: "secret"}},
	}, []provider{e.fake.provider()})
	if err != nil {
		t.Fatal(err)
	}
	e.a.Routes(e.mux)
	return e
}

func (e *env) do(method, target string, cookies []*http.Cookie, hdr map[string]string) *httptest.ResponseRecorder {
	r := httptest.NewRequest(method, target, nil)
	for _, c := range cookies {
		r.AddCookie(c)
	}
	for k, v := range hdr {
		r.Header.Set(k, v)
	}
	w := httptest.NewRecorder()
	e.mux.ServeHTTP(w, r)
	return w
}

func cookieNamed(w *httptest.ResponseRecorder, name string) *http.Cookie {
	for _, c := range w.Result().Cookies() {
		if c.Name == name {
			return c
		}
	}
	return nil
}

// start leaves for the provider and returns the state and the cookie that goes with it.
func (e *env) start() (state string, cookie *http.Cookie) {
	e.t.Helper()
	w := e.do("GET", "/api/auth/login/fake", nil, nil)
	if w.Code != http.StatusFound {
		e.t.Fatalf("login: %d %s", w.Code, w.Body)
	}
	loc, err := url.Parse(w.Header().Get("Location"))
	if err != nil {
		e.t.Fatal(err)
	}
	q := loc.Query()
	if q.Get("code_challenge_method") != "S256" || q.Get("code_challenge") == "" || q.Get("client_id") != "id" || q.Get("response_type") != "code" {
		e.t.Fatalf("authorize request: %v", q)
	}
	if want := e.a.base.String() + "/api/auth/callback/fake"; q.Get("redirect_uri") != want {
		e.t.Fatalf("redirect_uri %q, want %q", q.Get("redirect_uri"), want)
	}
	e.fake.challenge = q.Get("code_challenge")
	return q.Get("state"), cookieNamed(w, e.a.stateCookie())
}

func (e *env) callback(query string, cookie *http.Cookie) *httptest.ResponseRecorder {
	var cookies []*http.Cookie
	if cookie != nil {
		cookies = []*http.Cookie{cookie}
	}
	return e.do("GET", "/api/auth/callback/fake?"+query, cookies, nil)
}

// signIn goes through the whole flow and returns the session cookie.
func (e *env) signIn() *http.Cookie {
	e.t.Helper()
	state, sc := e.start()
	w := e.callback("code=good-code&state="+state, sc)
	if w.Code != http.StatusFound || w.Header().Get("Location") != "/" {
		e.t.Fatalf("callback: %d to %q", w.Code, w.Header().Get("Location"))
	}
	c := cookieNamed(w, e.a.sessionCookie())
	if c == nil || c.Value == "" {
		e.t.Fatal("no session cookie")
	}
	return c
}

func (e *env) identify(c *http.Cookie) (store.User, bool) {
	r := httptest.NewRequest("GET", "/", nil)
	if c != nil {
		r.AddCookie(c)
	}
	return e.a.Identify(r)
}

func TestSigningInMakesOneAccountAndASession(t *testing.T) {
	e := newEnv(t, "http://localhost:8080")
	c := e.signIn()
	u, ok := e.identify(c)
	if !ok || u.Provider != "fake" || u.Subject != "u-1" || u.DisplayName != "Fay" {
		t.Fatalf("Identify = %+v, %v", u, ok)
	}
	if !c.HttpOnly || c.SameSite != http.SameSiteLaxMode || c.Path != "/" {
		t.Errorf("session cookie %+v", c)
	}
	// Only a hash of the token is in the database.
	if _, err := e.db.Session(context.Background(), []byte(c.Value), e.clock); err == nil {
		t.Error("the raw token is a valid session key: it must be hashed")
	}
	// Signing in again is the same person, with a new session.
	e.fake.name = "Renamed upstream"
	c2 := e.signIn()
	u2, ok := e.identify(c2)
	if !ok || u2.ID != u.ID || u2.DisplayName != "Fay" {
		t.Errorf("second sign-in: %+v, %v (want the same account, name kept)", u2, ok)
	}
	if c2.Value == c.Value {
		t.Error("two sign-ins share a session token")
	}
	if _, ok := e.identify(c); !ok {
		t.Error("the first session ended when the second began")
	}
	// me tells the client who is signed in.
	w := e.do("GET", "/api/auth/me", []*http.Cookie{c}, nil)
	var me struct{ User *struct{ ID, Name string } }
	if err := json.Unmarshal(w.Body.Bytes(), &me); err != nil || me.User == nil || me.User.ID != u.ID || me.User.Name != "Fay" {
		t.Errorf("me = %s (%v)", w.Body, err)
	}
	if w := e.do("GET", "/api/auth/me", nil, nil); strings.TrimSpace(w.Body.String()) != `{"user":null}` {
		t.Errorf("anonymous me = %s", w.Body)
	}
}

func TestAStateWorksOnceAndOnlyInTheBrowserThatStartedIt(t *testing.T) {
	e := newEnv(t, "http://localhost:8080")
	state, sc := e.start()
	for name, tc := range map[string]struct {
		query  string
		cookie *http.Cookie
	}{
		"no cookie":    {"code=good-code&state=" + state, nil},
		"other cookie": {"code=good-code&state=" + state, &http.Cookie{Name: sc.Name, Value: "other"}},
		"no state":     {"code=good-code", sc},
		"made up":      {"code=good-code&state=made-up", &http.Cookie{Name: sc.Name, Value: "made-up"}},
	} {
		w := e.callback(tc.query, tc.cookie)
		if w.Code != http.StatusFound || !strings.Contains(w.Header().Get("Location"), "login_error=state") || cookieNamed(w, e.a.sessionCookie()) != nil {
			t.Errorf("%s: %d to %q", name, w.Code, w.Header().Get("Location"))
		}
	}
	if e.fake.tokenSeen != 0 {
		t.Error("the provider was asked for a token despite a bad state")
	}
	// The failures above did not use the state up, so the real one still works, once.
	if w := e.callback("code=good-code&state="+state, sc); cookieNamed(w, e.a.sessionCookie()) == nil {
		t.Fatalf("the real callback failed: %q", w.Header().Get("Location"))
	}
	if w := e.callback("code=good-code&state="+state, sc); !strings.Contains(w.Header().Get("Location"), "login_error=state") {
		t.Errorf("a state was accepted twice: %q", w.Header().Get("Location"))
	}
}

func TestSignInsThatGoWrongLeaveNobodySignedIn(t *testing.T) {
	e := newEnv(t, "http://localhost:8080")
	check := func(name, query, reason string) {
		t.Helper()
		state, sc := e.start()
		w := e.callback(strings.ReplaceAll(query, "STATE", state), sc)
		if loc := w.Header().Get("Location"); !strings.Contains(loc, "login_error="+reason) || cookieNamed(w, e.a.sessionCookie()) != nil {
			t.Errorf("%s: redirected to %q", name, loc)
		}
	}
	check("the person said no", "error=access_denied&state=STATE", "denied")
	check("no code", "state=STATE", "denied")
	check("a bad code", "code=bad&state=STATE", "failed")
	e.fake.userFails = true
	check("the user lookup fails", "code=good-code&state=STATE", "failed")
	e.fake.userFails = false
	e.fake.subject = ""
	check("no id in the reply", "code=good-code&state=STATE", "failed")
	if users, _ := e.db.UserByIdentity(context.Background(), "fake", ""); users.ID != "" {
		t.Error("an account was made without a provider id")
	}
}

func TestLoggingOutEndsTheSession(t *testing.T) {
	e := newEnv(t, "http://localhost:8080")
	c := e.signIn()
	if w := e.do("POST", "/api/auth/logout", []*http.Cookie{c}, map[string]string{"Origin": "https://evil.example"}); w.Code != http.StatusForbidden {
		t.Errorf("logout from another origin: %d", w.Code)
	}
	if _, ok := e.identify(c); !ok {
		t.Fatal("a refused logout ended the session")
	}
	e.a.opts.ReturnHosts = []string{"localhost:*"}
	w := e.do("POST", "/api/auth/logout", []*http.Cookie{c}, map[string]string{"Origin": "http://localhost:5173"}) // an allowed web client
	if w.Code != http.StatusNoContent {
		t.Fatalf("logout from an allowed web client: %d", w.Code)
	}
	if _, ok := e.identify(c); ok {
		t.Fatal("the session survived")
	}
	c = e.signIn()
	w = e.do("POST", "/api/auth/logout", []*http.Cookie{c}, map[string]string{"Origin": "http://localhost:8080"})
	if w.Code != http.StatusNoContent {
		t.Fatalf("logout: %d", w.Code)
	}
	if cleared := cookieNamed(w, e.a.sessionCookie()); cleared == nil || cleared.MaxAge >= 0 {
		t.Errorf("the cookie was not cleared: %+v", cleared)
	}
	if _, ok := e.identify(c); ok {
		t.Error("the session still works after logging out")
	}
}

func TestSessionsExpire(t *testing.T) {
	e := newEnv(t, "http://localhost:8080")
	c := e.signIn()
	e.clock = e.clock.Add(59 * time.Minute)
	if _, ok := e.identify(c); !ok {
		t.Error("the session ended early")
	}
	e.clock = e.clock.Add(2 * time.Minute)
	if _, ok := e.identify(c); ok {
		t.Error("the session outlived its time")
	}
	if _, ok := e.identify(nil); ok {
		t.Error("no cookie counts as signed in")
	}
	if _, ok := e.identify(&http.Cookie{Name: e.a.sessionCookie(), Value: "guess"}); ok {
		t.Error("a made-up token counts as signed in")
	}
}

func TestHTTPSMakesTheCookiesSecureAndHostBound(t *testing.T) {
	e := newEnv(t, "https://cards.example.com/")
	_, sc := e.start()
	if sc.Name != "__Host-cw_oauth" || !sc.Secure || !sc.HttpOnly {
		t.Errorf("state cookie %+v", sc)
	}
	c := e.signIn()
	if c.Name != "__Host-cw_session" || !c.Secure || !c.HttpOnly || c.Path != "/" || c.Domain != "" || c.MaxAge != 3600 {
		t.Errorf("session cookie %+v", c)
	}
	if want := "https://cards.example.com/api/auth/callback/fake"; e.a.oauth["fake"].RedirectURL != want {
		t.Errorf("redirect %q, want %q", e.a.oauth["fake"].RedirectURL, want)
	}
}

func TestConfigAndUnknownProviders(t *testing.T) {
	e := newEnv(t, "http://localhost:8080")
	w := e.do("GET", "/api/auth/config", nil, nil)
	if got := strings.TrimSpace(w.Body.String()); got != `{"mode":"oauth","providers":[{"name":"fake","label":"Fake"}]}` {
		t.Errorf("config = %s", got)
	}
	if w := e.do("GET", "/api/auth/login/nope", nil, nil); w.Code != http.StatusNotFound {
		t.Errorf("unknown provider: %d", w.Code)
	}
	if w := e.do("GET", "/api/auth/callback/nope?code=x&state=y", nil, nil); w.Code != http.StatusNotFound {
		t.Errorf("unknown provider callback: %d", w.Code)
	}
}

func TestOnlyManySignInsInFlightAreRefused(t *testing.T) {
	e := newEnv(t, "http://localhost:8080")
	for i := 0; i < maxPending; i++ {
		if w := e.do("GET", "/api/auth/login/fake", nil, nil); w.Code != http.StatusFound {
			t.Fatalf("sign-in %d refused: %d", i, w.Code)
		}
	}
	if w := e.do("GET", "/api/auth/login/fake", nil, nil); w.Code != http.StatusServiceUnavailable {
		t.Errorf("one too many: %d", w.Code)
	}
	e.clock = e.clock.Add(loginTimeout + time.Second) // the ones that never came back are forgotten
	if w := e.do("GET", "/api/auth/login/fake", nil, nil); w.Code != http.StatusFound {
		t.Errorf("after they expired: %d", w.Code)
	}
}

func TestNewOAuthChecksItsSettings(t *testing.T) {
	db, err := store.OpenSQLite(filepath.Join(t.TempDir(), "cardwars.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	creds := map[string]Credentials{"github": {"id", "secret"}}
	for name, o := range map[string]OAuthOptions{
		"no store":           {PublicURL: "https://x.example", Credentials: creds},
		"no public url":      {Store: db, Credentials: creds},
		"a url without http": {Store: db, PublicURL: "cards.example.com", Credentials: creds},
		"no credentials":     {Store: db, PublicURL: "https://x.example"},
		"half a credential":  {Store: db, PublicURL: "https://x.example", Credentials: map[string]Credentials{"github": {ClientID: "id"}}},
	} {
		if _, err := NewOAuth(o); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
	a, err := NewOAuth(OAuthOptions{Store: db, PublicURL: "https://x.example", Credentials: map[string]Credentials{"github": {"id", "secret"}, "discord": {"id", "secret"}}})
	if err != nil || len(a.providers) != 2 {
		t.Fatalf("NewOAuth = %v, %v", a, err)
	}
}

func TestProvidersReadTheirUsers(t *testing.T) {
	for _, tc := range []struct {
		p                 provider
		body, subject, nm string
	}{
		{discord(), `{"id":"80351110224678912","username":"nelly","global_name":"Nelly"}`, "80351110224678912", "Nelly"},
		{discord(), `{"id":"80351110224678912","username":"nelly","global_name":null}`, "80351110224678912", "nelly"},
		{github(), `{"id":583231,"login":"octocat","name":"The Octocat"}`, "583231", "The Octocat"},
		{github(), `{"id":583231,"login":"octocat","name":null}`, "583231", "octocat"},
	} {
		subject, name, err := tc.p.parse([]byte(tc.body))
		if err != nil || subject != tc.subject || name != tc.nm {
			t.Errorf("%s %s: %q %q %v", tc.p.name, tc.body, subject, name, err)
		}
	}
	for _, p := range []provider{discord(), github()} {
		if _, _, err := p.parse([]byte(`{"login":"x"}`)); err == nil {
			t.Errorf("%s: a reply with no id was accepted", p.name)
		}
		if _, _, err := p.parse([]byte(`not json`)); err == nil {
			t.Errorf("%s: garbage was accepted", p.name)
		}
	}
}

func TestNamesAreCleaned(t *testing.T) {
	if got := cleanName("  Lu\x00kas\x07  ", "x"); got != "Lukas" {
		t.Errorf("got %q", got)
	}
	if got := cleanName("\x00\x01", "fallback"); got != "fallback" {
		t.Errorf("got %q", got)
	}
	if got := cleanName(strings.Repeat("a", 100), "x"); len(got) != maxNameLen {
		t.Errorf("length %d", len(got))
	}
}

func TestNoneNeverSignsAnyoneIn(t *testing.T) {
	mux := http.NewServeMux()
	None().Routes(mux)
	if _, ok := None().Identify(httptest.NewRequest("GET", "/", nil)); ok {
		t.Error("anyone is signed in without sign-in")
	}
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, httptest.NewRequest("GET", "/api/auth/config", nil))
	if got := strings.TrimSpace(w.Body.String()); got != `{"mode":"none","providers":[]}` {
		t.Errorf("config = %s", got)
	}
}

func TestSignInReturnsToAnAllowedPageOnly(t *testing.T) {
	e := newEnv(t, "https://cards.example.com")
	e.a.opts.ReturnHosts = []string{"localhost:*"}
	for _, tc := range []struct{ asked, want string }{
		{"", "/"},
		{"https://cards.example.com/play?x=1#top", "https://cards.example.com/play"},
		{"http://localhost:5173/", "http://localhost:5173/"},
		{"https://evil.example/", "/"},
		{"https://cards.example.com.evil.example/", "/"},
		{"https://user:pw@cards.example.com/", "/"},
		{"javascript:alert(1)", "/"},
		{"//evil.example/", "/"},
		{"/just/a/path", "/"},
	} {
		if got := e.a.returnTarget(tc.asked); got != tc.want {
			t.Errorf("return=%q goes to %q, want %q", tc.asked, got, tc.want)
		}
	}
	// End to end: the page asked for is where they land, with or without success.
	w := e.do("GET", "/api/auth/login/fake?return="+url.QueryEscape("http://localhost:5173/"), nil, nil)
	loc, _ := url.Parse(w.Header().Get("Location"))
	e.fake.challenge = loc.Query().Get("code_challenge")
	state, sc := loc.Query().Get("state"), cookieNamed(w, e.a.stateCookie())
	if w := e.callback("code=good-code&state="+state, sc); w.Header().Get("Location") != "http://localhost:5173/" {
		t.Errorf("signed in, went to %q", w.Header().Get("Location"))
	}
	w = e.do("GET", "/api/auth/login/fake?return="+url.QueryEscape("http://localhost:5173/"), nil, nil)
	loc, _ = url.Parse(w.Header().Get("Location"))
	state, sc = loc.Query().Get("state"), cookieNamed(w, e.a.stateCookie())
	if w := e.callback("error=access_denied&state="+state, sc); w.Header().Get("Location") != "http://localhost:5173/?login_error=denied" {
		t.Errorf("turned down, went to %q", w.Header().Get("Location"))
	}
}
