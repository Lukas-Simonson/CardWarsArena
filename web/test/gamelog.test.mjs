import { test } from "node:test";
import assert from "node:assert/strict";
import { gameLogText, logFileName } from "../js/gamelog.js";

const cards = { finn: { id: "finn", name: "Finn" }, jake: { id: "jake", name: "Jake" } };
const state = {
  log: ["Lukas draws a card", "── Bot's turn ──", "Bot plays Cool Dog", "*** Lukas wins! ***"],
  names: ["Lukas", "Bot"], session: { room: "B69K" },
  view: { turn: 7, you: 0, players: [{ hero: { card: "finn" } }, { hero: { card: "jake" } }] },
};

test("the log is whole, oldest first, with a header and the markers taken off", () => {
  assert.equal(gameLogText(state, cards), [
    "Card Wars Arena game log",
    "Room: B69K · Turn 7",
    "Players: Lukas (Hero: Finn), Bot (Hero: Jake)",
    "",
    "Lukas draws a card",
    "",
    "── Bot's turn ──",
    "Bot plays Cool Dog",
    "",
    "Lukas wins!",
    "",
  ].join("\n"));
});

test("it still reads with no view, no room or no log", () => {
  const t = gameLogText({ log: [], view: null, names: [], session: null }, cards);
  assert.equal(t, "Card Wars Arena game log\n\nNothing has happened yet.\n");
  const bare = gameLogText({ ...state, session: null, view: { turn: 2, you: 1, players: [{}, {}] }, names: [] }, cards);
  assert.match(bare, /Turn 2\nPlayers: Opponent, You\n/);
});

test("a log at the client's limit says the start is missing", () => {
  assert.match(gameLogText({ ...state, log: ["a", "b"] }, cards, 2), /start of the game is missing/);
  assert.doesNotMatch(gameLogText({ ...state, log: ["a"] }, cards, 2), /missing/);
});

test("file names", () => {
  assert.equal(logFileName("B69K"), "card-wars-arena-b69k-log.txt");
  assert.equal(logFileName(""), "card-wars-arena-game-log.txt");
});
