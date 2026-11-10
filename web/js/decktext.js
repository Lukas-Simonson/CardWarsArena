// A deck as plain text, to copy, save, post in a chat or type by hand. Pure: no DOM, no network.
//
//   Name: Corn Farmers
//   Hero: Finn
//
//   3 Corn Dog
//   2 Husker Worm
//
// A line with a count and a name is a card. `Name:` and `Hero:` lines set the deck's name and Hero. Blank lines
// and lines starting with # or // are ignored. Cards are found by name, ignoring case, spacing and punctuation,
// so a deck copied from somewhere else still reads. A line that cannot be used is reported, never dropped quietly.
//
// The same reading also covers the text Card Dweeb gives for a deck (see parseDweeb).
import { MAX_COPIES, MAX_CARDS, emptyDeck, total, heroes } from "./deckmodel.js";

/** The deck as text: its name and Hero (when it has them), a blank line, then `count name` for each card. */
export function formatDeck(deck, cards) {
  const head = [];
  if (deck.name?.trim()) head.push(`Name: ${deck.name.trim()}`);
  const hero = deck.hero && cards[deck.hero];
  if (hero) head.push(`Hero: ${hero.name}`);
  const lines = Object.entries(deck.counts)
    .map(([id, n]) => [cards[id]?.name ?? id, n])
    .sort((a, b) => a[0].localeCompare(b[0]))
    .map(([name, n]) => `${n} ${name}`);
  return [...head, ...(head.length && lines.length ? [""] : []), ...lines].join("\n") + (head.length || lines.length ? "\n" : "");
}

/** A name with case, accents, spacing and punctuation taken out, so "Corn  Dog!" and "corn dog" match. */
export const nameKey = (s) => s.normalize("NFKD").replace(/[^\p{L}\p{N}]/gu, "").toLowerCase();

function distance(a, b) {
  if (Math.abs(a.length - b.length) > 2) return 3;
  let prev = Array.from({ length: b.length + 1 }, (_, j) => j);
  for (let i = 1; i <= a.length; i++) {
    const row = [i];
    for (let j = 1; j <= b.length; j++) row[j] = Math.min(prev[j] + 1, row[j - 1] + 1, prev[j - 1] + (a[i - 1] === b[j - 1] ? 0 : 1));
    prev = row;
  }
  return prev[b.length];
}

const index = (list) => {
  const m = new Map();
  for (const c of list) m.set(nameKey(c.name), [...(m.get(nameKey(c.name)) ?? []), c]);
  return m;
};

/** The closest name to what was typed, if one is a letter or two away. */
function nearest(key, list) {
  let best = null, bd = 3;
  for (const c of list) {
    const d = distance(key, nameKey(c.name));
    if (d < bd) { best = c; bd = d; }
  }
  return best;
}

const lines = (text) => String(text ?? "").replace(/^﻿/, "").split(/\r\n|\r|\n/);

/**
 * What a reader needs to turn names into a deck: finds cards and Heroes in the catalog and keeps the result
 * and its issues. `out` is { name, hero, counts, issues, any }: `issues` lists { line, text, reason, level }
 * for what could not be used ("error") or was changed ("warning"); `any` says whether anything was read.
 */
function reader(cards, { heroWord = "Hero:" } = {}) {
  const playable = Object.values(cards).filter((c) => c.type !== "hero" && c.modeled);
  const byName = index(playable), byAny = index(Object.values(cards));
  const heroList = heroes(cards), heroByName = index(heroList);
  const out = { name: "", hero: "", counts: {}, issues: [], any: false };
  const issue = (line, raw, reason, level = "error") => out.issues.push({ line, text: raw.trim(), reason, level });
  const named = (list, key) => list.get(key) ?? [];

  return {
    out, issue,

    setName(value) { out.name = value.slice(0, 40); out.any = true; },

    setHero(line, raw, value) {
      const found = named(heroByName, nameKey(value));
      if (found.length === 1) { out.hero = found[0].id; out.any = true; return; }
      const other = named(byAny, nameKey(value));
      if (other.length) issue(line, raw, `${other[0].name} is not a Hero the game can play`);
      else {
        const near = nearest(nameKey(value), heroList);
        issue(line, raw, `no Hero is called “${value}”` + (near ? `. Did you mean ${near.name}?` : ""));
      }
    },

    addCard(line, raw, count, name) {
      const key = nameKey(name);
      if (!key) { issue(line, raw, "no card name"); return; }
      if (count === 0) { issue(line, raw, "0 copies, so nothing was added", "warning"); return; }

      const found = named(byName, key);
      if (found.length > 1) { issue(line, raw, `more than one card is called “${name}”: ${found.map((c) => c.id).join(", ")}`); return; }
      if (found.length === 0) {
        const other = named(byAny, key)[0];
        if (other?.type === "hero") issue(line, raw, `${other.name} is a Hero: put it on a “${heroWord}” line, not in the cards`);
        else if (other) issue(line, raw, `${other.name}'s rules are not implemented yet`);
        else {
          const near = nearest(key, playable);
          issue(line, raw, `no card is called “${name}”` + (near ? `. Did you mean ${near.name}?` : ""));
        }
        return;
      }

      const c = found[0];
      out.any = true;
      const have = out.counts[c.id] ?? 0;
      let n = Math.min(count + have, MAX_COPIES);
      if (count + have > MAX_COPIES) issue(line, raw, `${c.name} is limited to ${MAX_COPIES} copies`, "warning");
      if (total({ counts: out.counts }) - have + n > MAX_CARDS) {
        n = Math.max(0, MAX_CARDS - (total({ counts: out.counts }) - have));
        issue(line, raw, `a deck holds at most ${MAX_CARDS} cards`, "warning");
      }
      if (n > 0) out.counts[c.id] = n;
    },
  };
}

/** Reads deck text (the format above) against the catalog. Returns { name, hero, counts, issues, any }. */
export function parseDeck(text, cards) {
  const r = reader(cards);
  lines(text).forEach((raw, i) => {
    const line = i + 1, s = raw.trim();
    if (!s || s.startsWith("#") || s.startsWith("//")) return;

    const head = /^(name|deck|hero)\s*:\s*(.*)$/i.exec(s);
    if (head) {
      const value = head[2].trim();
      if (head[1].toLowerCase() !== "hero") r.setName(value);
      else if (value) r.setHero(line, raw, value);
      return;
    }

    const m = /^(?:(\d+)\s*[x×]?\s+|(\d+)[x×]\s*)?(.+)$/i.exec(s);
    const count = m[1] !== undefined ? Number(m[1]) : m[2] !== undefined ? Number(m[2]) : 1;
    r.addCard(line, raw, count, m[3].trim());
  });
  return r.out;
}

// ---------- Card Dweeb

const HEADING = /^(hero|landscapes?|creatures?|spells?|buildings?)\s*:?\s*(\(\d+\))?$/i;

/**
 * Reads the text Card Dweeb gives for a deck: headings (Hero, Landscapes, Creatures, Spells, Buildings), each
 * followed by its lines. The Hero section is one name; the others are `2 - Corn Dog`. The Landscapes section is
 * skipped: a deck's Landscapes come from its cards, so they are not cards to add. Returns what parseDeck does.
 */
export function parseDweeb(text, cards) {
  const r = reader(cards, { heroWord: "Hero" });
  let section = "";
  let heroSeen = false;
  lines(text).forEach((raw, i) => {
    const line = i + 1, s = raw.trim();
    if (!s) return;

    const heading = HEADING.exec(s);
    if (heading) { section = heading[1].toLowerCase().replace(/s$/, ""); heroSeen = false; return; }
    if (section === "landscape") return;
    if (section === "hero") {
      if (heroSeen) r.issue(line, raw, "a deck has one Hero; this line was ignored", "warning");
      else { heroSeen = true; r.setHero(line, raw, s); }
      return;
    }

    // "2 - Big Foot" and "2 Big Foot" are both a count and a name; with no count it is one copy.
    const m = /^(\d+)\s*(?:[-–—:]|[x×](?=\s))?\s*(.+)$/i.exec(s);
    r.addCard(line, raw, m ? Number(m[1]) : 1, (m ? m[2] : s).trim());
  });
  return r.out;
}

/** Does this text look like it came from Card Dweeb? It has headings for the kinds of card. */
export const looksLikeDweeb = (text) => lines(text).some((l) => /^(landscapes?|creatures?|spells?|buildings?)\s*:?\s*(\(\d+\))?$/i.test(l.trim()));

export const FORMATS = [["auto", "Detect it"], ["list", "Deck list"], ["dweeb", "Card Dweeb"]];

/**
 * Deck text read into a deck the builder can show: a draft with no id, ready to be saved or changed.
 * `format` is "list", "dweeb" or "auto" (the default: Card Dweeb when it has its headings, otherwise a list).
 */
export const importDeck = (text, cards, format = "auto") => {
  const dweeb = format === "dweeb" || (format === "auto" && looksLikeDweeb(text));
  const r = dweeb ? parseDweeb(text, cards) : parseDeck(text, cards);
  return { deck: { ...emptyDeck(), name: r.name, hero: r.hero, counts: r.counts }, issues: r.issues, any: r.any, format: dweeb ? "dweeb" : "list" };
};

/** A file name for a deck: "Corn Farmers" becomes "corn-farmers.txt". */
export const fileName = (deck) => (deck.name?.trim().normalize("NFKD").replace(/[^\p{L}\p{N}]+/gu, "-").replace(/^-+|-+$/g, "").toLowerCase() || "deck") + ".txt";
