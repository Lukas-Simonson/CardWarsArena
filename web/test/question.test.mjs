import { test } from "node:test";
import assert from "node:assert/strict";
import { ROLE_CLASS, SURFACE, CHOICE_SURFACE, run, fromPrompt, fromChoice, fromMulligan } from "../js/question.js";
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
  assert.deepEqual(q.options.map((o) => o.label), ["1", "2"], "no unit, so the bare number");
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
  assert.equal(fromChoice(ctx, { kind: "redirect", card: "cool-dog" }, [], true), null, "still a hand-built pop-up (audit: 2d)");
  assert.equal(fromChoice(ctx, null, [], true), null);
});

// Cards from your own hand are a click on the hand (W2), with the sentence and any decline on the bar.
const handChoice = (kind, extra = {}) => ({ kind, card: "cool-dog", player: 0, ...extra });
const hand = [{ kind: "choose", card: 10 }, { kind: "choose", card: 11 }];

test("a discard is asked on the bar and answered by clicking the hand, and says how many are left", () => {
  const q = fromChoice(ctx, handChoice("discard", { count: 2 }), hand, true);
  assert.equal(q.surface, "bar");
  assert.deepEqual(Object.keys(q.handPicks).sort(), ["10", "11"]);
  assert.deepEqual(q.handPicks[11], { type: "action", action: hand[1] });
  assert.deepEqual(q.options, [], "nothing else to press: the cards are the answer");
  assert.match(q.text, /2 cards/);
  assert.match(fromChoice(ctx, handChoice("discard", { count: 1 }), hand, true).text, /1 card\b/, "and one fewer after each pick");
});

test("ransom and upkeep put the other answer in a red button, and rebound in a neutral one", () => {
  const none = { kind: "choose" };
  const ransom = fromChoice(ctx, handChoice("ransom", { min_cost: 2 }), [...hand, none], true);
  assert.equal(ransom.options.length, 1);
  assert.equal(ransom.options[0].role, "danger");
  assert.deepEqual(ransom.options[0].act, { type: "action", action: none });
  assert.match(ransom.text, /cost 2 or more/);
  assert.match(fromChoice(ctx, handChoice("ransom", { min_cost: 0 }), [...hand, none], true).text, /click a card in your hand/i);
  const upkeep = fromChoice(ctx, handChoice("upkeep", { count: 2 }), [...hand, none], true);
  assert.equal(upkeep.options[0].role, "danger");
  assert.match(upkeep.options[0].label, /Destroy Cool Dog/);
  assert.equal(ransom.options[0].label, "Take Cool Dog's effect");
  const rebound = fromChoice(ctx, handChoice("rebound"), [...hand, none], true);
  assert.deepEqual([rebound.options.length, rebound.skip.act.action], [0, none]);
  assert.equal(rebound.skip.label, "Don't return Cool Dog");
  assert.equal(Object.keys(rebound.handPicks).length, 2, "the cards are picks, the answer without a card is not");
});

test("with no answer that has no card, there is no decline button", () => {
  assert.equal(fromChoice(ctx, handChoice("ransom"), hand, true).options.length, 0);
  assert.equal(fromChoice(ctx, handChoice("rebound"), hand, true).skip, null);
});

test("the four hand choices are on the bar, never a pop-up (W2)", () => {
  for (const kind of ["discard", "ransom", "upkeep", "rebound"]) assert.equal(CHOICE_SURFACE[kind], "bar", kind);
});

// Short decisions on the bar (W3), and the cards of a search in a pop-up (W4).
test("an amount says what each number does when the engine says what it is of", () => {
  const legal = [{ kind: "choose", card: 1 }, { kind: "choose", card: 2 }];
  assert.deepEqual(fromChoice(ctx, { kind: "amount", card: "cool-dog", options: [1, 2], unit: "damage" }, legal, true).options.map((o) => o.label), ["1 damage", "2 damage"]);
  assert.deepEqual(fromChoice(ctx, { kind: "amount", card: "cool-dog", options: [1, 2], unit: "heal" }, legal, true).options.map((o) => o.label), ["Heal 1", "Heal 2"]);
});

test("moving damage asks how much between which Creatures, and shows the result of each amount", () => {
  const v = view();
  v.players[0].lanes[0].creature.damage = 5; // The Pig, lane 0
  v.players[0].lanes[2].creature.damage = 1; // Uni-Knight, lane 2
  const q = fromChoice({ ...ctx, view: v }, { kind: "move_damage", card: "cool-dog", lane: 0, other: 2, count: 3 }, [1, 2, 3].map((n) => ({ kind: "choose", card: n })), true);
  assert.equal(q.surface, "bar");
  assert.match(q.text, /from The Pig to Uni-Knight/);
  assert.deepEqual(q.options.map((o) => o.label), ["Move 1", "Move 2", "Move 3"]);
  assert.equal(q.options[1].result, "The Pig 5 → 3, Uni-Knight 1 → 3");
});

test("a Camel's question shows the leaving card, and the skip says what it does not do", () => {
  const legal = [{ kind: "choose", card: 22 }, { kind: "choose" }]; // 22 is the Uni-Knight in lane 2: the Camel
  const q = fromChoice(ctx, { kind: "camel", card: "uni-knight", revealed: [{ uid: 5, card: "the-pig", owner: 0 }], lanes: [2] }, legal, true);
  assert.equal(q.surface, "bar");
  assert.equal(q.subject.name, "The Pig");
  assert.equal(q.source.name, "Uni-Knight");
  assert.deepEqual(q.options.map((o) => o.label), ["Put it under Uni-Knight"]);
  assert.equal(q.skip.label, "Don't put it under Uni-Knight");
  assert.match(q.text, /The Pig is leaving play/);
});

test("a Landscape type is a row of buttons in the bar, named by the type", () => {
  const legal = [1, 2, 7].map((n) => ({ kind: "choose", card: n }));
  const q = fromChoice(ctx, { kind: "landscape", card: "cool-dog", lane: 1 }, legal, true);
  assert.equal(q.surface, "bar");
  assert.deepEqual(q.options.map((o) => o.label), ["Cornfield", "Blue Plains", "Lava Flats"]);
  assert.match(q.text, /lane 2/);
});

test("searching the deck shows the cards in a pop-up (they are not on the table)", () => {
  const q = fromChoice(ctx, { kind: "search", card: "cool-dog", revealed: [{ uid: 5, card: "blood-fortress", owner: 0 }] }, [{ kind: "choose", card: 5 }], true);
  assert.equal(q.surface, "popup");
  assert.equal(q.options[0].card.name, "Blood Fortress");
});

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

test("every choice kind on the bar or the pop-up is listed, so a new one has to be placed", () => {
  assert.deepEqual(Object.keys(CHOICE_SURFACE).sort(), ["amount", "camel", "discard", "landscape", "move_damage", "ransom", "rebound", "reveal", "search", "upkeep"]);
});
