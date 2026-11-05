import { test } from "node:test";
import assert from "node:assert/strict";
import { shelf, pickDeck, landscapeTitle, boxColour, diamondColour } from "../js/decklist.js";

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
  assert.deepEqual(finn, { value: "Finn Vs Jake Finn's Deck", name: "Finn's Deck", sub: "Starter · Hero: Finn", lands: blue, size: 40, disabled: false, title: "Blue Plains" });
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

test("a deck without a Hero says only what kind it is", () => {
  const [noHero] = shelf([{ ...presets[0], hero: "" }], [], cards).starters;
  assert.equal(noHero.sub, "Starter");
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
