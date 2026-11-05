import { test } from "node:test";
import assert from "node:assert/strict";
import { initialOf } from "../js/avatar.js";

test("the letter for a player without a picture is the first of their name, in capitals", () => {
  assert.equal(initialOf("ada"), "A");
  assert.equal(initialOf("  Lukas "), "L");
  assert.equal(initialOf("éclair"), "É");
});

test("a name with nothing in it gets a question mark", () => {
  assert.equal(initialOf(""), "?");
  assert.equal(initialOf("   "), "?");
  assert.equal(initialOf(undefined), "?");
});
