// The question gallery (gallery/index.html): every scenario in gallery-data.js, drawn through the game's own question code.
import { h } from "../js/dom.js";
import { fromQuestion } from "../js/question.js";
import { questionBar, questionPopup } from "../js/questionui.js";
import { scenarios, cards } from "./data.js";

const noop = () => {};
const on = { onAction: noop, onKind: noop, onToken: noop, onSkip: noop, onCancel: noop, onConfirm: noop, onInspect: noop };

document.getElementById("phone").addEventListener("change", (e) => document.body.classList.toggle("phone", e.target.checked));

document.getElementById("gallery").append(...scenarios.map((s, i) => {
  // The seat that decides sees the options; the other seat's question has none, so it is only the sentence.
  const asked = fromQuestion({ view: { you: s.q.player, over: false }, cards }, s.q, true);
  const board = Object.keys(asked.boardPicks).length, handN = Object.keys(asked.handPicks).length;
  return h("section", { class: "gitem", "data-kind": s.kind },
    h("h2", {}, `${i + 1}. ${s.title} `, h("code", {}, `${s.kind} · ${s.q.answer}`)),
    s.note && h("p", { class: "gnote" }, s.note),
    h("div", { class: "gframe" }, asked.surface === "popup" ? questionPopup(asked, on, { again: true }) : questionBar(asked, on, { you: true })),
    (board || handN) ? h("p", { class: "gboard" }, `${board ? `${board} on the board glow` : ""}${board && handN ? ", " : ""}${handN ? `${handN} in the hand glow` : ""}: click to answer`) : null);
}));
