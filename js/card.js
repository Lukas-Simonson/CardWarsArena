// Draws one card: the printed scan when the catalog has one, otherwise a
// frame in the Landscape's colours built from the card's data.
import { h, svg } from "./dom.js";
import { landscapeName } from "./describe.js";

const typeName = (t) => (t ? t[0].toUpperCase() + t.slice(1) : "");

const HILLS = `
<circle cx="76" cy="18" r="8" fill="var(--orb)" stroke="#1f1a12" stroke-width="1.5" vector-effect="non-scaling-stroke"/>
<path d="M0 40 Q 22 24 48 38 T 100 32 V 70 H 0 Z" fill="var(--hill1)" stroke="#1f1a12" stroke-width="2" vector-effect="non-scaling-stroke"/>
<path d="M0 54 Q 30 40 62 52 T 100 48 V 70 H 0 Z" fill="var(--hill2)" stroke="#1f1a12" stroke-width="2" vector-effect="non-scaling-stroke"/>`;

/** The designed frame: cost orb, name, landscape art, type line, text, stats. */
export function frame(card) {
  const isCreature = card.type === "creature";
  const text = card.text ?? "";
  return h("div", { class: "frame" },
    h("div", { class: "top" },
      h("div", { class: "cost" }, card.cost ?? "–"),
      h("div", { class: `nm ${(card.name ?? "").length > 14 ? "long" : ""}` }, card.name ?? card.id)),
    h("div", { class: "art" }, svg(HILLS, { viewBox: "0 0 100 70", preserveAspectRatio: "none" })),
    h("div", { class: "tl" }, [landscapeName(card.landscape), typeName(card.type)].filter(Boolean).join(" ")),
    h("div", { class: `tx ${text.length > 90 ? "long" : ""}` }, text),
    isCreature && h("div", { class: "nums" }, h("span", { title: "Attack" }, card.attack ?? 0), h("span", { title: "Defense" }, card.defense ?? 0)));
}

/**
 * @param {object} card  catalog entry (id, name, type, landscape, cost, attack, defense, text, image_url)
 * @param {{atk?:number, def?:number, damage?:number}} [live]  current values for a Creature in play
 */
export function cardEl(card, live = null) {
  const c = card ?? { id: "?", name: "?" };
  const el = h("div", { class: "card", "data-land": c.landscape ?? "rainbow" });
  if (c.image_url) {
    const img = h("img", {
      src: c.image_url, alt: c.name, draggable: "false",
      onerror: () => { img.remove(); el.prepend(frame(c)); },
    });
    el.append(img);
  } else {
    el.append(frame(c));
  }
  if (live && c.type === "creature") {
    const left = (live.def ?? c.defense ?? 0) - (live.damage ?? 0);
    el.append(
      h("span", { class: `badge atk ${live.atk > (c.attack ?? 0) ? "up" : ""}`, title: "Attack" }, live.atk ?? c.attack ?? 0),
      h("span", { class: `badge def ${live.damage > 0 ? "hurt" : ""}`, title: "Defense left" }, left));
  }
  return el;
}
