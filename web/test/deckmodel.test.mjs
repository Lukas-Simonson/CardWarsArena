import { test } from "node:test";
import assert from "node:assert/strict";
import {
  emptyDeck, fromCards, toCards, total, copies, whyNot, add, remove, landscapeTypes, landscapeRow, costCurve, problems, mix, heroes, pool, changed,
  MAX_COPIES, MAX_CARDS,
} from "../js/deckmodel.js";

const cards = {
  pup: { id: "pup", name: "Corn Pup", type: "creature", landscape: "cornfield", text: "FLOOP >>> Draw a card.", modeled: true },
  ice: { id: "ice", name: "Icicle", type: "creature", landscape: "icy_lands", modeled: true },
  sea: { id: "sea", name: "Seaweed", type: "spell", landscape: "blue_plains", modeled: true },
  rain: { id: "rain", name: "Rainbow Cat", type: "creature", landscape: "rainbow", modeled: true },
  hall: { id: "hall", name: "Great Hall", type: "building", landscape: "cornfield", modeled: true },
  wip: { id: "wip", name: "Unfinished", type: "spell", landscape: "cornfield", modeled: false },
  finn: { id: "finn", name: "Finn", type: "hero", modeled: true },
  jake: { id: "jake", name: "Jake", type: "hero", modeled: true },
  ghost: { id: "ghost", name: "Ghost Hero", type: "hero", modeled: false },
};

test("a deck round-trips to the server's list of ids, one per copy", () => {
  const d = fromCards(["pup", "ice", "pup"], { name: "x", hero: "finn", id: "d1" });
  assert.deepEqual(d.counts, { pup: 2, ice: 1 });
  assert.deepEqual([d.name, d.hero, d.id], ["x", "finn", "d1"]);
  assert.deepEqual(toCards(d), ["ice", "pup", "pup"]);
  assert.equal(total(d), 3);
});

test("copies go in up to three, and come out", () => {
  let d = emptyDeck();
  for (let i = 0; i < 5; i++) d = add(d, "pup");
  assert.equal(copies(d, "pup"), MAX_COPIES);
  assert.match(whyNot(d, "pup"), /Already 3 copies/);
  assert.equal(whyNot(d, "ice"), "");
  d = remove(d, "pup");
  assert.equal(copies(d, "pup"), 2);
  d = remove(remove(d, "pup"), "pup");
  assert.deepEqual(d.counts, {}); // a card with no copies is not listed
  assert.equal(remove(d, "pup"), d);
});

test("operations leave the deck they were given alone", () => {
  const d = fromCards(["pup"]);
  add(d, "ice");
  remove(d, "pup");
  assert.deepEqual(d.counts, { pup: 1 });
});

test("a deck holds at most 120 cards", () => {
  const counts = {};
  for (let i = 0; i < MAX_CARDS / MAX_COPIES; i++) counts[`c${i}`] = MAX_COPIES;
  const full = { ...emptyDeck(), counts };
  assert.equal(total(full), MAX_CARDS);
  assert.match(whyNot(full, "pup"), /at most 120/);
  assert.equal(add(full, "pup"), full);
});

test("Landscape types come from the cards, and Rainbow adds none", () => {
  const d = fromCards(["pup", "rain", "hall", "ice"]);
  assert.deepEqual(landscapeTypes(d, cards), ["cornfield", "icy_lands"]);
  assert.deepEqual(landscapeTypes(fromCards(["rain"]), cards), []);
  assert.deepEqual(landscapeTypes(fromCards(["nope"]), cards), []);
});

test("problems use the server's words", () => {
  assert.deepEqual(problems(emptyDeck(), cards), ["choose a Hero", "0 cards: a deck needs at least 40"]);
  const three = fromCards(["pup", "ice", "sea"], { hero: "finn" });
  assert.ok(problems(three, cards).includes("uses 3 Landscape types: a deck can use at most 2"));
  assert.ok(problems(fromCards(["rain"], { hero: "finn" }), cards).includes("needs at least one Landscape type (some card that is not Rainbow)"));
  const counts = {};
  for (let i = 0; i < 14; i++) counts[`x${i}`] = 3; // 42 cards of nothing in particular
  cards.x0 = { id: "x0", landscape: "cornfield" };
  assert.deepEqual(problems({ ...emptyDeck(), hero: "jake", counts: { ...counts, x0: 3 } }, cards).filter((p) => /cards:/.test(p)), []);
  delete cards.x0;
});

test("the mix counts creatures, spells and buildings", () => {
  assert.deepEqual(mix(fromCards(["pup", "pup", "sea", "hall", "rain"]), cards), { creature: 3, spell: 1, building: 1 });
});

test("only playable heroes can lead a deck", () => {
  assert.deepEqual(heroes(cards).map((h) => h.id), ["finn", "jake"]);
});

test("the pool has no Heroes and no cards the game cannot play, and can be narrowed", () => {
  const ids = (o) => pool(cards, o).map((c) => c.id);
  assert.deepEqual(ids(), ["pup", "hall", "ice", "rain", "sea"]); // alphabetical by name
  assert.deepEqual(ids({ type: "spell" }), ["sea"]);
  assert.deepEqual(ids({ landscape: "cornfield" }), ["pup", "hall"]);
  assert.deepEqual(ids({ text: "draw" }), ["pup"]); // rules text is searched too
  assert.deepEqual(ids({ text: "  ICICLE " }), ["ice"]);
  assert.deepEqual(ids({ text: "cat", type: "creature", landscape: "rainbow" }), ["rain"]);
  assert.deepEqual(ids({ text: "zzz" }), []);
});

test("a deck counts as changed only when something saved differs", () => {
  const saved = fromCards(["pup", "ice"], { name: "Mine", hero: "finn", id: "d1" });
  assert.equal(changed({ ...saved }, saved), false);
  assert.equal(changed(add(saved, "pup"), saved), true);
  assert.equal(changed({ ...saved, name: "Yours" }, saved), true);
  assert.equal(changed({ ...saved, hero: "jake" }, saved), true);
  assert.equal(changed(emptyDeck(), null), false);
  assert.equal(changed(add(emptyDeck(), "pup"), null), true);
});

test("the four Landscapes: one type fills them all, two take two each, the larger first", () => {
  assert.deepEqual(landscapeRow(fromCards(["pup", "rain"]), cards), ["cornfield", "cornfield", "cornfield", "cornfield"]);
  assert.deepEqual(landscapeRow(fromCards(["ice", "pup", "pup"]), cards), ["cornfield", "cornfield", "icy_lands", "icy_lands"]);
  assert.deepEqual(landscapeRow(fromCards(["ice", "ice", "pup"]), cards), ["icy_lands", "icy_lands", "cornfield", "cornfield"]);
  assert.deepEqual(landscapeRow(fromCards(["ice", "ice", "pup", "pup"]), cards), ["cornfield", "cornfield", "icy_lands", "icy_lands"]); // a tie goes alphabetically
  assert.deepEqual(landscapeRow(fromCards(["pup", "ice", "sea"]), cards), []); // three types: cannot be told
  assert.deepEqual(landscapeRow(emptyDeck(), cards), []);
});

test("the cost curve runs from 0 to the dearest card, and at least to 2", () => {
  const c = { a: { cost: 0 }, b: { cost: 1 }, d: { cost: 4 } };
  assert.deepEqual(costCurve(emptyDeck(), c), [{ cost: 0, count: 0 }, { cost: 1, count: 0 }, { cost: 2, count: 0 }]);
  assert.deepEqual(costCurve(fromCards(["a", "b", "b", "d"]), c).map((x) => x.count), [1, 2, 0, 0, 1]);
});
