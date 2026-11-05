// Package store keeps what must outlive a game: who the players are, their
// sessions and their decks. Games themselves stay in memory. The Store
// interface is all the rest of the server sees, so the database behind it can
// change.
package store

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"time"
)

var (
	// ErrNotFound means there is no such user, session or deck (or it is not the caller's).
	ErrNotFound = errors.New("store: not found")
	// ErrExists means that identity already has an account.
	ErrExists = errors.New("store: already exists")
)

// User is a player's account. Provider and Subject say who vouches for them, for
// example ("github", "583231"); ID is ours and never changes.
type User struct {
	ID          string
	Provider    string
	Subject     string
	DisplayName string
	AvatarURL   string // the person's picture at their provider; empty when they have none
	Created     time.Time
}

// Deck is a saved deck. Cards holds one card id per copy, as the engine takes a
// deck. The store does not check deck rules: that is for whoever saves it.
type Deck struct {
	ID      string
	Owner   string
	Name    string
	Hero    string
	Cards   []string
	Created time.Time
	Updated time.Time
}

// Store is the server's persistent state.
type Store interface {
	// CreateUser makes an account and its profile; ErrExists if the identity is taken.
	CreateUser(ctx context.Context, provider, subject, displayName, avatarURL string) (User, error)
	// UserByIdentity finds the account a provider's subject signs in to.
	UserByIdentity(ctx context.Context, provider, subject string) (User, error)
	User(ctx context.Context, id string) (User, error)
	SetDisplayName(ctx context.Context, id, name string) error
	SetAvatar(ctx context.Context, id, avatarURL string) error

	// CreateSession records a session by the hash of its token. Session returns
	// the user it belongs to, unless it is unknown or expired at now.
	CreateSession(ctx context.Context, userID string, tokenHash []byte, expires time.Time) error
	Session(ctx context.Context, tokenHash []byte, now time.Time) (userID string, err error)
	DeleteSession(ctx context.Context, tokenHash []byte) error
	DeleteExpiredSessions(ctx context.Context, now time.Time) (int, error)

	// SaveDeck adds a deck when d.ID is empty and otherwise replaces the owner's
	// deck with that id (ErrNotFound if it is not theirs). The result has its id and times.
	SaveDeck(ctx context.Context, d Deck) (Deck, error)
	Deck(ctx context.Context, owner, id string) (Deck, error)
	// Decks lists an owner's decks, newest change first.
	Decks(ctx context.Context, owner string) ([]Deck, error)
	DeleteDeck(ctx context.Context, owner, id string) error

	Close() error
}

// newID returns a random UUID (version 4) as text.
func newID() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", fmt.Errorf("store: random id: %w", err)
	}
	b[6] = b[6]&0x0f | 0x40
	b[8] = b[8]&0x3f | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:]), nil
}
