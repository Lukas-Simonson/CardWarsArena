// Animations for the battle scene. The scene is rebuilt from the view on every
// update, so nothing can transition between renders; instead plan() compares
// the previous and the new view (plus the update's events) and lists what the
// player should see happen, and applyEffects() plays that on the fresh DOM.
//
// plan() is pure. applyEffects() uses the Web Animations API and touches only
// elements the renderer marks with data-seat / data-lane / data-uid.
import { cardEl } from "./card.js";

const MAX_HP = 25;
export const HIT_DELAY = 260;   // the blow lands this long after the lunge starts
export const DIE_DELAY = 420;   // a destroyed card lingers a moment first

/** The transform the renderer gives a Creature in each status (see .tile .at). */
const STATUS_XF = { ready: "none", flooped: "rotate(-90deg) scale(0.7)", fought: "rotate(90deg) scale(0.7)" };

/** uid -> {seat, lane, thing} for the Creatures (or Buildings) in a view. */
function index(view, key) {
  const out = new Map();
  view.players.forEach((p, seat) => p.lanes.forEach((l, lane) => l[key] && out.set(l[key].uid, { seat, lane, thing: l[key] })));
  return out;
}

/**
 * What to show for the step from `prev` to `view`.
 * @returns {object[]} effects, each { t, ... } (see applyEffects)
 */
export function plan(prev, view, events = [], ctx = {}) {
  if (!prev || !view) return []; // first view (joining or rejoining): nothing happened
  const fx = [];
  const me = view.you;

  const diedUids = new Set(events.filter((e) => e.kind === "die").map((e) => e.card));
  const cBefore = index(prev, "creature"), cAfter = index(view, "creature");
  for (const [uid, a] of cAfter) {
    const b = cBefore.get(uid);
    if (!b || b.seat !== a.seat || b.lane !== a.lane) fx.push({ t: "drop", seat: a.seat, lane: a.lane });
    else if (b.thing.status !== a.thing.status) fx.push({ t: "settle", seat: a.seat, lane: a.lane, from: STATUS_XF[b.thing.status] ?? "none" });
  }
  for (const [uid, b] of cBefore) {
    if (cAfter.has(uid)) continue;
    fx.push({ t: "ghost", seat: b.seat, lane: b.lane, card: b.thing.card, status: b.thing.status, delay: diedUids.has(uid) ? DIE_DELAY : 0 });
  }

  const bBefore = index(prev, "building"), bAfter = index(view, "building");
  for (const [uid, a] of bAfter) if (!bBefore.has(uid)) fx.push({ t: "build", seat: a.seat, lane: a.lane });

  const fought = events.some((e) => e.kind === "attack");
  for (const e of events) {
    switch (e.kind) {
      case "attack": fx.push({ t: "lunge", seat: e.player, lane: e.lane ?? 0, up: e.player === me }); break;
      case "damage": fx.push({ t: "hit", seat: e.player, lane: e.lane ?? 0, amount: e.amount, delay: fought ? HIT_DELAY : 0 }); break;
      case "hp_loss": fx.push({ t: "hp", seat: e.player, amount: e.amount, delay: fought ? HIT_DELAY : 0 }); break;
      case "turn_start": fx.push({ t: "turn", seat: e.player, mine: e.player === me }); break;
      case "play": if (ctx.cards?.[e.card_id]?.type === "spell") fx.push({ t: "cast", card: e.card_id, seat: e.player }); break;
    }
  }

  view.players.forEach((p, seat) => {
    if (p.hp !== prev.players[seat].hp) fx.push({ t: "hpbar", seat, from: pct(prev.players[seat].hp), delay: fought ? HIT_DELAY : 0 });
  });

  const hadHand = new Set((prev.players[me].hand ?? []).map((c) => c.uid));
  (view.players[me].hand ?? []).filter((c) => !hadHand.has(c.uid)).forEach((c, i) => fx.push({ t: "deal", uid: c.uid, delay: i * 120 }));
  if (view.players[1 - me].hand_count > prev.players[1 - me].hand_count) fx.push({ t: "oppdraw" });
  return fx;
}

/**
 * How long the scene should rest on an update before showing the next, so the
 * motion above can finish and the player can see what happened.
 */
export function paceFor(events = []) {
  const kinds = new Set(events.map((e) => e.kind));
  if (kinds.has("turn_start") || kinds.has("attack") || kinds.has("die")) return 1400;
  if (kinds.has("play") || kinds.has("floop") || kinds.has("activate") || kinds.has("damage") || kinds.has("destroy")) return 1100;
  if (kinds.has("draw")) return 650;
  return 800;
}

const pct = (hp) => Math.max(0, Math.min(100, Math.round((hp / MAX_HP) * 100)));

// ---------- playing them

const reduced = () => globalThis.matchMedia?.("(prefers-reduced-motion: reduce)").matches;

/** Play the effects on the freshly rendered scene in `root`. */
export function applyEffects(root, fx, ctx) {
  if (!fx.length || reduced()) return;
  const q = (sel) => root.querySelector(sel);
  const tile = (seat, lane) => q(`.tile[data-seat="${seat}"][data-lane="${lane}"]`);
  const felt = q(".felt");
  const play = (el, frames, opts) => el?.animate?.(frames, { fill: "backwards", easing: "ease-out", composite: "add", ...opts });
  const spawn = (parent, el, frames, opts) => {
    if (!parent) return;
    parent.append(el);
    const a = el.animate(frames, { fill: "both", easing: "ease-out", ...opts });
    a.onfinish = () => el.remove();
  };
  const text = (cls, s) => Object.assign(document.createElement("em"), { className: cls, textContent: s });

  for (const f of fx) {
    switch (f.t) {
      case "drop":
        play(tile(f.seat, f.lane)?.querySelector(".card"), [{ transform: `translateY(${f.seat === ctx.view.you ? "50px" : "-50px"}) scale(1.6)`, opacity: 0 }, { opacity: 1, offset: 0.6 }, { transform: "none" }], { duration: 380, composite: "replace", easing: "cubic-bezier(.2,.9,.3,1.2)" });
        break;
      case "settle":
        play(tile(f.seat, f.lane)?.querySelector(".at"), [{ transform: `translate(-50%, -50%) ${f.from}` }], { duration: 320, composite: "replace" });
        break;
      case "build":
        play(q(`.strip[data-seat="${f.seat}"][data-lane="${f.lane}"] .row`), [{ transform: "scale(1.4)", opacity: 0 }, { transform: "none", opacity: 1 }], { duration: 320, composite: "replace" });
        break;
      case "lunge": {
        const dy = f.up ? -34 : 34;
        play(tile(f.seat, f.lane)?.querySelector(".card"), [{ transform: "none" }, { transform: `translateY(${dy}px) scale(1.1)`, offset: 0.4 }, { transform: `translateY(${dy}px) scale(1.1)`, offset: 0.55 }, { transform: "none" }], { duration: 520, easing: "ease-in-out" });
        break;
      }
      case "hit": {
        const t = tile(f.seat, f.lane);
        play(t?.querySelector(".card"), [{ transform: "none", filter: "none" }, { transform: "translateX(-7px) rotate(-3deg)", filter: "brightness(1.9) saturate(2)", offset: 0.2 }, { transform: "translateX(7px) rotate(3deg)", offset: 0.45 }, { transform: "translateX(-4px)", offset: 0.7 }, { transform: "none", filter: "none" }], { duration: 420, delay: f.delay });
        if (t && f.amount) spawn(t, text("fx-float", `-${f.amount}`), [{ opacity: 0, transform: "translate(-50%, 0) scale(.5)" }, { opacity: 1, transform: "translate(-50%, -12px) scale(1.2)", offset: 0.25 }, { opacity: 0, transform: "translate(-50%, -52px) scale(1)" }], { duration: 900, delay: f.delay });
        break;
      }
      case "hp": {
        const shield = q(`.shield[data-seat="${f.seat}"]`);
        play(shield, [{ transform: "none", filter: "none" }, { transform: "translateX(-6px) rotate(-6deg)", filter: "brightness(1.6) hue-rotate(-30deg)", offset: 0.25 }, { transform: "translateX(6px) rotate(6deg)", offset: 0.5 }, { transform: "none", filter: "none" }], { duration: 480, delay: f.delay });
        if (shield && f.amount) spawn(shield, text("fx-float side", `-${f.amount}`), [{ opacity: 0, transform: "translate(0, 0) scale(.5)" }, { opacity: 1, transform: "translate(0, -10px) scale(1.2)", offset: 0.25 }, { opacity: 0, transform: "translate(0, -44px) scale(1)" }], { duration: 1000, delay: f.delay });
        break;
      }
      case "hpbar":
        play(q(`.hpbar[data-seat="${f.seat}"] > div`), [{ width: `${f.from}%` }], { duration: 600, delay: f.delay, composite: "replace" });
        break;
      case "ghost": {
        const t = tile(f.seat, f.lane);
        if (!t || !ctx.cards?.[f.card]) break;
        const at = document.createElement("div");
        at.className = `at ghost ${f.status === "ready" ? "" : f.status}`;
        at.append(cardEl(ctx.cards[f.card]));
        spawn(t, at, [{ opacity: 1, transform: "none" }, { opacity: 1, transform: "scale(1.05) rotate(-4deg)", offset: 0.3 }, { opacity: 0, transform: "scale(.5) rotate(14deg)", filter: "grayscale(1)" }], { duration: 560, delay: f.delay, easing: "ease-in" });
        break;
      }
      case "deal":
        play(q(`.hbtn[data-uid="${f.uid}"]`), [{ transform: "translate(160px, -140px) scale(.4) rotate(25deg)", opacity: 0 }, { opacity: 1, offset: 0.5 }, { transform: "none" }], { duration: 420, delay: f.delay, composite: "replace" });
        break;
      case "oppdraw":
        play([...root.querySelectorAll(".back")].pop(), [{ transform: "translateY(40px) scale(.5)", opacity: 0 }, { transform: "none", opacity: 1 }], { duration: 300, composite: "replace" });
        break;
      case "turn": {
        const el = document.createElement("div");
        el.className = `fx-banner ${f.mine ? "mine" : "theirs"}`;
        el.textContent = f.mine ? "Your turn!" : `${ctx.names?.[f.seat] || "Opponent"}'s turn`;
        spawn(felt, el, [{ opacity: 0, transform: "translate(-50%, -50%) scale(.6)" }, { opacity: 1, transform: "translate(-50%, -50%) scale(1.05)", offset: 0.2 }, { opacity: 1, transform: "translate(-50%, -50%) scale(1)", offset: 0.75 }, { opacity: 0, transform: "translate(-50%, -50%) scale(1.15)" }], { duration: 1300 });
        break;
      }
      case "cast": {
        const c = ctx.cards?.[f.card];
        if (!c) break;
        const el = document.createElement("div");
        el.className = "fx-cast";
        el.append(cardEl(c));
        spawn(felt, el, [{ opacity: 0, transform: "translate(-50%, -50%) scale(.5) rotate(-8deg)" }, { opacity: 1, transform: "translate(-50%, -50%) scale(1) rotate(0)", offset: 0.25 }, { opacity: 1, transform: "translate(-50%, -50%) scale(1)", offset: 0.7 }, { opacity: 0, transform: "translate(-50%, -62%) scale(1.15)" }], { duration: 1100 });
        break;
      }
    }
  }
}
