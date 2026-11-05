import { test } from "node:test";
import assert from "node:assert/strict";
import { deckOptions, pickDeck } from "../js/decklist.js";

const presets = [{ set: "Finn Vs Jake", name: "Finn's Deck", owner: "Finn" }, { set: "Finn Vs Jake", name: "Jake's Deck", owner: "Jake" }];
const saved = [
  { id: "a1", name: "Ponies", playable: true, problems: [] },
  { id: "b2", name: "Wip", playable: false, problems: ["12 cards: a deck needs at least 40", "choose a Hero"] },
];

test("starter decks are asked for by words, saved ones as my:<id>", () => {
  const o = deckOptions(presets, saved);
  assert.deepEqual(o.starters[0], { label: "Finn — Finn's Deck", value: "Finn Vs Jake Finn's Deck" });
  assert.equal(o.mine[0].value, "my:a1");
  assert.equal(o.mine[0].disabled, false);
});

test("an unfinished deck is listed but cannot be chosen, and says why", () => {
  const wip = deckOptions(presets, saved).mine[1];
  assert.deepEqual([wip.label, wip.disabled], ["Wip (unfinished)", true]);
  assert.match(wip.title, /needs at least 40; choose a Hero/);
});

test("a guest has only the starters", () => {
  assert.deepEqual(deckOptions(presets, []).mine, []);
});

test("the remembered deck is kept while it can be chosen, otherwise the first that can", () => {
  const o = deckOptions(presets, saved);
  assert.equal(pickDeck(o, "my:a1"), "my:a1");
  assert.equal(pickDeck(o, "my:b2"), "Finn Vs Jake Finn's Deck"); // unfinished
  assert.equal(pickDeck(o, "my:gone"), "Finn Vs Jake Finn's Deck"); // deleted, or signed out
  assert.equal(pickDeck(o, null), "Finn Vs Jake Finn's Deck");
  assert.equal(pickDeck(deckOptions([], []), "x"), "");
});
