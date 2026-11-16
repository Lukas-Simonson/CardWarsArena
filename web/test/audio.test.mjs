import { test } from "node:test";
import assert from "node:assert/strict";
import { createEngine, MAX_VOICES } from "../js/audio.js";

// A stand-in for the browser's audio: it records what was started.
function stubAudio({ state = "running", failDecode = new Set() } = {}) {
  const log = { started: [], created: 0, stopped: 0 };
  const node = () => ({ connect() {}, gain: { value: 1 } });
  class Ctx {
    constructor() { this.currentTime = 0; this.state = state; this.destination = {}; log.created++; }
    async resume() { this.state = "running"; }
    createGain() { return node(); }
    createBufferSource() {
      const s = { buffer: null, playbackRate: { value: 1 }, connect() {}, start: (when) => log.started.push({ buffer: s.buffer, when, rate: s.playbackRate.value, gain: s.gain }), stop: () => { log.stopped++; } };
      return s;
    }
    async decodeAudioData(data) {
      if (failDecode.has(data.tag)) throw new Error("cannot decode");
      return { duration: 0.5, tag: data.tag };
    }
  }
  const fetched = [];
  const fetchBytes = async (url) => { fetched.push(url); return { tag: url, slice() { return this; } }; };
  return { Ctx, log, fetchBytes, fetched };
}

const cues = {
  a: { files: ["a1", "a2"], gain: 0.5, cooldown: 100 },
  b: { files: ["b1"], gain: 0.5, cooldown: 0 },
  hit1: { files: ["h1"], cooldown: 0, group: "hits" },
  hit2: { files: ["h2"], cooldown: 0, group: "hits" },
  core: { files: ["c1"], cooldown: 0, core: true, priority: 3 },
  low: { files: ["l1"], cooldown: 0, priority: 1 },
};

async function started(opts = {}, io = {}) {
  const au = stubAudio(opts);
  const engine = createEngine({ AudioContextClass: au.Ctx, fetchBytes: au.fetchBytes, canPlay: () => true, cues, random: () => 0.5, ...io });
  await engine.preload();
  await engine.unlock();
  return { engine, ...au };
}

test("nothing plays before the browser allows it, and asking is not an error", async () => {
  const au = stubAudio();
  const engine = createEngine({ AudioContextClass: au.Ctx, fetchBytes: au.fetchBytes, canPlay: () => true, cues });
  assert.equal(engine.play("a"), false);
  assert.equal(au.log.created, 0, "no context until a click or key press");
  await engine.preload();
  assert.ok(au.fetched.length > 0, "the files are fetched ahead of time");
  assert.equal(engine.play("a"), false);
  await engine.unlock();
  assert.equal(engine.play("a"), true);
});

test("a context the browser keeps suspended does not break anything, and the next gesture tries again", async () => {
  let allow = false;
  const au = stubAudio();
  class Strict extends au.Ctx {
    constructor() { super(); this.state = "suspended"; }
    async resume() { if (!allow) throw new Error("not allowed"); this.state = "running"; }
  }
  const engine = createEngine({ AudioContextClass: Strict, fetchBytes: au.fetchBytes, canPlay: () => true, cues });
  assert.equal(await engine.unlock(), false);
  assert.equal(engine.play("a"), false);
  allow = true;
  assert.equal(await engine.unlock(), true);
  assert.equal(engine.play("a"), true);
});

test("with no audio support at all it quietly does nothing", async () => {
  const engine = createEngine({ fetchBytes: async () => ({}), canPlay: () => false, cues });
  await engine.preload();
  assert.equal(await engine.unlock(), false);
  assert.equal(engine.play("a"), false);
  assert.doesNotThrow(() => engine.configure({ muted: true }));
});

test("files that will not load or decode are skipped, and the other format is tried", async () => {
  const au = stubAudio({ failDecode: new Set(["audio/a1.ogg"]) });
  const fetchBytes = async (url) => { if (url.endsWith("b1.ogg") && url.endsWith("b1.m4a")) throw new Error("404"); return au.fetchBytes(url); };
  const engine = createEngine({ AudioContextClass: au.Ctx, fetchBytes, canPlay: () => true, cues, random: () => 0 });
  await engine.preload();
  await engine.unlock();
  assert.ok(au.fetched.includes("audio/a1.m4a"), "a1 fell back to .m4a");
  assert.equal(engine.play("a"), true);
  const gone = createEngine({ AudioContextClass: au.Ctx, fetchBytes: async () => { throw new Error("offline"); }, canPlay: () => true, cues });
  await gone.preload();
  await gone.unlock();
  assert.equal(gone.play("a"), false);
});

test("a browser that cannot play Ogg is given .m4a first", async () => {
  const au = stubAudio();
  const engine = createEngine({ AudioContextClass: au.Ctx, fetchBytes: au.fetchBytes, canPlay: (t) => !t.includes("ogg"), cues });
  await engine.preload();
  assert.ok(au.fetched.every((u) => u.endsWith(".m4a")));
});

test("a cue does not repeat inside its cooldown, but another cue does", async () => {
  const { engine, log } = await started();
  assert.equal(engine.play("a"), true);
  assert.equal(engine.play("a"), false);
  assert.equal(engine.play("b"), true);
  assert.equal(engine.play("b"), true, "no cooldown: it may repeat");
  assert.equal(log.started.length, 3);
});

test("a delayed sound starts later, and the cooldown counts from when it starts", async () => {
  const { engine, log } = await started();
  assert.equal(engine.play("a", { delay: 500 }), true);
  assert.equal(log.started[0].when, 0.5);
  assert.equal(engine.play("a", { delay: 550 }), false, "50 ms after the first: too soon");
  assert.equal(engine.play("a", { delay: 700 }), true);
});

test("cues of a group share a limit: several hits at once are not a wall of noise", async () => {
  const { engine } = await started();
  const results = [engine.play("hit1"), engine.play("hit2"), engine.play("hit1"), engine.play("hit2"), engine.play("hit1")];
  assert.deepEqual(results, [true, true, true, false, false]);
  assert.equal(engine.play("hit1", { delay: 400 }), true, "later, the window has passed");
});

test("only so many sounds play at once: the least important gives way, or the new one is dropped", async () => {
  const { engine, log } = await started();
  for (let i = 0; i < MAX_VOICES; i++) assert.equal(engine.play("low"), true);
  assert.equal(engine.play("low"), false, "as important as the weakest: dropped");
  assert.equal(engine.play("core"), true, "more important: one of the others stops");
  assert.equal(log.stopped, 1);
});

test("each play varies a little in pitch and loudness", async () => {
  let n = 0;
  const seq = [0.5, 0.0, 0.0, 0.0, 1.0, 1.0, 1.0];
  const { engine, log } = await started({}, { random: () => seq[n++ % seq.length] });
  engine.play("b");
  engine.play("b");
  const [x, y] = log.started.map((s) => s.rate);
  assert.ok(x < 1 && y > 1, `rates ${x} ${y}`);
  assert.ok(Math.abs(x - 1) <= 0.04 + 1e-9 && Math.abs(y - 1) <= 0.04 + 1e-9);
});

test("muted, hidden or asking for fewer sounds silences what it should", async () => {
  let visible = true;
  const { engine } = await started({}, { visible: () => visible });
  engine.configure({ muted: true });
  assert.equal(engine.play("core"), false);
  engine.configure({ muted: false });
  visible = false;
  assert.equal(engine.play("core"), false, "a hidden page is silent");
  visible = true;
  engine.configure({ quiet: true });
  assert.equal(engine.play("low"), false, "not a core sound");
  assert.equal(engine.play("core"), true);
  assert.equal(engine.play("unknown_cue"), false);
});
