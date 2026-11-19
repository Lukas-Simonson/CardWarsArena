-- Shared, anonymous totals for the whole site (games played, Floops, ...): a name and a number, nothing else.
-- No player, room or game is recorded here. Writes add to the number, so they never overwrite one another.
CREATE TABLE stats (
    name  TEXT PRIMARY KEY,
    value INTEGER NOT NULL
);
