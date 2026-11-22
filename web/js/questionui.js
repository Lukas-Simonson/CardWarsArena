// The two surfaces a question (see question.js) is drawn on. Both show the same parts in the same order (A1-A4):
// the card that asked, the sentence, the options, then the skip, and on the bar Cancel and Confirm (B8). Buttons take
// their colour from their role (B1-B5), are at least 44 px high (B6) and are spaced by one variable (B7).
import { h } from "./dom.js";
import { cardEl } from "./card.js";
import { t } from "./i18n.js";
import { ROLE_CLASS, run } from "./question.js";

/**
 * The card that asked, small, for the start of the bar or the head of a pop-up (A1). On the bar, hovering or focusing it
 * shows the card in "Card up close" so it can be read, and pressing it does the same (the pop-up covers that panel, so
 * there it is only a picture).
 */
function sourceChip(source, on) {
  const id = source.card?.id;
  if (!on?.onInspect || !id) return h("span", { class: "qsource", role: "img", "aria-label": t("question.asked_by", { name: source.name }), title: source.name }, cardEl(source.card));
  return h("button", {
    type: "button", class: "qsource", "aria-label": t("question.read_card", { name: source.name }), title: source.name,
    onclick: () => on.onInspect(id), onmouseenter: () => on.onInspect(id, true), onfocus: () => on.onInspect(id, true),
  }, cardEl(source.card));
}

/** One button for an option, a skip, a Cancel or a Confirm. */
function optionButton(opt, role, on, extra = "") {
  return h("button", { type: "button", class: `${ROLE_CLASS[opt.role ?? role]} qopt${extra}`, onclick: () => run(opt.act, on) },
    opt.label, opt.result ? h("small", { class: "qresult" }, opt.result) : null);
}

/** The bar above the hand: the card that asked, the sentence, the options, Skip, Cancel, Confirm. */
export function questionBar(q, on, { you = false } = {}) {
  const buttons = [
    ...q.options.map((o) => optionButton(o, "choice", on)),
    q.skip && optionButton(q.skip, "neutral", on),
    q.cancel && optionButton(q.cancel, "neutral", on),
    q.confirm && optionButton(q.confirm, "commit", on),
  ].filter(Boolean);
  // The card and the sentence stay together; the buttons wrap as a group beside or under them.
  return h("div", { role: "status", class: `well prompt qbar ${you ? "you" : ""}` },
    h("div", { class: "qline" }, q.source && sourceChip(q.source, on), q.subject && sourceChip({ card: q.subject, name: q.subject.name }, on), h("span", { class: "qtext" }, q.text)),
    buttons.length > 0 && h("div", { class: "qopts" }, buttons));
}

/**
 * A pop-up of cards (W4): only for cards the player must see that are not on the table. The card that asked is the head;
 * the options that are cards are shown as cards, any others as buttons, and the skip comes last.
 */
export function questionPopup(q, on, { again = false } = {}) {
  const cards = q.options.filter((o) => o.card), buttons = q.options.filter((o) => !o.card);
  const cardButton = (o) => h("button", { type: "button", class: "modalopt cardopt", "aria-label": o.label, onclick: () => run(o.act, on) },
    cardEl(o.card), h("span", { class: "cap" }, o.label));
  const row = [...buttons.map((o) => optionButton(o, "choice", on, " modalopt")), q.skip && optionButton(q.skip, "neutral", on, " modalopt")].filter(Boolean);
  return h("div", { class: `modal-backdrop${again ? " again" : ""}` },
    h("div", { class: "modal parch qpopup", role: "dialog", "aria-modal": "true", "aria-labelledby": "modal-h" },
      h("header", { class: "qhead" }, q.source && sourceChip(q.source, null), h("h2", { id: "modal-h" }, q.source?.name ?? "")),
      h("p", {}, q.text),
      cards.length > 0 && h("div", { class: "modalcards" }, cards.map(cardButton)),
      row.length > 0 && h("div", { class: "modalrow modalrow2" }, row)));
}
