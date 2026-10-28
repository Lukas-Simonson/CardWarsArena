import { test } from "node:test";
import assert from "node:assert/strict";
import { plan, HIT_DELAY, DIE_DELAY } from "../js/anim.js";
import { view, cards } from "./fixtures.mjs";

const next = (mutate) => { const v = structuredClone(view()); mutate(v); return v; };
const ctx = { cards };
const kinds = (fx) => fx.map((f) => f.t);

test("nothing is animated for the first view or an unchanged one", () => {
  assert.deepEqual(plan(null, view(), [], ctx), []);
  assert.deepEqual(plan(view(), view(), [], ctx), []);
});

test("a Creature that appears drops in on its side; one that leaves leaves a ghost", () => {
  const entered = next((v) => { v.players[0].lanes[3].creature = { uid: 90, card: "cool-dog", owner: 0, damage: 0, status: "ready", attack: 2, defense: 7 }; });
  assert.deepEqual(plan(view(), entered, [], ctx), [{ t: "drop", seat: 0, lane: 3 }]);

  const gone = next((v) => { v.players[1].lanes[0].creature = null; });
  const fx = plan(view(), gone, [{ kind: "die", player: 1, card: 30 }], ctx);
  assert.deepEqual(fx, [{ t: "ghost", seat: 1, lane: 0, card: "cool-dog", status: "ready", delay: DIE_DELAY }]);
  assert.equal(plan(view(), gone, [{ kind: "bounce", player: 1, card: 30 }], ctx)[0].delay, 0, "bounced, not destroyed: no lingering");
});

test("a Creature that moves lanes drops into the new one and leaves no ghost", () => {
  const moved = next((v) => { const c = v.players[1].lanes[0].creature; v.players[1].lanes[0].creature = null; v.players[1].lanes[2].creature = c; });
  assert.deepEqual(plan(view(), moved, [{ kind: "move", player: 1, card: 30, lane: 2 }], ctx), [{ t: "drop", seat: 1, lane: 2 }]);
});

test("a Fight: lunge first, then the blow lands", () => {
  const after = next((v) => { v.players[1].hp = 21; v.players[0].lanes[0].creature.status = "fought"; });
  const events = [{ kind: "attack", player: 0, card: 20 }, { kind: "hp_loss", player: 1, amount: 4 }];
  const fx = plan(view(), after, events, ctx);
  assert.deepEqual(fx.find((f) => f.t === "lunge"), { t: "lunge", seat: 0, lane: 0, up: true });
  assert.deepEqual(fx.find((f) => f.t === "hp"), { t: "hp", seat: 1, amount: 4, delay: HIT_DELAY });
  assert.equal(fx.find((f) => f.t === "hpbar").from, 88, "the bar slides down from where it was");
  assert.deepEqual(fx.find((f) => f.t === "settle"), { t: "settle", seat: 0, lane: 0, from: "none" });

  const dmg = plan(view(), view(), [{ kind: "attack", player: 1, card: 30, lane: 2 }, { kind: "damage", player: 0, card: 22, lane: 2, amount: 2 }], ctx);
  assert.deepEqual(dmg.map((f) => [f.t, f.seat, f.lane]), [["lunge", 1, 2], ["hit", 0, 2]]);
  assert.equal(dmg[1].delay, HIT_DELAY);
  assert.equal(plan(view(), view(), [{ kind: "damage", player: 1, card: 30, lane: 0, amount: 1 }], ctx)[0].delay, 0, "a spell's damage is immediate");
});

test("cards drawn, turns and spells", () => {
  const drew = next((v) => { v.players[0].hand.push({ uid: 99, card: "the-pig", owner: 0 }, { uid: 98, card: "the-pig", owner: 0 }); v.players[1].hand_count = 5; });
  const fx = plan(view(), drew, [], ctx);
  assert.deepEqual(fx.filter((f) => f.t === "deal").map((f) => [f.uid, f.delay]), [[99, 0], [98, 120]]);
  assert.ok(kinds(fx).includes("oppdraw"));

  const turn = plan(view(), view(), [{ kind: "turn_start", player: 1 }, { kind: "play", player: 0, card: 11, card_id: "corn-scepter" }, { kind: "play", player: 0, card: 10, card_id: "cool-dog" }], ctx);
  assert.deepEqual(turn, [{ t: "turn", seat: 1, mine: false }, { t: "cast", card: "corn-scepter", seat: 0 }]);
});

test("a new Building pops in", () => {
  const built = next((v) => { v.players[0].lanes[3].building = { uid: 95, card: "blood-fortress", owner: 0, status: "ready" }; });
  assert.deepEqual(plan(view(), built, [], ctx), [{ t: "build", seat: 0, lane: 3 }]);
});
