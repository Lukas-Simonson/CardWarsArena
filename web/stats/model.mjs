// What the /stats page shows, worked out from GET /api/stats (shape: server/docs/protocol.md, "Statistics"). Pure, so
// it is tested without a page (test/stats.test.mjs).

/** a / b as a whole percent, or null when there is nothing to divide by. */
export const pct = (a, b) => (b > 0 ? Math.round((a / b) * 100) : null);

/** Entries of a count map as [key, n], most first, ties by key. */
export const ranked = (map = {}) => Object.entries(map).sort((a, b) => b[1] - a[1] || a[0].localeCompare(b[0]));

/** The cards played, most first, named from the catalogue (`names`: id -> name) or by id when it is not there. */
export function topCards(played, names = {}, limit = Infinity) {
  return ranked(played).slice(0, limit).map(([id, n]) => ({ id, name: names[id] ?? id, n }));
}

/** The decks by how often they were played, each with its win rate. */
export function deckRows(decks = {}) {
  return Object.entries(decks)
    .map(([hero, d]) => ({ hero, started: d.started, won: d.won, rate: pct(d.won, d.started) }))
    .sort((a, b) => b.started - a.started || a.hero.localeCompare(b.hero));
}

/** The headline numbers and the splits, ready to draw. Works for a server that has counted nothing. */
export function summarize(r) {
  const started = r.games.started.total, completed = r.games.completed.total;
  const vsBot = r.results.player_wins + r.results.bot_wins.total;
  return {
    started, completed, abandoned: r.games.abandoned,
    completion: pct(completed, started),
    cardsPlayed: r.plays.cards.total, damage: r.combat.damage_dealt, destroyed: r.combat.creatures_destroyed,
    kinds: { started: r.games.started, completed: r.games.completed },
    vsBot: { player: r.results.player_wins, bot: r.results.bot_wins.total, playerRate: pct(r.results.player_wins, vsBot), games: vsBot },
    byBot: ranked(r.results.bot_wins.by_bot),
    byType: ranked(r.plays.cards.by_type),
    actions: [["floops", r.plays.floops], ["fights", r.plays.fights], ["hero_abilities", r.plays.hero_abilities]],
    decks: deckRows(r.decks),
  };
}
