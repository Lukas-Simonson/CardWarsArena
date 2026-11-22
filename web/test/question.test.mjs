import { test } from "node:test";
import assert from "node:assert/strict";
import { ROLE_CLASS, SURFACE, CHOICE_SURFACE, run, fromPrompt, fromChoice } from "../js/question.js";
import { view, cards } from "./fixtures.mjs";

const ctx = { view: view(), cards, names: ["Finn", "Jake"] };
const handlers = () => { const calls = []; const on = new Proxy({}, { get: (_, k) => (...a) => calls.push([k, ...a]) }); return { on, calls }; };

test("every role has one button class, and none is shared", () => {
  assert.deepEqual(Object.keys(ROLE_CLASS).sort(), ["choice", "commit", "danger", "neutral", "special"]);
  assert.equal(new Set(Object.values(ROLE_CLASS)).size, 5);
});

test("only the guide's two surfaces exist, and every choice kind on the shared question names one", () => {
  assert.deepEqual(Object.values(SURFACE).sort(), ["bar", "popup"]);
  for (const [kind, surface] of Object.entries(CHOICE_SURFACE)) assert.ok(Object.values(SURFACE).includes(surface), kind);
  assert.equal(CHOICE_SURFACE.amount, "bar", "a short decision is on the bar (W3)");
  assert.equal(CHOICE_SURFACE.reveal, "popup", "cards that are not on the table are shown in a pop-up (W4)");
});

test("the board prompt becomes a question with the options, then the skip, Cancel and Confirm", () => {
  const q = fromPrompt({ text: "Pick", kinds: [["floop", "Floop"]], modes: [["mode:0", "Heal"]], skip: "Don't heal", cancel: true, confirm: "Cast Corn Dog" }, cards["the-pig"]);
  assert.equal(q.surface, "bar");
  assert.equal(q.source.name, "The Pig");
  assert.deepEqual(q.options.map((o) => [o.label, o.role]), [["Floop", "choice"], ["Heal", "choice"]]);
  assert.equal(q.skip.label, "Don't heal");
  assert.equal(q.cancel.label, "Cancel");
  assert.equal(q.confirm.label, "Cast Corn Dog");
  assert.equal(fromPrompt({ text: "x", kinds: [], modes: [], skip: "", cancel: false, confirm: "" }, null).source, null, "no card, no source chip");
});

test("pressing an option does what its act says", () => {
  const { on, calls } = handlers();
  const q = fromPrompt({ text: "x", kinds: [["floop", "Floop"]], modes: [["mode:1", "Damage"]], skip: "Skip it", cancel: true, confirm: "Go" }, null);
  [...q.options, q.skip, q.cancel, q.confirm].forEach((o) => run(o.act, on));
  assert.deepEqual(calls, [["onKind", "floop"], ["onToken", "mode:1"], ["onSkip"], ["onCancel"], ["onConfirm"]]);
});

test("a choice of an amount is a short decision on the bar, one button for each number", () => {
  const choice = { kind: "amount", card: "cool-dog", player: 0, options: [1, 2] };
  const legal = [{ kind: "choose", card: 1 }, { kind: "choose", card: 2 }];
  const q = fromChoice(ctx, choice, legal, true);
  assert.equal(q.surface, "bar");
  assert.equal(q.source.name, "Cool Dog");
  assert.match(q.text, /1 and 2|1, 2|1 or 2/);
  assert.deepEqual(q.options.map((o) => o.label), ["1", "2"]);
  assert.deepEqual(q.options[1].act, { type: "action", action: legal[1] });
});

test("revealed cards are a pop-up of cards, each answering with its own action", () => {
  const choice = { kind: "reveal", card: "cool-dog", player: 0, revealed: [{ uid: 5, card: "the-pig", owner: 0 }, { uid: 6, card: "uni-knight", owner: 0 }] };
  const legal = [{ kind: "choose", card: 6 }, { kind: "choose", card: 5 }];
  const q = fromChoice(ctx, choice, legal, true);
  assert.equal(q.surface, "popup");
  assert.deepEqual(q.options.map((o) => o.label), ["The Pig", "Uni-Knight"]);
  assert.ok(q.options.every((o) => o.card));
  assert.equal(q.options[0].act.action.card, 5);
});

test("a choice that is not mine, is over, or is not on the shared question yet asks nothing here", () => {
  const choice = { kind: "amount", card: "cool-dog", player: 0, options: [1, 2] };
  assert.equal(fromChoice(ctx, choice, [], false), null);
  assert.equal(fromChoice({ ...ctx, view: { ...ctx.view, over: true } }, choice, [], true), null);
  assert.equal(fromChoice(ctx, { kind: "discard", card: "cool-dog" }, [], true), null, "still a hand-built pop-up (audit: 2b)");
  assert.equal(fromChoice(ctx, null, [], true), null);
});
