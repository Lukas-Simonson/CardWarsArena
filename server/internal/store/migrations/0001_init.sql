-- A player is identified by who vouches for them (provider) and that provider's
-- own id for them (subject). Nothing else about them is kept: no email, no password.
CREATE TABLE users (
    id       TEXT PRIMARY KEY,
    provider TEXT NOT NULL,
    subject  TEXT NOT NULL,
    created  INTEGER NOT NULL,
    UNIQUE (provider, subject)
);

CREATE TABLE profiles (
    user_id      TEXT PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    display_name TEXT NOT NULL
);

-- Only a hash of the session token is stored, so a leaked database cannot be used to sign in.
CREATE TABLE sessions (
    token_hash BLOB PRIMARY KEY,
    user_id    TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created    INTEGER NOT NULL,
    expires    INTEGER NOT NULL
);
CREATE INDEX sessions_expires ON sessions(expires);

-- cards is a JSON array of card ids, one entry per copy, as the engine takes a deck.
CREATE TABLE decks (
    id      TEXT PRIMARY KEY,
    owner   TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name    TEXT NOT NULL,
    hero    TEXT NOT NULL,
    cards   TEXT NOT NULL,
    created INTEGER NOT NULL,
    updated INTEGER NOT NULL
);
CREATE INDEX decks_owner ON decks(owner);
