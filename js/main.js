// Wires the pieces together: the lobby form, the connection, the state
// reducer and the renderer. Everything here is plumbing; the logic is in the
// other modules.
import { Connection, wsUrl, normaliseServer } from "./net.js";
import { initialState, reduce, setStatus, leave } from "./state.js";
import { renderGame } from "./render.js";
import { handSelection, laneSelection, sameSelection } from "./describe.js";

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
const ui = { selection: null, inspect: null, discard: null };
let conn = null;

// ---------- state changes

function dispatch(msg) {
  const before = state;
  state = reduce(state, msg, cards);
  if (msg.type === "update") ui.selection = null; // the position changed: old clicks no longer apply
  if (msg.type === "joined") sstore("cw.session", JSON.stringify({ server, room: msg.room, token: msg.token }));
  if (msg.type === "room_closed") { sstore("cw.session", null); resetUi(); }
  if (msg.type === "error" && (msg.code === "bad_token" || msg.code === "room_not_found")) sstore("cw.session", null);
  if (state !== before) render();
}

function resetUi() { ui.selection = null; ui.inspect = null; ui.discard = null; }

function send(msg) {
  if (!conn?.send(msg)) dispatch({ type: "error", code: "not_connected", message: "not connected to the server" });
}

// ---------- user intent

const on = {
  onHand(uid, cardId) {
    const sel = handSelection(uid);
    ui.selection = sameSelection(ui.selection, sel) ? null : sel;
    ui.inspect = cardId;
    render();
  },
  onLane(lane, source, cardId) {
    const sel = laneSelection(lane, source);
    ui.selection = sameSelection(ui.selection, sel) ? null : sel;
    ui.inspect = cardId;
    render();
  },
  onInspect(cardId) { ui.inspect = cardId; render(); },
  onDiscard(seat) { ui.discard = ui.discard === seat ? null : seat; render(); },
  onClearSelection() { ui.selection = null; render(); },
  onAction(action) { send({ type: "action", action, seq: state.seq }); },
  onLeave() {
    if (state.session) send({ type: "leave" });
    state = leave(state);
    sstore("cw.session", null);
    resetUi();
    render();
  },
};

const form = () => ({ name: $("name").value, deck: $("deck").value });

$("vsbot").onclick = () => send({ type: "create", ...form(), vs_bot: true, bot_deck: $("botdeck").value || undefined });
$("create").onclick = () => send({ type: "create", ...form() });
$("join").onclick = () => send({ type: "join", room: $("code").value.trim(), ...form() });
$("leave").onclick = on.onLeave;
$("applyServer").onclick = () => applyServer($("server").value);
$("server").onkeydown = (e) => e.key === "Enter" && applyServer($("server").value);
$("code").onkeydown = (e) => e.key === "Enter" && $("join").click();
$("name").onchange = () => store("cw.name", $("name").value);
$("deck").onchange = () => store("cw.deck", $("deck").value);

// ---------- rendering

function render() {
  const inRoom = !!state.session;
  $("lobby").hidden = inRoom;
  $("game").hidden = !inRoom;
  $("leave").hidden = !inRoom;
  const s = $("status");
  s.textContent = { open: "connected", connecting: "connecting…", reconnecting: "reconnecting…", closed: "disconnected" }[state.status] ?? state.status;
  s.className = "chip " + (state.status === "open" ? "ok" : state.status === "closed" ? "bad" : "warn");
  const badge = $("roombadge");
  badge.hidden = !inRoom;
  if (inRoom) badge.textContent = `Room ${state.session.room} · seat ${state.session.seat}${state.session.vsBot ? " · vs bot" : ""}`;

  const banner = $("banner");
  const msg = state.error || catalogError || (!inRoom && state.closedReason ? `The room closed: ${state.closedReason}.` : "");
  banner.hidden = !msg;
  banner.textContent = msg;
  banner.className = "banner" + (!state.error && !catalogError ? " info" : "");

  $("lobby").querySelectorAll("button:not(#applyServer)").forEach((b) => (b.disabled = state.status !== "open" || !decks.length));
  if (inRoom) renderGame($("game"), { state, cards, ui }, on);
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
    onMessage: dispatch,
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
