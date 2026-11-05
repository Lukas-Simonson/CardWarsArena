package store

import (
	"context"
	"database/sql"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	_ "modernc.org/sqlite" // pure Go driver: the server stays a static binary
)

//go:embed migrations/*.sql
var migrations embed.FS

// SQLite is a Store in one SQLite file.
type SQLite struct{ db *sql.DB }

var _ Store = (*SQLite)(nil)

// OpenSQLite opens (creating it if needed) the database at path and brings its
// schema up to date.
func OpenSQLite(path string) (*SQLite, error) {
	q := url.Values{}
	q.Add("_pragma", "foreign_keys(1)")
	q.Add("_pragma", "journal_mode(WAL)")
	q.Add("_pragma", "busy_timeout(5000)")
	q.Add("_pragma", "temp_store(memory)") // the container's root filesystem is read-only
	db, err := sql.Open("sqlite", "file:"+path+"?"+q.Encode())
	if err != nil {
		return nil, fmt.Errorf("store: open %s: %w", path, err)
	}
	// One connection: SQLite has a single writer anyway, and this makes "database is locked" impossible.
	db.SetMaxOpenConns(1)
	s := &SQLite{db: db}
	if err := s.migrate(); err != nil {
		db.Close()
		return nil, err
	}
	return s, nil
}

func (s *SQLite) Close() error { return s.db.Close() }

// migrate applies, in order and once each, the files in migrations/. The
// version is the number the file name starts with. A migration never changes
// after it has shipped; a change is a new file.
func (s *SQLite) migrate() error {
	if _, err := s.db.Exec(`CREATE TABLE IF NOT EXISTS schema_migrations (version INTEGER PRIMARY KEY, applied INTEGER NOT NULL)`); err != nil {
		return fmt.Errorf("store: migrations table: %w", err)
	}
	names, err := fs.Glob(migrations, "migrations/*.sql")
	if err != nil {
		return err
	}
	sort.Strings(names)
	for _, name := range names {
		base := name[strings.LastIndex(name, "/")+1:]
		version, err := strconv.Atoi(strings.SplitN(base, "_", 2)[0])
		if err != nil {
			return fmt.Errorf("store: migration %s: name must start with a number", base)
		}
		var done int
		if err := s.db.QueryRow(`SELECT COUNT(*) FROM schema_migrations WHERE version = ?`, version).Scan(&done); err != nil {
			return err
		}
		if done > 0 {
			continue
		}
		body, err := migrations.ReadFile(name)
		if err != nil {
			return err
		}
		tx, err := s.db.Begin()
		if err != nil {
			return err
		}
		if _, err := tx.Exec(string(body)); err != nil {
			tx.Rollback()
			return fmt.Errorf("store: migration %s: %w", base, err)
		}
		if _, err := tx.Exec(`INSERT INTO schema_migrations (version, applied) VALUES (?, ?)`, version, time.Now().Unix()); err != nil {
			tx.Rollback()
			return err
		}
		if err := tx.Commit(); err != nil {
			return err
		}
	}
	return nil
}

func (s *SQLite) CreateUser(ctx context.Context, provider, subject, displayName, avatarURL string) (User, error) {
	id, err := newID()
	if err != nil {
		return User{}, err
	}
	now := time.Now().Truncate(time.Second)
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return User{}, err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `INSERT INTO users (id, provider, subject, created) VALUES (?, ?, ?, ?)`, id, provider, subject, now.Unix()); err != nil {
		if isUnique(err) {
			return User{}, ErrExists
		}
		return User{}, err
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO profiles (user_id, display_name, avatar_url) VALUES (?, ?, ?)`, id, displayName, avatarURL); err != nil {
		return User{}, err
	}
	if err := tx.Commit(); err != nil {
		return User{}, err
	}
	return User{ID: id, Provider: provider, Subject: subject, DisplayName: displayName, AvatarURL: avatarURL, Created: now}, nil
}

const userSelect = `SELECT u.id, u.provider, u.subject, p.display_name, p.avatar_url, u.created FROM users u JOIN profiles p ON p.user_id = u.id`

func scanUser(row *sql.Row) (User, error) {
	var u User
	var created int64
	err := row.Scan(&u.ID, &u.Provider, &u.Subject, &u.DisplayName, &u.AvatarURL, &created)
	if errors.Is(err, sql.ErrNoRows) {
		return User{}, ErrNotFound
	}
	u.Created = time.Unix(created, 0)
	return u, err
}

func (s *SQLite) UserByIdentity(ctx context.Context, provider, subject string) (User, error) {
	return scanUser(s.db.QueryRowContext(ctx, userSelect+` WHERE u.provider = ? AND u.subject = ?`, provider, subject))
}

func (s *SQLite) User(ctx context.Context, id string) (User, error) {
	return scanUser(s.db.QueryRowContext(ctx, userSelect+` WHERE u.id = ?`, id))
}

func (s *SQLite) SetDisplayName(ctx context.Context, id, name string) error {
	res, err := s.db.ExecContext(ctx, `UPDATE profiles SET display_name = ? WHERE user_id = ?`, name, id)
	return affected(res, err)
}

func (s *SQLite) SetAvatar(ctx context.Context, id, avatarURL string) error {
	res, err := s.db.ExecContext(ctx, `UPDATE profiles SET avatar_url = ? WHERE user_id = ?`, avatarURL, id)
	return affected(res, err)
}

func (s *SQLite) CreateSession(ctx context.Context, userID string, tokenHash []byte, expires time.Time) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO sessions (token_hash, user_id, created, expires) VALUES (?, ?, ?, ?)`,
		tokenHash, userID, time.Now().Unix(), expires.Unix())
	return err
}

func (s *SQLite) Session(ctx context.Context, tokenHash []byte, now time.Time) (string, error) {
	var userID string
	err := s.db.QueryRowContext(ctx, `SELECT user_id FROM sessions WHERE token_hash = ? AND expires > ?`, tokenHash, now.Unix()).Scan(&userID)
	if errors.Is(err, sql.ErrNoRows) {
		return "", ErrNotFound
	}
	return userID, err
}

func (s *SQLite) DeleteSession(ctx context.Context, tokenHash []byte) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM sessions WHERE token_hash = ?`, tokenHash)
	return err
}

func (s *SQLite) DeleteExpiredSessions(ctx context.Context, now time.Time) (int, error) {
	res, err := s.db.ExecContext(ctx, `DELETE FROM sessions WHERE expires <= ?`, now.Unix())
	if err != nil {
		return 0, err
	}
	n, err := res.RowsAffected()
	return int(n), err
}

func (s *SQLite) SaveDeck(ctx context.Context, d Deck) (Deck, error) {
	cards, err := json.Marshal(d.Cards)
	if err != nil {
		return Deck{}, err
	}
	now := time.Now().Truncate(time.Second)
	if d.ID == "" {
		if d.ID, err = newID(); err != nil {
			return Deck{}, err
		}
		d.Created, d.Updated = now, now
		_, err = s.db.ExecContext(ctx, `INSERT INTO decks (id, owner, name, hero, cards, created, updated) VALUES (?, ?, ?, ?, ?, ?, ?)`,
			d.ID, d.Owner, d.Name, d.Hero, string(cards), now.Unix(), now.Unix())
		return d, err
	}
	res, err := s.db.ExecContext(ctx, `UPDATE decks SET name = ?, hero = ?, cards = ?, updated = ? WHERE id = ? AND owner = ?`,
		d.Name, d.Hero, string(cards), now.Unix(), d.ID, d.Owner)
	if err := affected(res, err); err != nil {
		return Deck{}, err
	}
	return s.Deck(ctx, d.Owner, d.ID)
}

const deckSelect = `SELECT id, owner, name, hero, cards, created, updated FROM decks`

type scanner interface{ Scan(...any) error }

func scanDeck(row scanner) (Deck, error) {
	var d Deck
	var cards string
	var created, updated int64
	if err := row.Scan(&d.ID, &d.Owner, &d.Name, &d.Hero, &cards, &created, &updated); err != nil {
		return Deck{}, err
	}
	if err := json.Unmarshal([]byte(cards), &d.Cards); err != nil {
		return Deck{}, fmt.Errorf("store: deck %s: %w", d.ID, err)
	}
	d.Created, d.Updated = time.Unix(created, 0), time.Unix(updated, 0)
	return d, nil
}

func (s *SQLite) Deck(ctx context.Context, owner, id string) (Deck, error) {
	d, err := scanDeck(s.db.QueryRowContext(ctx, deckSelect+` WHERE id = ? AND owner = ?`, id, owner))
	if errors.Is(err, sql.ErrNoRows) {
		return Deck{}, ErrNotFound
	}
	return d, err
}

func (s *SQLite) Decks(ctx context.Context, owner string) ([]Deck, error) {
	rows, err := s.db.QueryContext(ctx, deckSelect+` WHERE owner = ? ORDER BY updated DESC, id`, owner)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Deck
	for rows.Next() {
		d, err := scanDeck(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

func (s *SQLite) DeleteDeck(ctx context.Context, owner, id string) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM decks WHERE id = ? AND owner = ?`, id, owner)
	return affected(res, err)
}

// affected turns "changed no rows" into ErrNotFound.
func affected(res sql.Result, err error) error {
	if err != nil {
		return err
	}
	if n, err := res.RowsAffected(); err != nil {
		return err
	} else if n == 0 {
		return ErrNotFound
	}
	return nil
}

func isUnique(err error) bool {
	return err != nil && strings.Contains(err.Error(), "UNIQUE constraint failed")
}
