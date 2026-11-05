// Wires the pieces together: the lobby form, the connection, the state
// reducer and the renderer. Everything here is plumbing; the logic is in the
// other modules.
import { Connection, wsUrl, normaliseServer } from "./net.js";
import { initialState, reduce, setStatus, leave } from "./state.js";
import { renderGame, updateDetail } from "./render.js";
import { isMyDecision } from "./state.js";
import { ActionPicker, sources } from "./picker.js";
import { plan, applyEffects, paceFor, reduced, restOf, STAGE_MS, TURN_PAUSE_MS, bareTurnChange } from "./anim.js";
import { Pacer } from "./pacer.js";
import { preloadArt } from "./preload.js";
import { loginUrl, accountView, loginErrorText, withoutLoginError } from "./account.js";
import { shelf, pickDeck, inBoxOrder, boxColour, diamondColour } from "./decklist.js";
import { createBuilder } from "./builder.js";
import { h } from "./dom.js";
import { fillAvatar } from "./avatar.js";

const $ = (id) => document.getElementById(id);
const store = (k, v) => { try { v == null ? localStorage.removeItem(k) : localStorage.setItem(k, v); } catch {} };
const load = (k) => { try { return localStorage.getItem(k); } catch { return null; } };
const sstore = (k, v) => { try { v == null ? sessionStorage.removeItem(k) : sessionStorage.setItem(k, v); } catch {} };
const sload = (k) => { try { return sessionStorage.getItem(k); } catch { return null; } };

const params = new URLSearchParams(location.search);
// Served together with the game server (Docker, a reverse proxy) the server is the page's own
// origin; the dev static server on :5173 talks to a game server on :8080.
const defaultServer = location.port === "5173" ? "http://localhost:8080" : location.origin;
let server = normaliseServer(params.get("server") || load("cw.server") || defaultServer);
let state = initialState();
let cards = {};
let decks = [];
let catalogError = "";
let account = accountView(null, null);
let myDecks = []; // the signed-in player's saved decks
let selectedDeck = ""; // the deck on the shelf that will be played
// What the player is in the middle of: the card they clicked (source), the
// picker that tracks their choices for it, and what the card on show is.
const ui = { source: "", picker: null, awaitingKind: false, inspect: null, discard: null };
let conn = null;
let staging = false; // a cast spell is on show; what it did is shown when it ends
let stageTimer = null;
let modalSeq = -1;
let actSeq = -1; // the update whose Floop buttons and highlights have been shown
let pendingFx = null; // what to animate on the next render: the step from one view to the next

// ---------- state changes

// Updates are shown one at a time, each given time to play out, so a bot that
// moves in quick succession can be followed. Rests are skipped when it is our
// turn to act: there is nothing to watch, and our own move should answer at once.
const pacer = new Pacer({
  deliver: dispatch,
  holdFor: (msg) => (msg.view?.phase === "mulligan" ? 0 : paceFor(msg.events, cards)),
  // Our own moves answer at once; but the turn never changes while something is
  // still playing, and not until a short pause after it.
  skipWait: (msg) => isMyDecision(state) && !bareTurnChange(msg),
  gapFor: (msg) => (bareTurnChange(msg) && state.view?.phase !== "mulligan" ? TURN_PAUSE_MS : 0),
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
  if (msg.view) preloadArt(msg.view.preload, cards);
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
  onAbility(lane, source, index) { select(`lane:${lane}:${source}`, `activate:${index}`); },
  onHeroAbility(index) { select("hero", `activate:${index}`); },
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

const form = () => ({ name: $("name").value, deck: selectedDeck });

$("vsbot").onclick = () => send({ type: "create", ...form(), vs_bot: true, bot_deck: $("botdeck").value || undefined, bot: $("bot").value || undefined });
$("create").onclick = () => send({ type: "create", ...form(), vs_bot: false });
const joinRoom = () => { const room = $("code").value.trim().toUpperCase(); if (room) send({ type: "join", room, ...form() }); };
$("join").onclick = joinRoom;
$("code").addEventListener("keydown", (e) => { if (e.key === "Enter") joinRoom(); });
$("applyServer").onclick = () => applyServer($("server").value);
$("server").addEventListener("keydown", (e) => { if (e.key === "Enter") applyServer($("server").value); });
$("name").onchange = () => { if (!account.user) store("cw.name", $("name").value); showWho(); };
$("name").oninput = showWho;
$("build").onclick = openBuilder;
$("bot").onchange = () => store("cw.bot", $("bot").value);

// ---------- deck builder

const builder = createBuilder({
  root: $("builder"),
  server: () => server,
  cards: () => cards,
  starters: () => decks,
  onClose: () => render(),
  onSaved: (list) => { myDecks = list; renderShelf(); },
});

function openBuilder() { builder.open().then(render); render(); }

// ---------- account

/** Ask the server what sign-in it offers and who this player is. A server without it just shows nothing. */
async function loadAccount() {
  let config = null, me = null;
  try {
    const opts = { credentials: "include" };
    const c = await fetch(`${server}/api/auth/config`, opts);
    if (c.ok) {
      config = await c.json();
      const m = await fetch(`${server}/api/auth/me`, opts);
      if (m.ok) me = await m.json();
    }
  } catch {}
  account = accountView(config, me);
  myDecks = [];
  if (account.user) {
    try {
      const r = await fetch(`${server}/api/my/decks`, { credentials: "include" });
      if (r.ok) myDecks = await r.json();
    } catch {}
  }
  renderAccount();
  renderShelf();
}

function renderAccount() {
  const box = $("account"), name = $("name"), here = location.origin + location.pathname;
  box.hidden = !account.show;
  if (account.user) {
    box.replaceChildren(h("button", { type: "button", class: "btn-steel", onclick: signOut }, "Sign out"));
    name.value = account.user.name; // the server plays you under this name whatever is sent
    name.disabled = true;
  } else {
    box.replaceChildren(...account.providers.map((p) => h("a", { class: "btn-purple", href: loginUrl(server, p.name, here), title: `Sign in with ${p.label} to build and keep decks` }, `Sign in with ${p.label}`)));
    name.disabled = false;
    name.value = load("cw.name") || "Player";
  }
  const build = $("build");
  build.hidden = !account.show; // a server with no accounts has no decks to build
  build.disabled = !account.user;
  build.title = account.user ? "" : "Sign in to build and keep your own decks";
  showWho();
}

/** The player chip: the first letter of the name, and whether they are signed in. */
function showWho() {
  fillAvatar($("avatar"), $("name").value, account.user?.avatar);
  $("whosub").textContent = account.user ? "Signed in" : account.show ? "Guest: sign in to keep decks" : "Your name at the table";
}

async function signOut() {
  builder.close();
  try { await fetch(`${server}/api/auth/logout`, { method: "POST", credentials: "include" }); } catch {}
  sstore("cw.session", null);
  state = leave(state);
  await loadAccount();
  connect(); // who a connection is is settled when it opens
  render();
}

// ---------- rendering

function render() {
  const inRoom = !!state.session;
  if (inRoom && builder.isOpen()) builder.close(); // a game has started: the builder has nothing to say
  $("lobby").hidden = inRoom || builder.isOpen();
  $("game").hidden = !inRoom;
  const s = $("status");
  s.textContent = { open: "connected", connecting: "connecting…", reconnecting: "reconnecting…", closed: "disconnected" }[state.status] ?? state.status;
  s.className = "conn " + (state.status === "open" ? "ok" : state.status === "closed" ? "bad" : "warn");
  const banner = $("banner");
  const gone = inRoom && state.view && state.opponent.connected === false ? "Your opponent has disconnected. The game waits for them to come back." : "";
  const msg = state.error || catalogError || gone || (!inRoom && state.closedReason ? `The room closed: ${state.closedReason}.` : "");
  banner.hidden = !msg;
  banner.textContent = msg;
  banner.className = "banner" + (!state.error && !catalogError ? " info" : "");
  for (const id of ["vsbot", "create", "join"]) $(id).disabled = state.status !== "open" || !decks.length;
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
    // A dialog waits for the motion of this update to finish; a re-render of the same update does not replay it.
    const game = $("game");
    const rest = restOf(effects.filter((f) => f.t !== "cast"));
    game.style.setProperty("--modal-delay", `${Math.max(350, rest + 150)}ms`);
    // What can be done now (buttons, highlights) waits for the motion too, but not on a re-render of the same update.
    game.style.setProperty("--act-delay", `${rest}ms`);
    game.classList.toggle("acts-again", actSeq === state.seq);
    actSeq = state.seq;
    renderGame(game, { state, cards, ui: { ...ui, modalAgain: modalSeq === state.seq } }, on);
    if (state.view?.choice) {
      modalSeq = state.seq;
      if (!game.querySelector(".modal")?.contains(document.activeElement)) game.querySelector(".modalopt")?.focus();
    }
    if (fx) applyEffects($("game"), effects.filter((f) => f.t !== "cast"), ctxNow());
    pendingFx = null; // later re-renders (a click, a hover) must not replay it
  }
}

// ---------- connecting

function fillDecks(list) {
  decks = inBoxOrder(list.filter((d) => d.playable));
  const bots = $("botdeck");
  bots.replaceChildren(new Option("(the other starter deck)", ""));
  for (const d of decks) bots.append(new Option(`${d.owner || d.name} — ${d.name}`, `${d.set} ${d.name}`));
  renderShelf();
}

/** "Pick your deck": a box for each starter deck, then the player's own when signed in. The chosen one is lifted. */
function renderShelf() {
  const boxes = shelf(decks, myDecks, cards);
  selectedDeck = pickDeck(boxes, selectedDeck || load("cw.deck"));
  const box = (b) => h("button", {
    type: "button", class: "deckbox", disabled: b.disabled, title: b.title || null,
    "aria-pressed": b.value === selectedDeck ? "true" : "false", style: `background:${boxColour(b.lands[0])}`,
    onclick: () => { selectedDeck = b.value; store("cw.deck", b.value); renderShelf(); },
  },
    h("div", { class: "lid", "aria-hidden": "true" }),
    h("div", { class: "dbody" },
      h("div", { class: "lands", "aria-hidden": "true" }, b.lands.map((l) => h("i", { style: `background:${diamondColour(l)}` }))),
      h("div", { class: "parch plate" }, h("b", {}, b.name), h("span", {}, b.sub)),
      h("div", { class: "dfoot" }, h("span", {}, `${b.size} cards`), b.value === selectedDeck && h("span", { class: "picked" }, "Selected"))));
  $("shelf").replaceChildren(...[...boxes.starters, ...boxes.mine].map(box));
  $("lobbyhint").textContent = !decks.length ? "The server reports no playable decks." : account.show && !account.user ? "Sign in to build and keep your own decks." : "";
}

/** The kinds of bot the server offers (an older server without /api/bots keeps the plain "Random"). The Heuristic bot is the default. */
function fillBots({ default: first, bots }) {
  const sel = $("bot");
  sel.replaceChildren(...bots.map((name) => new Option(name[0].toUpperCase() + name.slice(1), name)));
  const saved = load("cw.bot");
  sel.value = bots.includes(saved) ? saved : bots.includes("heuristic") ? "heuristic" : first;
}

async function loadCatalog() {
  catalogError = "";
  try {
    const [c, d, b] = await Promise.all([fetch(`${server}/api/cards`), fetch(`${server}/api/decks`), fetch(`${server}/api/bots`).catch(() => null)]);
    if (!c.ok || !d.ok) throw new Error(`HTTP ${c.status}/${d.status}`);
    if (b?.ok) fillBots(await b.json());
    cards = Object.fromEntries((await c.json()).map((x) => [x.id, x]));
    fillDecks(await d.json());
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
  await loadAccount();
  connect();
  render();
}

$("server").value = server;
$("name").value = load("cw.name") || "Player";
showWho();
store("cw.server", server);
render();
await loadCatalog();
await loadAccount();
connect();
const loginError = loginErrorText(location.search);
if (loginError) {
  dispatch({ type: "error", code: "login_error", message: loginError });
  history.replaceState(null, "", withoutLoginError(location.href));
}
render();
