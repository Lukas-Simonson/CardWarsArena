-- The four Landscapes a player chose for a deck, as a JSON array of Landscape names in Lane order.
-- An empty array means they did not choose, and the Landscapes come from the deck's cards.
ALTER TABLE decks ADD COLUMN landscapes TEXT NOT NULL DEFAULT '[]';
