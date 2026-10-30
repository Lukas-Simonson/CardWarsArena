// Draws one card: the printed scan from the catalog's image_url.
import { h } from "./dom.js";

/** What stands in for a scan that is missing or fails to load: just the name. */
const placeholder = (card) => h("div", { class: "noart" }, card.name ?? card.id);

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
      onerror: () => { img.remove(); el.prepend(placeholder(c)); },
    });
    el.append(img);
  } else {
    el.append(placeholder(c));
  }
  if (live && c.type === "creature") {
    const left = (live.def ?? c.defense ?? 0) - (live.damage ?? 0);
    el.append(
      h("span", { class: `badge atk ${live.atk > (c.attack ?? 0) ? "up" : ""}`, title: "Attack" }, live.atk ?? c.attack ?? 0),
      h("span", { class: `badge def ${live.damage > 0 ? "hurt" : ""}`, title: "Defense left" }, left));
  }
  return el;
}
