// Wires the pieces together: the lobby form, the connection, the state
// reducer and the renderer. Everything here is plumbing; the logic is in the
// other modules.
import { Connection, wsUrl, normaliseServer } from "./net.js";
import { initialState, reduce, setStatus, leave } from "./state.js";
import { renderGame, updateDetail } from "./render.js";
import { isMyDecision } from "./state.js";
import { ActionPicker, sources } from "./picker.js";
import { plan, applyEffects, paceFor, reduced, STAGE_MS, TURN_PAUSE_MS } from "./anim.js";
import { Pacer } from "./pacer.js";

const $ = (id) => document.getElementById(id);
const store = (k, v) => { try { v == null ? localStorage.removeItem(k) : localStorage.setItem(k, v); } catch {} };
const load = (k) => { try { return localStorage.getItem(k); } catch { return null; } };
const sstore = (k, v) => { try { v == null ? sessionStorage.removeItem(k) : sessionStorage.setItem(k, v); } catch {} };
const sload = (k) => { try { return sessionStorage.getItem(k); } catch { return null; } };

const params = new URLSearchParams(location.search);
let server = normaliseServer(params.get("server") || load("cw.server") || "http://localhost:8080");
let state = initialState();
let cards = {};
let decks = [];
let catalogError = "";
// What the player is in the middle of: the card they clicked (source), the
// picker that tracks their choices for it, and what the card on show is.
const ui = { source: "", picker: null, awaitingKind: false, inspect: null, discard: null };
let conn = null;
let staging = false; // a cast spell is on show; what it did is shown when it ends
let stageTimer = null;
let pendingFx = null; // what to animate on the next render: the step from one view to the next

// ---------- state changes

// Updates are shown one at a time, each given time to play out, so a bot that
// moves in quick succession can be followed. Rests are skipped when it is our
// turn to act: there is nothing to watch, and our own move should answer at once.
const turnChanges = (msg) => (msg.events ?? []).some((e) => e.kind === "turn_start");
const pacer = new Pacer({
  deliver: dispatch,
  holdFor: (msg) => (msg.view?.phase === "mulligan" ? 0 : paceFor(msg.events, cards)),
  // Our own moves answer at once; but the turn never changes while something is
  // still playing, and not until a short pause after it.
  skipWait: (msg) => isMyDecision(state) && !turnChanges(msg),
  gapFor: (msg) => (turnChanges(msg) && state.view?.phase !== "mulligan" ? TURN_PAUSE_MS : 0),
});

/** Messages from the server: updates wait their turn; everything else is immediate. */
function onServerMessage(msg) {
  if (msg.type === "update") return pacer.push(msg);
  if (msg.type === "joined" || msg.type === "room_closed") pacer.clear();
  dispatch(msg);
}

function dispatch(msg) {
  const before = state;
  state = reduce(state, msg, cards);
  if (msg.type === "update") resetSelection(); // the position changed: old clicks no longer apply
  if (msg.type === "update" && before.view && state.seq !== before.seq) pendingFx = { seq: state.seq, prev: before.view, events: msg.events ?? [] };
  if (msg.type === "joined") sstore("cw.session", JSON.stringify({ server, room: msg.room, token: msg.token }));
  if (msg.type === "room_closed") { sstore("cw.session", null); resetUi(); }
  if (msg.type === "error" && (msg.code === "bad_token" || msg.code === "room_not_found")) sstore("cw.session", null);
  if (state !== before) render();
}

function resetSelection() { ui.source = ""; ui.picker = null; ui.awaitingKind = false; }
function resetUi() { resetSelection(); ui.inspect = null; ui.discard = null; }

function send(msg) {
  if (!conn?.send(msg)) dispatch({ type: "error", code: "not_connected", message: "not connected to the server" });
}

// ---------- user intent

const ctxNow = () => ({ view: state.view, cards, names: state.names, known: state.known });
const sourcesNow = () => (isMyDecision(state) ? sources(state.view.legal) : {});

function sendAction(action) {
  if (staging) return;
  send({ type: "action", action, seq: state.seq });
  resetSelection();
  render();
}

/** Click a card that can act: start a picker over everything it can do (or one kind). */
function select(key, kind = "") {
  if (staging) return;
  const actions = sourcesNow()[key];
  if (!actions) return;
  ui.source = key;
  ui.picker = new ActionPicker(actions, ctxNow(), kind);
  ui.awaitingKind = kind === "" && ui.picker.kinds().length > 1;
  const only = ui.picker.candidates;
  // In the Fight phase a Creature can only attack: no need to ask twice.
  if (state.view.phase === "fight" && only.length === 1 && only[0].kind === "attack") return sendAction(only[0]);
  render();
}

/** Send the action if the clicks so far have settled on exactly one. */
function settle() {
  const action = ui.picker?.result();
  if (action) sendAction(action);
  else render();
}

const toggle = (key, kind) => (ui.source === key && !kind ? (resetSelection(), render()) : select(key, kind));

const on = {
  onHand(uid, cardId) { ui.inspect = cardId; toggle(`hand:${uid}`); },
  onLane(lane, source, cardId) { if (cardId) ui.inspect = cardId; toggle(`lane:${lane}:${source}`); },
  onFloop(lane, source) { select(`lane:${lane}:${source}`, "floop"); },
  onPending() { select("pending"); },
  onToken(token) { if (!staging && ui.picker?.pick(token)) settle(); },
  onKind(kind) { select(ui.source, kind); },
  onSkip() { if (!staging && ui.picker?.skip()) settle(); },
  onConfirm() { const a = ui.picker?.confirm(); if (a) sendAction(a); },
  onCancel() { resetSelection(); render(); },
  onAction(action) { sendAction(action); },
  onInspect(cardId, hover = false) {
    ui.inspect = cardId;
    if (hover) updateDetail($("game"), ctxNow(), cardId);
    else render();
  },
  onDiscard(seat) { ui.discard = ui.discard === seat ? null : seat; render(); },
  onLeave() {
    if (state.session) send({ type: "leave" });
    pacer.clear();
    clearTimeout(stageTimer); staging = false; pendingFx = null;
    state = leave(state);
    sstore("cw.session", null);
    resetUi();
    render();
  },
};

document.addEventListener("keydown", (e) => {
  if (e.key === "Escape" && (ui.picker || ui.discard != null)) { ui.discard = null; resetSelection(); render(); }
});

const form = () => ({ name: $("name").value, deck: $("deck").value });

$("vsbot").onclick = () => send({ type: "create", ...form(), vs_bot: true, bot_deck: $("botdeck").value || undefined });
$("applyServer").onclick = () => applyServer($("server").value);
$("server").onkeydown = (e) => e.key === "Enter" && applyServer($("server").value);
$("name").onchange = () => store("cw.name", $("name").value);
$("deck").onchange = () => store("cw.deck", $("deck").value);

// ---------- rendering

function render() {
  const inRoom = !!state.session;
  $("lobby").hidden = inRoom;
  $("game").hidden = !inRoom;
  const s = $("status");
  s.textContent = { open: "connected", connecting: "connecting…", reconnecting: "reconnecting…", closed: "disconnected" }[state.status] ?? state.status;
  s.className = "conn " + (state.status === "open" ? "ok" : state.status === "closed" ? "bad" : "warn");
  const banner = $("banner");
  const msg = state.error || catalogError || (!inRoom && state.closedReason ? `The room closed: ${state.closedReason}.` : "");
  banner.hidden = !msg;
  banner.textContent = msg;
  banner.className = "banner" + (!state.error && !catalogError ? " info" : "");
  $("lobby").querySelectorAll("#vsbot").forEach((b) => (b.disabled = state.status !== "open" || !decks.length));
  if (inRoom && !staging) {
    const fx = pendingFx?.seq === state.seq && state.view ? pendingFx : null;
    const effects = fx ? plan(fx.prev, state.view, fx.events, ctxNow()) : [];
    const casts = effects.filter((f) => f.t === "cast");
    if (casts.length && !fx.staged && !reduced()) {
      // A spell is shown first, over the position it was cast in; its effects
      // (damage, a death) play once it has been seen.
      const before = { ...state, view: { ...fx.prev, legal: [], awaiting: [] } };
      renderGame($("game"), { state: before, cards, ui: { ...ui, source: "", picker: null, awaitingKind: false, discard: null } }, on);
      applyEffects($("game"), casts, { ...ctxNow(), view: before.view });
      staging = fx.staged = true;
      stageTimer = setTimeout(() => { staging = false; render(); }, STAGE_MS);
      return;
    }
    renderGame($("game"), { state, cards, ui }, on);
    if (fx) applyEffects($("game"), effects.filter((f) => f.t !== "cast"), ctxNow());
    pendingFx = null; // later re-renders (a click, a hover) must not replay it
  }
}

// ---------- connecting

function fillDecks(list) {
  decks = list.filter((d) => d.playable);
  const mine = $("deck"), bots = $("botdeck");
  mine.replaceChildren();
  bots.replaceChildren(new Option("(the other starter deck)", ""));
  for (const d of decks) {
    const label = `${d.owner || d.name} — ${d.name}`;
    mine.append(new Option(label, `${d.set} ${d.name}`));
    bots.append(new Option(label, `${d.set} ${d.name}`));
  }
  mine.value = load("cw.deck") && [...mine.options].some((o) => o.value === load("cw.deck")) ? load("cw.deck") : mine.options[0]?.value ?? "";
}

async function loadCatalog() {
  catalogError = "";
  try {
    const [c, d] = await Promise.all([fetch(`${server}/api/cards`), fetch(`${server}/api/decks`)]);
    if (!c.ok || !d.ok) throw new Error(`HTTP ${c.status}/${d.status}`);
    cards = Object.fromEntries((await c.json()).map((x) => [x.id, x]));
    fillDecks(await d.json());
    $("lobbyhint").textContent = decks.length ? "" : "The server reports no playable decks.";
  } catch (err) {
    catalogError = `Cannot load cards from ${server} (${err.message}). Is the server running, and was it started with -origins ${location.host}?`;
  }
}

function connect() {
  conn?.close();
  conn = new Connection({
    url: wsUrl(server),
    onMessage: onServerMessage,
    onStatus(status) {
      state = setStatus(state, status);
      if (status === "open") rejoin();
      render();
    },
  });
  conn.open();
}

/** After (re)connecting, take our seat back if we had one. */
function rejoin() {
  let saved = null;
  try { saved = JSON.parse(sload("cw.session") ?? "null"); } catch {}
  if (saved && saved.server === server) conn.send({ type: "rejoin", room: saved.room, token: saved.token });
}

async function applyServer(text) {
  const s = normaliseServer(text);
  if (!s) return;
  server = s;
  store("cw.server", server);
  $("server").value = server;
  pacer.clear();
  state = initialState();
  resetUi();
  await loadCatalog();
  connect();
  render();
}

$("server").value = server;
$("name").value = load("cw.name") || "Player";
store("cw.server", server);
render();
await loadCatalog();
connect();
render();
