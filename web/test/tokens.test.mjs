import test from "node:test";
import assert from "node:assert/strict";
import { tokenChips } from "../js/tokens.js";

test("only the kinds a Creature has are shown, in the usual order", () => {
  assert.deepEqual(tokenChips(undefined), []);
  assert.deepEqual(tokenChips({ attack: 0 }), []);
  assert.deepEqual(tokenChips({ burn: 1, attack: 2 }).map((c) => [c.kind, c.n]), [["attack", 2], ["burn", 1]]);
});

test("a kind this client does not know is left out, not shown wrongly", () => {
  assert.deepEqual(tokenChips({ mystery: 3 }), []);
});

test("attack tokens hang under ATK and burn tokens under DEF", () => {
  assert.deepEqual(tokenChips({ attack: 1, burn: 1 }).map((c) => c.under), ["atk", "def"]);
});
