import { test } from "node:test";
import assert from "node:assert/strict";
import { FAN_MAX, isCrowded, fanPose } from "../js/handlayout.js";

test("a hand fans until it holds more than ten cards, then wraps flat", () => {
  assert.equal(FAN_MAX, 10);
  assert.equal(isCrowded(0), false);
  assert.equal(isCrowded(7), false);
  assert.equal(isCrowded(10), false);
  assert.equal(isCrowded(11), true);
  assert.equal(isCrowded(30), true);
});

test("a fanned hand tips outwards from the middle", () => {
  assert.deepEqual(fanPose(0, 1), { rotate: 0, drop: 0 });
  assert.deepEqual(fanPose(1, 3), { rotate: 0, drop: 0 });
  assert.deepEqual(fanPose(0, 3), { rotate: -3, drop: 3 });
  assert.deepEqual(fanPose(2, 3), { rotate: 3, drop: 3 });
  assert.deepEqual(fanPose(0, 2), { rotate: -1.5, drop: 1.5 });
});
