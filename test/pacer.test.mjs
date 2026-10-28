import { test } from "node:test";
import assert from "node:assert/strict";
import { Pacer } from "../js/pacer.js";
import { paceFor } from "../js/anim.js";

/** A pacer on a fake clock. */
function rig(hold) {
  let t = 0, timers = [];
  const delivered = [];
  const p = new Pacer({
    deliver: (m) => delivered.push([t, m.n]),
    holdFor: hold,
    now: () => t,
    timers: {
      setTimeout: (fn, ms) => { const id = timers.length; timers.push({ at: t + ms, fn, id }); return id; },
      clearTimeout: (id) => { timers = timers.filter((x) => x.id !== id); },
    },
  });
  const advance = (ms) => {
    const end = t + ms;
    for (;;) {
      const next = timers.filter((x) => x.at <= end).sort((a, b) => a.at - b.at)[0];
      if (!next) break;
      timers = timers.filter((x) => x !== next);
      t = next.at;
      next.fn();
    }
    t = end;
  };
  return { p, delivered, advance };
}

test("the first update is shown at once; later ones wait for the rest", () => {
  const { p, delivered, advance } = rig(() => 1000);
  p.push({ n: 1 }); p.push({ n: 2 }); p.push({ n: 3 });
  assert.deepEqual(delivered, [[0, 1]]);
  assert.equal(p.waiting, 2);
  advance(999);
  assert.equal(delivered.length, 1);
  advance(1);
  assert.deepEqual(delivered, [[0, 1], [1000, 2]]);
  advance(1000);
  assert.deepEqual(delivered, [[0, 1], [1000, 2], [2000, 3]]);
});

test("after the rest an update arriving later is shown immediately", () => {
  const { p, delivered, advance } = rig(() => 500);
  p.push({ n: 1 });
  advance(2000);
  p.push({ n: 2 });
  assert.deepEqual(delivered, [[0, 1], [2000, 2]]);
});

test("a zero rest passes updates straight through", () => {
  const { p, delivered } = rig(() => 0);
  p.push({ n: 1 }); p.push({ n: 2 });
  assert.deepEqual(delivered, [[0, 1], [0, 2]]);
});

test("clear drops what is waiting", () => {
  const { p, delivered, advance } = rig(() => 1000);
  p.push({ n: 1 }); p.push({ n: 2 });
  p.clear();
  advance(5000);
  assert.deepEqual(delivered, [[0, 1]]);
});

test("busier updates rest longer", () => {
  assert.ok(paceFor([{ kind: "attack" }]) > paceFor([{ kind: "draw" }]));
  assert.ok(paceFor([{ kind: "turn_start" }, { kind: "draw" }]) >= paceFor([{ kind: "play" }]));
  assert.ok(paceFor([]) > 0);
});
