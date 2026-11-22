// What the bar above the player's hand says and offers, from the current
// selection. Pure: it only reads the view, the context and the picker.
import { describeAction, cardName, seatName, labelText, summaryText } from "./describe.js";
import { t, fmtList } from "./i18n.js";
import { kindLabel, kindKey, parseToken } from "./picker.js";

/**
 * @returns {{text:string, cancel:boolean, skip:string, confirm:string, kinds:[string,string][]}}
 *   text    one line of guidance
 *   cancel  show a Cancel button
 *   skip    label for the "None"/"Done" button, or "" for none
 *   confirm label for a Confirm button, or "" for none
 *   kinds   when the selected card can do several things, the choices
 *   modes   when a chosen target can be used in several ways (heal it or damage it): [token, label] buttons
 */
export function buildPrompt(ctx, myDecision, source, picker, awaitingKind) {
  const out = { text: "", cancel: false, skip: "", confirm: "", kinds: [], modes: [] };
  const v = ctx.view;
  if (!v) return out;
  if (v.over) return { ...out, text: t("prompt.game_over") };
  if (v.choice) {
    const by = cardName(ctx, v.choice.card);
    if (myDecision) return { ...out, text: t("prompt.choice_mine", { name: by }) };
    return { ...out, text: t("prompt.choice_theirs", { who: seatName(ctx, v.choice.player), name: by }) };
  }
  if (!myDecision) {
    const who = fmtList(v.awaiting.map((s) => (s === v.you ? t("prompt.you") : seatName(ctx, s))));
    return { ...out, text: v.awaiting.length ? t("prompt.waiting", { who }) : "" };
  }
  if (v.phase === "mulligan") return { ...out, text: t("prompt.mulligan") };
  if (!source) return { ...out, text: idleText(v) };

  out.cancel = true;
  const name = sourceName(ctx, source);
  if (!picker) return out;
  if (awaitingKind) {
    out.text = t("prompt.what_to_do", { name });
    out.kinds = picker.kinds().map((k) => [k, kindLabel(k, picker.candidates.find((a) => kindKey(a) === k)?.via)]);
    return out;
  }
  const opts = picker.options();
  if (opts.tokens.length === 0 && !opts.can_skip) {
    const action = picker.confirm();
    if (action) {
      const summary = summaryText(ctx, action, name);
      out.text = summary ?? describeAction(ctx, action);
      out.confirm = summary && !(action.targets ?? []).some((s) => s?.length) ? t("confirm.anyway", { name }) : confirmLabel(action, name);
    }
    return out;
  }
  if (opts.modes.length && opts.modes.length === opts.tokens.length) {
    out.text = t("prompt.choose_mode", { name });
    out.modes = opts.modes.map((m) => [m.token, labelText(m.label)]);
    return out;
  }
  if (source.startsWith("thaw:")) {
    out.text = t("prompt.thaw");
    return out;
  }
  const label = picker.label();
  const optional = opts.optional ? "yes" : "no";
  const { n, of } = picker.stepInfo();
  const prefix = of > 1 ? t("prompt.step_n", { n, of }) : ""; // M1: a card with several steps numbers them
  out.text = label ? t("prompt.step_labelled", { label: labelText(label), step: instruction(opts.tokens, true), name, optional, prefix }) : t("prompt.step", { step: instruction(opts.tokens), name, optional, prefix });
  if (opts.can_skip) out.skip = opts.skip_label;
  return out;
}

function idleText(v) {
  if (v.pending) return t("prompt.idle_pending");
  if (v.phase === "fight") return t("prompt.idle_fight");
  return t("prompt.idle");
}

/** "Choose a lane", "Choose a target", … from what may be clicked. */
export function instruction(tokens, inline = false) {
  const msg = (key) => t(inline ? `${key}_inline` : key);
  const kinds = new Set(tokens.map((t) => parseToken(t).kind));
  if (kinds.size === 1) {
    switch ([...kinds][0]) {
      case "dest":
      case "lane": return msg("prompt.choose_lane");
      case "hand_card": return msg("prompt.choose_hand_card");
      case "discard_card": return msg(new Set(tokens.map((t) => parseToken(t).player)).size > 1 ? "prompt.choose_discard_card_either" : "prompt.choose_discard_card");
      case "landscape": return msg("prompt.choose_landscape");
      case "building": return msg("prompt.choose_building");
    }
  }
  return msg("prompt.choose_target");
}

function confirmLabel(a, name) {
  switch (a.kind) {
    case "play_card": return t("confirm.cast", { name });
    case "attack": return t("confirm.attack");
    case "floop": return t("confirm.floop", { name });
    case "activate": return t("confirm.activate");
    case "play_pending": return t("confirm.play", { name });
    case "thaw": return t("confirm.thaw");
    default: return t("confirm.default");
  }
}

/** The card name behind a source key ("hand:12", "lane:2:creature", "hero", "pending"). */
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
  } else if (p[0] === "thaw") {
    return t("prompt.frozen_landscape", { lane: Number(p[1]) + 1 });
  } else if (p[0] === "hero" && me.hero) {
    return cardName(ctx, me.hero.card);
  } else if (p[0] === "pending" && v.pending) {
    return cardName(ctx, v.pending.card);
  }
  return t("prompt.this_card");
}
