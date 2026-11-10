package store

import (
	"context"
	"errors"
	"io/fs"
	"path/filepath"
	"reflect"
	"regexp"
	"testing"
	"time"
)

var ctx = context.Background()

func open(t *testing.T) (*SQLite, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "cardwars.db")
	s, err := OpenSQLite(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	return s, path
}

func user(t *testing.T, s Store, subject string) User {
	t.Helper()
	u, err := s.CreateUser(ctx, "github", subject, "Player "+subject, "")
	if err != nil {
		t.Fatal(err)
	}
	return u
}

func TestMigrationsRunOnceAndKeepData(t *testing.T) {
	s, path := open(t)
	u := user(t, s, "1")
	s.Close()
	again, err := OpenSQLite(path)
	if err != nil {
		t.Fatal(err)
	}
	defer again.Close()
	got, err := again.UserByIdentity(ctx, "github", "1")
	if err != nil || got.ID != u.ID {
		t.Fatalf("after reopening: %+v, %v", got, err)
	}
	files, err := fs.Glob(migrations, "migrations/*.sql")
	if err != nil || len(files) == 0 {
		t.Fatalf("no migrations found: %v", err)
	}
	var n int
	if err := again.db.QueryRow(`SELECT COUNT(*) FROM schema_migrations`).Scan(&n); err != nil || n != len(files) {
		t.Errorf("%d migrations recorded (%v), want %d, one for each file", n, err, len(files))
	}
}

func TestUsers(t *testing.T) {
	s, _ := open(t)
	u := user(t, s, "42")
	if !regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`).MatchString(u.ID) {
		t.Errorf("id %q is not a version 4 UUID", u.ID)
	}
	if _, err := s.CreateUser(ctx, "github", "42", "Again", ""); !errors.Is(err, ErrExists) {
		t.Errorf("same identity twice: %v, want ErrExists", err)
	}
	if other, err := s.CreateUser(ctx, "discord", "42", "Other", ""); err != nil || other.ID == u.ID {
		t.Errorf("the same subject at another provider is another account: %+v, %v", other, err)
	}
	byID, err := s.User(ctx, u.ID)
	if err != nil || byID.DisplayName != "Player 42" || byID.Provider != "github" || byID.Subject != "42" {
		t.Errorf("User = %+v, %v", byID, err)
	}
	if _, err := s.UserByIdentity(ctx, "github", "nobody"); !errors.Is(err, ErrNotFound) {
		t.Errorf("unknown identity: %v", err)
	}
	if err := s.SetDisplayName(ctx, u.ID, "Renamed"); err != nil {
		t.Fatal(err)
	}
	if got, _ := s.User(ctx, u.ID); got.DisplayName != "Renamed" {
		t.Errorf("display name %q", got.DisplayName)
	}
	if err := s.SetDisplayName(ctx, "missing", "x"); !errors.Is(err, ErrNotFound) {
		t.Errorf("renaming nobody: %v", err)
	}
}

func TestSessionsExpireAndCanBeDeleted(t *testing.T) {
	s, _ := open(t)
	u := user(t, s, "1")
	now := time.Now()
	live, dead := []byte("live-hash"), []byte("dead-hash")
	if err := s.CreateSession(ctx, u.ID, live, now.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	if err := s.CreateSession(ctx, u.ID, dead, now.Add(-time.Minute)); err != nil {
		t.Fatal(err)
	}
	if id, err := s.Session(ctx, live, now); err != nil || id != u.ID {
		t.Errorf("live session: %q, %v", id, err)
	}
	if _, err := s.Session(ctx, dead, now); !errors.Is(err, ErrNotFound) {
		t.Errorf("expired session: %v", err)
	}
	if _, err := s.Session(ctx, []byte("unknown"), now); !errors.Is(err, ErrNotFound) {
		t.Errorf("unknown session: %v", err)
	}
	if n, err := s.DeleteExpiredSessions(ctx, now); err != nil || n != 1 {
		t.Errorf("purged %d (%v), want 1", n, err)
	}
	if err := s.DeleteSession(ctx, live); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Session(ctx, live, now); !errors.Is(err, ErrNotFound) {
		t.Errorf("deleted session still works: %v", err)
	}
}

func TestDecksBelongToTheirOwner(t *testing.T) {
	s, _ := open(t)
	a, b := user(t, s, "a"), user(t, s, "b")
	cards := []string{"pony", "pony", "husker-champion"}
	d, err := s.SaveDeck(ctx, Deck{Owner: a.ID, Name: "Ponies", Hero: "finn", Cards: cards})
	if err != nil {
		t.Fatal(err)
	}
	if d.ID == "" || d.Created.IsZero() || d.Updated.IsZero() {
		t.Fatalf("saved deck lacks id or times: %+v", d)
	}
	got, err := s.Deck(ctx, a.ID, d.ID)
	if err != nil || got.Name != "Ponies" || got.Hero != "finn" || !reflect.DeepEqual(got.Cards, cards) {
		t.Errorf("Deck = %+v, %v", got, err)
	}
	if _, err := s.Deck(ctx, b.ID, d.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("another player read the deck: %v", err)
	}
	if _, err := s.SaveDeck(ctx, Deck{ID: d.ID, Owner: b.ID, Name: "Stolen", Hero: "jake", Cards: cards}); !errors.Is(err, ErrNotFound) {
		t.Errorf("another player overwrote the deck: %v", err)
	}
	if err := s.DeleteDeck(ctx, b.ID, d.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("another player deleted the deck: %v", err)
	}

	if got.Landscapes == nil || len(got.Landscapes) != 0 {
		t.Errorf("a deck with no chosen Landscapes has %#v, want an empty list", got.Landscapes)
	}
	chosen, err := s.SaveDeck(ctx, Deck{Owner: a.ID, Name: "Chosen", Hero: "finn", Cards: cards, Landscapes: []string{"icy_lands", "icy_lands", "cornfield", "cornfield"}})
	if err != nil {
		t.Fatal(err)
	}
	if back, err := s.Deck(ctx, a.ID, chosen.ID); err != nil || !reflect.DeepEqual(back.Landscapes, chosen.Landscapes) {
		t.Errorf("chosen Landscapes came back as %v (%v)", back.Landscapes, err)
	}
	if err := s.DeleteDeck(ctx, a.ID, chosen.ID); err != nil {
		t.Fatal(err)
	}

	d.Name, d.Cards = "Fewer ponies", []string{"pony"}
	updated, err := s.SaveDeck(ctx, d)
	if err != nil || updated.ID != d.ID || updated.Name != "Fewer ponies" || !reflect.DeepEqual(updated.Cards, []string{"pony"}) || updated.Updated.Before(updated.Created) {
		t.Errorf("update = %+v, %v", updated, err)
	}
	if _, err := s.SaveDeck(ctx, Deck{Owner: a.ID, Name: "Second", Hero: "jake", Cards: []string{}}); err != nil {
		t.Fatal(err)
	}
	if list, err := s.Decks(ctx, a.ID); err != nil || len(list) != 2 {
		t.Errorf("Decks = %d (%v), want 2", len(list), err)
	}
	if list, err := s.Decks(ctx, b.ID); err != nil || len(list) != 0 {
		t.Errorf("the other player's decks = %d (%v), want 0", len(list), err)
	}
	if err := s.DeleteDeck(ctx, a.ID, d.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Deck(ctx, a.ID, d.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("deleted deck: %v", err)
	}
}

func TestADeckNeedsARealOwner(t *testing.T) {
	s, _ := open(t)
	if _, err := s.SaveDeck(ctx, Deck{Owner: "nobody", Name: "x", Hero: "finn"}); err == nil {
		t.Error("a deck was saved for an unknown owner")
	}
}

func TestAnAvatarIsKeptAndCanBeRefreshed(t *testing.T) {
	s, _ := open(t)
	u, err := s.CreateUser(ctx, "github", "7", "Ada", "https://avatars.githubusercontent.com/u/7?v=4")
	if err != nil || u.AvatarURL != "https://avatars.githubusercontent.com/u/7?v=4" {
		t.Fatalf("CreateUser = %+v, %v", u, err)
	}
	if got, _ := s.UserByIdentity(ctx, "github", "7"); got.AvatarURL != u.AvatarURL {
		t.Errorf("read back %q", got.AvatarURL)
	}
	if err := s.SetAvatar(ctx, u.ID, "https://avatars.githubusercontent.com/u/7?v=5"); err != nil {
		t.Fatal(err)
	}
	if got, _ := s.User(ctx, u.ID); got.AvatarURL != "https://avatars.githubusercontent.com/u/7?v=5" {
		t.Errorf("after refresh %q", got.AvatarURL)
	}
	if err := s.SetAvatar(ctx, u.ID, ""); err != nil {
		t.Fatal(err)
	}
	if got, _ := s.User(ctx, u.ID); got.AvatarURL != "" {
		t.Errorf("a person who removed their avatar still has %q", got.AvatarURL)
	}
	if err := s.SetAvatar(ctx, "missing", "x"); !errors.Is(err, ErrNotFound) {
		t.Errorf("setting the avatar of nobody: %v", err)
	}
}

func TestMigrationTwoKeepsExistingAccounts(t *testing.T) {
	path := filepath.Join(t.TempDir(), "old.db")
	s, err := OpenSQLite(path)
	if err != nil {
		t.Fatal(err)
	}
	u := user(t, s, "1")
	// Go back to a database as version 1 left it: no avatar column, and the second migration not recorded.
	if _, err := s.db.Exec(`ALTER TABLE profiles DROP COLUMN avatar_url`); err != nil {
		t.Fatal(err)
	}
	if _, err := s.db.Exec(`DELETE FROM schema_migrations WHERE version = 2`); err != nil {
		t.Fatal(err)
	}
	s.Close()
	again, err := OpenSQLite(path)
	if err != nil {
		t.Fatal(err)
	}
	defer again.Close()
	got, err := again.User(ctx, u.ID)
	if err != nil || got.DisplayName != u.DisplayName || got.AvatarURL != "" {
		t.Errorf("after migrating: %+v, %v", got, err)
	}
}
