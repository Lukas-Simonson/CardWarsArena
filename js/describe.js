// Turns protocol objects (actions, events, target references) into text, and
// decides which actions belong to a clicked card. Pure functions: no DOM.
//
// ctx = { view, cards, names, known }
//   view   the current view from the server
//   cards  catalog by id (from GET /api/cards)
//   names  seat display names
//   known  uid -> card id for every card ever seen (so events can name cards
//          that have since left play or gone to a hidden zone)

export const LANDSCAPE_NAMES = {
  cornfield: "Cornfield", blue_plains: "Blue Plains", useless_swamp: "Useless Swamp",
  nice_lands: "Nice Lands", sandy_lands: "Sandy Lands", icy_lands: "Icy Lands", rainbow: "Rainbow",
};

export const landscapeName = (l) => LANDSCAPE_NAMES[l] ?? l ?? "";

export function cardName(ctx, id) {
  return ctx.cards?.[id]?.name ?? id ?? "?";
}

export function instName(ctx, uid) {
  const id = ctx.known?.[uid];
  return id ? cardName(ctx, id) : `card #${uid}`;
}

export function seatName(ctx, seat) {
  return ctx.names?.[seat] || `Seat ${seat}`;
}

const side = (ctx, player) => (player === ctx.view.you ? "your" : "opp's");
const laneNo = (lane) => (lane ?? 0) + 1;

/** One chosen target, e.g. "Cool Dog (opp's, lane 2)". */
export function describeRef(ctx, r) {
  const l = ctx.view.players[r.player]?.lanes[r.lane ?? 0] ?? {};
  const where = `${side(ctx, r.player)}, lane ${laneNo(r.lane)}`;
  switch (r.kind) {
    case "creature": return `${l.creature ? cardName(ctx, l.creature.card) : "creature"} (${where})`;
    case "building": return `${l.building ? cardName(ctx, l.building.card) : "building"} (${where})`;
    case "landscape": return `${l.face_down ? "face-down " : ""}${landscapeName(l.landscape)} landscape (${where})`;
    case "lane": return `lane ${laneNo(r.lane)} (${side(ctx, r.player)} side)`;
    default: return instName(ctx, r.card);
  }
}

function withTargets(ctx, text, label, slots) {
  if (!slots || slots.length === 0) return text;
  // An optional slot with nothing chosen may arrive as null or [].
  const parts = slots.map((s) => (s?.length ? s.map((r) => describeRef(ctx, r)).join(" + ") : "none"));
  return `${text}  [${label}: ${parts.join("; ")}]`;
}

/** A readable sentence for an action the server listed as legal. */
export function describeAction(ctx, a) {
  const v = ctx.view;
  const me = v.players[v.you];
  switch (a.kind) {
    case "keep_hand": return "Keep your opening hand";
    case "mulligan": return "Mulligan: shuffle your hand back and draw 5 (only once)";
    case "draw": return "Spend 1 Action to draw a card";
    case "end_main": return v.turn === 1 ? "End your turn (no Fight on the first turn)" : "Finish: go to the Fight phase";
    case "play_card": {
      const inst = me.hand.find((c) => c.uid === a.card);
      const c = ctx.cards?.[inst?.card];
      const name = c?.name ?? instName(ctx, a.card);
      const isSpell = c?.type === "spell";
      let t = `${isSpell ? "Cast" : "Play"} ${name} (cost ${c?.cost ?? "?"})${isSpell ? "" : ` into lane ${laneNo(a.lane)}`}`;
      t = withTargets(ctx, t, "discard", a.cost_targets);
      return withTargets(ctx, t, "targets", a.targets);
    }
    case "play_pending": {
      const c = ctx.cards?.[v.pending?.card];
      const t = `Play ${c?.name ?? "the stolen card"} for free${c?.type === "spell" ? "" : ` into lane ${laneNo(a.lane)}`}`;
      return withTargets(ctx, t, "targets", a.targets);
    }
    case "choose": {
      const ch = v.choice;
      if (ch?.kind === "discard") return `Discard ${instName(ctx, a.card)}`;
      if (ch?.kind === "reveal") return `Put ${instName(ctx, a.card)} on the bottom of your deck`;
      return withTargets(ctx, `Yes, use ${cardName(ctx, ch?.card)}`, "targets", a.targets);
    }
    case "decline": return "No thanks";
    case "attack": {
      const mine = me.lanes[a.lane ?? 0]?.creature;
      const foe = v.players[1 - v.you].lanes[a.lane ?? 0]?.creature;
      const who = mine ? cardName(ctx, mine.card) : "your Creature";
      return `Fight in lane ${laneNo(a.lane)}: ${who} ${foe ? `fights ${cardName(ctx, foe.card)}` : "attacks the opposing Hero directly"}`;
    }
    case "floop":
    case "activate": {
      const l = me.lanes[a.lane ?? 0] ?? {};
      const src = a.source === "building" ? l.building : l.creature;
      const name = src ? cardName(ctx, src.card) : "?";
      const verb = a.kind === "floop" ? `Floop ${name}` : `Use ${name}'s ability`;
      return withTargets(ctx, `${verb} (lane ${laneNo(a.lane)})`, "targets", a.targets);
    }
    default: return `(unknown action: ${JSON.stringify(a)})`;
  }
}

/** True if describeAction understands the action (used by the smoke test). */
export const isKnownAction = (ctx, a) => !describeAction(ctx, a).startsWith("(unknown");

/** One line for the game log, as seen by this seat. */
export function describeEvent(ctx, e) {
  const who = seatName(ctx, e.player);
  // The server names publicly revealed cards; fall back to what we have seen.
  const n = () => (e.card_id ? cardName(ctx, e.card_id) : instName(ctx, e.card));
  switch (e.kind) {
    case "mulligan": return `${who} takes a mulligan`;
    case "keep_hand": return `${who} keeps their hand`;
    case "turn_start": return `── ${who}'s turn ──`;
    case "draw": return e.card ? `${who} draws ${n()}` : `${who} draws a card`;
    case "play": return `${who} plays ${n()}`;
    case "replace": return `${n()} is replaced`;
    case "attack": return `${who} fights with ${n()} (lane ${laneNo(e.lane)})`;
    case "damage": return `${n()} takes ${e.amount} damage`;
    case "hp_loss": return `${who} loses ${e.amount} HP`;
    case "die": return `${n()} is destroyed`;
    case "game_over": return `*** ${who} wins! ***`;
    case "move": return `${n()} moves to lane ${laneNo(e.lane)}`;
    case "flip": return `${who}'s lane ${laneNo(e.lane)} Landscape is flipped`;
    case "ready": return `${n()} is readied`;
    case "bounce": return `${n()} returns to ${who}'s hand`;
    case "destroy": return `${n()} is destroyed`;
    case "return": return `${who} returns ${n()} from the discard pile to hand`;
    case "deck_top": return `${who} puts ${n()} on top of their deck`;
    case "actions": return `${who} gains ${e.amount} Action(s)`;
    case "floop": return `${who} Floops ${n()}`;
    case "activate": return `${who} uses ${n()}'s ability`;
    case "discard": return `${who} discards ${n()}`;
    case "steal": return `${who} steals ${n()} from the opponent's hand`;
    case "abilities": return `${n()}'s abilities change`;
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
  if (a.kind === "attack") return sel.source === "creature" && (a.lane ?? 0) === sel.lane;
  if (a.kind === "floop" || a.kind === "activate") return src === sel.source && (a.lane ?? 0) === sel.lane;
  return false;
}

export const filterActions = (actions, sel) => actions.filter((a) => actionMatches(a, sel));
