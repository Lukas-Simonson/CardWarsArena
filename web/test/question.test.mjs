import { test } from "node:test";
import assert from "node:assert/strict";
import { ROLE_CLASS, SURFACE, run, fromPrompt, fromQuestion, fromMulligan } from "../js/question.js";
import { words } from "../js/describe.js";
import { view, cards } from "./fixtures.mjs";

const ctx = { view: view(), cards, names: ["Finn", "Jake"] };
const handlers = () => { const calls = []; const on = new Proxy({}, { get: (_, k) => (...a) => calls.push([k, ...a]) }); return { on, calls }; };

test("every role has one button class, and none is shared", () => {
  assert.deepEqual(Object.keys(ROLE_CLASS).sort(), ["choice", "commit", "danger", "neutral", "special"]);
  assert.equal(new Set(Object.values(ROLE_CLASS)).size, 5);
  assert.deepEqual(Object.values(SURFACE).sort(), ["bar", "popup"]);
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

// ---- the server's question: the shapes below are what the engine sends (engine/question_test.go)

const msg = (key, params) => ({ key, ...(params && { params }) });
const ask = (over) => ({ player: 0, card: "cool-dog", text: msg("question.x"), answer: "buttons", options: [], ...over });
const choose = (id) => ({ type: "action", action: { kind: "choose", option: id } });

test("a question is answered with its option's id and nothing else", () => {
  const q = fromQuestion(ctx, ask({ options: [{ id: 0, label: msg("question.amount_n", { n: 1, unit: "damage" }) }, { id: 1, label: msg("question.amount_n", { n: 2, unit: "damage" }) }] }), true);
  assert.deepEqual(q.options[1].act, choose(1));
  const { on, calls } = handlers();
  run(q.options[1].act, on);
  assert.deepEqual(calls, [["onAction", { kind: "choose", option: 1 }]]);
});

test("a question that is not mine, is over, or does not exist asks nothing", () => {
  assert.equal(fromQuestion(ctx, ask(), false), null);
  assert.equal(fromQuestion(ctx, ask({ player: 1 }), true), null, "the other player's");
  assert.equal(fromQuestion({ ...ctx, view: { ...ctx.view, over: true } }, ask(), true), null);
  assert.equal(fromQuestion(ctx, null, true), null);
});

test("an option is sorted by what it has: a card of the hand, a board object, a card to pick, or a button", () => {
  const q = fromQuestion(ctx, ask({
    options: [
      { id: 0, ref: { kind: "hand_card", player: 0, card: 10 } },
      { id: 1, ref: { kind: "landscape", player: 1, lane: 2 } },
      { id: 2, ref: { kind: "creature", player: 0, lane: 0 } },
      { id: 3, ref: { kind: "discard_card", player: 1, card: 77 } },
      { id: 4, card: { uid: 5, card: "the-pig", owner: 0 } },
      { id: 5, label: msg("question.take_it"), role: "danger" },
    ],
    skip: { id: 6, label: { text: "Don't freeze a Landscape" } },
  }), true);
  assert.deepEqual(Object.keys(q.handPicks), ["10"]);
  assert.deepEqual(q.handPicks[10], choose(0));
  assert.deepEqual(Object.keys(q.boardPicks).sort(), ["creature:0:0", "discard_card:1:77", "landscape:1:2"]);
  assert.deepEqual(q.boardPicks["landscape:1:2"], choose(1));
  assert.equal(q.options.length, 2, "a card to pick and a button");
  assert.deepEqual([q.options[0].label, q.options[0].card.name], ["The Pig", "The Pig"]);
  assert.deepEqual([q.options[1].label, q.options[1].role], ["Take Cool Dog's effect", "danger"]);
  assert.deepEqual([q.skip.label, q.skip.act], ["Don't freeze a Landscape", choose(6)]);
});

test("the answer says where a question is shown: a pop-up only for cards that are not on the table", () => {
  assert.equal(fromQuestion(ctx, ask({ answer: "cards" }), true).surface, "popup");
  for (const answer of ["board", "hand", "buttons"]) assert.equal(fromQuestion(ctx, ask({ answer }), true).surface, "bar", answer);
});

test("the card that asked, and a second card it is about, are shown", () => {
  const q = fromQuestion(ctx, ask({ subject: { uid: 5, card: "the-pig", owner: 0 } }), true);
  assert.equal(q.source.name, "Cool Dog");
  assert.equal(q.subject.name, "The Pig");
  assert.equal(fromQuestion(ctx, ask(), true).subject, null);
});

// ---- the words

test("a message is a catalogue key with parameters, and the cards in it are named", () => {
  const q = ask({ subject: { uid: 5, card: "the-pig", owner: 0 } });
  const w = (key, params) => words(ctx, msg(key, params), q);
  assert.equal(w("question.discard", { n: 1 }), "Discard 1 card from your hand: click a card.");
  assert.equal(w("question.discard", { n: 2 }), "Discard 2 cards from your hand: click a card.");
  assert.equal(w("question.camel"), "The Pig is leaving play. Put it underneath Cool Dog instead? It stays there until Cool Dog leaves play.");
  assert.equal(w("question.upkeep", { n: 2 }), "Click 2 cards in your hand to discard, or destroy Cool Dog.");
  assert.equal(w("question.landscape", { lane: 2 }), "Choose the type your lane 2 Landscape becomes until the start of your next turn.");
  assert.equal(w("question.amount", { options: [1, 2] }), "Choose how much (1 and 2).");
  assert.equal(w("question.amount_n", { n: 2, unit: "damage" }), "2 damage");
  assert.equal(w("question.amount_n", { n: 2, unit: "heal" }), "Heal 2");
  assert.equal(w("landscape.lava_flats"), "Lava Flats");
  // a parameter named x_card is a card id, worded as that card's name
  assert.equal(w("question.move_damage", { from_card: "the-pig", to_card: "uni-knight" }), "Move how much damage from The Pig to Uni-Knight?");
  assert.equal(w("question.move_result", { from_card: "the-pig", a1: 5, a2: 3, to_card: "uni-knight", b1: 1, b2: 3 }), "The Pig 5 → 3, Uni-Knight 1 → 3");
});

test("a target is worded from its parameters: the label, what it is, and whether it is optional", () => {
  const w = (params) => words(ctx, msg("question.target", params), ask());
  assert.equal(w({ label: "Freeze", optional: true, what: "landscape" }), "Freeze: Choose a Landscape for Cool Dog (optional)");
  assert.equal(w({ optional: false, what: "creature" }), "Choose a Creature for Cool Dog");
  assert.equal(w({ optional: true, what: "discard_card" }), "Choose a card from a discard pile for Cool Dog (optional)");
  assert.equal(w({ optional: false, what: "target" }), "Choose a target for Cool Dog");
});

test("a card's own label is looked up as a label, and shown as written when it has no message", () => {
  assert.equal(words(ctx, { text: "Don't freeze a Landscape" }), "Don't freeze a Landscape");
  assert.equal(words(ctx, { text: "Something no card says" }), "Something no card says");
  assert.equal(words(ctx, { key: "skip.none" }), "Skip");
});

// ---- the mulligan is not a question from the engine

test("the mulligan is two buttons on the bar: Keep (commit) and Mulligan (a plain choice, not red)", () => {
  const v = view({ phase: "mulligan" });
  const legal = [{ kind: "keep_hand" }, { kind: "mulligan" }];
  const q = fromMulligan({ ...ctx, view: v }, legal, true);
  assert.deepEqual(q.options.map((o) => [o.label, o.role]), [["Keep", "commit"], ["Mulligan", "choice"]]);
  assert.equal(q.source, null, "nothing asks it");
  assert.equal(fromMulligan({ ...ctx, view: v }, [{ kind: "keep_hand" }], true).options.length, 1, "once used, only Keep");
  assert.equal(fromMulligan(ctx, legal, true), null, "outside the mulligan");
  assert.equal(fromMulligan({ ...ctx, view: v }, legal, false), null, "not mine to answer");
});
