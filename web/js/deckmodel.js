// A deck being built, and the rules it is judged by. Pure: no DOM, no network.
//
// The server decides what may be saved and what may be played; these functions
// only give the builder live feedback, using the same rules and the same words.
// A deck is { id, name, hero, counts } where counts maps a card id to its copies.

export const MIN_CARDS = 40;
export const MAX_COPIES = 3;
export const MAX_CARDS = 120;
export const MAX_LANDSCAPE_TYPES = 2;

export const emptyDeck = () => ({ id: null, name: "", hero: "", counts: {} });

/** A deck from the list of card ids the server keeps (one id per copy). */
export function fromCards(cards, extra = {}) {
  const counts = {};
  for (const id of cards) counts[id] = (counts[id] ?? 0) + 1;
  return { ...emptyDeck(), ...extra, counts };
}

/** The card ids the server takes: one per copy, in a steady order. */
export function toCards(deck) {
  return Object.keys(deck.counts).sort().flatMap((id) => Array(deck.counts[id]).fill(id));
}

export const total = (deck) => Object.values(deck.counts).reduce((n, c) => n + c, 0);
export const copies = (deck, id) => deck.counts[id] ?? 0;

/** Why one more copy of a card cannot go in, or "" when it can. */
export function whyNot(deck, id) {
  if (copies(deck, id) >= MAX_COPIES) return `Already ${MAX_COPIES} copies`;
  if (total(deck) >= MAX_CARDS) return `A deck holds at most ${MAX_CARDS} cards`;
  return "";
}

export function add(deck, id) {
  return whyNot(deck, id) ? deck : { ...deck, counts: { ...deck.counts, [id]: copies(deck, id) + 1 } };
}

export function remove(deck, id) {
  const n = copies(deck, id);
  if (n === 0) return deck;
  const counts = { ...deck.counts };
  if (n === 1) delete counts[id];
  else counts[id] = n - 1;
  return { ...deck, counts };
}

/** The Landscape types the deck's cards need. Rainbow cards fit any, so they add none. */
export function landscapeTypes(deck, cards) {
  const seen = new Set();
  for (const id of Object.keys(deck.counts)) {
    const ls = cards[id]?.landscape;
    if (ls && ls !== "rainbow") seen.add(ls);
  }
  return [...seen].sort();
}

/**
 * The four Landscapes the deck is played with, as the server works them out: one type fills every Lane,
 * two take two Lanes each (the type with more cards first). Empty when the cards do not say (none or
 * more than two types).
 */
export function landscapeRow(deck, cards) {
  const count = {};
  for (const [id, n] of Object.entries(deck.counts)) {
    const ls = cards[id]?.landscape;
    if (ls && ls !== "rainbow") count[ls] = (count[ls] ?? 0) + n;
  }
  const kinds = Object.keys(count).sort((a, b) => count[b] - count[a] || a.localeCompare(b));
  if (kinds.length === 1) return [kinds[0], kinds[0], kinds[0], kinds[0]];
  if (kinds.length === 2) return [kinds[0], kinds[0], kinds[1], kinds[1]];
  return [];
}

/** How many cards of each cost the deck holds, from cost 0 up to the dearest card (at least up to 2). */
export function costCurve(deck, cards) {
  const byCost = {};
  let top = 2;
  for (const [id, n] of Object.entries(deck.counts)) {
    const cost = cards[id]?.cost ?? 0;
    byCost[cost] = (byCost[cost] ?? 0) + n;
    top = Math.max(top, cost);
  }
  return Array.from({ length: top + 1 }, (_, cost) => ({ cost, count: byCost[cost] ?? 0 }));
}

/** What stops the deck being played, in the server's words ([] when nothing does). */
export function problems(deck, cards) {
  const out = [];
  const n = total(deck);
  if (!deck.hero) out.push("choose a Hero");
  if (n < MIN_CARDS) out.push(`${n} cards: a deck needs at least ${MIN_CARDS}`);
  const types = landscapeTypes(deck, cards).length;
  if (types > MAX_LANDSCAPE_TYPES) out.push(`uses ${types} Landscape types: a deck can use at most ${MAX_LANDSCAPE_TYPES}`);
  else if (types === 0 && n > 0) out.push("needs at least one Landscape type (some card that is not Rainbow)");
  return out;
}

/** How many creatures, spells and buildings the deck holds. */
export function mix(deck, cards) {
  const out = { creature: 0, spell: 0, building: 0 };
  for (const [id, n] of Object.entries(deck.counts)) {
    const t = cards[id]?.type;
    if (t in out) out[t] += n;
  }
  return out;
}

/** Heroes the game can play, by name. */
export const heroes = (cards) => Object.values(cards).filter((c) => c.type === "hero" && c.modeled).sort((a, b) => a.name.localeCompare(b.name));

/**
 * The cards that may be put in a deck (not Heroes; only those whose rules the game plays), narrowed by
 * a search of the name and rules text, a type and a Landscape, in alphabetical order.
 */
export function pool(cards, { text = "", type = "", landscape = "" } = {}) {
  const q = text.trim().toLowerCase();
  return Object.values(cards)
    .filter((c) => c.type !== "hero" && c.modeled)
    .filter((c) => !type || c.type === type)
    .filter((c) => !landscape || c.landscape === landscape)
    .filter((c) => !q || c.name.toLowerCase().includes(q) || (c.text ?? "").toLowerCase().includes(q))
    .sort((a, b) => a.name.localeCompare(b.name));
}

/** Is there anything here that has not been saved? Compares with the deck as last saved (or an empty one). */
export function changed(deck, saved) {
  const base = saved ?? emptyDeck();
  return deck.name !== base.name || deck.hero !== base.hero || toCards(deck).join() !== toCards(base).join();
}
