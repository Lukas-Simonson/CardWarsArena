// Turns protocol objects (actions, events, target references) into text, and
// decides which actions belong to a clicked card. Pure functions: no DOM.
// Every sentence is one message in the locale catalogue, with its names and numbers as parameters,
// so a language can reorder the words.
//
// ctx = { view, cards, names, known }
//   view   the current view from the server
//   cards  catalog by id (from GET /api/cards)
//   names  seat display names
//   known  uid -> card id for every card ever seen (so events can name cards
//          that have since left play or gone to a hidden zone)

import { t, has } from "./i18n.js";

export const LANDSCAPE_NAMES = new Proxy({}, { get: (_, l) => (typeof l === "string" && has(`landscape.${l}`) ? t(`landscape.${l}`) : undefined) });

/** The Landscape types in the order the server numbers them (a "landscape" choice's `card` is 1 + the index). */
export const LANDSCAPE_ORDER = ["cornfield", "blue_plains", "useless_swamp", "nice_lands", "sandy_lands", "icy_lands"];

export const landscapeName = (l) => (l && has(`landscape.${l}`) ? t(`landscape.${l}`) : l ?? "");

/**
 * What the server calls a target ("Heal", "Freeze"): text written into the card data. The catalogue may translate
 * it under label.<the text in lower case, with _ for anything else>; without that, it is shown as it came.
 */
export function labelText(label) {
  const key = `label.${String(label).toLowerCase().replace(/[^a-z0-9]+/g, "_").replace(/^_|_$/g, "")}`;
  return has(key) ? t(key) : label;
}

export function cardName(ctx, id) {
  return ctx.cards?.[id]?.name ?? id ?? "?";
}

export function instName(ctx, uid) {
  const id = ctx.known?.[uid];
  return id ? cardName(ctx, id) : t("card.unknown", { uid });
}

export function seatName(ctx, seat) {
  return ctx.names?.[seat] || t("seat.default", { seat });
}

const side = (ctx, player) => (player === ctx.view.you ? "you" : "opp");
const laneNo = (lane) => (lane ?? 0) + 1;

/** One chosen target, e.g. "Cool Dog (opp's, lane 2)". */
export function describeRef(ctx, r) {
  const l = ctx.view.players[r.player]?.lanes[r.lane ?? 0] ?? {};
  const where = { side: side(ctx, r.player), lane: laneNo(r.lane) };
  switch (r.kind) {
    case "creature": return t("ref.creature", { ...where, name: l.creature ? cardName(ctx, l.creature.card) : t("noun.creature") });
    case "building": return t("ref.building", { ...where, name: l.building ? cardName(ctx, l.building.card) : t("noun.building") });
    case "landscape": return t("ref.landscape", { ...where, type: landscapeName(l.landscape), down: l.face_down ? "yes" : "no" });
    case "lane": return t("ref.lane", where);
    case "player": return t(r.player === ctx.view.you ? "ref.player_you" : "ref.player_opp");
    default: return instName(ctx, r.card);
  }
}

function withTargets(ctx, text, label, slots, labels) {
  if (!slots || slots.length === 0) return text;
  // An optional slot with nothing chosen may arrive as null or [].
  const parts = slots.map((s, i) => {
    const what = s?.length ? s.map((r) => describeRef(ctx, r)).join(t("targets.and")) : t("targets.none");
    return labels?.[i] ? t("targets.slot", { label: labelText(labels[i]), what }) : what;
  });
  return t("targets.with", { text, label: t(`targets.label_${label}`), parts: parts.join(t("targets.sep")) });
}

/** The sentence for an action, or null when this client does not know the kind. */
function actionText(ctx, a) {
  const v = ctx.view;
  const me = v.players[v.you];
  switch (a.kind) {
    case "keep_hand": return t("action.keep_hand");
    case "mulligan": return t("action.mulligan");
    case "draw": return t("action.draw");
    case "end_main": return t(v.turn === 1 ? "action.end_main_first" : "action.end_main");
    case "play_card": {
      const inst = me.hand.find((c) => c.uid === a.card);
      const c = ctx.cards?.[inst?.card];
      const name = c?.name ?? instName(ctx, a.card);
      let text = t("action.play_card", { name, cost: c?.cost ?? "?", spell: c?.type === "spell" ? "yes" : "no", lane: laneNo(a.lane) });
      text = withTargets(ctx, text, "discard", a.cost_targets);
      return withTargets(ctx, text, "targets", a.targets, a.labels);
    }
    case "play_pending": {
      const c = ctx.cards?.[v.pending?.card];
      const text = t("action.play_pending", { name: c?.name ?? t("action.stolen_card"), spell: c?.type === "spell" ? "yes" : "no", lane: laneNo(a.lane) });
      return withTargets(ctx, text, "targets", a.targets, a.labels);
    }
    case "choose": return chooseText(ctx, a);
    case "thaw": return t("action.thaw", { card: instName(ctx, a.card), lane: laneNo(a.lane) });
    case "attack": {
      const mine = me.lanes[a.lane ?? 0]?.creature;
      const foe = v.players[1 - v.you].lanes[a.lane ?? 0]?.creature;
      return t("action.attack", { lane: laneNo(a.lane), who: mine ? cardName(ctx, mine.card) : t("noun.your_creature"), foe: foe ? cardName(ctx, foe.card) : "", hero: foe ? "no" : "yes" });
    }
    case "floop":
    case "activate": {
      if (a.source === "hero") return withTargets(ctx, t("action.hero_ability", { name: cardName(ctx, me.hero?.card) }), "targets", a.targets, a.labels);
      const l = me.lanes[a.lane ?? 0] ?? {};
      const src = a.source === "building" ? l.building : l.creature;
      const name = src ? cardName(ctx, src.card) : "?";
      const text = t(a.kind === "floop" ? (a.via === "hero" ? "action.floop_hero" : "action.floop") : "action.activate", { name, lane: laneNo(a.lane) });
      return withTargets(ctx, text, "targets", a.targets, a.labels);
    }
    default: return null;
  }
}

function chooseText(ctx, a) {
  const v = ctx.view;
  const ch = v.choice;
  const card = ch ? cardName(ctx, ch.card) : "";
  switch (ch?.kind) {
    case "upkeep": return a.card ? t("choose.discard", { card: instName(ctx, a.card) }) : t("choose.upkeep_destroy", { name: card });
    case "discard": return t("choose.discard", { card: instName(ctx, a.card) });
    case "reveal": return t("choose.reveal", { card: instName(ctx, a.card) });
    case "search": return t("choose.search", { card: instName(ctx, a.card) });
    case "rebound": return a.card ? t("choose.rebound_return", { card: instName(ctx, a.card), name: card }) : t("choose.rebound_leave", { name: card });
    case "redirect": return a.card ? t("choose.redirect_other", { card: instName(ctx, a.card) }) : t("choose.redirect_same", { name: card });
    case "move_damage": return t("choose.move_damage", { n: a.card });
    case "camel": return a.card ? t("choose.camel_put", { card: instName(ctx, ch.revealed?.[0]?.uid), name: card }) : t("choose.camel_let_go", { card: instName(ctx, ch.revealed?.[0]?.uid) });
    case "landscape": return t("choose.landscape", { lane: laneNo(ch.lane), type: landscapeName(LANDSCAPE_ORDER[(a.card ?? 1) - 1]) });
    case "target":
      return a.targets?.[0]?.length
        ? withTargets(ctx, t("choose.target", { name: card, label: ch.label ? labelText(ch.label) : t("choose.target_default") }), "target", a.targets)
        : t("choose.target_none", { name: card });
    case "ransom": return a.card ? t("choose.ransom_pay", { card: instName(ctx, a.card), name: card }) : t("choose.ransom_take", { name: card });
    case "amount": return t("choose.amount", { name: card, n: a.card });
    case "move": return t("choose.move", { name: card, lane: laneNo(a.lane), theirs: movesToTheirSide(ch, v.you) ? "yes" : "no" });
    default: return t("choose.default");
  }
}

/**
 * What a card with optional parts will do, said before it is played: the parts chosen and the parts declined, or
 * that it will do nothing when every part was declined. null for an action whose card has no wording for skipping.
 */
export function summaryText(ctx, a, name) {
  const skips = a.skip_labels ?? [], labels = a.labels ?? [], slots = a.targets ?? [];
  if (!skips.some(Boolean)) return null;
  if (slots.every((s) => !s?.length)) return t("summary.nothing", { name });
  const parts = [];
  slots.forEach((s, i) => {
    if (s?.length) parts.push(withLabel(labels[i], s.map((r) => describeRef(ctx, r)).join(t("targets.and"))));
    else if (skips[i] && !slotFilled(a, i)) parts.push(labelText(skips[i]));
  });
  return t("summary.sentence", { parts: parts.join(t("summary.sep")) });
}

/** Is target slot i filled, or, when it shares a target with others (Group), is any slot of its group? */
export function slotFilled(a, i) {
  const g = a.groups?.[i];
  return (a.targets ?? []).some((s, j) => s?.length && (j === i || (g && a.groups[j] === g)));
}

const withLabel = (label, what) => (label ? t("targets.slot", { label: labelText(label), what }) : what);

/** A readable sentence for an action the server listed as legal. */
export function describeAction(ctx, a) {
  return actionText(ctx, a) ?? t("action.unknown", { action: JSON.stringify(a) });
}

/** True if describeAction understands the action (used by the smoke test). */
export const isKnownAction = (ctx, a) => actionText(ctx, a) !== null;

/**
 * Does a "move" choice send the Creature to the opponent's side? The server leaves `move_side` out when it is
 * seat 0 (it is a zero), so a missing value means seat 0, not "somewhere else".
 */
export const movesToTheirSide = (choice, you) => (choice?.move_side ?? 0) !== you;

/** One line for the game log, as seen by this seat. */
export function describeEvent(ctx, e) {
  const who = seatName(ctx, e.player);
  // The server names publicly revealed cards; fall back to what we have seen.
  const card = e.card_id ? cardName(ctx, e.card_id) : instName(ctx, e.card);
  const lane = laneNo(e.lane);
  const p = { who, card, lane, amount: e.amount };
  switch (e.kind) {
    case "mulligan": return t("event.mulligan", p);
    case "keep_hand": return t("event.keep_hand", p);
    case "turn_start": return e.turn ? t("event.turn_start_n", { ...p, turn: e.turn }) : t("event.turn_start", p);
    case "draw": return t(e.card ? "event.draw_card" : "event.draw", p);
    case "play": return t("event.play", p);
    case "replace": return t("event.replace", p);
    case "attack": return t("event.attack", p);
    case "damage": return t("event.damage", p);
    case "hp_loss": return t("event.hp_loss", p);
    case "hp_gain": return t("event.hp_gain", p);
    case "heal": return t("event.heal", p);
    case "die": return t("event.die", p);
    case "game_over": return t("event.game_over", p);
    case "move": return t("event.move", p);
    case "remove": return t("event.remove", p);
    case "under": return t("event.under", p);
    case "retype": return t("event.retype", { ...p, type: landscapeName(LANDSCAPE_ORDER[(e.amount ?? 1) - 1]) });
    case "flip": return t("event.flip", p);
    case "freeze": return t("event.freeze", p);
    case "thaw": return t("event.thaw", p);
    case "ready": return t("event.ready", p);
    case "bounce": return t("event.bounce", p);
    case "destroy": return t("event.destroy", p);
    case "return": return t("event.return", p);
    case "deck_top": return t("event.deck_top", p);
    case "actions": return t("event.actions", p);
    case "floop": return t("event.floop", p);
    case "activate": return t("event.activate", p);
    case "discard": return t("event.discard", p);
    case "steal": return t("event.steal", p);
    case "abilities": return t("event.abilities", p);
    case "reveal": return t("event.reveal", p);
    default: return e.kind;
  }
}

// ----- selection: clicking a card narrows the action list

/** What a click can select: a hand card, or a creature/building in one of my lanes. */
export const handSelection = (uid) => ({ type: "hand", uid });
export const laneSelection = (lane, source) => ({ type: "lane", lane, source });

export function sameSelection(a, b) {
  if (!a || !b) return false;
  return a.type === b.type && a.uid === b.uid && a.lane === b.lane && a.source === b.source;
}

/** Does this action belong to the selected card? */
export function actionMatches(a, sel) {
  if (!sel) return true;
  if (sel.type === "hand") return a.kind === "play_card" && a.card === sel.uid;
  const src = a.source ?? "creature";
  if (a.source === "hero") return sel.source === "hero";
  if (a.kind === "attack") return sel.source === "creature" && (a.lane ?? 0) === sel.lane;
  if (a.kind === "floop" || a.kind === "activate") return src === sel.source && (a.lane ?? 0) === sel.lane;
  return false;
}

export const filterActions = (actions, sel) => actions.filter((a) => actionMatches(a, sel));
