// Cards shown from a hand (Fly Swatter shows the whole hand) get a pop-up of their own, not just a line in the log.

import { h } from "./dom.js";
import { cardEl } from "./card.js";
import { t } from "./i18n.js";

/** The reveal events of one update, grouped by the player whose hand it is: [{ player, cards: [card ids] }]. */
export function revealGroups(events) {
  const out = [];
  for (const e of events ?? []) {
    if (e.kind !== "reveal" || !e.card_id) continue;
    let g = out.find((x) => x.player === (e.player ?? 0));
    if (!g) out.push((g = { player: e.player ?? 0, cards: [] }));
    g.cards.push(e.card_id);
  }
  return out;
}

/** "You show your hand" / "Sam shows their hand", for a group of n cards. */
export function revealTitle(group, you, name) {
  return t(group.player === you ? "reveal.title_you" : "reveal.title_them", { n: group.cards.length, name: name || t("reveal.opponent") });
}

/** Open one pop-up for a group of revealed cards; it closes with its button, Escape or a click outside. */
export function showReveal(group, { you, name, cards }) {
  const dlg = h("dialog", { class: "modal parch revealdialog", "aria-label": t("reveal.label") },
    h("h2", {}, revealTitle(group, you, name)),
    h("div", { class: "modalcards" }, group.cards.map((id) =>
      h("div", { class: "modalopt cardopt" }, cardEl(cards[id] ?? { id, name: id }), h("span", { class: "cap" }, cards[id]?.name ?? id)))),
    h("div", { class: "modalrow" }, h("button", { type: "button", class: "btn-green modalopt", onclick: () => dlg.close() }, t("common.ok"))));
  dlg.addEventListener("close", () => dlg.remove());
  dlg.addEventListener("click", (e) => { if (e.target === dlg) dlg.close(); });
  document.body.append(dlg);
  dlg.showModal();
  dlg.querySelector("button")?.focus();
}
