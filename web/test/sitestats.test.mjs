import { test } from "node:test";
import assert from "node:assert/strict";
import { summary, short } from "../js/sitestats.js";

test("a few totals are picked from the server's counts, cards added up over their types", () => {
  const got = summary({ kept: true, counts: { games_completed: 12, "cards_played.creature": 100, "cards_played.spell": 20, "card_played.finn": 7, floops: 9, bot_wins: 5, fights: 3 } });
  assert.deepEqual(got, [{ key: "games", n: 12 }, { key: "cards", n: 120 }, { key: "floops", n: 9 }, { key: "bot_wins", n: 5 }]);
});

test("nothing is shown when stats are off, failed, or nothing has been played", () => {
  assert.deepEqual(summary(null), []);
  assert.deepEqual(summary({}), []);
  assert.deepEqual(summary({ kept: false, counts: {} }), []);
  assert.deepEqual(summary({ counts: { fights: 4 } }), []); // nothing from the chosen few
});

test("long numbers are shortened for a phone, short ones are not", () => {
  assert.equal(short(9999, "en"), "9,999");
  assert.equal(short(12345, "en"), "12.3K");
  assert.equal(short(3400000, "en"), "3.4M");
});
