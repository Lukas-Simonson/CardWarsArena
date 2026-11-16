import { test } from "node:test";
import assert from "node:assert/strict";
import { CUES, GROUPS, hitCue, soundsFor, soundsOf, allFiles } from "../js/sounds.js";
import { plan } from "../js/anim.js";
import { problems } from "../tools/audio-lib.mjs";

test("hits sound heavier the harder they are", () => {
  assert.equal(hitCue(1), "hit_small");
  assert.equal(hitCue(2), "hit_small");
  assert.equal(hitCue(3), "hit_medium");
  assert.equal(hitCue(5), "hit_medium");
  assert.equal(hitCue(6), "hit_big");
  assert.equal(hitCue(undefined ?? 0), "hit_small");
});

test("each planned effect has the sound for it, at its own delay", () => {
  const fx = [
    { t: "drop", seat: 0, lane: 1 }, { t: "build", seat: 0, lane: 1 }, { t: "cast", card: "x", seat: 0 }, { t: "lunge", seat: 0, lane: 0 },
    { t: "hit", seat: 1, lane: 0, amount: 7, delay: 260 }, { t: "hp", seat: 1, amount: 2, delay: 260 }, { t: "heal", seat: 0, lane: 0, amount: 1, delay: 0 },
    { t: "hpgain", seat: 0, amount: 1, delay: 0 }, { t: "ghost", seat: 1, lane: 0, delay: 420 }, { t: "deal", uid: 3, delay: 120 }, { t: "oppdraw", delay: 0 },
    { t: "turn", mine: true, delay: 1500 }, { t: "turn", mine: false, delay: 1500 }, { t: "over", won: true }, { t: "over", won: false },
  ];
  assert.deepEqual(soundsFor(fx).map((s) => [s.cue, s.delay]), [
    ["play_creature", 0], ["play_building", 0], ["cast", 0], ["attack", 0], ["hit_big", 260], ["hero_hit", 260], ["heal", 0], ["heal_hero", 0],
    ["destroy", 420], ["draw", 120], ["draw_opp", 0], ["turn_yours", 1500], ["turn_theirs", 1500], ["win", 0], ["lose", 0],
  ]);
});

test("effects with nothing to hear give no sound", () => {
  assert.deepEqual(soundsFor([{ t: "settle", seat: 0, lane: 0 }, { t: "hpbar", seat: 0, from: 100, to: 80 }, { t: "mystery" }]), []);
  assert.deepEqual(soundsOf({ t: "nothing" }), []);
});

test("every cue the mapping names exists, and its group is a known one", () => {
  const names = ["play_creature", "play_building", "cast", "attack", "hit_small", "hit_medium", "hit_big", "hero_hit", "heal", "heal_hero", "destroy", "draw", "draw_opp", "turn_yours", "turn_theirs", "discard", "floop", "ability", "shuffle", "reveal", "freeze", "fight", "win", "lose"];
  for (const n of names) assert.ok(CUES[n], n);
  for (const c of Object.values(CUES)) if (c.group) assert.ok(GROUPS[c.group]);
  assert.ok(allFiles().length > 20);
});

// The sounds follow the plan, so they are in step with what is seen.
test("an attack that kills is heard as the lunge, the blow and the death, in that order", () => {
  const v = (hand = []) => ({
    you: 0, phase: "fight", turn: 3, active: 0,
    players: [
      { hp: 25, deck_count: 10, hand_count: 0, hand, discard: [], lanes: [{ creature: { uid: 1, card: "a", status: "ready" } }, {}, {}, {}] },
      { hp: 25, deck_count: 10, hand_count: 0, discard: [], lanes: [{ creature: { uid: 2, card: "b", status: "ready" } }, {}, {}, {}] },
    ],
  });
  const after = v();
  after.players[0].lanes[0].creature.status = "fought";
  after.players[1].lanes[0] = {};
  const fx = plan(v(), after, [{ kind: "attack", player: 0, card: 1, lane: 0 }, { kind: "damage", player: 1, card: 2, lane: 0, amount: 4 }, { kind: "die", player: 1, card: 2 }], { cards: {} });
  const heard = [...soundsFor(fx)].sort((a, b) => a.delay - b.delay);
  assert.deepEqual(heard.map((s) => s.cue), ["attack", "hit_medium", "destroy"]);
  const at = Object.fromEntries(soundsFor(fx).map((s) => [s.cue, s.delay]));
  assert.ok(at.attack < at.hit_medium && at.hit_medium < at.destroy, JSON.stringify(at));
});

test("floops, abilities, discards, the Fight phase and the end of the game are heard too", () => {
  const view = (phase) => ({ you: 0, phase, turn: 3, active: 0, winner: 0, players: [{ hp: 25, hand_count: 0, hand: [], discard: [], lanes: [{}, {}, {}, {}] }, { hp: 25, hand_count: 0, discard: [], lanes: [{}, {}, {}, {}] }] });
  const events = [{ kind: "floop", player: 0, card: 1 }, { kind: "activate", player: 0, card: 1 }, { kind: "discard", player: 1, card: 4 }, { kind: "mulligan", player: 0 }, { kind: "game_over", player: 0 }];
  const fx = plan(view("main"), view("fight"), events, { cards: {} });
  assert.deepEqual(soundsFor(fx).map((s) => s.cue), ["floop", "ability", "discard", "shuffle", "win", "fight"]);
  assert.equal(soundsFor(plan(view("main"), view("main"), [{ kind: "game_over", player: 1 }], { cards: {} }))[0].cue, "lose");
});

test("the sound files are credited, licensed, in both formats, all used, and small", async () => {
  assert.deepEqual(await problems(), []);
});
