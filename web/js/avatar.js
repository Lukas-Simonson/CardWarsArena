// A player's picture: the image their sign-in provider has for them, or the first letter of their name.
import { h } from "./dom.js";

/** The letter shown when there is no picture, or it does not load. */
export const initialOf = (name) => (name ?? "").trim()[0]?.toUpperCase() ?? "?";

/**
 * Fill an avatar box. The letter is there at once and stays if there is no picture or it fails to load;
 * the picture covers it once it has loaded.
 * @param {HTMLElement} box
 * @param {string} name   the player's name
 * @param {string} [url]  the picture's address
 */
export function fillAvatar(box, name, url) {
  box.replaceChildren(initialOf(name));
  if (!url) return;
  const img = h("img", { src: url, alt: "", draggable: "false", referrerpolicy: "no-referrer", decoding: "async" });
  img.addEventListener("load", () => box.classList.add("pic"));
  img.addEventListener("error", () => img.remove());
  box.append(img);
}

/** A new avatar box for a seat of the game. */
export function avatarEl(name, url, attrs = {}) {
  const box = h("div", { class: "well avatar", "aria-hidden": "true", ...attrs });
  fillAvatar(box, name, url);
  return box;
}
