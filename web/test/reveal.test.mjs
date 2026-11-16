import { test } from "node:test";
import assert from "node:assert/strict";
import { revealGroups, revealTitle } from "../js/reveal.js";

test("reveal events are grouped by whose hand they show", () => {
  const events = [
    { kind: "damage", player: 1 },
    { kind: "reveal", player: 1, card: 5, card_id: "pony" },
    { kind: "reveal", player: 1, card: 6, card_id: "cool-dog" },
    { kind: "reveal", player: 0, card: 7, card_id: "pony" },
    { kind: "reveal", player: 0, card: 8 }, // no card named: nothing to show
  ];
  assert.deepEqual(revealGroups(events), [{ player: 1, cards: ["pony", "cool-dog"] }, { player: 0, cards: ["pony"] }]);
  assert.deepEqual(revealGroups(undefined), []);
});

test("the pop-up says whose hand it is", () => {
  assert.equal(revealTitle({ player: 0, cards: ["a", "b"] }, 0, "Me"), "You show 2 cards from your hand");
  assert.equal(revealTitle({ player: 1, cards: ["a"] }, 0, "Sam"), "Sam shows a card from their hand");
  assert.equal(revealTitle({ player: 1, cards: ["a"] }, 0, ""), "Your opponent shows a card from their hand");
});
