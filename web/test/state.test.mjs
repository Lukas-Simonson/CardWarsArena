import test from "node:test";
import assert from "node:assert/strict";
import { initialState, reduce, learnCards, setStatus, leave, isMyDecision, LOG_LIMIT } from "../js/state.js";
import { cards, view } from "./fixtures.mjs";

const joined = (over = {}) => ({ type: "joined", room: "ABCD", seat: 0, token: "tok", vs_bot: true, names: ["Finn", "Bot"], ...over });
const update = (seq, v = view(), events = []) => ({ type: "update", seq, view: v, events, names: ["Finn", "Bot"] });

test("joining records the session and names", () => {
  const s = reduce(initialState(), joined(), cards);
  assert.deepEqual(s.session, { room: "ABCD", token: "tok", seat: 0, vsBot: true });
  assert.deepEqual(s.names, ["Finn", "Bot"]);
});

test("avatars come with the names, and are kept when an update has none", () => {
  const pics = ["https://img.example/finn.png", ""];
  let s = reduce(initialState(), joined({ avatars: pics }), cards);
  assert.deepEqual(s.avatars, pics);
  s = reduce(s, update(1), cards); // an update without avatars (an older server) keeps what we had
  assert.deepEqual(s.avatars, pics);
  s = reduce(s, { ...update(2), avatars: ["", "https://img.example/bot.png"] }, cards);
  assert.deepEqual(s.avatars, ["", "https://img.example/bot.png"]);
});

test("a server that sends no avatars leaves them empty", () => {
  assert.deepEqual(reduce(initialState(), joined(), cards).avatars, []);
});

test("updates replace the view and log what happened, naming cards from the view", () => {
  let s = reduce(initialState(), joined(), cards);
  s = reduce(s, update(1, view(), [{ kind: "play", player: 0, card: 20 }, { kind: "turn_start", player: 1 }]), cards);
  assert.equal(s.seq, 1);
  assert.deepEqual(s.log, ["Finn plays The Pig", "── Turn 4: Bot's turn ──"]);
  // The view's turn is the last turn_start's, so two in one update count back from it.
  const two = reduce(initialState(), update(1, view({ turn: 6 }), [{ kind: "turn_start", player: 0 }, { kind: "draw", player: 0 }, { kind: "turn_start", player: 1 }]), cards);
  assert.deepEqual(two.log.filter((l) => l.startsWith("──")), ["── Turn 5: Finn's turn ──", "── Turn 6: Bot's turn ──"]);
  assert.equal(s.waiting, false);
});

test("an older update arriving late is ignored; the same seq (a rejoin) is accepted", () => {
  let s = reduce(initialState(), joined(), cards);
  s = reduce(s, update(5, view({ turn: 5 })), cards);
  const stale = reduce(s, update(4, view({ turn: 4 })), cards);
  assert.equal(stale.view.turn, 5);
  const again = reduce(s, update(5, view({ turn: 5, actions_left: 0 })), cards);
  assert.equal(again.view.actions_left, 0);
});

test("cards that leave the visible game can still be named later", () => {
  let s = reduce(initialState(), joined(), cards);
  s = reduce(s, update(1), cards);
  // The Pig (uid 20) disappears from the next view but an event refers to it.
  const gone = view();
  gone.players[0].lanes[0].creature = null;
  s = reduce(s, update(2, gone, [{ kind: "bounce", player: 1, card: 20 }]), cards);
  assert.equal(s.log.at(-1), "The Pig returns to Bot's hand");
});

test("a card that only ever appears in events is named from card_id, now and later", () => {
  let s = reduce(initialState(), joined(), cards);
  // Legion of Earlings enters play and bounces itself: it is in no view.
  s = reduce(s, update(1, view(), [{ kind: "play", player: 1, card: 777, card_id: "cool-dog" }, { kind: "bounce", player: 1, card: 777, card_id: "cool-dog" }]), cards);
  assert.deepEqual(s.log, ["Bot plays Cool Dog", "Cool Dog returns to Bot's hand"]);
  assert.equal(s.known[777], "cool-dog");
  s = reduce(s, update(2, view(), [{ kind: "play", player: 1, card: 777 }]), cards); // an old event without card_id still resolves
  assert.equal(s.log.at(-1), "Bot plays Cool Dog");
});

test("the log is capped", () => {
  let s = reduce(initialState(), joined(), cards);
  const events = Array.from({ length: LOG_LIMIT + 50 }, () => ({ kind: "actions", player: 0, amount: 1 }));
  s = reduce(s, update(1, view(), events), cards);
  assert.equal(s.log.length, LOG_LIMIT);
});

test("errors are kept until the next update", () => {
  let s = reduce(initialState(), joined(), cards);
  s = reduce(s, { type: "error", code: "stale_state", message: "the game has moved on" }, cards);
  assert.equal(s.error, "stale_state: the game has moved on");
  s = reduce(s, update(1), cards);
  assert.equal(s.error, "");
});

test("waiting and opponent presence", () => {
  let s = reduce(initialState(), joined({ vs_bot: false }), cards);
  s = reduce(s, { type: "waiting", room: "ABCD" }, cards);
  assert.equal(s.waiting, true);
  s = reduce(s, { type: "opponent", connected: true, name: "Ann" }, cards);
  assert.deepEqual(s.opponent, { connected: true, name: "Ann" });
  s = reduce(s, update(1), cards);
  assert.equal(s.waiting, false);
});

test("room_closed clears the room and says why", () => {
  let s = reduce(initialState(), joined(), cards);
  s = reduce(s, update(1), cards);
  s = reduce(s, { type: "room_closed", message: "opponent left" }, cards);
  assert.equal(s.session, null);
  assert.equal(s.view, null);
  assert.equal(s.closedReason, "opponent left");
});

test("joining a different room starts fresh; rejoining the same room keeps the log", () => {
  let s = reduce(initialState(), joined(), cards);
  s = reduce(s, update(1, view(), [{ kind: "play", player: 0, card: 20 }]), cards);
  const same = reduce(s, joined(), cards);
  assert.equal(same.log.length, 1);
  const other = reduce(s, joined({ room: "WXYZ" }), cards);
  assert.equal(other.log.length, 0);
  assert.equal(other.view, null);
});

test("unknown message types are ignored", () => {
  const s = initialState();
  assert.equal(reduce(s, { type: "future_thing" }, cards), s);
  assert.equal(reduce(s, { type: "hello", protocol: 1 }, cards), s);
});

test("status and leave", () => {
  assert.equal(setStatus(initialState(), "open").status, "open");
  let s = reduce(initialState(), joined(), cards);
  s = reduce(s, update(1), cards);
  const l = leave(s);
  assert.equal(l.session, null);
  assert.equal(l.seq, 0);
});

test("learnCards sees hands, discards, lanes and the pending card", () => {
  const v = view({ pending: { uid: 99, card: "field-reaper", owner: 1 } });
  const known = learnCards({}, v);
  assert.equal(known[10], "cool-dog");
  assert.equal(known[13], "uni-knight");
  assert.equal(known[20], "the-pig");
  assert.equal(known[21], "blood-fortress");
  assert.equal(known[99], "field-reaper");
});

test("learnCards names a Hero, so the log can say whose ability was used", () => {
  const v = view();
  v.players[0].hero = { uid: 2, card: "the-pig", owner: 0 };
  assert.equal(learnCards({}, v)[2], "the-pig");
});

test("isMyDecision needs awaiting and legal actions", () => {
  const withLegal = reduce(reduce(initialState(), joined(), cards), update(1, view({ legal: [{ kind: "draw" }] })), cards);
  assert.equal(isMyDecision(withLegal), true);
  const noLegal = reduce(reduce(initialState(), joined(), cards), update(1, view()), cards);
  assert.equal(isMyDecision(noLegal), false);
  assert.equal(isMyDecision(initialState()), false);
});
