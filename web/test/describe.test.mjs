import test from "node:test";
import assert from "node:assert/strict";
import {
  describeAction, describeEvent, describeRef, isKnownAction, actionMatches, filterActions,
  handSelection, laneSelection, sameSelection, instName, landscapeName, } from "../js/describe.js";
import { ctxFor, view } from "./fixtures.mjs";

const ctx = ctxFor();

test("a Floop the Hero grants is told apart from the card's own", () => {
  const c = ctxFor(view());
  assert.equal(describeAction(c, { kind: "floop", lane: 0 }), "Floop The Pig (lane 1)");
  assert.equal(describeAction(c, { kind: "floop", lane: 0, index: 1, via: "hero" }), "Floop The Pig with your Hero's Floop (lane 1)");
});

test("simple actions", () => {
  assert.equal(describeAction(ctx, { kind: "keep_hand" }), "Keep your opening hand");
  assert.match(describeAction(ctx, { kind: "mulligan" }), /^Mulligan/);
  assert.equal(describeAction(ctx, { kind: "draw" }), "Spend 1 Action to draw a card");
  assert.equal(describeAction(ctx, { kind: "end_main" }), "Finish: go to the Fight phase");
  assert.match(describeAction(ctxFor(view({ turn: 1 })), { kind: "end_main" }), /no Fight on the first turn/);
});

test("playing a creature names the card, cost and lane (lane 0 is omitted by the server)", () => {
  assert.equal(describeAction(ctx, { kind: "play_card", card: 10, lane: 2 }), "Play Cool Dog (cost 2) into lane 3");
  assert.equal(describeAction(ctx, { kind: "play_card", card: 10 }), "Play Cool Dog (cost 2) into lane 1");
});

test("casting a spell lists its targets", () => {
  const a = { kind: "play_card", card: 11, targets: [[{ kind: "creature", player: 1, lane: 0 }]] };
  assert.equal(describeAction(ctx, a), "Cast Corn Scepter (cost 1)  [targets: Cool Dog (opp's, lane 1)]");
});

test("optional empty target slots read as none", () => {
  const a = { kind: "play_card", card: 10, lane: 1, targets: [[]] };
  assert.match(describeAction(ctx, a), /\[targets: none\]$/);
});

test("a null target slot (how Go encodes an empty list) is read as none", () => {
  const a = { kind: "play_card", card: 10, lane: 1, targets: [null] };
  assert.match(describeAction(ctx, a), /\[targets: none\]$/);
  assert.equal(isKnownAction(ctx, a), true);
});

test("additional costs and several slots", () => {
  const a = {
    kind: "play_card", card: 10, lane: 0,
    cost_targets: [[{ kind: "hand_card", player: 0, card: 12 }]],
    targets: [[{ kind: "creature", player: 1, lane: 0 }], [{ kind: "lane", player: 0, lane: 3 }]],
  };
  assert.equal(describeAction(ctx, a), "Play Cool Dog (cost 2) into lane 1  [discard: Blood Fortress]  [targets: Cool Dog (opp's, lane 1); lane 4 (your side)]");
});

test("fighting names both sides, or the Hero when the lane is empty", () => {
  assert.equal(describeAction(ctx, { kind: "attack" }), "Fight in lane 1: The Pig fights Cool Dog");
  assert.equal(describeAction(ctx, { kind: "attack", lane: 2 }), "Fight in lane 3: Uni-Knight attacks the opposing Hero directly");
});

test("Floop and activated abilities, for creatures and buildings", () => {
  const land = { kind: "landscape", player: 1, lane: 0 };
  assert.equal(describeAction(ctx, { kind: "floop", targets: [[land]] }), "Floop The Pig (lane 1)  [targets: Cornfield landscape (opp's, lane 1)]");
  assert.equal(describeAction(ctx, { kind: "activate", lane: 2, targets: [[{ kind: "creature", player: 1, lane: 2 }]] }), "Use Uni-Knight's ability (lane 3)  [targets: creature (opp's, lane 3)]");
  assert.equal(describeAction(ctx, { kind: "floop", lane: 1, source: "building" }), "Floop Blood Fortress (lane 2)");
});

test("a face-down landscape target says so", () => {
  assert.equal(describeRef(ctx, { kind: "landscape", player: 1, lane: 1 }), "face-down Cornfield landscape (opp's, lane 2)");
  assert.equal(describeRef(ctx, { kind: "player", player: 1 }), "your opponent");
  assert.equal(describeRef(ctx, { kind: "player", player: 0 }), "yourself");
});

test("playing the stolen card", () => {
  const v = view({ pending: { uid: 40, card: "cool-dog", owner: 1 } });
  assert.equal(describeAction(ctxFor(v), { kind: "play_pending", lane: 3 }), "Play Cool Dog for free into lane 4");
  const s = view({ pending: { uid: 41, card: "corn-scepter", owner: 1 } });
  assert.match(describeAction(ctxFor(s), { kind: "play_pending", targets: [[{ kind: "creature", player: 1, lane: 0 }]] }), /^Play Corn Scepter for free  \[targets/);
});

test("unknown actions are flagged, not hidden", () => {
  const a = { kind: "teleport_me" };
  assert.equal(isKnownAction(ctx, a), false);
  assert.match(describeAction(ctx, a), /^\(unknown action/);
  assert.equal(isKnownAction(ctx, { kind: "draw" }), true);
});

test("unknown cards fall back to the id or uid instead of crashing", () => {
  assert.equal(instName(ctx, 999), "card #999");
  assert.equal(instName({ ...ctx, known: { 5: "mystery" }, cards: {} }, 5), "mystery");
});

test("events", () => {
  const e = (kind, rest = {}) => describeEvent(ctx, { kind, player: 0, ...rest });
  assert.equal(e("turn_start", { player: 1 }), "── Jake's turn ──");
  assert.equal(e("turn_start", { player: 1, turn: 3 }), "── Turn 3: Jake's turn ──");
  assert.equal(e("draw", { card: 10 }), "Finn draws Cool Dog");
  assert.equal(e("draw", { player: 1 }), "Jake draws a card"); // hidden draw has no card
  assert.equal(e("play", { card: 20 }), "Finn plays The Pig");
  assert.equal(e("attack", { card: 20, lane: 1 }), "Finn fights with The Pig (lane 2)");
  assert.equal(e("damage", { card: 30, amount: 3 }), "Cool Dog takes 3 damage");
  assert.equal(e("damage", { card: 30, amount: 2, cause: "burn" }), "Cool Dog takes 2 damage from burning");
  assert.equal(e("hp_loss", { player: 1, amount: 2, cause: "burn" }), "Jake loses 2 HP from burning");
  assert.equal(e("hp_loss", { player: 1, amount: 4 }), "Jake loses 4 HP");
  assert.equal(e("game_over", { player: 1 }), "*** Jake wins! ***");
  assert.equal(e("move", { card: 20, lane: 3 }), "The Pig moves to lane 4");
  assert.equal(e("bounce", { card: 30, player: 1 }), "Cool Dog returns to Jake's hand");
  assert.equal(e("steal", { card: 30 }), "Finn steals Cool Dog from the opponent's hand");
  assert.equal(e("something_new"), "something_new");
});

test("events name a card by card_id even if it was never in a view", () => {
  const e = { kind: "play", player: 1, card: 999, card_id: "the-pig" };
  assert.equal(describeEvent(ctx, e), "Jake plays The Pig");
  assert.equal(describeEvent(ctx, { kind: "bounce", player: 1, card: 999, card_id: "cool-dog" }), "Cool Dog returns to Jake's hand");
});

test("every event kind the server can send has a description", () => {
  const kinds = ["mulligan", "keep_hand", "turn_start", "draw", "play", "replace", "attack", "damage", "hp_loss", "hp_gain", "heal", "die", "game_over", "move", "flip", "ready", "bounce", "destroy", "return", "deck_top", "actions", "floop", "activate", "discard", "steal", "abilities", "under", "retype"];
  for (const k of kinds) assert.notEqual(describeEvent(ctx, { kind: k, player: 0, card: 20, amount: 1 }), k, `${k} falls through to its raw name`);
});

test("landscape names", () => {
  assert.equal(landscapeName("blue_plains"), "Blue Plains");
  assert.equal(landscapeName("weird"), "weird");
});

test("selecting a hand card keeps only the plays of that card", () => {
  const actions = [
    { kind: "draw" },
    { kind: "play_card", card: 10, lane: 0 },
    { kind: "play_card", card: 10, lane: 1 },
    { kind: "play_card", card: 11 },
    { kind: "end_main" },
  ];
  assert.equal(filterActions(actions, handSelection(10)).length, 2);
  assert.equal(filterActions(actions, handSelection(11)).length, 1);
  assert.equal(filterActions(actions, null).length, 5);
});

test("selecting a lane keeps its Fight, Floop and abilities, matching the right card", () => {
  const actions = [
    { kind: "attack" },                                   // lane 0
    { kind: "attack", lane: 2 },
    { kind: "floop" },                                    // creature, lane 0
    { kind: "floop", lane: 1, source: "building" },
    { kind: "activate", lane: 2, index: 0 },
    { kind: "play_card", card: 10, lane: 0 },
  ];
  const sel = (lane, source) => filterActions(actions, laneSelection(lane, source)).map((a) => a.kind);
  assert.deepEqual(sel(0, "creature"), ["attack", "floop"]);
  assert.deepEqual(sel(1, "building"), ["floop"]);
  assert.deepEqual(sel(1, "creature"), []);
  assert.deepEqual(sel(2, "creature"), ["attack", "activate"]);
  assert.equal(actionMatches({ kind: "attack" }, laneSelection(0, "building")), false);
});

test("selections compare by value", () => {
  assert.ok(sameSelection(handSelection(1), handSelection(1)));
  assert.ok(!sameSelection(handSelection(1), handSelection(2)));
  assert.ok(!sameSelection(laneSelection(0, "creature"), laneSelection(0, "building")));
  assert.ok(!sameSelection(null, handSelection(1)));
});

// An answer is {choose, option: id}: it reads as the option it names, from the view's question.
test("an answer to a question reads as its option", async () => {
  const { describeAction, isKnownAction } = await import("../js/describe.js");
  const { view, ctxFor } = await import("./fixtures.mjs");
  const msg = (key, params) => ({ key, ...(params && { params }) });
  const ask = (q, extra = {}) => ctxFor(view({ question: { player: 0, card: "the-pig", answer: "buttons", text: msg("question.x"), options: [], ...q } }), extra);
  const answer = (id) => ({ kind: "choose", option: id });
  // a card to pick in a pop-up
  const reveal = ask({ answer: "cards", options: [{ id: 0, card: { uid: 40, card: "the-pig", owner: 0 } }, { id: 1, card: { uid: 41, card: "cool-dog", owner: 0 } }] });
  assert.equal(describeAction(reveal, answer(1)), "Cool Dog");
  // a card of the hand, and a board object
  const hand = ask({ answer: "hand", options: [{ id: 0, ref: { kind: "hand_card", player: 0, card: 10 } }], skip: { id: 1, label: msg("question.leave_it") } });
  assert.equal(describeAction(hand, answer(0)), "Cool Dog");
  assert.equal(describeAction(hand, answer(1)), "Don't return The Pig");
  const board = ask({ answer: "board", options: [{ id: 0, ref: { kind: "landscape", player: 1, lane: 2 } }], skip: { id: 1, label: { text: "Don't freeze a Landscape" } } });
  assert.match(describeAction(board, answer(0)), /landscape \(opp's, lane 3\)/);
  assert.equal(describeAction(board, answer(1)), "Don't freeze a Landscape");
  // a button, with or without what it does
  const amount = ask({ options: [{ id: 0, label: msg("question.amount_n", { n: 1, unit: "damage" }) }, { id: 1, label: msg("question.amount_n", { n: 2, unit: "damage" }) }] });
  assert.equal(describeAction(amount, answer(1)), "2 damage");
  const move = ask({ options: [{ id: 0, label: msg("question.move_n", { n: 2 }), result: msg("question.move_result", { from_card: "the-pig", a1: 4, a2: 2, to_card: "cool-dog", b1: 1, b2: 3 }) }] });
  assert.equal(describeAction(move, answer(0)), "Move 2 (The Pig 4 → 2, Cool Dog 1 → 3)");
  for (const c of [reveal, hand, board, amount, move]) assert.equal(isKnownAction(c, answer(0)), true);
  assert.equal(describeAction(amount, answer(9)), "Choose", "an option that is not in the question");
});

test("showing a card in hand reads in the log", () => {
  assert.match(describeEvent(ctx, { kind: "reveal", player: 0, card: 10 }), /reveals Cool Dog$/);
});

test("a Hero's ability is described by the Hero, and selects as the Hero", () => {
  const v = view();
  v.players[0].hero = { uid: 2, card: "the-pig", owner: 0 };
  const c = ctxFor(v);
  const a = { kind: "activate", source: "hero", index: 0, targets: [[{ kind: "creature", player: 1, lane: 0 }]] };
  assert.equal(describeAction(c, a), "Use The Pig's ability  [targets: Cool Dog (opp's, lane 1)]");
  assert.equal(isKnownAction(c, a), true);
  assert.equal(actionMatches(a, { type: "lane", lane: 0, source: "hero" }), true);
  assert.equal(actionMatches(a, laneSelection(0, "creature")), false);
});

test("freezing and thawing, and the new actions, read in plain words", () => {
  const ctx = ctxFor();
  assert.match(describeEvent(ctx, { kind: "freeze", player: 0, lane: 1 }), /lane 2 Landscape is frozen/);
  assert.match(describeEvent(ctx, { kind: "thaw", player: 0, lane: 1 }), /lane 2 Landscape thaws/);
  assert.match(describeAction(ctx, { kind: "thaw", lane: 3, card: 10 }), /remove the frozen token from your lane 4 Landscape/);
});
