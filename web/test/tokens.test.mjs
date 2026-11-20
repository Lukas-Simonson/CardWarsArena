import test from "node:test";
import assert from "node:assert/strict";
import { tokenChips } from "../js/tokens.js";

test("only the kinds a Creature has are shown, in the usual order", () => {
  assert.deepEqual(tokenChips(undefined), []);
  assert.deepEqual(tokenChips({ attack: 0 }), []);
  assert.deepEqual(tokenChips({ burn: 1, attack: 2 }).map((c) => [c.kind, c.n]), [["attack", 2], ["burn", 1]]);
});

test("each kind says how many, with a pip per token up to a limit", () => {
  const [a, b] = tokenChips({ attack: 2, burn: 9 });
  assert.deepEqual([a.label, a.pips, a.under], ["+2", 2, "atk"]);
  assert.deepEqual([b.label, b.pips, b.under], ["🔥9", 6, "def"]);
});

test("a kind this client does not know is left out, not shown wrongly", () => {
  assert.deepEqual(tokenChips({ mystery: 3 }), []);
});
