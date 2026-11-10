import test from "node:test";
import assert from "node:assert/strict";
import { formatDeck, parseDeck, importDeck, nameKey, fileName } from "../js/decktext.js";
import { emptyDeck, fromCards, total } from "../js/deckmodel.js";

const cards = {
  pup: { id: "pup", name: "Corn Pup", type: "creature", landscape: "cornfield", modeled: true },
  dog: { id: "dog", name: "Corn Dog", type: "creature", landscape: "cornfield", modeled: true },
  bug: { id: "bug", name: "Bug's Life", type: "creature", landscape: "icy_lands", modeled: true },
  ice: { id: "ice", name: "Icicle", type: "spell", landscape: "icy_lands", modeled: true },
  wip: { id: "wip", name: "Unfinished", type: "spell", landscape: "cornfield", modeled: false },
  finn: { id: "finn", name: "Finn the Human", type: "hero", modeled: true },
  jake: { id: "jake", name: "Jake the Dog", type: "hero", modeled: true },
  twin1: { id: "twin1", name: "Twin", type: "creature", landscape: "cornfield", modeled: true },
  twin2: { id: "twin2", name: "TWIN", type: "creature", landscape: "icy_lands", modeled: true },
};

test("a deck is written as its name, its Hero, then a count and a name for each card", () => {
  const d = fromCards(["pup", "dog", "dog", "ice"], { name: "Farm", hero: "finn" });
  assert.equal(formatDeck(d, cards), "Name: Farm\nHero: Finn the Human\n\n2 Corn Dog\n1 Corn Pup\n1 Icicle\n");
});

test("a deck with no name or Hero is just its cards, and an empty deck is empty text", () => {
  assert.equal(formatDeck(fromCards(["pup"]), cards), "1 Corn Pup\n");
  assert.equal(formatDeck(emptyDeck(), cards), "");
  assert.equal(formatDeck({ ...emptyDeck(), name: "Only a name" }, cards), "Name: Only a name\n");
});

test("what is written can be read back", () => {
  const d = fromCards(["pup", "dog", "dog", "ice", "ice", "ice"], { name: "Farm", hero: "jake" });
  const r = parseDeck(formatDeck(d, cards), cards);
  assert.deepEqual(r.issues, []);
  assert.equal(r.name, "Farm");
  assert.equal(r.hero, "jake");
  assert.deepEqual(r.counts, d.counts);
});

test("names are matched ignoring case, spacing, punctuation and accents", () => {
  assert.equal(nameKey("  Corn   DOG! "), "corndog");
  assert.equal(nameKey("Bug’s Life"), nameKey("bugs life"));
  const r = parseDeck("2 corn   dog\n1 BUGS LIFE\n1 Ícicle", cards);
  assert.deepEqual(r.issues, []);
  assert.deepEqual(r.counts, { dog: 2, bug: 1, ice: 1 });
});

test("a count may be written 3, 2x or 2×, or left out (one copy)", () => {
  const r = parseDeck("3 Corn Dog\n2x Corn Pup\n2× Icicle\nBug's Life", cards);
  assert.deepEqual(r.issues, []);
  assert.deepEqual(r.counts, { dog: 3, pup: 2, ice: 2, bug: 1 });
});

test("comments, blank lines, Windows line endings and a byte order mark are fine", () => {
  const r = parseDeck("﻿# my deck\r\n// notes\r\n\r\nName: A\r\n2 Corn Dog\r\n", cards);
  assert.deepEqual(r.issues, []);
  assert.equal(r.name, "A");
  assert.deepEqual(r.counts, { dog: 2 });
});

test("a line that cannot be used is reported with its line number, and the rest still loads", () => {
  const r = parseDeck("Hero: Finn the Human\n2 Corn Dog\n3 Corn Dogg\n1 Nonsense Card", cards);
  assert.equal(r.hero, "finn");
  assert.deepEqual(r.counts, { dog: 2 });
  assert.equal(r.issues.length, 2);
  assert.equal(r.issues[0].line, 3);
  assert.match(r.issues[0].reason, /no card is called “Corn Dogg”\. Did you mean Corn Dog\?/);
  assert.equal(r.issues[1].line, 4);
  assert.doesNotMatch(r.issues[1].reason, /Did you mean/);
});

test("copies past the limit are cut to it, and said", () => {
  const r = parseDeck("5 Corn Dog", cards);
  assert.deepEqual(r.counts, { dog: 3 });
  assert.deepEqual(r.issues.map((i) => [i.line, i.level]), [[1, "warning"]]);
  const split = parseDeck("2 Corn Dog\n2 Corn Dog", cards);
  assert.deepEqual(split.counts, { dog: 3 });
  assert.equal(split.issues.length, 1);
});

test("a Hero belongs on the Hero line, a card the game cannot play is named as such", () => {
  const r = parseDeck("1 Finn the Human\n1 Unfinished\nHero: Corn Dog\nHero: Jak the Dog", cards);
  assert.deepEqual(r.counts, {});
  assert.equal(r.hero, "");
  assert.match(r.issues[0].reason, /Finn the Human is a Hero/);
  assert.match(r.issues[1].reason, /Unfinished's rules are not implemented/);
  assert.match(r.issues[2].reason, /Corn Dog is not a Hero/);
  assert.match(r.issues[3].reason, /Did you mean Jake the Dog\?/);
});

test("two cards with one name are refused, not guessed", () => {
  const r = parseDeck("1 Twin", cards);
  assert.deepEqual(r.counts, {});
  assert.match(r.issues[0].reason, /more than one card is called “Twin”: twin1, twin2/);
});

test("zero copies change nothing", () => {
  const r = parseDeck("0 Corn Dog", cards);
  assert.deepEqual(r.counts, {});
  assert.equal(r.issues[0].level, "warning");
});

test("a deck never holds more than the most cards the server allows", () => {
  const many = {};
  for (let i = 0; i < 50; i++) many[`c${i}`] = { id: `c${i}`, name: `Card ${String.fromCharCode(65 + (i % 26))}${"x".repeat(Math.floor(i / 26))}${i}`, type: "creature", landscape: "cornfield", modeled: true };
  const text = Object.values(many).map((c) => `3 ${c.name}`).join("\n");
  const r = parseDeck(text, many);
  assert.equal(total({ counts: r.counts }), 120);
  assert.ok(r.issues.some((i) => /at most 120/.test(i.reason)));
});

test("importing gives an unsaved draft: no id, the text's name and Hero", () => {
  const { deck, issues, any } = importDeck("Name: Imported\nHero: Jake the Dog\n3 Corn Dog", cards);
  assert.deepEqual(deck, { id: null, name: "Imported", hero: "jake", counts: { dog: 3 } });
  assert.deepEqual(issues, []);
  assert.equal(any, true);
});

test("text with nothing usable reads as nothing", () => {
  assert.equal(importDeck("", cards).any, false);
  assert.equal(importDeck("hello there\nlorem ipsum", cards).any, false);
  assert.equal(importDeck("hello there", cards).issues.length, 1);
});

test("a deck's file name comes from its name", () => {
  assert.equal(fileName({ name: "Corn Farmers!" }), "corn-farmers.txt");
  assert.equal(fileName({ name: "  " }), "deck.txt");
  assert.equal(fileName({}), "deck.txt");
});
