import { test } from "node:test";
import assert from "node:assert/strict";
import { pct, ranked, topCards, deckRows, summarize } from "../stats/model.mjs";

const report = {
  v: 2, kept: true,
  games: { started: { total: 140, bot: 90, person: 50 }, completed: { total: 120, bot: 80, person: 40 }, abandoned: 20 },
  results: { player_wins: 75, bot_wins: { total: 45, by_bot: { random: 14, heuristic: 31 } } },
  plays: { cards: { total: 3800, by_type: { creature: 2411, spell: 900, building: 489 } }, floops: 903, fights: 1500, hero_abilities: 210 },
  combat: { damage_dealt: 21000, creatures_destroyed: 2300 },
  decks: { finn: { started: 40, won: 14 }, custom: { started: 60, won: 30 }, jake: { started: 0, won: 0 } },
  cards: { played: { "corn-dog": 87, "blue-candy": 64, "a-card": 87 } },
};

test("a percent is whole, and null with nothing to divide", () => {
  assert.equal(pct(1, 3), 33);
  assert.equal(pct(2, 3), 67);
  assert.equal(pct(0, 0), null);
});

test("counts are ranked most first, ties by name", () => {
  assert.deepEqual(ranked({ b: 1, a: 1, c: 5 }), [["c", 5], ["a", 1], ["b", 1]]);
  assert.deepEqual(ranked(undefined), []);
});

test("top cards are named from the catalogue, or by id, and limited", () => {
  const top = topCards(report.cards.played, { "corn-dog": "Corn Dog" }, 2);
  assert.deepEqual(top, [{ id: "a-card", name: "a-card", n: 87 }, { id: "corn-dog", name: "Corn Dog", n: 87 }]);
});

test("decks are by how often they were played, with a win rate", () => {
  assert.deepEqual(deckRows(report.decks).map((d) => [d.hero, d.rate]), [["custom", 50], ["finn", 35], ["jake", null]]);
});

test("the summary has the headline numbers and the splits", () => {
  const s = summarize(report);
  assert.equal(s.completion, 86);
  assert.deepEqual([s.started, s.completed, s.abandoned, s.destroyed], [140, 120, 20, 2300]);
  assert.deepEqual(s.vsBot, { player: 75, bot: 45, playerRate: 63, games: 120 });
  assert.deepEqual(s.byBot[0], ["heuristic", 31]);
  assert.equal(s.byType[0][0], "creature");
});

test("a server that has counted nothing still summarizes", () => {
  const zero = { games: { started: { total: 0, bot: 0, person: 0 }, completed: { total: 0, bot: 0, person: 0 }, abandoned: 0 },
    results: { player_wins: 0, bot_wins: { total: 0, by_bot: {} } }, plays: { cards: { total: 0, by_type: { creature: 0 } }, floops: 0, fights: 0, hero_abilities: 0 },
    combat: { damage_dealt: 0, creatures_destroyed: 0 }, decks: {}, cards: { played: {} } };
  const s = summarize(zero);
  assert.equal(s.completion, null);
  assert.equal(s.vsBot.playerRate, null);
  assert.deepEqual(s.decks, []);
});
