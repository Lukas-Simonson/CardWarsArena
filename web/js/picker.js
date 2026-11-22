// Turns the server's flat list of legal actions into the clicks a player makes.
//
// Every legal action starts from a *source* (a hand card, a Creature or
// Building on my side, or the stolen card) and then needs a series of choices:
// a destination lane, a discard, targets. Each choice is a *token* naming a
// thing on the table ("dest:2", "creature:1:0", "hand_card:57"). The picker
// keeps the actions that agree with the clicks so far and says which tokens may
// be clicked next, so the UI never needs to know any card's rules.
//
// Pure: no DOM. ctx = { view, cards } as in describe.js.

import { t } from "./i18n.js";
import { labelText, slotFilled } from "./describe.js";

// choose answers a Choice (see the modal in render.js): they have no source on the board.
const GENERAL_KINDS = new Set(["keep_hand", "mulligan", "draw", "end_main", "choose"]);

export const isGeneral = (a) => GENERAL_KINDS.has(a.kind);

/** Where an action comes from: "hand:12", "lane:2:creature", "hero", "pending", or "" for general actions. */
export function sourceKey(a) {
  switch (a.kind) {
    case "play_card": return `hand:${a.card}`;
    case "play_pending": return "pending";
    case "thaw": return `thaw:${a.lane ?? 0}`; // a frozen Landscape of mine
    case "attack": return `lane:${a.lane ?? 0}:creature`;
    case "floop":
    case "activate": return a.source === "hero" ? "hero" : `lane:${a.lane ?? 0}:${a.source ?? "creature"}`;
    default: return "";
  }
}

/**
 * What a source can do: "attack", "floop", "floop:1", "activate:0", "play_card". A card's first Floop is plain
 * "floop"; a second (one its Hero grants it on top of its own) is "floop:1".
 */
export const kindKey = (a) => (a.kind === "activate" ? `activate:${a.index ?? 0}` : a.kind === "floop" && a.index ? `floop:${a.index}` : a.kind);

/** The Floop kinds a source's actions offer, in order: ["floop"], or ["floop", "floop:1"] when it has two. */
export const floopKinds = (actions) => [...new Set(actions.filter((a) => a.kind === "floop").map(kindKey))].sort();

export function kindLabel(key, via = "") {
  if (key === "attack") return t("kind.attack");
  if (key === "floop" || key.startsWith("floop:")) return t(via === "hero" ? "kind.floop_hero" : "kind.floop");
  if (key.startsWith("activate")) return t("kind.activate");
  if (key === "thaw") return t("kind.thaw");
  return t("kind.play");
}

/**
 * Does playing this action need a deliberate Confirm, not just the last click? Yes when an optional part that the card
 * words a skip for (skip_labels) was left out: the player sees what will and will not happen first.
 */
export const needsConfirm = (a) => (a.skip_labels ?? []).some((l, i) => l && !slotFilled(a, i));

/**
 * Slots that share their targets (Green Candy: heal one Creature or damage one; Apple Pieclops: heal some, damage
 * others) are asked as "which Creature", then "what to do to it", as often as the card allows. Returns { at: [first,
 * second] } as indices into the action's targets, or null when the action has no group of exactly two.
 */
function groupPlan(a) {
  const idx = (a.groups ?? []).map((g, i) => (g ? i : -1)).filter((i) => i >= 0);
  return idx.length === 2 && a.groups[idx[0]] === a.groups[idx[1]] ? { at: idx } : null;
}

// Within a grouped step each choice is one token "creature:0:1|mode:0": the Creature and which of the group's slots
// (0 = the first, 1 = the second) it goes in. The player makes it in two clicks; the picker remembers the first.
const MODE = "mode:";
export const isModeToken = (t) => t.startsWith(MODE);
const isPair = (t) => t.includes("|");
const creatureOf = (t) => t.split("|")[0];
const modeOf = (t) => t.split("|")[1];

/** What the group's slot j does, as the card words it ("Heal 1 damage"). */
export function modeLabel(a, token) {
  const plan = groupPlan(a);
  return plan ? (a.labels?.[plan.at[Number(token.slice(MODE.length))]] ?? "") : "";
}

/**
 * Whose discard piles the pile viewer shows: every pile a card may be taken from right now (both, for a card that takes
 * from either, even when one is empty: the server lists them in `piles`), plus the one the player opened with the
 * Discard button. Mine comes first.
 */
export function discardSeats(tokens, opened, me, piles = []) {
  const wanted = [...tokens].map(parseToken).filter((p) => p.kind === "discard_card");
  const seats = new Set([...wanted.map((p) => p.player), ...(wanted.length ? piles : [])]);
  if (opened != null) seats.add(opened);
  return [...seats].sort((a, b) => (a === me ? -1 : b === me ? 1 : a - b));
}

/** Group the non-general legal actions by source key. */
export function sources(legal) {
  const out = {};
  for (const a of legal) {
    const key = sourceKey(a);
    if (key) (out[key] ??= []).push(a);
  }
  return out;
}

export function tokenOfRef(r) {
  if (r.kind === "hand_card") return `hand_card:${r.card}`;
  if (r.kind === "discard_card") return `discard_card:${r.player}:${r.card}`;
  return `${r.kind}:${r.player}:${r.lane ?? 0}`;
}

/** {kind, player?, lane?, uid?} for any token. */
export function parseToken(t) {
  const p = t.split(":");
  switch (p[0]) {
    case "dest": return { kind: "dest", lane: Number(p[1]) };
    case "hand_card": return { kind: "hand_card", uid: Number(p[1]) };
    case "discard_card": return { kind: "discard_card", player: Number(p[1]), uid: Number(p[2]) };
    default: return { kind: p[0], player: Number(p[1]), lane: Number(p[2]) };
  }
}

/** The catalog card behind a play action, or null. */
export function cardOfPlay(a, ctx) {
  const v = ctx.view;
  let id = null;
  if (a.kind === "play_card") id = v.players[v.you].hand?.find((c) => c.uid === a.card)?.card;
  else if (a.kind === "play_pending") id = v.pending?.card;
  return id ? (ctx.cards?.[id] ?? { id }) : null;
}

/** Does an action put a card into a lane (as opposed to a Spell, which has none)? */
export function needsLane(a, ctx) {
  const c = cardOfPlay(a, ctx);
  return !!c && c.type !== "spell";
}

/** What each of an action's choices is for, in the order of slotsOf ("" where the server said nothing). */
export function slotLabels(a, ctx) {
  if (a.kind === "thaw") return [""];
  const out = [];
  if (needsLane(a, ctx)) out.push("");
  for (const _ of a.cost_targets ?? []) out.push("");
  const plan = groupPlan(a);
  (a.targets ?? []).forEach((_, i) => { if (!plan || i !== plan.at[1]) out.push(plan && i === plan.at[0] ? "" : (a.labels?.[i] ?? "")); });
  return out;
}

/** The choices an action needs, in order: each slot is a sorted list of tokens. */
export function slotsOf(a, ctx) {
  if (a.kind === "thaw") return [[`hand_card:${a.card}`]]; // the card to discard
  const out = [];
  if (needsLane(a, ctx)) out.push([`dest:${a.lane ?? 0}`]);
  for (const group of [a.cost_targets, a.targets]) {
    for (const slot of group ?? []) out.push((slot ?? []).map(tokenOfRef).sort());
  }
  const plan = groupPlan(a);
  if (plan) {
    // the group's two slots become one, of "creature|mode" tokens; the second slot's place is closed up
    const base = out.length - (a.targets?.length ?? 0), [x, y] = plan.at;
    out[base + x] = [...out[base + x].map((t) => `${t}|${MODE}0`), ...out[base + y].map((t) => `${t}|${MODE}1`)].sort();
    out.splice(base + y, 1);
  }
  return out;
}

const sorted = (a) => [...a].sort();
const same = (a, b) => a.length === b.length && a.every((x, i) => x === b[i]);

export class ActionPicker {
  /** @param {object[]} actions one source's legal actions; @param {string} kind restrict to one kind key */
  constructor(actions, ctx, kind = "") {
    this.ctx = ctx;
    this.candidates = [];
    this.done = [];   // completed choice slots, each a sorted list of tokens
    this.cur = [];    // tokens clicked in the slot being filled
    this.acted = false; // the player has made at least one choice
    this.pending = null; // in a grouped step, the Creature clicked while its mode is still to be chosen
    this.slots = new Map();
    for (const a of actions) {
      if (kind === "" || kindKey(a) === kind) {
        this.candidates.push(a);
        this.slots.set(a, slotsOf(a, ctx));
      }
    }
    // Leading slots that every action leaves empty (an unused optional choice)
    // need no click.
    this.#advance();
  }

  /** The distinct kinds among the candidates (Fight or Floop or an ability). */
  kinds() {
    return [...new Set(this.candidates.map(kindKey))];
  }

  /** Candidates that agree with every click so far. */
  consistent() {
    const i = this.done.length;
    return this.candidates.filter((a) => {
      const s = this.slots.get(a);
      if (!this.done.every((d, k) => k < s.length && same(s[k], d))) return false;
      return i < s.length ? this.cur.every((t) => s[i].includes(t)) : this.cur.length === 0;
    });
  }

  /**
   * Which step this is, and how many there are, for "Step 2 of 3". Only slots with something to click count, so a step
   * that cannot apply is not counted, and the total shrinks when a skipped step takes a later one with it.
   */
  stepInfo() {
    const live = this.consistent();
    const longest = Math.max(0, ...live.map((a) => this.slots.get(a).length));
    let of = 0, n = 0;
    for (let i = 0; i < longest; i++) {
      if (!live.some((a) => (this.slots.get(a)[i] ?? []).length > 0)) continue;
      of++;
      if (i <= this.done.length) n = of;
    }
    return { n: Math.max(n, 1), of };
  }

  /** What the slot being filled is for, as the server labelled it ("" if it did not). */
  label() {
    const i = this.done.length;
    for (const a of this.consistent()) {
      const l = slotLabels(a, this.ctx)[i];
      if (l) return l;
    }
    return "";
  }

  /** What may be clicked next: { tokens, can_skip, skip_label }. */
  options() {
    const i = this.done.length;
    const live = this.consistent();
    const tokens = new Set();
    for (const a of live) {
      const s = this.slots.get(a);
      if (i < s.length) for (const t of s[i]) if (!this.cur.includes(t)) tokens.add(t);
    }
    let list = [...tokens].sort(); // a stable order, whatever order the server listed the actions in
    // the slot may be closed early if some candidate wants exactly what is chosen
    const cur = sorted(this.cur);
    const can_skip = !this.pending && list.length > 0 && live.some((a) => {
      const s = this.slots.get(a);
      return i < s.length && same(cur, s[i]);
    });
    // Leaving a slot empty says what it does where the card words it ("Don't move a Building").
    const skip = this.cur.length === 0 ? live.map((a) => a.skip_labels?.[i]).find(Boolean) : "";
    // When a labelled step follows, say so: "Next: Deal 1 damage to" is clearer than "Done".
    const next = live.map((a) => slotLabels(a, this.ctx)[i + 1]).find(Boolean);
    // A grouped step offers the Creatures first, then (once one is clicked) what to do to it.
    const pairing = list.some(isPair);
    let modes = [];
    if (pairing) {
      if (this.pending) {
        const ms = [...new Set(list.filter((t) => creatureOf(t) === this.pending).map(modeOf))].sort();
        modes = ms.map((token) => ({ token, label: live.map((a) => modeLabel(a, token)).find(Boolean) ?? "" }));
        list = ms;
      } else {
        list = [...new Set(list.map(creatureOf))].sort();
      }
    }
    const piles = [...new Set(live.flatMap((a) => a.piles ?? []))];
    return {
      tokens: list, modes, pairing, piles, can_skip, optional: can_skip && this.cur.length === 0,
      skip_label: skip ? labelText(skip) : next ? t("skip.next", { label: labelText(next) }) : this.cur.length === 0 ? t("skip.none") : t("skip.done"),
    };
  }

  /** Close slots that every remaining candidate agrees are finished. */
  #advance() {
    for (;;) {
      const live = this.consistent();
      const i = this.done.length;
      const open = live.filter((a) => i < this.slots.get(a).length);
      if (open.length > 0 && open.every((a) => same(sorted(this.cur), this.slots.get(a)[i]))) {
        this.done.push(sorted(this.cur));
        this.cur = [];
      } else {
        return;
      }
    }
  }

  /** Click a token. False if it is not currently allowed. */
  pick(token) {
    const o = this.options();
    if (!o.tokens.includes(token)) return false;
    this.acted = true;
    if (o.pairing && !isModeToken(token)) {
      this.pending = token; // its mode comes next
      return true;
    }
    this.cur.push(o.pairing ? `${this.pending}|${token}` : token);
    this.pending = null;
    this.#advance();
    return true;
  }

  /** Close the current slot with what is chosen ("None" for an optional choice). */
  skip() {
    if (!this.options().can_skip) return false;
    this.done.push(sorted(this.cur));
    this.cur = [];
    this.acted = true;
    this.#advance();
    return true;
  }

  /**
   * The action to send, once the clicks leave exactly one possibility. Before
   * the player has clicked anything this is null even if only one action
   * remains: that case is confirm() (a Spell with no choices still wants a
   * deliberate click), and likewise when an optional part was declined (needsConfirm).
   * After a click, choices that are now forced fill in.
   */
  result() {
    const a = this.acted ? this.confirm() : null;
    return a && !needsConfirm(a) ? a : null;
  }

  /** The remaining single action, if there is exactly one: for a Confirm button. */
  confirm() {
    const live = this.consistent();
    return live.length === 1 ? live[0] : null;
  }

  /** Number of choices made so far. */
  clicks() {
    return this.done.length + this.cur.length;
  }
}
