import { test } from "node:test";
import assert from "node:assert/strict";
import { shelf, pickDeck, inBoxOrder, landscapeTitle, boxColour, diamondColour } from "../js/decklist.js";

const cards = {
  pup: { id: "pup", name: "Corn Pup", landscape: "cornfield" },
  ice: { id: "ice", name: "Icicle", landscape: "icy_lands" },
  finn: { id: "finn", name: "Finn", type: "hero" },
  jake: { id: "jake", name: "Jake", type: "hero" },
};
const blue = ["blue_plains", "blue_plains", "blue_plains", "blue_plains"];
const presets = [
  { set: "Finn Vs Jake", name: "Finn's Deck", owner: "Finn", hero: "finn", size: 40, landscapes: blue },
  { set: "Finn Vs Jake", name: "Jake's Deck", owner: "Jake", hero: "jake", size: 40, landscapes: ["cornfield", "cornfield", "cornfield", "cornfield"] },
];
const saved = [
  { id: "a1", name: "Ponies", hero: "finn", cards: ["pup", "pup", "ice"], playable: true, problems: [] },
  { id: "b2", name: "", hero: "", cards: ["pup"], playable: false, problems: ["choose a Hero", "1 cards: a deck needs at least 40"] },
];

test("starters are asked for by words, titled with the deck's own name and led by their Hero", () => {
  const [finn] = shelf(presets, [], cards).starters;
  assert.deepEqual(finn, { value: "Finn Vs Jake Finn's Deck", name: "Finn's Deck", sub: "Hero: Finn", lands: blue, size: 40, disabled: false, title: "Blue Plains" });
});

test("a saved deck is asked for as my:<id>, with its Landscapes worked out from its cards", () => {
  const [ponies] = shelf(presets, saved, cards).mine;
  assert.equal(ponies.value, "my:a1");
  assert.deepEqual(ponies.lands, ["cornfield", "cornfield", "icy_lands", "icy_lands"]);
  assert.deepEqual([ponies.name, ponies.sub, ponies.size, ponies.disabled, ponies.title], ["Ponies", "Custom · Hero: Finn", 3, false, "Cornfield + Icy Lands"]);
});

test("an unfinished deck is on the shelf but cannot be taken, and says why", () => {
  const wip = shelf(presets, saved, cards).mine[1];
  assert.deepEqual([wip.name, wip.sub, wip.disabled], ["Untitled deck", "Unfinished", true]);
  assert.equal(wip.title, "choose a Hero; 1 cards: a deck needs at least 40");
});

test("a starter deck says only who leads it, and has no subtitle without a Hero", () => {
  assert.equal(shelf(presets, [], cards).starters[1].sub, "Hero: Jake");
  assert.equal(shelf([{ ...presets[0], hero: "" }], [], cards).starters[0].sub, "");
});

test("a guest has only the starters", () => {
  assert.deepEqual(shelf(presets, [], cards).mine, []);
});

test("titles name each Landscape once", () => {
  assert.equal(landscapeTitle(blue), "Blue Plains");
  assert.equal(landscapeTitle(["cornfield", "cornfield", "icy_lands", "icy_lands"]), "Cornfield + Icy Lands");
  assert.equal(landscapeTitle([]), "");
});

test("the remembered deck is kept while it can be taken, otherwise the first that can", () => {
  const boxes = shelf(presets, saved, cards);
  assert.equal(pickDeck(boxes, "my:a1"), "my:a1");
  assert.equal(pickDeck(boxes, "my:b2"), "Finn Vs Jake Finn's Deck"); // unfinished
  assert.equal(pickDeck(boxes, "my:gone"), "Finn Vs Jake Finn's Deck"); // deleted, or signed out
  assert.equal(pickDeck(boxes, null), "Finn Vs Jake Finn's Deck");
  assert.equal(pickDeck(shelf([], [], cards), "x"), "");
});

test("every Landscape has a colour, and an unknown one still gets one", () => {
  for (const l of ["cornfield", "blue_plains", "useless_swamp", "sandy_lands", "nice_lands", "icy_lands"]) {
    assert.match(boxColour(l), /^#[0-9a-f]{6}$/);
    assert.match(diamondColour(l), /^#[0-9a-f]{6}$/);
  }
  assert.match(boxColour("nope"), /^#[0-9a-f]{6}$/);
  assert.match(diamondColour(undefined), /^#[0-9a-f]{6}$/);
});

test("the starter decks are laid out as the game pairs them", () => {
  const heroes = ["gunter", "cake", "bmo", "finn", "fionna", "lumpy-space-princess", "jake", "lady-rainicorn", "princess-bubblegum"];
  assert.deepEqual(inBoxOrder(heroes.map((hero) => ({ hero }))).map((d) => d.hero),
    ["finn", "jake", "bmo", "lady-rainicorn", "princess-bubblegum", "lumpy-space-princess", "fionna", "cake", "gunter"]);
});

test("a saved deck is shown with the Landscapes the server says it is played with, chosen or not", () => {
  const chosen = [{ id: "c", name: "Chosen", hero: "finn", cards: ["pup", "pup"], playable: true, landscapes: ["icy_lands", "icy_lands", "blue_plains", "blue_plains"], landscapes_chosen: true }];
  const [box] = shelf(presets, chosen, cards).mine;
  assert.deepEqual(box.lands, ["icy_lands", "icy_lands", "blue_plains", "blue_plains"]);
  assert.equal(box.title, "Icy Lands + Blue Plains");
});

test("a deck the order does not name follows the rest, in the order it came, and nothing is lost", () => {
  const list = [{ hero: "new-b" }, { hero: "cake" }, { hero: "new-a" }, { hero: "finn" }];
  assert.deepEqual(inBoxOrder(list).map((d) => d.hero), ["finn", "cake", "new-b", "new-a"]);
  assert.deepEqual(inBoxOrder([]), []);
  assert.equal(list[0].hero, "new-b"); // the list it was given is left alone
});

import { botDeckFor, RANDOM } from "../js/decklist.js";

test("a named bot deck is used as is, even the player's own", () => {
  const starters = [{ value: "A Finn" }, { value: "A Jake" }];
  assert.equal(botDeckFor("A Jake", starters, "A Jake"), "A Jake");
});

test("Random never picks the player's own deck while another is left", () => {
  const starters = [{ value: "A Finn" }, { value: "A Jake" }, { value: "B BMO" }];
  for (const r of [0, 0.3, 0.5, 0.99]) assert.notEqual(botDeckFor(RANDOM, starters, "A Jake", () => r), "A Jake");
  assert.equal(botDeckFor(RANDOM, starters, "A Jake", () => 0), "A Finn");
  assert.equal(botDeckFor(RANDOM, starters, "A Jake", () => 0.99), "B BMO");
});

test("a saved deck can be named, but Random never picks one", () => {
  const starters = [{ value: "A Finn" }, { value: "A Jake" }];
  const mine = [{ value: "my:1" }, { value: "my:2", disabled: true }];
  assert.equal(botDeckFor("my:1", starters, "A Finn", () => 0, mine), "my:1");
  assert.equal(botDeckFor("my:2", starters, "A Finn", () => 0, mine), "A Jake"); // unfinished: not offered
  for (const r of [0, 0.5, 0.99]) assert.ok(starters.some((b) => b.value === botDeckFor(RANDOM, starters, "my:1", () => r, mine)));
});

test("Random with one deck, an own deck that is not a starter, or no decks", () => {
  assert.equal(botDeckFor(RANDOM, [{ value: "A Finn" }], "A Finn"), "A Finn");
  assert.equal(botDeckFor(RANDOM, [{ value: "A Finn" }, { value: "A Jake" }], "my:7", () => 0.6), "A Jake");
  assert.equal(botDeckFor(RANDOM, [], ""), undefined);
  assert.equal(botDeckFor("gone", [{ value: "A Finn" }], "", () => 0), "A Finn");
});
