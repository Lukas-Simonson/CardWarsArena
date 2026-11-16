import { test } from "node:test";
import assert from "node:assert/strict";
import { errorText, problemText, problemTexts, apiError, closedText } from "../js/errors.js";

const cards = { "cool-dog": { id: "cool-dog", name: "Cool Dog" } };

test("a rule that was broken is worded from its reason, whatever the server's English says", () => {
  assert.equal(errorText({ code: "illegal_action", reason: "not_your_turn", message: "ZZZ" }, cards), "It is not your turn.");
  assert.equal(errorText({ code: "illegal_action", reason: "wrong_phase" }, cards), "You can't do that in this phase.");
});

test("a code without a reason is worded from the code, and the params fill the message", () => {
  assert.equal(errorText({ code: "room_not_found" }, cards), "There is no such room. Check the code and try again.");
  assert.equal(errorText({ code: "deck_limit", params: { max: 50 } }, cards), "You can keep at most 50 decks: delete one first.");
  assert.equal(errorText({ code: "bad_request", reason: "unknown_message_type", params: { type: "fly" } }, cards), "The server does not know a message of type “fly”.");
});

test("an unknown reason falls back to the code, an unknown code to the server's English", () => {
  assert.equal(errorText({ code: "illegal_action", reason: "brand_new_rule", message: "the new rule says no" }, cards), "That move is not allowed: the new rule says no");
  assert.equal(errorText({ code: "from_the_future", message: "something happened" }, cards), "from_the_future: something happened");
  assert.equal(errorText({ code: "from_the_future" }, cards), "from_the_future");
  assert.equal(errorText(null, cards), "");
});

test("cards in a message are named from the catalogue, by id", () => {
  assert.equal(problemText({ code: "unmodelled", params: { card: "cool-dog" }, text: "Cool Dog's rules are not implemented yet" }, cards), "Cool Dog's rules are not implemented yet.");
  assert.equal(problemText({ code: "too_many_copies", params: { card: "cool-dog", max: 3 } }, cards), "More than 3 copies of Cool Dog.");
  assert.equal(problemText({ code: "unmodelled", params: { card: "unseen" } }, cards), "unseen's rules are not implemented yet.");
});

test("a deck that cannot be played lists its problems", () => {
  const err = { code: "deck_not_playable", params: { deck: "Mine", problems: [{ code: "choose_hero" }, { code: "deck_too_small", params: { n: 3, min: 40 } }] } };
  assert.equal(errorText(err, cards), "Mine cannot be played yet: Choose a Hero.; 3 cards: a deck needs at least 40.");
  assert.deepEqual(problemTexts([{ code: "choose_hero" }, "plain text", { code: "mystery", text: "English" }], cards), ["Choose a Hero.", "plain text", "English"]);
  assert.deepEqual(problemTexts(undefined), []);
});

test("an API error is told by its code", () => {
  assert.equal(apiError({ error: "sign in first" }), null);
  assert.deepEqual(apiError({ error: "sign in first", code: "sign_in_required" }), { code: "sign_in_required", params: undefined, message: "sign in first" });
  assert.equal(errorText(apiError({ error: "x", code: "sign_in_required" }), cards), "Sign in first.");
});

test("why a room closed is worded from its code", () => {
  assert.equal(closedText("opponent_left", "opponent left"), "The room closed: your opponent left.");
  assert.equal(closedText("", "something else"), "The room closed: something else.");
});
