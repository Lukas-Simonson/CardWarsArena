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

/**
 * Reads deck text against the catalog.
 * Returns { name, hero, counts, issues, any }: `issues` is a list of { line, text, reason, level } for the
 * lines that could not be used ("error") or were changed ("warning"); `any` says whether anything was read.
 */
export function parseDeck(text, cards) {
  const playable = Object.values(cards).filter((c) => c.type !== "hero" && c.modeled);
  const byName = index(playable), byAny = index(Object.values(cards));
  const heroList = heroes(cards), heroByName = index(heroList);
  const out = { name: "", hero: "", counts: {}, issues: [], any: false };
  const issue = (line, raw, reason, level = "error") => out.issues.push({ line, text: raw.trim(), reason, level });
  const named = (list, key) => (list.get(key) ?? []);

  String(text ?? "").replace(/^﻿/, "").split(/\r\n|\r|\n/).forEach((raw, i) => {
    const line = i + 1, s = raw.trim();
    if (!s || s.startsWith("#") || s.startsWith("//")) return;

    const head = /^(name|deck|hero)\s*:\s*(.*)$/i.exec(s);
    if (head) {
      const what = head[1].toLowerCase(), value = head[2].trim();
      if (what !== "hero") { out.name = value.slice(0, 40); out.any = true; return; }
      if (!value) return;
      const found = named(heroByName, nameKey(value));
      if (found.length === 1) { out.hero = found[0].id; out.any = true; return; }
      const other = named(byAny, nameKey(value));
      if (other.length) issue(line, raw, `${other[0].name} is not a Hero the game can play`);
      else {
        const near = nearest(nameKey(value), heroList);
        issue(line, raw, `no Hero is called “${value}”` + (near ? `. Did you mean ${near.name}?` : ""));
      }
      return;
    }

    const m = /^(?:(\d+)\s*[x×]?\s+|(\d+)[x×]\s*)?(.+)$/i.exec(s);
    const count = m[1] !== undefined ? Number(m[1]) : m[2] !== undefined ? Number(m[2]) : 1;
    const name = m[3].trim(), key = nameKey(name);
    if (!key) { issue(line, raw, "no card name"); return; }
    if (count === 0) { issue(line, raw, "0 copies, so nothing was added", "warning"); return; }

    const found = named(byName, key);
    if (found.length > 1) { issue(line, raw, `more than one card is called “${name}”: ${found.map((c) => c.id).join(", ")}`); return; }
    if (found.length === 0) {
      const other = named(byAny, key)[0];
      if (other?.type === "hero") issue(line, raw, `${other.name} is a Hero: put it on a “Hero:” line, not in the cards`);
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
  });
  return out;
}

/** Deck text read into a deck the builder can show: a draft with no id, ready to be saved or changed. */
export const importDeck = (text, cards) => {
  const r = parseDeck(text, cards);
  return { deck: { ...emptyDeck(), name: r.name, hero: r.hero, counts: r.counts }, issues: r.issues, any: r.any };
};

/** A file name for a deck: "Corn Farmers" becomes "corn-farmers.txt". */
export const fileName = (deck) => (deck.name?.trim().normalize("NFKD").replace(/[^\p{L}\p{N}]+/gu, "-").replace(/^-+|-+$/g, "").toLowerCase() || "deck") + ".txt";
