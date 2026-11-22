// A question is everything a surface needs to ask the player one thing. It is pure data: the surfaces (the bar above
// the hand, a pop-up of cards) only draw it, so a question looks the same whichever card asks it. The rules this follows
// are in docs/interactions.md (the codes in brackets: W1, A1, B1...).
//
// A question from the engine arrives already described (the view's `question`, see server/docs/protocol.md): who asked,
// the words as a message key and parameters, where it is answered and the options. fromQuestion() words it and sorts its
// options by what they are: a board object to click, a card of the hand to click, a card to pick in a pop-up, or a
// button. There is no code for any one kind of choice.
//
//   { source: { card, name },            A1: the card that asked
//     text,                              A2: one sentence naming what is decided
//     options: [{ label, result?, role, card?, act }],   A3, A5
//     skip: { label, act } | null,       A4: the last option, always neutral
//     cancel: { label, act } | null,     V2: back out of the whole card (the board prompt only)
//     confirm: { label, act } | null,    A6, V3
//     subject: card | null,              a second card the question is about (the card leaving play, for a Camel)
//     boardPicks: { token: act },        W1: the board objects that answer the question when clicked (`creature:0:2` is
//                                        the Creature in lane 3 of mine); the board highlights them
//     handPicks: { uid: act },           W2: the cards in the hand that answer the question when clicked
//     surface: "bar" | "popup" }         W3, W4
//
// `act` says what pressing does; run() turns it into a call on the handlers `on` that main.js gives the renderer.
import { t } from "./i18n.js";
import { cardName, words } from "./describe.js";
import { tokenOfRef } from "./picker.js";

/** The button class for each role (B1-B5). The one place that says what a colour means. */
export const ROLE_CLASS = { choice: "btn-purple", commit: "btn-green", neutral: "btn-steel", danger: "btn-red", special: "btn-ice" };

export const SURFACE = { bar: "bar", popup: "popup" };

/** Do what an option's `act` says, with the handlers `on`. */
export function run(act, on) {
  switch (act?.type) {
    case "action": return on.onAction(act.action);
    case "kind": return on.onKind(act.key);
    case "token": return on.onToken(act.token);
    case "skip": return on.onSkip();
    case "cancel": return on.onCancel();
    case "confirm": return on.onConfirm();
  }
}

const cardOf = (ctx, id) => ctx.cards?.[id] ?? { id, name: id };

/**
 * The board prompt (what buildPrompt says for the current selection) as a question on the bar. `card` is the card that
 * is being played or used, when there is one.
 */
export function fromPrompt(prompt, card) {
  return {
    source: card ? { card, name: card.name } : null,
    text: prompt.text,
    options: [
      ...prompt.kinds.map(([key, label]) => ({ label, role: "choice", act: { type: "kind", key } })),
      ...prompt.modes.map(([token, label]) => ({ label, role: "choice", act: { type: "token", token } })),
    ],
    skip: prompt.skip ? { label: prompt.skip, act: { type: "skip" } } : null,
    cancel: prompt.cancel ? { label: t("prompt.cancel"), act: { type: "cancel" } } : null,
    confirm: prompt.confirm ? { label: prompt.confirm, act: { type: "confirm" } } : null,
    surface: SURFACE.bar,
  };
}

/**
 * The server's question (the view's `question`) as a question to draw, or null when it is not mine to answer. Every
 * answer is `{ kind: "choose", option: id }`. An option is sorted by what it has: a `ref` to a card of the hand is a
 * hand pick (W2), any other `ref` a board pick (W1), a `card` a card in a pop-up (W4), a `label` a button (W3).
 */
export function fromQuestion(ctx, q, mine) {
  if (!q || !mine || q.player !== ctx.view.you || ctx.view.over) return null;
  const act = (o) => ({ type: "action", action: { kind: "choose", option: o.id } });
  const out = {
    source: q.card ? { card: cardOf(ctx, q.card), name: cardName(ctx, q.card) } : null,
    subject: q.subject ? cardOf(ctx, q.subject.card) : null,
    text: words(ctx, q.text, q), options: [], skip: null, cancel: null, confirm: null, handPicks: {}, boardPicks: {},
    surface: q.answer === "cards" ? SURFACE.popup : SURFACE.bar,
  };
  for (const o of q.options ?? []) {
    if (o.ref?.kind === "hand_card") out.handPicks[o.ref.card] = act(o);
    else if (o.ref) out.boardPicks[tokenOfRef(o.ref)] = act(o);
    else out.options.push({
      label: o.card ? cardName(ctx, o.card.card) : words(ctx, o.label, q), role: o.role === "danger" ? "danger" : "choice", act: act(o),
      ...(o.card && { card: cardOf(ctx, o.card.card) }), ...(o.result && { result: words(ctx, o.result, q) }),
    });
  }
  if (q.skip) out.skip = { label: words(ctx, q.skip.label, q), act: act(q.skip) };
  return out;
}

/**
 * The opening hand's decision as a question on the bar (W3): keep it, or mulligan once. Nothing asks it, so there is no
 * source card. Null outside the mulligan or when it is not mine to answer.
 */
export function fromMulligan(ctx, legal, mine) {
  const v = ctx.view;
  if (!mine || v?.phase !== "mulligan" || v.over) return null;
  const keep = legal.find((a) => a.kind === "keep_hand"), mulligan = legal.find((a) => a.kind === "mulligan");
  if (!keep) return null;
  return {
    source: null, subject: null, text: t("prompt.mulligan"), handPicks: {}, boardPicks: {}, skip: null, cancel: null, confirm: null, surface: SURFACE.bar,
    options: [
      { label: t("mull.keep"), role: "commit", act: { type: "action", action: keep } },
      ...(mulligan ? [{ label: t("mull.mulligan"), role: "choice", act: { type: "action", action: mulligan } }] : []),
    ],
  };
}
