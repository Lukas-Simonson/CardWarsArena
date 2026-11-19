// The line of shared totals on the home screen, from GET /api/stats. Pure: no DOM, no network. The numbers are for the
// whole site; nothing here is about a player.

/** The numbers to show, in order: [{ key, n }]. Empty when the server has none to give (stats off, or nothing played yet). */
export function summary(answer) {
  const c = answer?.counts;
  if (!c || typeof c !== "object") return [];
  const sum = (prefix) => Object.entries(c).filter(([k]) => k.startsWith(prefix)).reduce((a, [, v]) => a + (Number(v) || 0), 0);
  const all = [
    { key: "games", n: Number(c.games_completed) || 0 },
    { key: "cards", n: sum("cards_played.") },
    { key: "floops", n: Number(c.floops) || 0 },
    { key: "bot_wins", n: Number(c.bot_wins) || 0 },
  ];
  return all.some((s) => s.n > 0) ? all : [];
}

/** A number for a narrow place: full up to 9,999, then "12K", "3.4M" in the locale's own way. */
export function short(n, tag) {
  return new Intl.NumberFormat(tag, n >= 10000 ? { notation: "compact", maximumFractionDigits: 1 } : {}).format(n);
}
