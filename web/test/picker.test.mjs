import { test } from "node:test";
import assert from "node:assert/strict";
import { ActionPicker, sources, sourceKey, kindKey, slotsOf, tokenOfRef, parseToken, isGeneral, floopKinds, kindLabel } from "../js/picker.js";
import { buildPrompt } from "../js/prompt.js";
import { view, ctxFor } from "./fixtures.mjs";

const creatureRef = (player, lane) => ({ kind: "creature", player, lane });
const play = (card, lane, targets) => ({ kind: "play_card", card, lane, ...(targets ? { targets } : {}) });

test("general actions have no source; others group by source", () => {
  const legal = [{ kind: "draw" }, { kind: "end_main" }, play(10, 0), play(10, 3), { kind: "attack", lane: 2 }, { kind: "floop", lane: 0, source: "creature" }, { kind: "activate", lane: 1, source: "building", index: 1 }];
  assert.ok(isGeneral(legal[0]) && isGeneral(legal[1]));
  const s = sources(legal);
  assert.deepEqual(Object.keys(s).sort(), ["hand:10", "lane:0:creature", "lane:1:building", "lane:2:creature"]);
  assert.equal(s["hand:10"].length, 2);
  assert.equal(sourceKey({ kind: "play_pending" }), "pending");
  assert.equal(kindKey(legal[6]), "activate:1");
});

test("tokens round-trip", () => {
  assert.equal(tokenOfRef({ kind: "creature", player: 1, lane: 2 }), "creature:1:2");
  assert.equal(tokenOfRef({ kind: "discard_card", player: 0, card: 13 }), "discard_card:0:13");
  assert.equal(tokenOfRef({ kind: "hand_card", player: 0, card: 12 }), "hand_card:12");
  assert.deepEqual(parseToken("dest:3"), { kind: "dest", lane: 3 });
  assert.deepEqual(parseToken("creature:1:2"), { kind: "creature", player: 1, lane: 2 });
  assert.deepEqual(parseToken("discard_card:0:13"), { kind: "discard_card", player: 0, uid: 13 });
});

test("a Creature needs a lane; a Spell does not", () => {
  const ctx = ctxFor();
  assert.deepEqual(slotsOf(play(10, 2), ctx), [["dest:2"]]);
  assert.deepEqual(slotsOf(play(11, 0, [[creatureRef(1, 0)]]), ctx), [["creature:1:0"]]);
});

test("a Creature is placed with one click on a lane", () => {
  const ctx = ctxFor();
  const p = new ActionPicker([play(10, 0), play(10, 1), play(10, 3)], ctx);
  assert.equal(p.result(), null, "nothing is sent before a click");
  assert.deepEqual(p.options().tokens, ["dest:0", "dest:1", "dest:3"]);
  assert.equal(p.pick("dest:9"), false);
  assert.ok(p.pick("dest:1"));
  assert.deepEqual(p.result(), play(10, 1));
});

test("a choice-free Spell waits for confirm()", () => {
  const ctx = ctxFor();
  const p = new ActionPicker([{ kind: "play_card", card: 11 }], ctx);
  assert.equal(p.result(), null);
  assert.deepEqual(p.options().tokens, []);
  assert.deepEqual(p.confirm(), { kind: "play_card", card: 11 });
});

test("targets narrow the actions and an optional slot offers None", () => {
  const ctx = ctxFor();
  const a = [play(11, 0, [[creatureRef(1, 0)]]), play(11, 0, [[creatureRef(1, 2)]]), play(11, 0, [[]])];
  const p = new ActionPicker(a, ctx);
  assert.deepEqual(p.options().tokens, ["creature:1:0", "creature:1:2"]);
  assert.ok(p.options().can_skip);
  assert.equal(p.options().skip_label, "None");
  assert.ok(p.skip());
  assert.deepEqual(p.result(), a[2]);
});

test("slots that every action leaves empty need no click", () => {
  const ctx = ctxFor();
  const a = [
    play(11, 0, [[], [], []]),
    play(11, 0, [[], [], [{ kind: "lane", player: 0, lane: 1 }]]),
    play(11, 0, [[], [], [{ kind: "lane", player: 0, lane: 2 }]]),
  ];
  const p = new ActionPicker(a, ctx);
  assert.deepEqual(p.options().tokens, ["lane:0:1", "lane:0:2"], "the first two empty slots are already closed");
  assert.ok(p.options().can_skip, "the third is optional");
  assert.ok(p.pick("lane:0:2"));
  assert.deepEqual(p.result(), a[2]);
});

test("several targets in one slot are chosen one by one", () => {
  const ctx = ctxFor();
  const both = [creatureRef(1, 0), creatureRef(1, 2)];
  const a = [play(11, 0, [[creatureRef(1, 0)]]), play(11, 0, [both])];
  const p = new ActionPicker(a, ctx);
  assert.ok(p.pick("creature:1:0"));
  assert.equal(p.result(), null, "still ambiguous: Done or one more");
  assert.equal(p.options().skip_label, "Done");
  assert.deepEqual(p.options().tokens, ["creature:1:2"]);
  assert.ok(p.pick("creature:1:2"));
  assert.deepEqual(p.result(), a[1]);
});

test("a source with several kinds lists them, and restricts to one", () => {
  const ctx = ctxFor();
  const acts = [{ kind: "attack", lane: 0 }, { kind: "floop", lane: 0, source: "creature", targets: [[creatureRef(1, 0)]] }];
  assert.deepEqual(new ActionPicker(acts, ctx).kinds(), ["attack", "floop"]);
  const f = new ActionPicker(acts, ctx, "floop");
  assert.deepEqual(f.options().tokens, ["creature:1:0"]);
  assert.ok(f.pick("creature:1:0"));
  assert.deepEqual(f.result(), acts[1]);
});

test("cost targets come before effect targets", () => {
  const ctx = ctxFor();
  const a = { kind: "play_card", card: 11, cost_targets: [[{ kind: "hand_card", player: 0, card: 12 }]], targets: [[creatureRef(1, 0)]] };
  const p = new ActionPicker([a], ctx);
  assert.deepEqual(p.options().tokens, ["hand_card:12"]);
  assert.ok(p.pick("hand_card:12"));
  assert.deepEqual(p.options().tokens, ["creature:1:0"]);
  assert.ok(p.pick("creature:1:0"));
  assert.deepEqual(p.result(), a);
});

test("token order does not depend on the order the server listed actions", () => {
  const ctx = ctxFor();
  const a = [play(10, 3), play(10, 0), play(10, 1)];
  assert.deepEqual(new ActionPicker(a, ctx).options().tokens, new ActionPicker([...a].reverse(), ctx).options().tokens);
});

// ---- prompt

const myTurn = (over) => ctxFor(view({ legal: [{ kind: "draw" }], ...over }));

test("prompt: waiting, idle, fight phase", () => {
  assert.match(buildPrompt(ctxFor(view({ awaiting: [1] })), false, "", null, false).text, /Waiting for Jake/);
  assert.match(buildPrompt(myTurn(), true, "", null, false).text, /click a glowing card/);
  assert.match(buildPrompt(myTurn({ phase: "fight" }), true, "", null, false).text, /Fight!/);
  assert.equal(buildPrompt(myTurn({ over: true }), true, "", null, false).text, "Game over");
});

test("prompt: choosing a lane, a target, and confirming", () => {
  const ctx = myTurn();
  const lanes = new ActionPicker([play(10, 0), play(10, 1)], ctx);
  const p1 = buildPrompt(ctx, true, "hand:10", lanes, false);
  assert.equal(p1.text, "Choose a lane for Cool Dog");
  assert.ok(p1.cancel);
  const spell = new ActionPicker([{ kind: "play_card", card: 11 }], ctx);
  const p2 = buildPrompt(ctx, true, "hand:11", spell, false);
  assert.equal(p2.confirm, "Cast Corn Scepter");
  const targets = new ActionPicker([play(11, 0, [[creatureRef(1, 0)]]), play(11, 0, [[]])], ctx);
  const p3 = buildPrompt(ctx, true, "hand:11", targets, false);
  assert.equal(p3.text, "Choose a target for Corn Scepter (optional)");
  assert.equal(p3.skip, "None");
});

test("prompt: asks which kind first", () => {
  const ctx = myTurn();
  const acts = [{ kind: "attack", lane: 0 }, { kind: "floop", lane: 0, source: "creature" }];
  const p = buildPrompt(ctx, true, "lane:0:creature", new ActionPicker(acts, ctx), true);
  assert.equal(p.text, "What should The Pig do?");
  assert.deepEqual(p.kinds, [["attack", "Fight"], ["floop", "Floop"]]);
});

// ---- choices (the modal)

test("choose answers a Choice and belongs to no card on the board", () => {
  assert.ok(isGeneral({ kind: "choose", card: 12 }));
  assert.deepEqual(sources([{ kind: "choose", card: 12 }]), {});
});

test("prompt: a Choice tells the decider to use the dialog and everyone else to wait", () => {
  const choice = { player: 0, kind: "discard", card: "corn-scepter", count: 2 };
  const mine = ctxFor(view({ choice, legal: [{ kind: "choose", card: 10 }] }));
  assert.match(buildPrompt(mine, true, "", null, false).text, /Corn Scepter: make your choice in the dialog/);
  const theirs = ctxFor(view({ choice: { ...choice, player: 1 }, awaiting: [1] }));
  assert.match(buildPrompt(theirs, false, "", null, false).text, /Waiting for Jake to decide \(Corn Scepter\)/);
});

test("the picker says what each choice is for", async () => {
  const { ActionPicker } = await import("../js/picker.js");
  const { ctxFor, view } = await import("./fixtures.mjs");
  const ref = (lane) => ({ kind: "creature", player: 0, lane });
  const act = (heal, hurt) => ({ kind: "activate", lane: 0, targets: [heal, hurt], labels: ["Heal 1 damage from", "Deal 1 damage to"] });
  const p = new ActionPicker([act([ref(1)], []), act([], [ref(1)]), act([ref(1)], [ref(2)])], ctxFor(view()));
  assert.equal(p.label(), "Heal 1 damage from");
  p.pick("creature:0:1"); // both remaining choices heal it, so that slot is done
  assert.equal(p.label(), "Deal 1 damage to");

  const q = new ActionPicker([act([], []), act([ref(1)], []), act([ref(1), ref(2)], []), act([], [ref(1)])], ctxFor(view()));
  assert.equal(q.options().skip_label, "Next: Deal 1 damage to", "the button names the step it leads to");
  q.pick("creature:0:1");
  assert.equal(q.options().skip_label, "Next: Deal 1 damage to");
  q.skip();
  assert.equal(q.options().skip_label, "None", "the last step just ends");
});

test("prompt: a labelled choice says what it is for", () => {
  const ctx = ctxFor(view());
  const me = ctx.view.players[ctx.view.you];
  const mine = me.lanes.findIndex((l) => l.creature);
  const ref = (lane) => ({ kind: "creature", player: ctx.view.you, lane });
  const act = (heal, hurt) => ({ kind: "activate", lane: mine, targets: [heal, hurt], labels: ["Heal 1 damage from", "Deal 1 damage to"] });
  const picker = new ActionPicker([act([ref(mine)], []), act([], [ref(mine)])], ctx);
  const p = buildPrompt(ctx, true, `lane:${mine}:creature`, picker, false);
  assert.match(p.text, /^Heal 1 damage from: choose a target for /);
});

test("a Hero's ability is its own source and is prompted by the Hero's name", () => {
  const ability = { kind: "activate", source: "hero", index: 0, targets: [[{ kind: "hand_card", player: 0, card: 10 }], [creatureRef(1, 0)]] };
  assert.equal(sourceKey(ability), "hero");
  assert.deepEqual(Object.keys(sources([{ kind: "draw" }, ability, play(10, 0)])).sort(), ["hand:10", "hero"]);
  const v = view({ legal: [ability] });
  v.players[0].hero = { uid: 2, card: "the-pig", owner: 0 };
  const picker = new ActionPicker([ability], ctxFor(v));
  const prompt = buildPrompt(ctxFor(v), true, "hero", picker, false);
  assert.match(prompt.text, /The Pig/);
});

test("removing a frozen token is a source of its own: a Landscape, and the card to discard", () => {
  const ctx = ctxFor();
  const legal = [
    { kind: "thaw", lane: 2, card: 31 }, { kind: "thaw", lane: 2, card: 32 }, { kind: "thaw", lane: 0, card: 31 }, { kind: "draw" },
  ];
  assert.equal(sourceKey(legal[0]), "thaw:2");
  assert.equal(kindKey(legal[0]), "thaw");
  assert.deepEqual(Object.keys(sources(legal)).sort(), ["thaw:0", "thaw:2"]);
  assert.deepEqual(slotsOf(legal[0], ctx), [["hand_card:31"]]);

  const p = new ActionPicker(sources(legal)["thaw:2"], ctx, "thaw");
  assert.equal(p.result(), null, "nothing is discarded before a click");
  assert.deepEqual(p.options().tokens, ["hand_card:31", "hand_card:32"]);
  assert.equal(p.pick("hand_card:32"), true);
  assert.deepEqual(p.result(), { kind: "thaw", lane: 2, card: 32 });
});


test("a card's two Floops are told apart, the first keeping the plain key", () => {
  const own = { kind: "floop", lane: 0 }, granted = { kind: "floop", lane: 0, index: 1, via: "hero" };
  assert.equal(kindKey(own), "floop");
  assert.equal(kindKey(granted), "floop:1");
  assert.deepEqual(floopKinds([own, granted, { ...granted, targets: [] }, { kind: "attack", lane: 0 }]), ["floop", "floop:1"]);
  assert.deepEqual(floopKinds([granted]), ["floop:1"]); // only the granted one is usable: no choice to make
  assert.equal(kindLabel("floop:1", "hero"), "Floop (Hero's)");
  assert.equal(kindLabel("floop"), "Floop");
});

// Reclaim Landscape: flip a face-down Landscape, move a Building to a Lane without one; every part optional, and the Lane
// is asked only once a Building is chosen (the server lists exactly the combinations that make sense).
const reclaim = (() => {
  const flip = { kind: "landscape", player: 0, lane: 1 }, building = { kind: "building", player: 0, lane: 0 };
  const labels = ["Flip", "Move", "Move to"], skip_labels = ["Don't flip a Landscape", "Don't move a Building", ""];
  const act = (targets) => ({ kind: "play_card", card: 11, targets, labels, skip_labels });
  const lanes = [2, 3].map((lane) => ({ kind: "lane", player: 0, lane }));
  return [
    act([[], [], []]), act([[flip], [], []]),
    ...lanes.flatMap((l) => [act([[], [building], [l]]), act([[flip], [building], [l]])]),
  ];
})();

test("Reclaim Landscape: skipping the Building never asks for a Lane, and the skip says what it skips", () => {
  const ctx = myTurn();
  const p = new ActionPicker(reclaim, ctx);
  const o1 = p.options();
  assert.deepEqual(o1.tokens, ["landscape:0:1"]);
  assert.equal(o1.skip_label, "Don't flip a Landscape");
  assert.equal(o1.optional, true);
  assert.equal(buildPrompt(ctx, true, "hand:11", p, false).text, "Flip: choose a Landscape for Corn Scepter (optional)");
  assert.ok(p.skip());
  const o2 = p.options();
  assert.deepEqual(o2.tokens, ["building:0:0"]);
  assert.equal(o2.skip_label, "Don't move a Building");
  assert.ok(p.skip());
  // the Lane question is not asked: nothing is left to click, and the card is not played by that click
  assert.deepEqual(p.options().tokens, []);
  assert.equal(p.result(), null);
  const prompt = buildPrompt(ctx, true, "hand:11", p, false);
  assert.equal(prompt.text, "This won't do anything: Corn Scepter has nothing chosen.");
  assert.equal(prompt.confirm, "Cast Corn Scepter anyway");
  assert.deepEqual(p.confirm().targets, [[], [], []]);
});

test("Reclaim Landscape: flip only waits for a Confirm that says what is skipped", () => {
  const ctx = myTurn();
  const p = new ActionPicker(reclaim, ctx);
  assert.ok(p.pick("landscape:0:1"));
  assert.equal(p.options().skip_label, "Don't move a Building");
  assert.ok(p.skip());
  assert.equal(p.result(), null);
  const prompt = buildPrompt(ctx, true, "hand:11", p, false);
  assert.match(prompt.text, /^Flip .*\(your, lane 2\)\. Don't move a Building\.$/);
  assert.equal(prompt.confirm, "Cast Corn Scepter");
});

test("Reclaim Landscape: move only asks which Lane, then summarizes before it is cast", () => {
  const ctx = myTurn();
  const p = new ActionPicker(reclaim, ctx);
  assert.ok(p.skip()); // no flip
  assert.ok(p.pick("building:0:0"));
  const o = p.options();
  assert.deepEqual(o.tokens, ["lane:0:2", "lane:0:3"]);
  assert.equal(o.optional, false, "a Building to move needs somewhere to go");
  assert.equal(buildPrompt(ctx, true, "hand:11", p, false).text, "Move to: choose a lane for Corn Scepter");
  assert.ok(p.pick("lane:0:3"));
  assert.equal(p.result(), null, "the flip was declined, so it is confirmed first");
  assert.match(buildPrompt(ctx, true, "hand:11", p, false).text, /^Don't flip a Landscape\. Move building \(your, lane 1\)\. Move to lane 4 \(your side\)\.$/);
});

test("Reclaim Landscape: flip and move together are played at the last click", () => {
  const ctx = myTurn();
  const p = new ActionPicker(reclaim, ctx);
  assert.ok(p.pick("landscape:0:1"));
  assert.ok(p.pick("building:0:0"));
  assert.ok(p.pick("lane:0:2"));
  const a = p.result();
  assert.ok(a, "nothing was declined: no confirmation needed");
  assert.deepEqual(a.targets.map((s) => s.length), [1, 1, 1]);
});

test("a card without wording for skipping is played at the last click as before", () => {
  const ctx = myTurn();
  const p = new ActionPicker([play(11, 0, [[creatureRef(1, 0)]]), play(11, 0, [[]])], ctx);
  assert.ok(p.skip());
  assert.deepEqual(p.result().targets, [[]]);
});

// Heal-or-damage cards (Green Candy, Auto-Plucker): each half has its own skip wording, and choosing neither is confirmed.
test("heal-or-damage: skipping both parts warns, and a skip says what it skips", () => {
  const ctx = myTurn();
  const c = { kind: "creature", player: 0, lane: 0 };
  const labels = ["Heal 1 damage from", "Deal 1 damage to"], skip_labels = ["Don't heal", "Don't deal damage"];
  const act = (targets) => ({ kind: "play_card", card: 11, targets, labels, skip_labels });
  const p = new ActionPicker([act([[], []]), act([[c], []]), act([[], [c]])], ctx);
  assert.equal(p.options().skip_label, "Don't heal");
  assert.ok(p.skip());
  assert.equal(p.options().skip_label, "Don't deal damage");
  assert.ok(p.skip());
  assert.equal(p.result(), null);
  assert.equal(buildPrompt(ctx, true, "hand:11", p, false).confirm, "Cast Corn Scepter anyway");
});
