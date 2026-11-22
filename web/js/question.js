// A question is everything a surface needs to ask the player one thing. It is pure data: the surfaces (the bar above
// the hand, a pop-up of cards) only draw it, so a question looks the same whichever card asks it. The rules this follows
// are in docs/interactions.md (the codes in brackets: W3, A1, B1...); docs/interaction-audit.md says where each kind of
// engine choice is going.
//
//   { source: { card, name },            A1: the card that asked
//     text,                              A2: one sentence naming what is decided
//     options: [{ label, result?, role, card?, act }],   A3, A5
//     skip: { label, act } | null,       A4: the last option, always neutral
//     cancel: { label, act } | null,     V2: back out of the whole card (the board prompt only)
//     confirm: { label, act } | null,    A6, V3
//     subject: card | null,              a second card the question is about (the card leaving play, for a Camel)
//     handPicks: { uid: act },           W2: the cards in the hand that answer the question when clicked
//     surface: "bar" | "popup" }         W3, W4
//
// `act` says what pressing does; run() turns it into a call on the handlers `on` that main.js gives the renderer.
import { t } from "./i18n.js";
import { cardName, landscapeName, LANDSCAPE_ORDER } from "./describe.js";

/** The button class for each role (B1-B5). The one place that says what a colour means. */
export const ROLE_CLASS = { choice: "btn-purple", commit: "btn-green", neutral: "btn-steel", danger: "btn-red", special: "btn-ice" };

export const SURFACE = { bar: "bar", popup: "popup" };

/**
 * Where each kind of engine choice is asked once it is on the shared question. A kind that is not here is still a
 * hand-built pop-up in render.js's choiceModal and is listed in the audit with the step that moves it (2b to 2d).
 * A new kind must be added here, with a reason from the guide: W3 for a short decision, W4 only for cards that are not
 * on the table.
 */
export const CHOICE_SURFACE = {
  amount: SURFACE.bar, reveal: SURFACE.popup,
  // Cards from your own hand are a click on the hand, with the sentence and any decline on the bar (W2).
  discard: SURFACE.bar, ransom: SURFACE.bar, upkeep: SURFACE.bar, rebound: SURFACE.bar,
  // Short decisions that are not about a thing on the table are buttons in the bar (W3); a search of the deck shows
  // cards that are not on the table (W4).
  move_damage: SURFACE.bar, camel: SURFACE.bar, landscape: SURFACE.bar, search: SURFACE.popup,
};

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

const source = (ctx, cardId) => (cardId ? { card: ctx.cards?.[cardId] ?? { id: cardId, name: cardId }, name: cardName(ctx, cardId) } : null);

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
 * The player's own mid-effect choice as a question, or null when it is not theirs to answer or its kind is still a
 * hand-built pop-up (see CHOICE_SURFACE).
 */
export function fromChoice(ctx, choice, legal, mine) {
  if (!choice || !mine || !CHOICE_SURFACE[choice.kind] || ctx.view?.over) return null;
  const answers = legal.filter((a) => a.kind === "choose");
  const base = { source: source(ctx, choice.card), skip: null, cancel: null, confirm: null, handPicks: {}, subject: null, surface: CHOICE_SURFACE[choice.kind] };
  const name = cardName(ctx, choice.card);
  const act = (a) => ({ type: "action", action: a });
  // Each card of the hand that answers (an answer with a card), and the answer that has none (decline or take it).
  const handPicks = Object.fromEntries(answers.filter((a) => a.card).map((a) => [a.card, act(a)]));
  const noCard = answers.find((a) => !a.card);
  switch (choice.kind) {
    case "amount": // The Big Hen House: how much damage (W3)
      return { ...base, text: t("question.amount", { options: (choice.options ?? []).map(String) }), options: answers.map((a) => ({ label: t("question.amount_n", { n: a.card, unit: choice.unit ?? "" }), role: "choice", act: { type: "action", action: a } })) };
    case "reveal": // Ogre Gas: one of the cards taken off the deck (W4: they are not on the table)
      return {
        ...base, text: t("question.reveal"),
        options: (choice.revealed ?? []).flatMap((inst) => {
          const action = answers.find((a) => a.card === inst.uid);
          return action ? [{ label: cardName(ctx, inst.card), role: "choice", card: ctx.cards?.[inst.card] ?? { id: inst.card, name: inst.card }, act: { type: "action", action } }] : [];
        }),
      };
    case "search": // Deforestation: a Building from the deck (W4: the deck is not on the table)
      return {
        ...base, text: t("question.search"),
        options: (choice.revealed ?? []).flatMap((inst) => {
          const action = answers.find((a) => a.card === inst.uid);
          return action ? [{ label: cardName(ctx, inst.card), role: "choice", card: ctx.cards?.[inst.card] ?? { id: inst.card, name: inst.card }, act: act(action) }] : [];
        }),
      };
    case "move_damage": { // Friendship Bracelet: how much damage to move between two of your Creatures, with the result of each
      const mine = ctx.view.players[ctx.view.you].lanes;
      const from = mine[choice.lane]?.creature, to = mine[choice.other]?.creature;
      const [a, b] = [from, to].map((c) => (c ? cardName(ctx, c.card) : "?"));
      return {
        ...base, text: t("question.move_damage", { from: a, to: b }),
        options: answers.map((x) => ({
          label: t("question.move_n", { n: x.card }), role: "choice", act: act(x),
          result: from && to ? t("question.move_result", { from: a, a1: from.damage, a2: from.damage - x.card, to: b, b1: to.damage, b2: to.damage + x.card }) : undefined,
        })),
      };
    }
    case "camel": { // Carmel Camel: put the leaving card under it, or let it go
      const gone = choice.revealed?.[0];
      const puts = answers.filter((a) => a.card);
      return {
        ...base, subject: gone ? ctx.cards?.[gone.card] ?? { id: gone.card, name: gone.card } : null,
        text: t("question.camel", { card: gone ? cardName(ctx, gone.card) : "", name }),
        options: puts.map((a) => ({ label: t(puts.length > 1 ? "question.camel_put_lane" : "question.camel_put", { name, lane: (ctx.view.players[ctx.view.you].lanes.findIndex((l) => l.creature?.uid === a.card)) + 1 }), role: "choice", act: act(a) })),
        skip: noCard ? { label: t("question.camel_skip", { name }), act: act(noCard) } : null,
      };
    }
    case "landscape": // Silo of Freedom: the type its Lane's Landscape becomes (seven short buttons)
      return { ...base, text: t("question.landscape", { lane: (choice.lane ?? 0) + 1 }), options: answers.map((a) => ({ label: landscapeName(LANDSCAPE_ORDER[a.card - 1]), role: "choice", act: act(a) })) };
    case "discard": // pick N cards of your hand to discard (the count goes down as they are chosen)
      return { ...base, text: t("question.discard", { n: choice.count ?? 1 }), options: [], handPicks };
    case "ransom": // discard a card of at least a cost, or take what the card says (Blood Bath, Bums Away!)
      return { ...base, text: t("question.ransom", { min: choice.min_cost ?? 0 }), handPicks, options: noCard ? [{ label: t("question.take_it", { name }), role: "danger", act: act(noCard) }] : [] };
    case "upkeep": // Steakchop: discard cards, or destroy it
      return { ...base, text: t("question.upkeep", { n: choice.count ?? 1, name }), handPicks, options: noCard ? [{ label: t("question.destroy", { name }), role: "danger", act: act(noCard) }] : [] };
    case "rebound": // Impossible Possum: discard a card to return it, or leave it in the discard pile
      return { ...base, text: t("question.rebound", { name }), handPicks, skip: noCard ? { label: t("question.leave_it", { name }), act: act(noCard) } : null, options: [] };
  }
  return null;
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
    source: null, subject: null, text: t("prompt.mulligan"), handPicks: {}, skip: null, cancel: null, confirm: null, surface: SURFACE.bar,
    options: [
      { label: t("mull.keep"), role: "commit", act: { type: "action", action: keep } },
      ...(mulligan ? [{ label: t("mull.mulligan"), role: "choice", act: { type: "action", action: mulligan } }] : []),
    ],
  };
}
