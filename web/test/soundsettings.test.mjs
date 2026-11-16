import { test } from "node:test";
import assert from "node:assert/strict";
import { defaults, normalize, load, save, loudness, KEY } from "../js/soundsettings.js";

const memory = () => { const m = new Map(); return { getItem: (k) => m.get(k) ?? null, setItem: (k, v) => m.set(k, String(v)), m }; };

test("sound starts on at a moderate level, and quiet for someone who prefers less motion", () => {
  assert.deepEqual(defaults(), { volume: 0.6, effects: 1, muted: false, quiet: false });
  assert.equal(defaults({ reducedMotion: true }).quiet, true);
});

test("saved choices come back, and rubbish falls back to the defaults", () => {
  const s = memory();
  save(s, { volume: 0.2, effects: 0.5, muted: true, quiet: true });
  assert.deepEqual(load(s), { volume: 0.2, effects: 0.5, muted: true, quiet: true });
  s.setItem(KEY, "{not json");
  assert.deepEqual(load(s), defaults());
  s.setItem(KEY, JSON.stringify({ volume: "loud", effects: 7, muted: "yes" }));
  assert.deepEqual(load(s), { volume: 0.6, effects: 1, muted: false, quiet: false });
  assert.deepEqual(load(null), defaults());
});

test("levels stay between zero and one, and blocked storage is not an error", () => {
  assert.equal(normalize({ volume: -3 }).volume, 0);
  assert.equal(normalize({ volume: 3 }).volume, 1);
  const blocked = { getItem() { throw new Error("blocked"); }, setItem() { throw new Error("blocked"); } };
  assert.deepEqual(load(blocked), defaults());
  assert.doesNotThrow(() => save(blocked, defaults()));
});

test("loudness is the master volume, the effects level and the cue's own, and nothing when muted", () => {
  assert.equal(loudness({ volume: 0.5, effects: 0.5, muted: false }, 0.8), 0.2);
  assert.equal(loudness({ volume: 1, effects: 1, muted: true }, 1), 0);
});
