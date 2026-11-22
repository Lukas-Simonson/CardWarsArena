// Animations for the battle scene. The scene is rebuilt from the view on every
// update, so nothing can transition between renders; instead plan() compares
// the previous and the new view (plus the update's events) and lists what the
// player should see happen, and applyEffects() plays that on the fresh DOM.
//
// plan() is pure. applyEffects() uses the Web Animations API and touches only
// elements the renderer marks with data-seat / data-lane / data-uid.
import { cardEl } from "./card.js";
import { t } from "./i18n.js";
import { cardName, seatName } from "./describe.js";

const MAX_HP = 25;
export const HIT_DELAY = 260;   // the blow lands this long after the lunge starts
export const DIE_DELAY = 420;   // a destroyed card lingers a moment first
export const LUNGE_MS = 520;    // an attacker lunges before it turns sideways
export const CAST_MS = 2000;    // a cast spell stays on show this long
export const STAGE_MS = CAST_MS - 150; // ...and what it did is shown as the card fades
export const TURN_PAUSE_MS = 1000; // rest after the last action before the turn changes
export const BANNER_MS = 1300;
const AFTER_CAST_MS = 1300;     // time for the damage and a death that follow it

// What happens at the start of a new turn (burning, heals, a death it causes) comes after the turn banner, as its own
// step, not with the Fight that ended the previous turn. It all happens at once, as other effects on several cards do.
const START_OF_TURN = new Set(["damage", "hp_loss", "heal", "hp_gain", "die"]);

/** The events of the update that belong to the start of the new turn: those after its turn_start. */
export function startOfTurn(events = []) {
  const at = events.findIndex((e) => e.kind === "turn_start");
  return at < 0 ? [] : events.slice(at + 1).filter((e) => START_OF_TURN.has(e.kind));
}

// Floating numbers: those that start together on the same Creature or player are spread, so each can be read.
export const NUMBER_STAGGER = 100; // each starts this much after the one before it
const NUMBER_WINDOW = 500;         // numbers this close in time are on screen together
const NUMBER_KIND = { hit: "damage", hp: "damage", heal: "heal", hpgain: "heal" };

/**
 * Give the numbers that would overlap different places (dx, dy, rot, in px and degrees) and, a little, different
 * start times. Nothing is random, so the same update always looks the same. A Creature's numbers fan out sideways;
 * a player's go up and down beside the shield. Heal and damage on one target sit apart (heal left or up, damage right
 * or down). A number alone on its target is where it always was.
 */
export function spreadNumbers(fx) {
  const groups = new Map();
  for (const f of fx) {
    if (!NUMBER_KIND[f.t] || !f.amount) continue;
    const key = `${f.late ? "late" : "now"}:${f.t === "hit" || f.t === "heal" ? `c${f.seat}:${f.lane}` : `p${f.seat}`}`;
    if (!groups.has(key)) groups.set(key, []);
    groups.get(key).push(f);
  }
  for (const list of groups.values()) {
    const mixed = list.some((f) => NUMBER_KIND[f.t] === "heal") && list.some((f) => NUMBER_KIND[f.t] === "damage");
    const player = list[0].t === "hp" || list[0].t === "hpgain";
    const at = list.map((f) => f.delay);
    list.forEach((f, i) => {
      // how many of its own kind are showing already (heal and damage are kept apart by side instead)
      const k = Math.min(at.slice(0, i).filter((d, j) => at[i] - d < NUMBER_WINDOW && (!mixed || NUMBER_KIND[list[j].t] === NUMBER_KIND[f.t])).length, 4);
      const side = NUMBER_KIND[f.t] === "heal" ? -1 : 1;
      const fan = k === 0 ? 0 : (k % 2 ? 1 : -1) * Math.ceil(k / 2);
      f.dx = player ? k * 16 : fan * 26 + (mixed ? side * 20 : 0);
      f.dy = player ? fan * 26 + (mixed ? side * 20 : 0) : k % 3 ? -5 * (k % 3) : 0;
      f.rot = k === 0 ? 0 : (k % 2 ? 6 : -6);
      f.delay = at[i] + k * NUMBER_STAGGER;
    });
  }
}

/** How long the start-of-turn events take, from the moment the banner is over. */
function startOfTurnMs(late) {
  if (!late.length) return 0;
  return late.some((e) => e.kind === "die") ? DIE_DELAY + 560 : 900;
}

/** The transform the renderer gives a Creature in each status (see .tile .at). */
const STATUS_XF = { ready: "rotate(0deg) scale(1)", flooped: "rotate(-90deg) scale(0.7)", fought: "rotate(90deg) scale(0.7)" };
const xfOf = (status) => STATUS_XF[status] ?? STATUS_XF.ready;

/** The rotation (degrees) and scale a status transform from STATUS_XF stands for. */
export function turnOf(xf) {
  const m = /rotate\((-?[\d.]+)deg\) scale\(([\d.]+)\)/.exec(xf ?? "");
  return { rot: m ? Number(m[1]) : 0, scale: m ? Number(m[2]) : 1 };
}

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

  const lateEvents = startOfTurn(events), late = new Set(lateEvents);
  const diedUids = new Set(events.filter((e) => e.kind === "die" && !late.has(e)).map((e) => e.card));
  const newTurn = events.some((e) => e.kind === "turn_start");
  const diedLate = new Set(lateEvents.filter((e) => e.kind === "die").map((e) => e.card));
  const cBefore = index(prev, "creature"), cAfter = index(view, "creature");
  for (const [uid, a] of cAfter) {
    const b = cBefore.get(uid);
    if (!b || b.seat !== a.seat || b.lane !== a.lane) fx.push({ t: "drop", seat: a.seat, lane: a.lane });
    else if (b.thing.status !== a.thing.status) {
      // a Creature that attacked finishes its lunge upright, then turns
      const lunged = events.some((e) => e.kind === "attack" && e.card === uid);
      // a Creature readied by the new turn stays turned until the turn banner shows
      const refresh = newTurn && a.thing.status === "ready" && !lunged;
      fx.push({ t: "settle", seat: a.seat, lane: a.lane, from: xfOf(b.thing.status), to: xfOf(a.thing.status), delay: lunged ? LUNGE_MS : 0, ...(refresh && { refresh })});
    }
  }
  for (const [uid, b] of cBefore) {
    if (cAfter.has(uid)) continue;
    if (diedLate.has(uid)) fx.push({ t: "ghost", seat: b.seat, lane: b.lane, card: b.thing.card, status: b.thing.status, delay: DIE_DELAY, late: true });
    else fx.push({ t: "ghost", seat: b.seat, lane: b.lane, card: b.thing.card, status: b.thing.status, delay: diedUids.has(uid) ? DIE_DELAY : 0 });
  }

  const bBefore = index(prev, "building"), bAfter = index(view, "building");
  for (const [uid, a] of bAfter) if (!bBefore.has(uid)) fx.push({ t: "build", seat: a.seat, lane: a.lane });

  const fought = events.some((e) => e.kind === "attack");
  for (const e of events) {
    if (late.has(e)) continue; // shown after the turn banner, below
    switch (e.kind) {
      case "attack": fx.push({ t: "lunge", seat: e.player, lane: e.lane ?? 0, up: e.player === me }); break;
      case "damage": fx.push({ t: "hit", seat: e.player, lane: e.lane ?? 0, amount: e.amount, delay: fought ? HIT_DELAY : 0 }); break;
      case "hp_loss": fx.push({ t: "hp", seat: e.player, amount: e.amount, delay: fought ? HIT_DELAY : 0 }); break;
      case "heal": fx.push({ t: "heal", seat: e.player, lane: e.lane ?? 0, amount: e.amount, delay: 0 }); break;
      case "hp_gain": fx.push({ t: "hpgain", seat: e.player, amount: e.amount, delay: 0 }); break;
      case "turn_start": fx.push({ t: "turn", seat: e.player, mine: e.player === me }); break;
      case "play": if (ctx.cards?.[e.card_id]?.type === "spell") fx.push({ t: "cast", card: e.card_id, seat: e.player }); break;
    }
  }

  view.players.forEach((p, seat) => {
    // HP lost at the start of the new turn (burning) moves the bar in its own step, after the Fight's.
    const net = lateEvents.reduce((n, e) => n + (e.player !== seat ? 0 : e.kind === "hp_loss" ? e.amount : e.kind === "hp_gain" ? -e.amount : 0), 0);
    const mid = net ? p.hp + net : p.hp;
    if (mid !== prev.players[seat].hp) fx.push({ t: "hpbar", seat, from: pct(prev.players[seat].hp), to: pct(mid), delay: fought ? HIT_DELAY : 0 });
    if (net) fx.push({ t: "hpbar", seat, from: pct(mid), to: pct(p.hp), delay: 0, late: true });
  });

  // What the new turn starts with (burning), all at once, from the end of the banner (the time is added below).
  for (const e of lateEvents) {
    const burn = e.cause === "burn";
    switch (e.kind) {
      case "damage": fx.push({ t: "hit", seat: e.player, lane: e.lane ?? 0, amount: e.amount, delay: 0, burn, late: true }); break;
      case "hp_loss": fx.push({ t: "hp", seat: e.player, amount: e.amount, delay: 0, burn, late: true }); break;
      case "heal": fx.push({ t: "heal", seat: e.player, lane: e.lane ?? 0, amount: e.amount, delay: 0, late: true }); break;
      case "hp_gain": fx.push({ t: "hpgain", seat: e.player, amount: e.amount, delay: 0, late: true }); break;
    }
  }
  spreadNumbers(fx); // numbers that start together on one target must not sit on each other

  // Whatever the turn's last action did plays out, then a pause, then the turn
  // changes (banner and the new turn's draws).
  const body = fx.reduce((m, f) => (f.refresh || f.late ? m : Math.max(m, END[f.t]?.(f) ?? 0)), 0);
  const shift = newTurn && body > 0 ? body + TURN_PAUSE_MS : 0;
  for (const f of fx) if (f.t === "turn" || f.refresh) f.delay = shift;

  // Then, once the banner is over, what the new turn starts with.
  const startAt = shift + BANNER_MS;
  for (const f of fx) if (f.late) f.delay += startAt;

  const hadHand = new Set((prev.players[me].hand ?? []).map((c) => c.uid));
  (view.players[me].hand ?? []).filter((c) => !hadHand.has(c.uid)).forEach((c, i) => fx.push({ t: "deal", uid: c.uid, delay: i * 120 + shift }));
  if (view.players[1 - me].hand_count > prev.players[1 - me].hand_count) fx.push({ t: "oppdraw", delay: shift });
  return fx;
}

/**
 * How long the scene should rest on an update before showing the next, so the
 * motion above can finish and the player can see what happened.
 */
export function paceFor(events = [], cards = {}) {
  const lateEvents = startOfTurn(events), after = startOfTurnMs(lateEvents);
  const kinds = new Set(events.filter((e) => !lateEvents.includes(e)).map((e) => e.kind));
  const spell = events.some((e) => e.kind === "play" && cards[e.card_id]?.type === "spell");
  if (kinds.has("turn_start")) {
    // the last action plays out, a pause, then the turn banner
    const body = bodyOf(kinds);
    return (spell ? STAGE_MS : 0) + body + (body > 0 ? TURN_PAUSE_MS : 0) + BANNER_MS + after + 100;
  }
  if (spell) return STAGE_MS + AFTER_CAST_MS;
  if (kinds.has("attack") || kinds.has("die")) return 1400;
  if (kinds.has("play") || kinds.has("floop") || kinds.has("activate") || kinds.has("damage") || kinds.has("destroy")) return 1100;
  if (kinds.has("draw")) return 650;
  return 800;
}

/** A generous estimate of how long the effects of these event kinds take (the turn change excluded). */
function bodyOf(kinds) {
  if (kinds.has("attack")) return 1450;
  if (kinds.has("die") || kinds.has("damage") || kinds.has("destroy")) return 1250;
  if (kinds.has("play") || kinds.has("floop") || kinds.has("activate") || kinds.has("bounce") || kinds.has("move") || kinds.has("ready")) return 500;
  return 0;
}

/** When each effect finishes, from its start of the update (see applyEffects). */
const END = {
  drop: () => 380, settle: (f) => f.delay + 320, build: () => 320, lunge: () => LUNGE_MS,
  hit: (f) => f.delay + 900, heal: (f) => f.delay + 900, hpgain: (f) => f.delay + 1000, hp: (f) => f.delay + 1000, hpbar: (f) => f.delay + 600, ghost: (f) => f.delay + 560,
};

/** When the last of these effects is over (for what should wait for them, such as a dialog). */
export function restOf(fx) {
  return fx.reduce((m, f) => {
    const end = f.t === "turn" ? f.delay + BANNER_MS : f.t === "deal" || f.t === "oppdraw" ? f.delay + 500 : END[f.t]?.(f) ?? 0;
    return Math.max(m, end);
  }, 0);
}

const pct = (hp) => Math.max(0, Math.min(100, Math.round((hp / MAX_HP) * 100)));

// ---------- playing them

// A turn change with nothing else happening in the update (the turn was ended
// by a bare "end turn"): the pause before it has to come from the pacer. When the
// update also holds an action (the last Fight), plan() puts the pause after it.
export const bareTurnChange = (msg) => {
  const lateEvents = startOfTurn(msg.events);
  const ev = (msg.events ?? []).filter((e) => !lateEvents.includes(e)); // what burning does at the start of the turn is not an action
  return ev.some((e) => e.kind === "turn_start") && !ev.some((e) => !["turn_start", "draw", "actions", "ready"].includes(e.kind));
};

export const reduced = () => !!globalThis.matchMedia?.("(prefers-reduced-motion: reduce)").matches;

/** What a cast spell is announced with: "You cast Corn Scepter", "Bot cast Corn Scepter". */
export function castCaption(f, ctx) {
  const card = cardName(ctx, f.card);
  return f.seat === ctx.view?.you ? t("anim.cast_you", { card }) : t("anim.cast_them", { who: seatName(ctx, f.seat), card });
}

/**
 * Show cast spells: the card, large, over the middle of the battlefield, with who cast it. Several at once sit side
 * by side. With reduced motion it just stays put for the same time, without the movement.
 */
function showCasts(casts, ctx, felt) {
  casts.forEach((f, i) => {
    const c = ctx.cards?.[f.card];
    if (!c) return;
    const el = document.createElement("div");
    el.className = "fx-cast";
    el.setAttribute("role", "status");
    el.style.setProperty("--shift", `${(i - (casts.length - 1) / 2) * 150}px`); // several at once sit side by side
    el.append(cardEl(c), Object.assign(document.createElement("p"), { className: "caption", textContent: castCaption(f, ctx) }));
    felt?.append(el);
    const done = () => el.remove();
    if (reduced()) { setTimeout(done, CAST_MS); return; }
    const at = "translate(calc(-50% + var(--shift)), -50%)";
    el.animate([{ opacity: 0, transform: `${at} scale(.5) rotate(-8deg)` }, { opacity: 1, transform: `${at} scale(1) rotate(0)`, offset: 0.14 }, { opacity: 1, transform: `${at} scale(1)`, offset: 0.85 }, { opacity: 0, transform: "translate(calc(-50% + var(--shift)), -62%) scale(1.15)" }], { duration: CAST_MS, fill: "both", easing: "ease-out" }).onfinish = done;
  });
}

/**
 * The frames of a floating number: it rises, from where spreadNumbers placed it (dx, dy), tilted a little. A
 * Creature's number is centred on its tile, a player's sits beside the shield.
 */
function floatFrames(f, side, parent) {
  const dx = clampX(f.dx ?? 0, side, parent);
  const x = side ? `${dx}px` : `calc(-50% + ${dx}px)`, y = (n) => `${(f.dy ?? 0) + n}px`;
  const rot = `rotate(${f.rot ?? 0}deg)`;
  const lift = side ? [10, 44] : [12, 52];
  return [
    { opacity: 0, transform: `translate(${x}, ${y(0)}) scale(.5) ${rot}` },
    { opacity: 1, transform: `translate(${x}, ${y(-lift[0])}) scale(1.2) ${rot}`, offset: 0.25 },
    { opacity: 0, transform: `translate(${x}, ${y(-lift[1])}) scale(1) ${rot}` },
  ];
}

/** A Creature's number stays on its tile, however narrow the tile is (on a phone): the sideways spread is limited to its width. */
function clampX(dx, side, parent) {
  if (side || !parent?.offsetWidth) return dx;
  const room = Math.max(8, parent.offsetWidth / 2 - 16);
  return Math.max(-room, Math.min(room, dx));
}

/** With reduced motion the numbers do not move: each stays where it was placed for a moment, after its delay. */
function staticNumbers(root, fx) {
  for (const f of fx) {
    if (!NUMBER_KIND[f.t] || !f.amount) continue;
    const side = f.t === "hp" || f.t === "hpgain";
    const parent = side ? root.querySelector(`.shield[data-seat="${f.seat}"]`) : root.querySelector(`.tile[data-seat="${f.seat}"][data-lane="${f.lane}"]`);
    if (!parent) continue;
    const heal = NUMBER_KIND[f.t] === "heal";
    const el = Object.assign(document.createElement("em"), { className: `fx-float${side ? " side" : ""}${heal ? " heal" : ""}${f.burn ? " burn" : ""}`, textContent: `${heal ? "+" : "-"}${f.amount}` });
    el.style.transform = `translate(${side ? "0px" : "-50%"}, 0)`;
    el.style.marginInlineStart = `${clampX(f.dx ?? 0, side, parent)}px`;
    el.style.marginTop = `${f.dy ?? 0}px`;
    el.style.opacity = "0";
    parent.append(el);
    setTimeout(() => { el.style.opacity = "1"; }, f.delay ?? 0);
    setTimeout(() => el.remove(), (f.delay ?? 0) + 1400);
  }
}

/** Play the effects on the freshly rendered scene in `root`. */
export function applyEffects(root, fx, ctx) {
  showCasts(fx.filter((f) => f.t === "cast"), ctx, root.querySelector(".felt"));
  if (reduced()) staticNumbers(root, fx);
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
      case "settle": {
        // A turned card is smaller by width, not scale (see .tile .at), so the turn animates both.
        const at = tile(f.seat, f.lane)?.querySelector(".at");
        if (!at) break;
        const from = turnOf(f.from), to = turnOf(f.to);
        const w = parseFloat(getComputedStyle(at).width) || at.offsetWidth;
        play(at, [
          { transform: `translate(-50%, -50%) rotate(${from.rot}deg)`, width: `${(w * from.scale) / to.scale}px` },
          { transform: `translate(-50%, -50%) rotate(${to.rot}deg)`, width: `${w}px` },
        ], { duration: 320, delay: f.delay, composite: "replace" });
        break;
      }
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
        play(t?.querySelector(".card"), [{ transform: "none", filter: "none" }, { transform: "translateX(-7px) rotate(-3deg)", filter: f.burn ? "brightness(1.4) sepia(1) saturate(5) hue-rotate(-15deg)" : "brightness(1.9) saturate(2)", offset: 0.2 }, { transform: "translateX(7px) rotate(3deg)", offset: 0.45 }, { transform: "translateX(-4px)", offset: 0.7 }, { transform: "none", filter: "none" }], { duration: 420, delay: f.delay });
        if (t && f.amount) spawn(t, text(f.burn ? "fx-float burn" : "fx-float", `-${f.amount}`), floatFrames(f, false, t), { duration: 900, delay: f.delay });
        break;
      }
      case "heal": {
        const t = tile(f.seat, f.lane);
        if (t && f.amount) spawn(t, text("fx-float heal", `+${f.amount}`), floatFrames(f, false, t), { duration: 900, delay: f.delay });
        break;
      }
      case "hpgain": {
        const shield = q(`.shield[data-seat="${f.seat}"]`);
        if (shield && f.amount) spawn(shield, text("fx-float side heal", `+${f.amount}`), floatFrames(f, true), { duration: 1000, delay: f.delay });
        break;
      }
      case "hp": {
        const shield = q(`.shield[data-seat="${f.seat}"]`);
        play(shield, [{ transform: "none", filter: "none" }, { transform: "translateX(-6px) rotate(-6deg)", filter: f.burn ? "brightness(1.4) sepia(1) saturate(5) hue-rotate(-15deg)" : "brightness(1.6) hue-rotate(-30deg)", offset: 0.25 }, { transform: "translateX(6px) rotate(6deg)", offset: 0.5 }, { transform: "none", filter: "none" }], { duration: 480, delay: f.delay });
        if (shield && f.amount) spawn(shield, text(f.burn ? "fx-float side burn" : "fx-float side", `-${f.amount}`), floatFrames(f, true), { duration: 1000, delay: f.delay });
        break;
      }
      case "hpbar":
        play(q(`.hpbar[data-seat="${f.seat}"] > div`), [{ width: `${f.from}%` }, { width: `${f.to}%` }], { duration: 600, delay: f.delay, composite: "replace" });
        break;
      case "ghost": {
        const t = tile(f.seat, f.lane);
        if (!t || !ctx.cards?.[f.card]) break;
        const at = document.createElement("div");
        at.className = `at ghost ${f.status === "ready" ? "" : f.status}`;
        const face = cardEl(ctx.cards[f.card]);
        at.append(face);
        // Animate the card, not .at: .at carries the centring and the sideways
        // turn in its own transform, which an animation on it would replace.
        t.append(at);
        const a = face.animate([{ opacity: 1, transform: "none" }, { opacity: 1, transform: "scale(1.05) rotate(-4deg)", offset: 0.3 }, { opacity: 0, transform: "scale(.5) rotate(14deg)", filter: "grayscale(1)" }], { duration: 560, delay: f.delay, easing: "ease-in", fill: "both" });
        a.onfinish = () => at.remove();
        break;
      }
      case "deal":
        play(q(`.hbtn[data-uid="${f.uid}"]`), [{ transform: "translate(160px, -140px) scale(.4) rotate(25deg)", opacity: 0 }, { opacity: 1, offset: 0.5 }, { transform: "none" }], { duration: 420, delay: f.delay, composite: "replace" });
        break;
      case "oppdraw":
        play([...root.querySelectorAll(".back")].pop(), [{ transform: "translateY(40px) scale(.5)", opacity: 0 }, { transform: "none", opacity: 1 }], { duration: 300, delay: f.delay, composite: "replace" });
        break;
      case "turn": {
        const el = document.createElement("div");
        el.className = `fx-banner ${f.mine ? "mine" : "theirs"}`;
        el.textContent = f.mine ? t("anim.your_turn") : t("game.their_turn", { who: ctx.names?.[f.seat] || t("player.opponent") });
        spawn(felt, el, [{ opacity: 0, transform: "translate(-50%, -50%) scale(.6)" }, { opacity: 1, transform: "translate(-50%, -50%) scale(1.05)", offset: 0.2 }, { opacity: 1, transform: "translate(-50%, -50%) scale(1)", offset: 0.75 }, { opacity: 0, transform: "translate(-50%, -50%) scale(1.15)" }], { duration: BANNER_MS, delay: f.delay });
        break;
      }
    }
  }
}
