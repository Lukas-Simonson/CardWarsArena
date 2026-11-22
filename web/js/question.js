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
//     surface: "bar" | "popup" }         W3, W4
//
// `act` says what pressing does; run() turns it into a call on the handlers `on` that main.js gives the renderer.
import { t } from "./i18n.js";
import { cardName } from "./describe.js";

/** The button class for each role (B1-B5). The one place that says what a colour means. */
export const ROLE_CLASS = { choice: "btn-purple", commit: "btn-green", neutral: "btn-steel", danger: "btn-red", special: "btn-ice" };

export const SURFACE = { bar: "bar", popup: "popup" };

/**
 * Where each kind of engine choice is asked once it is on the shared question. A kind that is not here is still a
 * hand-built pop-up in render.js's choiceModal and is listed in the audit with the step that moves it (2b to 2d).
 * A new kind must be added here, with a reason from the guide: W3 for a short decision, W4 only for cards that are not
 * on the table.
 */
export const CHOICE_SURFACE = { amount: SURFACE.bar, reveal: SURFACE.popup };

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
  const base = { source: source(ctx, choice.card), skip: null, cancel: null, confirm: null, surface: CHOICE_SURFACE[choice.kind] };
  switch (choice.kind) {
    case "amount": // The Big Hen House: how much damage (W3)
      return { ...base, text: t("question.amount", { options: (choice.options ?? []).map(String) }), options: answers.map((a) => ({ label: String(a.card), role: "choice", act: { type: "action", action: a } })) };
    case "reveal": // Ogre Gas: one of the cards taken off the deck (W4: they are not on the table)
      return {
        ...base, text: t("question.reveal"),
        options: (choice.revealed ?? []).flatMap((inst) => {
          const action = answers.find((a) => a.card === inst.uid);
          return action ? [{ label: cardName(ctx, inst.card), role: "choice", card: ctx.cards?.[inst.card] ?? { id: inst.card, name: inst.card }, act: { type: "action", action } }] : [];
        }),
      };
  }
  return null;
}
