import { test } from "node:test";
import assert from "node:assert/strict";
import {
  emptyDeck, fromCards, toCards, total, copies, whyNot, add, remove, landscapeTypes, landscapeRow, costCurve, problems, mix, heroes, pool, changed,
  buckets, bucketOf, landscapesOf, chooseLandscape, autoLandscapes, unplayable, whyUnplayable, deckState, saveRequest,
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

// Two more Landscapes, for the decks that spread across four or five of them.
const wide = { ...cards,
  san: { id: "san", name: "Sandcastle", type: "creature", landscape: "sandy_lands", modeled: true },
  nice: { id: "nice", name: "Nice Guy", type: "creature", landscape: "nice_lands", modeled: true } };

test("problems are whole sentences", () => {
  assert.deepEqual(problems(emptyDeck(), cards), ["Choose a Hero.", "0 cards: a deck needs at least 40."]);
  const five = fromCards(["pup", "ice", "sea", "san", "nice"], { hero: "finn" });
  assert.ok(problems(five, wide).includes("Uses 5 Landscape types: choose your 4 Landscapes."));
  const four = fromCards(["pup", "ice", "sea", "san"], { hero: "finn" });
  assert.ok(!problems(four, wide).some((p) => /Landscape/.test(p)));
  assert.ok(problems(fromCards(["rain"], { hero: "finn" }), cards).includes("Choose your Landscapes: no card says which (they are all Rainbow)."));
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

test("the pool can be limited to what a deck has, or lacks, and to another deck's cards", () => {
  const ids = (o) => pool(cards, o).map((c) => c.id);
  const mine = fromCards(["pup", "pup", "ice"]).counts;
  assert.deepEqual(ids({ members: "in", counts: mine }), ["pup", "ice"]);
  assert.deepEqual(ids({ members: "out", counts: mine }), ["hall", "rain", "sea"]);
  assert.deepEqual(ids({ members: "in", counts: {} }), []); // an empty deck has no cards
  assert.deepEqual(ids({ members: "", counts: mine }), ["pup", "hall", "ice", "rain", "sea"]);
  // combined with the other filters
  assert.deepEqual(ids({ members: "in", counts: mine, landscape: "cornfield" }), ["pup"]);
  assert.deepEqual(ids({ members: "out", counts: mine, landscape: "cornfield" }), ["hall"]);
  // another deck's cards, and the same narrowed to those this deck does not run yet
  const other = fromCards(["pup", "sea", "sea", "ice"]).counts;
  assert.deepEqual(ids({ from: other }), ["pup", "ice", "sea"]);
  assert.deepEqual(ids({ from: other, members: "out", counts: mine }), ["sea"]);
  assert.deepEqual(ids({ from: other, type: "spell" }), ["sea"]);
  assert.deepEqual(ids({ from: {} }), []); // a deck with no cards shows none
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

test("the four Landscapes: one type fills them all, two take two each, three and four spread out, the larger first", () => {
  assert.deepEqual(landscapeRow(fromCards(["pup", "rain"]), cards), ["cornfield", "cornfield", "cornfield", "cornfield"]);
  assert.deepEqual(landscapeRow(fromCards(["ice", "pup", "pup"]), cards), ["cornfield", "cornfield", "icy_lands", "icy_lands"]);
  assert.deepEqual(landscapeRow(fromCards(["ice", "ice", "pup"]), cards), ["icy_lands", "icy_lands", "cornfield", "cornfield"]);
  assert.deepEqual(landscapeRow(fromCards(["ice", "ice", "pup", "pup"]), cards), ["cornfield", "cornfield", "icy_lands", "icy_lands"]); // a tie goes alphabetically
  assert.deepEqual(landscapeRow(fromCards(["pup", "ice", "sea"]), cards), ["blue_plains", "blue_plains", "cornfield", "icy_lands"]); // three: the first takes two lanes
  assert.deepEqual(landscapeRow(fromCards(["pup", "pup", "ice", "sea"]), cards), ["cornfield", "cornfield", "blue_plains", "icy_lands"]);
  assert.deepEqual(landscapeRow(fromCards(["pup", "pup", "ice", "sea", "san"]), wide), ["cornfield", "blue_plains", "icy_lands", "sandy_lands"]); // four: one each
  assert.deepEqual(landscapeRow(fromCards(["pup", "ice", "sea", "san", "nice"]), wide), []); // five: cannot be told
  assert.deepEqual(landscapeRow(emptyDeck(), cards), []);
});

test("the cost curve runs from 0 to the dearest card, and at least to 2", () => {
  const c = { a: { cost: 0 }, b: { cost: 1 }, d: { cost: 4 } };
  assert.deepEqual(costCurve(emptyDeck(), c), [{ cost: 0, count: 0 }, { cost: 1, count: 0 }, { cost: 2, count: 0 }]);
  assert.deepEqual(costCurve(fromCards(["a", "b", "b", "d"]), c).map((x) => x.count), [1, 2, 0, 0, 1]);
});

// Cards with costs, to say which would be hard to play.
const priced = {
  ...wide,
  big: { id: "big", name: "Big Ice", type: "creature", landscape: "icy_lands", cost: 3, modeled: true },
  mid: { id: "mid", name: "Mid Corn", type: "creature", landscape: "cornfield", cost: 2, modeled: true },
  cheap: { id: "cheap", name: "Cheap Sand", type: "spell", landscape: "sandy_lands", cost: 1, modeled: true },
  free: { id: "free", name: "Free Sand", type: "spell", landscape: "sandy_lands", cost: 0, modeled: true },
  rb: { id: "rb", name: "Rainbow Dude", type: "creature", landscape: "rainbow", cost: 4, modeled: true },
};

test("the Landscapes are the player's choice, or else what the cards suggest", () => {
  const d = fromCards(["pup", "pup", "ice"]);
  assert.deepEqual(landscapesOf(d, cards), ["cornfield", "cornfield", "icy_lands", "icy_lands"]);
  const picked = chooseLandscape(d, cards, 3, "nice_lands");
  assert.deepEqual(picked.landscapes, ["cornfield", "cornfield", "icy_lands", "nice_lands"]);
  assert.deepEqual(landscapesOf(picked, cards), picked.landscapes);
  assert.equal(autoLandscapes(picked).landscapes, null);
  assert.deepEqual(landscapesOf(autoLandscapes(picked), cards), ["cornfield", "cornfield", "icy_lands", "icy_lands"]);
});

test("choosing from a deck the cards cannot decide starts every Lane the same, to adjust", () => {
  const none = fromCards(["rain"]);
  assert.deepEqual(landscapesOf(none, cards), []);
  assert.deepEqual(chooseLandscape(none, cards, 2, "icy_lands").landscapes, ["icy_lands", "icy_lands", "icy_lands", "icy_lands"]);
});

test("chosen Landscapes mean the cards need not decide, so there is nothing to complain about", () => {
  const five = fromCards(["pup", "ice", "sea", "san", "nice"], { hero: "finn" });
  assert.ok(problems(five, wide).some((p) => /Landscape/.test(p)));
  assert.ok(!problems(chooseLandscape(five, wide, 0, "cornfield"), wide).some((p) => /Landscape/.test(p)));
  const rainbow = fromCards(["rain"], { hero: "finn" });
  assert.ok(!problems(chooseLandscape(rainbow, cards, 0, "cornfield"), cards).some((p) => /Landscape/.test(p)));
});

test("a change of Landscapes is a change to save", () => {
  const saved = fromCards(["pup"], { id: "a", name: "A", hero: "finn" });
  assert.equal(changed(saved, saved), false);
  assert.equal(changed(chooseLandscape(saved, cards, 0, "icy_lands"), saved), true);
  assert.equal(changed(autoLandscapes(chooseLandscape(saved, cards, 0, "icy_lands")), saved), false);
});

test("cards that may be unplayable: no Landscape of their type, or fewer than their cost", () => {
  const d = { ...fromCards(["big", "mid", "cheap", "free", "rb", "pup"]), landscapes: ["cornfield", "cornfield", "icy_lands", "icy_lands"] };
  const list = unplayable(d, priced);
  assert.deepEqual(list.map((u) => [u.name, u.never]), [["Cheap Sand", true], ["Big Ice", false]]);
  assert.equal(whyUnplayable(list[0]), "no Sandy Lands Landscape");
  assert.equal(whyUnplayable(list[1]), "costs 3 but the deck has 2 Icy Lands Landscapes");
  // Enough Landscapes of the type, a free card and a Rainbow card are all fine.
  assert.deepEqual(unplayable({ ...d, landscapes: ["icy_lands", "icy_lands", "icy_lands", "sandy_lands"] }, priced).map((u) => u.name), ["Mid Corn"]);
});

test("nothing is said about unplayable cards while the Landscapes are unclear", () => {
  assert.deepEqual(unplayable(fromCards(["rain"]), priced), []);
  assert.deepEqual(unplayable(fromCards(["pup", "ice", "sea", "san", "nice"]), priced), []);
});

test("one Landscape of a type for a one-cost card is enough", () => {
  const d = { ...fromCards(["cheap"]), landscapes: ["sandy_lands", "cornfield", "cornfield", "cornfield"] };
  assert.deepEqual(unplayable(d, priced), []);
});

test("the card search finds a card by the name shown, the English name or its text", () => {
  const cs = {
    chiot: { id: "chiot", name: "Chiot Maïs", name_en: "Corn Pup", type: "creature", landscape: "cornfield", modeled: true, text: "Piochez." },
    other: { id: "other", name: "Autre", type: "spell", landscape: "cornfield", modeled: true, text: "Draw." },
  };
  assert.deepEqual(pool(cs, { text: "chiot" }).map((c) => c.id), ["chiot"]);
  assert.deepEqual(pool(cs, { text: "corn pup" }).map((c) => c.id), ["chiot"]);
  assert.deepEqual(pool(cs, { text: "draw" }).map((c) => c.id), ["other"]);
});

test("the builder says whether it holds a new deck, a saved one being edited, or an unsaved copy of a starter", () => {
  assert.equal(deckState(emptyDeck()), "new");
  assert.equal(deckState(fromCards(["pup"], { name: "Imported" })), "new");
  assert.equal(deckState(fromCards(["pup"], { name: "Finn's deck" }), "Finn's Deck"), "copy");
  assert.equal(deckState(fromCards(["pup"], { id: "a1", name: "Ponies" })), "editing");
  assert.equal(deckState(fromCards(["pup"], { id: "a1", name: "Ponies" }), "Finn's Deck"), "editing"); // once saved it is its own deck
});

test("Save replaces a saved deck, while a new deck and Save as copy make new ones", () => {
  const copyName = (n) => `${n} (copy)`;
  const saved = fromCards(["pup"], { id: "a1", name: "Ponies" });
  assert.deepEqual(saveRequest(saved, saved, false, copyName), { method: "PUT", id: "a1", name: "Ponies", copy: false });
  assert.deepEqual(saveRequest(saved, saved, true, copyName), { method: "POST", id: null, name: "Ponies (copy)", copy: true });
  // a copy given its own name keeps it
  assert.deepEqual(saveRequest({ ...saved, name: "Ponies 2" }, saved, true, copyName), { method: "POST", id: null, name: "Ponies 2", copy: true });
  // a deck not saved yet has nothing to copy: both buttons would just make it
  const fresh = fromCards(["pup"], { name: "New" });
  assert.deepEqual(saveRequest(fresh, null, false, copyName), { method: "POST", id: null, name: "New", copy: false });
  assert.deepEqual(saveRequest(fresh, null, true, copyName), { method: "POST", id: null, name: "New", copy: false });
});

// Cost, ATK and DEF filters.
const stat = {
  a: { id: "a", name: "A", type: "creature", cost: 0, attack: 1, defense: 3, modeled: true },
  b: { id: "b", name: "B", type: "creature", cost: 1, attack: 4, defense: 6, modeled: true },
  c: { id: "c", name: "C", type: "creature", cost: 3, attack: 0, defense: 12, modeled: true },
  d: { id: "d", name: "D", type: "spell", cost: 1, modeled: true },
  e: { id: "e", name: "E", type: "building", cost: 6, modeled: true },
  f: { id: "f", name: "F", type: "spell", modeled: true },
};
const ids = (o) => pool(stat, o).map((c) => c.id);
const range = (min, max = min) => ({ min, max, key: `${min}-${max}` });

test("buckets: one for each value up to the cap, then one 'or more', from what the cards have", () => {
  assert.deepEqual(buckets([0, 1, 2, 3, 6]).map((b) => b.key), ["0-0", "1-1", "2-2", "3-3", "4+"]);
  assert.equal(buckets([0, 1, 6]).at(-1).max, Infinity);
  assert.deepEqual(buckets([0, 2]).map((b) => b.key), ["0-0", "1-1", "2-2"]); // nothing dearer: no "or more"
  assert.deepEqual(buckets([0, 3, 16], { step: 3, cap: 9 }).map((b) => b.key), ["0-2", "3-5", "6-8", "9+"]);
  assert.deepEqual(buckets([]).map((b) => b.key), ["0-0"]); // an empty pool still offers 0
  assert.deepEqual(buckets([undefined, null, 2]).map((b) => b.key), ["0-0", "1-1", "2-2"]); // cards with no number are ignored
  assert.equal(bucketOf(buckets([0, 1, 6]), 5).key, "4+");
});

test("cost: one, several (their union), and none meaning any; a card with no cost needs none chosen", () => {
  assert.deepEqual(ids({ cost: [range(0)] }), ["a"]);
  assert.deepEqual(ids({ cost: [range(0), range(1)] }), ["a", "b", "d"]);
  assert.deepEqual(ids({ cost: [{ min: 4, max: Infinity, key: "4+" }] }), ["e"]);
  assert.deepEqual(ids({}), ["a", "b", "c", "d", "e", "f"]);
  assert.deepEqual(ids({ cost: [] }), ["a", "b", "c", "d", "e", "f"]);
});

test("ATK and DEF: ranges that leave Creatures only", () => {
  assert.deepEqual(ids({ atk: [range(0, 1)] }), ["a", "c"]);
  assert.deepEqual(ids({ atk: [range(4, Infinity)] }), ["b"]);
  assert.deepEqual(ids({ def: [range(9, Infinity)] }), ["c"]);
  assert.deepEqual(ids({ def: [range(0, 2), range(6, 8)] }), ["b"]);
  assert.deepEqual(ids({ atk: [range(0, 6)] }), ["a", "b", "c"]); // no Spell or Building, though they have no ATK
});

test("cost, ATK and DEF combine with each other and with the other filters", () => {
  assert.deepEqual(ids({ cost: [range(0), range(1)], atk: [range(1, 4)] }), ["a", "b"]);
  assert.deepEqual(ids({ cost: [range(1)], type: "spell" }), ["d"]);
  assert.deepEqual(ids({ cost: [range(1)], text: "b" }), ["b"]);
  assert.deepEqual(ids({ atk: [range(0, 6)], type: "spell" }), []); // asking for Creature numbers among Spells
  assert.deepEqual(ids({ cost: [range(0), range(1)], members: "in", counts: { b: 1, d: 2 } }), ["b", "d"]);
});
