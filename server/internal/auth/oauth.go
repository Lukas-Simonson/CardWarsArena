package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"path"
	"strings"
	"sync"
	"time"
	"unicode"

	"golang.org/x/oauth2"

	"cardwars/internal/store"
)

const (
	defaultSessionTTL = 30 * 24 * time.Hour
	loginTimeout      = 10 * time.Minute // from leaving for the provider to coming back
	maxPending        = 10000            // sign-ins in flight; the login route is open to anyone
	maxUserReply      = 1 << 20
	maxNameLen        = 24
)

// Credentials are what registering the app with a provider gives you.
type Credentials struct{ ClientID, ClientSecret string }

// OAuthOptions configures sign-in with Discord and GitHub.
type OAuthOptions struct {
	Store store.Store
	// PublicURL is where browsers reach the server, e.g. https://cards.example.com
	// (no trailing slash needed). Providers send people back to it, and it decides
	// whether cookies are marked Secure.
	PublicURL string
	// Credentials by provider name ("discord", "github"); a provider without any is not offered.
	Credentials map[string]Credentials
	// ReturnHosts are host patterns ("localhost:*") besides the public URL's own host that a
	// sign-in may send people back to, for a web client served from another origin.
	ReturnHosts []string
	SessionTTL  time.Duration // default 30 days
	HTTPClient  *http.Client  // for calls to providers; default http.DefaultClient
	Now         func() time.Time
	Logger      *slog.Logger
}

// OAuth signs people in through an OAuth2 authorization-code flow with PKCE,
// then keeps them signed in with a session cookie.
type OAuth struct {
	opts      OAuthOptions
	base      *url.URL
	secure    bool
	providers []provider
	oauth     map[string]*oauth2.Config

	mu      sync.Mutex
	pending map[string]pending
}

// pending is a sign-in that has left for the provider and not come back yet.
type pending struct {
	provider string
	verifier string
	returnTo string // where to send the person afterwards
	expires  time.Time
}

var _ Authenticator = (*OAuth)(nil)

// NewOAuth offers every provider that has credentials; it is an error to offer none.
func NewOAuth(opts OAuthOptions) (*OAuth, error) {
	var ps []provider
	for _, p := range []provider{discord(), github()} {
		if c := opts.Credentials[p.name]; c.ClientID != "" && c.ClientSecret != "" {
			ps = append(ps, p)
		}
	}
	return newOAuth(opts, ps)
}

func newOAuth(opts OAuthOptions, ps []provider) (*OAuth, error) {
	if opts.Store == nil {
		return nil, errors.New("auth: sign-in needs a database")
	}
	base, err := url.Parse(strings.TrimRight(opts.PublicURL, "/"))
	if err != nil || (base.Scheme != "http" && base.Scheme != "https") || base.Host == "" {
		return nil, fmt.Errorf("auth: public URL %q must look like https://cards.example.com", opts.PublicURL)
	}
	if len(ps) == 0 {
		return nil, errors.New("auth: no sign-in provider has credentials")
	}
	if opts.SessionTTL == 0 {
		opts.SessionTTL = defaultSessionTTL
	}
	if opts.HTTPClient == nil {
		opts.HTTPClient = http.DefaultClient
	}
	if opts.Now == nil {
		opts.Now = time.Now
	}
	if opts.Logger == nil {
		opts.Logger = slog.New(slog.DiscardHandler)
	}
	a := &OAuth{opts: opts, base: base, secure: base.Scheme == "https", providers: ps, oauth: map[string]*oauth2.Config{}, pending: map[string]pending{}}
	for _, p := range ps {
		c := opts.Credentials[p.name]
		a.oauth[p.name] = &oauth2.Config{
			ClientID: c.ClientID, ClientSecret: c.ClientSecret, Endpoint: p.endpoint, Scopes: p.scopes,
			RedirectURL: a.base.String() + "/api/auth/callback/" + p.name,
		}
	}
	return a, nil
}

func (a *OAuth) Routes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/auth/config", a.handleConfig)
	mux.HandleFunc("GET /api/auth/me", a.handleMe)
	mux.HandleFunc("GET /api/auth/login/{provider}", a.handleLogin)
	mux.HandleFunc("GET /api/auth/callback/{provider}", a.handleCallback)
	mux.HandleFunc("POST /api/auth/logout", a.handleLogout)
}

// Run removes expired sessions until ctx ends.
func (a *OAuth) Run(ctx context.Context) {
	t := time.NewTicker(time.Hour)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			if n, err := a.opts.Store.DeleteExpiredSessions(ctx, a.opts.Now()); err != nil {
				a.opts.Logger.Warn("removing expired sessions", "err", err)
			} else if n > 0 {
				a.opts.Logger.Info("removed expired sessions", "count", n)
			}
		}
	}
}

// ---- cookies

// With https the cookies get the __Host- prefix: browsers then refuse them unless they are
// Secure, set on this exact host, and valid for the whole site.
func (a *OAuth) sessionCookie() string {
	if a.secure {
		return "__Host-cw_session"
	}
	return "cw_session"
}

func (a *OAuth) stateCookie() string {
	if a.secure {
		return "__Host-cw_oauth"
	}
	return "cw_oauth"
}

func (a *OAuth) cookie(name, value string, maxAge int) *http.Cookie {
	return &http.Cookie{Name: name, Value: value, Path: "/", MaxAge: maxAge, HttpOnly: true, Secure: a.secure, SameSite: http.SameSiteLaxMode}
}

func randomToken() (string, error) {
	var b [32]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b[:]), nil
}

// hashToken is what the database keeps of a session token.
func hashToken(token string) []byte {
	h := sha256.Sum256([]byte(token))
	return h[:]
}

// ---- who is this?

func (a *OAuth) Identify(r *http.Request) (store.User, bool) {
	c, err := r.Cookie(a.sessionCookie())
	if err != nil || c.Value == "" {
		return store.User{}, false
	}
	id, err := a.opts.Store.Session(r.Context(), hashToken(c.Value), a.opts.Now())
	if err != nil {
		return store.User{}, false
	}
	u, err := a.opts.Store.User(r.Context(), id)
	if err != nil {
		return store.User{}, false
	}
	return u, true
}

func (a *OAuth) handleConfig(w http.ResponseWriter, _ *http.Request) {
	cfg := Config{Mode: "oauth", Providers: []Provider{}}
	for _, p := range a.providers {
		cfg.Providers = append(cfg.Providers, Provider{Name: p.name, Label: p.label})
	}
	writeJSON(w, cfg)
}

func (a *OAuth) handleMe(w http.ResponseWriter, r *http.Request) {
	u, ok := a.Identify(r)
	if !ok {
		writeJSON(w, map[string]any{"user": nil})
		return
	}
	writeJSON(w, map[string]any{"user": map[string]string{"id": u.ID, "name": u.DisplayName}})
}

// ---- signing in

func (a *OAuth) provider(name string) *provider {
	for i := range a.providers {
		if a.providers[i].name == name {
			return &a.providers[i]
		}
	}
	return nil
}

func (a *OAuth) handleLogin(w http.ResponseWriter, r *http.Request) {
	p := a.provider(r.PathValue("provider"))
	if p == nil {
		http.NotFound(w, r)
		return
	}
	state, err := randomToken()
	if err != nil {
		http.Error(w, "try again", http.StatusInternalServerError)
		return
	}
	verifier := oauth2.GenerateVerifier()
	now := a.opts.Now()
	a.mu.Lock()
	for s, e := range a.pending { // drop the ones that never came back
		if now.After(e.expires) {
			delete(a.pending, s)
		}
	}
	if len(a.pending) >= maxPending {
		a.mu.Unlock()
		http.Error(w, "too many sign-ins in progress; try again shortly", http.StatusServiceUnavailable)
		return
	}
	a.pending[state] = pending{provider: p.name, verifier: verifier, returnTo: a.returnTarget(r.URL.Query().Get("return")), expires: now.Add(loginTimeout)}
	a.mu.Unlock()

	// The cookie ties the sign-in to this browser: the state in the URL alone could be someone else's.
	http.SetCookie(w, a.cookie(a.stateCookie(), state, int(loginTimeout.Seconds())))
	http.Redirect(w, r, a.oauth[p.name].AuthCodeURL(state, oauth2.S256ChallengeOption(verifier)), http.StatusFound)
}

// returnTarget is where to send people after signing in: the page they asked to come back to if it is
// on this site (or an allowed one), and otherwise the front page. Anything else would let a crafted
// link send people anywhere once they have signed in.
func (a *OAuth) returnTarget(raw string) string {
	u, err := url.Parse(raw)
	if raw == "" || err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" || u.User != nil {
		return "/"
	}
	if !a.ownHost(u.Host) {
		return "/"
	}
	u.RawQuery, u.Fragment = "", "" // the page, not whatever it was carrying
	return u.String()
}

// ownHost reports whether a host is this site's own or one of the web clients allowed to use it.
func (a *OAuth) ownHost(host string) bool {
	if strings.EqualFold(host, a.base.Host) {
		return true
	}
	for _, p := range a.opts.ReturnHosts {
		if m, _ := path.Match(strings.ToLower(p), strings.ToLower(host)); m {
			return true
		}
	}
	return false
}

// fail sends the person back to the web client with a reason it can show.
func (a *OAuth) fail(w http.ResponseWriter, r *http.Request, to, reason string, err error) {
	if err != nil {
		a.opts.Logger.Warn("sign-in failed", "reason", reason, "err", err)
	}
	http.SetCookie(w, a.cookie(a.stateCookie(), "", -1))
	http.Redirect(w, r, withQuery(to, "login_error", reason), http.StatusFound)
}

func withQuery(target, key, value string) string {
	u, err := url.Parse(target)
	if err != nil {
		return "/"
	}
	q := u.Query()
	q.Set(key, value)
	u.RawQuery = q.Encode()
	return u.String()
}

func (a *OAuth) handleCallback(w http.ResponseWriter, r *http.Request) {
	p := a.provider(r.PathValue("provider"))
	if p == nil {
		http.NotFound(w, r)
		return
	}
	q := r.URL.Query()
	state := q.Get("state")
	c, err := r.Cookie(a.stateCookie())
	if err != nil || state == "" || subtle.ConstantTimeCompare([]byte(c.Value), []byte(state)) != 1 {
		a.fail(w, r, "/", "state", nil)
		return
	}
	a.mu.Lock()
	pend, ok := a.pending[state]
	delete(a.pending, state) // a state works once
	a.mu.Unlock()
	if !ok || pend.provider != p.name || a.opts.Now().After(pend.expires) {
		a.fail(w, r, "/", "state", nil)
		return
	}
	if q.Get("error") != "" || q.Get("code") == "" {
		a.fail(w, r, pend.returnTo, "denied", nil) // they said no, or the provider did
		return
	}

	ctx, cancel := context.WithTimeout(context.WithValue(r.Context(), oauth2.HTTPClient, a.opts.HTTPClient), 15*time.Second)
	defer cancel()
	tok, err := a.oauth[p.name].Exchange(ctx, q.Get("code"), oauth2.VerifierOption(pend.verifier))
	if err != nil {
		a.fail(w, r, pend.returnTo, "failed", err)
		return
	}
	subject, name, err := a.whoIs(ctx, p, tok)
	if err != nil {
		a.fail(w, r, pend.returnTo, "failed", err)
		return
	}
	user, err := a.findOrCreate(ctx, p.name, subject, name)
	if err != nil {
		a.fail(w, r, pend.returnTo, "failed", err)
		return
	}
	token, err := randomToken()
	if err != nil {
		a.fail(w, r, pend.returnTo, "failed", err)
		return
	}
	expires := a.opts.Now().Add(a.opts.SessionTTL)
	if err := a.opts.Store.CreateSession(ctx, user.ID, hashToken(token), expires); err != nil {
		a.fail(w, r, pend.returnTo, "failed", err)
		return
	}
	http.SetCookie(w, a.cookie(a.stateCookie(), "", -1))
	http.SetCookie(w, a.cookie(a.sessionCookie(), token, int(a.opts.SessionTTL.Seconds())))
	http.Redirect(w, r, pend.returnTo, http.StatusFound)
}

// whoIs asks the provider who the token belongs to.
func (a *OAuth) whoIs(ctx context.Context, p *provider, tok *oauth2.Token) (subject, name string, err error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, p.userURL, nil)
	if err != nil {
		return "", "", err
	}
	req.Header.Set("Authorization", "Bearer "+tok.AccessToken)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "cardwars")
	res, err := a.opts.HTTPClient.Do(req)
	if err != nil {
		return "", "", err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return "", "", fmt.Errorf("%s answered %s", p.name, res.Status)
	}
	body, err := io.ReadAll(io.LimitReader(res.Body, maxUserReply))
	if err != nil {
		return "", "", err
	}
	subject, name, err = p.parse(body)
	if err == nil && subject == "" {
		err = fmt.Errorf("%s gave no id for the user", p.name) // never make an account for "nobody"
	}
	return subject, name, err
}

// findOrCreate returns the account for a provider's subject, making one on the first sign-in.
func (a *OAuth) findOrCreate(ctx context.Context, provider, subject, name string) (store.User, error) {
	u, err := a.opts.Store.UserByIdentity(ctx, provider, subject)
	if !errors.Is(err, store.ErrNotFound) {
		return u, err
	}
	u, err = a.opts.Store.CreateUser(ctx, provider, subject, cleanName(name, provider+" player"), "")
	if errors.Is(err, store.ErrExists) { // two first sign-ins at once
		return a.opts.Store.UserByIdentity(ctx, provider, subject)
	}
	return u, err
}

// cleanName keeps a name short and free of control characters.
func cleanName(n, fallback string) string {
	var b strings.Builder
	n = strings.TrimSpace(n)
	for _, r := range n {
		if unicode.IsControl(r) {
			continue
		}
		b.WriteRune(r)
		if b.Len() >= maxNameLen {
			break
		}
	}
	if s := strings.TrimSpace(b.String()); s != "" {
		return s
	}
	return fallback
}

// ---- signing out

func (a *OAuth) handleLogout(w http.ResponseWriter, r *http.Request) {
	// A cookie with SameSite=Lax is not sent on a cross-site POST, but check the origin as well.
	if o := r.Header.Get("Origin"); o != "" {
		if u, err := url.Parse(o); err != nil || (!a.ownHost(u.Host) && !strings.EqualFold(u.Host, r.Host)) {
			http.Error(w, "wrong origin", http.StatusForbidden)
			return
		}
	}
	if c, err := r.Cookie(a.sessionCookie()); err == nil && c.Value != "" {
		if err := a.opts.Store.DeleteSession(r.Context(), hashToken(c.Value)); err != nil {
			a.opts.Logger.Warn("deleting session", "err", err)
		}
	}
	http.SetCookie(w, a.cookie(a.sessionCookie(), "", -1))
	w.WriteHeader(http.StatusNoContent)
}
