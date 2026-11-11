// A deck being built, and the rules it is judged by. Pure: no DOM, no network.
//
// The server decides what may be saved and what may be played; these functions
// only give the builder live feedback, using the same rules and the same words.
// A deck is { id, name, hero, counts, landscapes } where counts maps a card id to its copies and landscapes is
// the four the player chose, one for each Lane, or null to let the cards decide.
import { landscapeName } from "./describe.js";

export const MIN_CARDS = 40;
export const MAX_COPIES = 3;
export const MAX_CARDS = 120;
export const LANES = 4;
export const LANDSCAPES = ["cornfield", "blue_plains", "useless_swamp", "nice_lands", "sandy_lands", "icy_lands"];

export const emptyDeck = () => ({ id: null, name: "", hero: "", counts: {}, landscapes: null });

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
 * The four Landscapes the cards suggest, as the server works them out when the player has not chosen: one type
 * fills every Lane, two take two Lanes each, three take one each with the most common also taking a fourth, and
 * four take one each. Types with more cards come first, then by name. Empty when the cards do not say (none
 * or more than four types).
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
  if (kinds.length === 3) return [kinds[0], kinds[0], kinds[1], kinds[2]];
  if (kinds.length === 4) return kinds;
  return [];
}

/** The Landscapes the deck is played with: the player's choice, or else what the cards suggest ([] if unclear). */
export const landscapesOf = (deck, cards) => deck.landscapes ?? landscapeRow(deck, cards);

/** Put a Landscape in one Lane. From an undecided deck every other Lane starts as the same type, to adjust. */
export function chooseLandscape(deck, cards, lane, landscape) {
  const row = landscapesOf(deck, cards);
  const next = row.length === LANES ? [...row] : Array(LANES).fill(landscape);
  next[lane] = landscape;
  return { ...deck, landscapes: next };
}

/** Let the cards decide the Landscapes again. */
export const autoLandscapes = (deck) => ({ ...deck, landscapes: null });

/**
 * The cards that may be hard or impossible to play with the Landscapes the deck has. A card needs as many
 * face-up Landscapes of its type as it costs (Rainbow cards and free ones need none), so a card is listed when
 * the deck has fewer such Landscapes than that. Some effects add Landscapes in play, hence "may". Each item is
 * { id, name, type, cost, have, never } where never means the deck has none of that Landscape at all.
 */
export function unplayable(deck, cards) {
  const row = landscapesOf(deck, cards);
  if (row.length !== LANES) return [];
  const out = [];
  for (const id of Object.keys(deck.counts)) {
    const c = cards[id];
    if (!c || !c.landscape || c.landscape === "rainbow" || !(c.cost > 0)) continue;
    const have = row.filter((l) => l === c.landscape).length;
    if (have < c.cost) out.push({ id, name: c.name, type: c.landscape, cost: c.cost, have, never: have === 0 });
  }
  return out.sort((a, b) => Number(b.never) - Number(a.never) || a.name.localeCompare(b.name));
}

/** Why a card in the list above may not be playable, in words. */
export const whyUnplayable = (u) => u.never
  ? `no ${landscapeName(u.type)} Landscape`
  : `costs ${u.cost} but the deck has ${u.have} ${landscapeName(u.type)} Landscape${u.have === 1 ? "" : "s"}`;

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
  if (!deck.landscapes) { // the cards decide, so they must be able to
    const types = landscapeTypes(deck, cards).length;
    if (types > LANES) out.push(`uses ${types} Landscape types: choose your ${LANES} Landscapes`);
    else if (types === 0 && n > 0) out.push("choose your Landscapes: no card says which (they are all Rainbow)");
  }
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
 * `members` narrows by a deck's cards: "in" keeps the cards that deck has, "out" those it does not, judged
 * by `counts` (card id -> copies; the deck being built). `from` is another deck's counts: only its cards stay.
 */
export function pool(cards, { text = "", type = "", landscape = "", members = "", counts = {}, from = null } = {}) {
  const q = text.trim().toLowerCase();
  return Object.values(cards)
    .filter((c) => c.type !== "hero" && c.modeled)
    .filter((c) => members !== "in" || (counts[c.id] ?? 0) > 0)
    .filter((c) => members !== "out" || !((counts[c.id] ?? 0) > 0))
    .filter((c) => !from || (from[c.id] ?? 0) > 0)
    .filter((c) => !type || c.type === type)
    .filter((c) => !landscape || c.landscape === landscape)
    .filter((c) => !q || c.name.toLowerCase().includes(q) || (c.text ?? "").toLowerCase().includes(q))
    .sort((a, b) => a.name.localeCompare(b.name));
}

/** Is there anything here that has not been saved? Compares with the deck as last saved (or an empty one). */
export function changed(deck, saved) {
  const base = saved ?? emptyDeck();
  return deck.name !== base.name || deck.hero !== base.hero || toCards(deck).join() !== toCards(base).join()
    || (deck.landscapes ?? []).join() !== (base.landscapes ?? []).join();
}
