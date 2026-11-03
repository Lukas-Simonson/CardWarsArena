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

// choose answers a Choice (see the modal in render.js): they have no source on the board.
const GENERAL_KINDS = new Set(["keep_hand", "mulligan", "draw", "end_main", "choose"]);

export const isGeneral = (a) => GENERAL_KINDS.has(a.kind);

/** Where an action comes from: "hand:12", "lane:2:creature", "pending", or "" for general actions. */
export function sourceKey(a) {
  switch (a.kind) {
    case "play_card": return `hand:${a.card}`;
    case "play_pending": return "pending";
    case "attack": return `lane:${a.lane ?? 0}:creature`;
    case "floop":
    case "activate": return `lane:${a.lane ?? 0}:${a.source ?? "creature"}`;
    default: return "";
  }
}

/** What a source can do: "attack", "floop", "activate:0", "play_card". */
export const kindKey = (a) => (a.kind === "activate" ? `activate:${a.index ?? 0}` : a.kind);

export function kindLabel(key) {
  if (key === "attack") return "Fight";
  if (key === "floop") return "Floop";
  if (key.startsWith("activate")) return "Use ability";
  return "Play";
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
  const out = [];
  if (needsLane(a, ctx)) out.push("");
  for (const _ of a.cost_targets ?? []) out.push("");
  (a.targets ?? []).forEach((_, i) => out.push(a.labels?.[i] ?? ""));
  return out;
}

/** The choices an action needs, in order: each slot is a sorted list of tokens. */
export function slotsOf(a, ctx) {
  const out = [];
  if (needsLane(a, ctx)) out.push([`dest:${a.lane ?? 0}`]);
  for (const group of [a.cost_targets, a.targets]) {
    for (const slot of group ?? []) out.push((slot ?? []).map(tokenOfRef).sort());
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
    const list = [...tokens].sort(); // a stable order, whatever order the server listed the actions in
    // the slot may be closed early if some candidate wants exactly what is chosen
    const cur = sorted(this.cur);
    const can_skip = list.length > 0 && live.some((a) => {
      const s = this.slots.get(a);
      return i < s.length && same(cur, s[i]);
    });
    // When a labelled step follows, say so: "Next: Deal 1 damage to" is clearer than "Done".
    const next = live.map((a) => slotLabels(a, this.ctx)[i + 1]).find(Boolean);
    return { tokens: list, can_skip, skip_label: next ? `Next: ${next}` : this.cur.length === 0 ? "None" : "Done" };
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
    if (!this.options().tokens.includes(token)) return false;
    this.cur.push(token);
    this.acted = true;
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
   * deliberate click). After a click, choices that are now forced fill in.
   */
  result() {
    return this.acted ? this.confirm() : null;
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
