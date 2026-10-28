// End-to-end check against a running server (not part of `npm test`, which
// needs no server):  node test/smoke.mjs [serverUrl] [games]
//
// Plays whole games against the bot over a real WebSocket, using the same
// net/state/describe modules as the browser, and fails if the client
// meets anything it cannot describe or name.
import { Connection, wsUrl, normaliseServer } from "../js/net.js";
import { initialState, reduce, setStatus } from "../js/state.js";
import { describeAction, isKnownAction } from "../js/describe.js";
import { ActionPicker, sources, isGeneral } from "../js/picker.js";

const server = normaliseServer(process.argv[2] ?? process.env.SERVER ?? "http://localhost:8080");
const games = Number(process.argv[3] ?? 5);

const cards = Object.fromEntries((await (await fetch(`${server}/api/cards`)).json()).map((c) => [c.id, c]));
const decks = (await (await fetch(`${server}/api/decks`)).json()).filter((d) => d.playable);
if (!decks.length) throw new Error("server reports no playable decks");

let seed = 1;
const rand = (n) => { seed = (seed * 1664525 + 1013904223) >>> 0; return seed % n; };

const problems = [];

/**
 * Choose like a person at the screen: click a card that can act, then keep
 * clicking whatever the picker offers until it settles on one action. Every
 * result must be one of the server's own legal actions, and the picker must
 * never leave the player stuck.
 */
function chooseByClicking(v, ctx, n) {
  const general = v.legal.filter(isGeneral);
  const src = sources(v.legal);
  const keys = Object.keys(src);
  const fallback = () => v.legal[rand(v.legal.length)];
  if (!keys.length || (general.length && rand(10) < 3)) return general.length ? general[rand(general.length)] : fallback();
  const key = keys[rand(keys.length)];
  const kinds = new ActionPicker(src[key], ctx).kinds();
  const picker = new ActionPicker(src[key], ctx, kinds.length > 1 ? kinds[rand(kinds.length)] : "");
  for (let i = 0; i < 30; i++) {
    const r = picker.result();
    if (r) { picked++; if (!v.legal.includes(r)) problems.push(`game ${n}: picker produced an action the server did not list`); return r; }
    const o = picker.options();
    if (o.tokens.length === 0 && !o.can_skip) {
      const c = picker.confirm();
      if (c) { picked++; return c; }
      problems.push(`game ${n}: picker dead end for ${key}: ${picker.consistent().length} left, e.g. ${JSON.stringify(picker.consistent().slice(0, 3))} after ${JSON.stringify([picker.done, picker.cur])}`);
      return fallback();
    }
    if (o.can_skip && (o.tokens.length === 0 || rand(3) === 0)) picker.skip();
    else picker.pick(o.tokens[rand(o.tokens.length)]);
  }
  problems.push(`game ${n}: picker did not settle for ${key}`);
  return fallback();
}
let actionsSeen = 0, described = 0, logLines = 0, picked = 0;

async function playOne(n) {
  let state = initialState();
  let finished, failed;
  const done = new Promise((res, rej) => { finished = res; failed = rej; });
  // Choose and send an action if it is our decision. Called on every update,
  // and again after a rate_limited error: that message was dropped, no update
  // follows, so the client itself has to try again.
  const act = () => {
    const v = state.view;
    if (!v || v.over || !(v.awaiting.includes(v.you) && v.legal.length)) return;
    const a = chooseByClicking(v, { view: v, cards, names: state.names, known: state.known }, n);
    conn.send({ type: "action", action: a, seq: state.seq });
  };
  const conn = new Connection({
    url: wsUrl(server),
    onStatus: (s) => { state = setStatus(state, s); if (s === "open") conn.send({ type: "create", name: `smoke${n}`, deck: `${decks[n % decks.length].set} ${decks[n % decks.length].name}`, vs_bot: true }); },
    onMessage(msg) {
      const before = state.log.length;
      state = reduce(state, msg, cards);
      for (const line of state.log.slice(before)) {
        logLines++;
        if (/card #\d+/.test(line)) problems.push(`game ${n}: unresolved card name in log: ${line}`);
      }
      if (msg.type === "error" && msg.code === "rate_limited") setTimeout(act, 150);
      else if (msg.type === "error" && msg.code !== "stale_state") problems.push(`game ${n}: server error ${msg.code}: ${msg.message}`);
      if (msg.type !== "update") return;
      const v = state.view;
      if (v.over) return finished(v);
      if (!(v.awaiting.includes(v.you) && v.legal.length)) return;
      const ctx = { view: v, cards, names: state.names, known: state.known };
      for (const a of v.legal) {
        actionsSeen++;
        if (isKnownAction(ctx, a)) described++;
        else problems.push(`game ${n}: cannot describe ${JSON.stringify(a)}`);
      }
      act();
    },
  });
  conn.open();
  const timeout = setTimeout(() => {
    const v = state.view;
    failed(new Error(`game ${n} timed out; last seq ${state.seq}, ` + (v ? `phase ${v.phase}, active ${v.active}, awaiting [${v.awaiting}], legal ${v.legal.length}, pending ${!!v.pending}, error "${state.error}"` : "no view")));
  }, 15_000);
  const view = await done;
  clearTimeout(timeout);
  conn.close();
  return view;
}

for (let n = 0; n < games; n++) {
  const v = await playOne(n);
  console.log(`game ${n + 1}/${games}: over after turn ${v.turn}, winner seat ${v.winner}`);
}
console.log(`${actionsSeen} legal actions offered, ${described} described, ${picked} chosen by clicking, ${logLines} log lines`);
if (problems.length) {
  console.error(`\n${problems.length} problem(s):`);
  for (const p of [...new Set(problems)].slice(0, 20)) console.error(" - " + p);
  process.exit(1);
}
console.log("ok");
