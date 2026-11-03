import test from "node:test";
import assert from "node:assert/strict";
import { Connection, wsUrl, normaliseServer } from "../js/net.js";

class FakeSocket {
  static all = [];
  constructor(url) {
    this.url = url;
    this.readyState = 0;
    this.sent = [];
    FakeSocket.all.push(this);
  }
  send(s) { this.sent.push(JSON.parse(s)); }
  close() { this.readyState = 3; }
  // test controls
  accept() { this.readyState = 1; this.onopen?.(); }
  receive(data) { this.onmessage?.({ data: typeof data === "string" ? data : JSON.stringify(data) }); }
  drop() { this.readyState = 3; this.onclose?.(); }
}

function fakeTimers() {
  const t = { pending: [], setTimeout(fn, ms) { const h = { fn, ms }; t.pending.push(h); return h; }, clearTimeout(h) { t.pending = t.pending.filter((x) => x !== h); } };
  return t;
}

function make(extra = {}) {
  FakeSocket.all = [];
  const msgs = [], statuses = [], timers = fakeTimers();
  const c = new Connection({ url: "ws://x/ws", onMessage: (m) => msgs.push(m), onStatus: (s) => statuses.push(s), WebSocketImpl: FakeSocket, timers, ...extra });
  return { c, msgs, statuses, timers, sock: () => FakeSocket.all.at(-1) };
}

test("opens, reports status, and delivers parsed messages", () => {
  const { c, msgs, statuses, sock } = make();
  c.open();
  assert.deepEqual(statuses, ["connecting"]);
  sock().accept();
  assert.deepEqual(statuses, ["connecting", "open"]);
  sock().receive({ type: "hello", protocol: 1 });
  assert.deepEqual(msgs, [{ type: "hello", protocol: 1 }]);
});

test("send serialises JSON, and refuses when not open", () => {
  const { c, sock } = make();
  assert.equal(c.send({ type: "ping" }), false);
  c.open();
  assert.equal(c.send({ type: "ping" }), false); // still connecting
  sock().accept();
  assert.equal(c.send({ type: "ping" }), true);
  assert.deepEqual(sock().sent, [{ type: "ping" }]);
});

test("garbage from the server is ignored", () => {
  const { c, msgs, sock } = make();
  c.open();
  sock().accept();
  sock().receive("not json {");
  assert.deepEqual(msgs, []);
});

test("reconnects with growing delays and resets after a success", () => {
  const { c, statuses, timers, sock } = make({ backoff: [100, 200, 400] });
  c.open();
  sock().accept();
  sock().drop();
  assert.equal(statuses.at(-1), "reconnecting");
  assert.equal(timers.pending[0].ms, 100);
  timers.pending.shift().fn();
  sock().drop(); // second attempt fails before opening
  assert.equal(timers.pending[0].ms, 200);
  timers.pending.shift().fn();
  sock().drop();
  assert.equal(timers.pending[0].ms, 400);
  timers.pending.shift().fn();
  sock().drop();
  assert.equal(timers.pending[0].ms, 400, "the delay stops growing");
  timers.pending.shift().fn();
  sock().accept();
  assert.equal(statuses.at(-1), "open");
  sock().drop();
  assert.equal(timers.pending[0].ms, 100, "a successful connection resets the delay");
});

test("close() stops reconnecting for good", () => {
  const { c, statuses, timers, sock } = make();
  c.open();
  sock().accept();
  c.close();
  assert.equal(statuses.at(-1), "closed");
  assert.equal(timers.pending.length, 0);
  assert.equal(c.send({ type: "ping" }), false);
});

test("close() while a reconnect is pending cancels it", () => {
  const { c, timers, sock } = make();
  c.open();
  sock().accept();
  sock().drop();
  assert.equal(timers.pending.length, 1);
  c.close();
  assert.equal(timers.pending.length, 0);
});

test("events from a replaced socket are ignored", () => {
  const { c, msgs, timers } = make();
  c.open();
  const first = FakeSocket.all[0];
  first.accept();
  first.drop();
  timers.pending.shift().fn();
  const second = FakeSocket.all[1];
  second.accept();
  first.receive({ type: "stale" }); // a late message from the dead socket
  first.drop();
  assert.deepEqual(msgs, []);
  assert.equal(timers.pending.length, 0, "the old socket closing must not schedule another reconnect");
});

test("ws urls", () => {
  assert.equal(wsUrl("http://localhost:8080"), "ws://localhost:8080/ws");
  assert.equal(wsUrl("https://game.example.com"), "wss://game.example.com/ws");
  assert.equal(wsUrl("http://h:1/anything?x=1"), "ws://h:1/ws");
});

test("server addresses are normalised", () => {
  assert.equal(normaliseServer("localhost:8080"), "http://localhost:8080");
  assert.equal(normaliseServer(" http://localhost:8080/ "), "http://localhost:8080");
  assert.equal(normaliseServer("wss://game.example.com/ws"), "https://game.example.com");
  assert.equal(normaliseServer(""), "");
});
