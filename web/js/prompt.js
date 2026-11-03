// What the bar above the player's hand says and offers, from the current
// selection. Pure: it only reads the view, the context and the picker.
import { describeAction, cardName, seatName } from "./describe.js";
import { kindLabel, parseToken } from "./picker.js";

/**
 * @returns {{text:string, cancel:boolean, skip:string, confirm:string, kinds:[string,string][]}}
 *   text    one line of guidance
 *   cancel  show a Cancel button
 *   skip    label for the "None"/"Done" button, or "" for none
 *   confirm label for a Confirm button, or "" for none
 *   kinds   when the selected card can do several things, the choices
 */
export function buildPrompt(ctx, myDecision, source, picker, awaitingKind) {
  const out = { text: "", cancel: false, skip: "", confirm: "", kinds: [] };
  const v = ctx.view;
  if (!v) return out;
  if (v.over) return { ...out, text: "Game over" };
  if (v.choice) {
    const by = cardName(ctx, v.choice.card);
    if (myDecision) return { ...out, text: `${by}: make your choice in the dialog.` };
    return { ...out, text: `Waiting for ${seatName(ctx, v.choice.player)} to decide (${by})…` };
  }
  if (!myDecision) {
    const who = v.awaiting.map((s) => (s === v.you ? "you" : seatName(ctx, s))).join(" and ");
    return { ...out, text: v.awaiting.length ? `Waiting for ${who}…` : "" };
  }
  if (v.phase === "mulligan") return { ...out, text: "Keep this hand, or mulligan once to draw a new five." };
  if (!source) return { ...out, text: idleText(v) };

  out.cancel = true;
  const name = sourceName(ctx, source);
  if (!picker) return out;
  if (awaitingKind) {
    out.text = `What should ${name} do?`;
    out.kinds = picker.kinds().map((k) => [k, kindLabel(k)]);
    return out;
  }
  const opts = picker.options();
  if (opts.tokens.length === 0 && !opts.can_skip) {
    const action = picker.confirm();
    if (action) {
      out.text = describeAction(ctx, action);
      out.confirm = confirmLabel(action, name);
    }
    return out;
  }
  const label = picker.label();
  out.text = label ? `${label}: ${instruction(opts.tokens).toLowerCase()} for ${name}` : `${instruction(opts.tokens)} for ${name}`;
  if (opts.can_skip) out.skip = opts.skip_label;
  return out;
}

function idleText(v) {
  if (v.pending) return "Play the stolen card for free.";
  if (v.phase === "fight") return "Fight! Click a glowing Creature to attack.";
  return "Your move: click a glowing card.";
}

/** "Choose a lane", "Choose a target", … from what may be clicked. */
export function instruction(tokens) {
  const kinds = new Set(tokens.map((t) => parseToken(t).kind));
  if (kinds.size === 1) {
    switch ([...kinds][0]) {
      case "dest":
      case "lane": return "Choose a lane";
      case "hand_card": return "Choose a card from your hand";
      case "discard_card": return "Choose a card from the discard pile";
      case "landscape": return "Choose a Landscape";
      case "building": return "Choose a Building";
    }
  }
  return "Choose a target";
}

function confirmLabel(a, name) {
  switch (a.kind) {
    case "play_card": return `Cast ${name}`;
    case "attack": return "Fight!";
    case "floop": return `Floop ${name}`;
    case "activate": return "Use ability";
    case "play_pending": return `Play ${name}`;
    default: return "Confirm";
  }
}

/** The card name behind a source key ("hand:12", "lane:2:creature", "pending"). */
export function sourceName(ctx, source) {
  const v = ctx.view;
  const me = v.players[v.you];
  const p = source.split(":");
  if (p[0] === "hand") {
    const inst = me.hand?.find((c) => c.uid === Number(p[1]));
    if (inst) return cardName(ctx, inst.card);
  } else if (p[0] === "lane") {
    const occ = me.lanes[Number(p[1])]?.[p[2]];
    if (occ) return cardName(ctx, occ.card);
  } else if (p[0] === "pending" && v.pending) {
    return cardName(ctx, v.pending.card);
  }
  return "this card";
}
