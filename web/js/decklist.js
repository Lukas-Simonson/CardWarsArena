// What the home screen's deck shelf offers: the starter decks, then (when signed in) the player's own.
// Pure: main.js fetches the lists and draws the shelf.
import { fromCards, landscapeRow } from "./deckmodel.js";
import { landscapeName } from "./describe.js";

// The colour of a deck's box and of its Landscape diamonds, as in the design's Lobby board.
const BOX = { cornfield: "#5ea83a", blue_plains: "#3f7fc0", useless_swamp: "#6e4f98", sandy_lands: "#c98a1f", nice_lands: "#c85a8f", icy_lands: "#4aa3b8" };
const DIAMOND = { cornfield: "#7cc74e", blue_plains: "#5aaee8", useless_swamp: "#9b7bbf", sandy_lands: "#f0b84a", nice_lands: "#f08fbc", icy_lands: "#9bdcec" };
const NEUTRAL = "#5e6b78";

export const boxColour = (landscape) => BOX[landscape] ?? NEUTRAL;
export const diamondColour = (landscape) => DIAMOND[landscape] ?? "#d6d0c4";

/** "Blue Plains", or "Cornfield + Blue Plains" for a deck of two types, from its four Landscapes. */
export function landscapeTitle(lands) {
  return [...new Set(lands.filter(Boolean))].map(landscapeName).join(" + ");
}

/** "Hero: Finn", with the kind of deck in front when it is not a starter ("Custom · Hero: Jake"). */
const subtitle = (kind, hero) => [kind, hero && `Hero: ${hero.name}`].filter(Boolean).join(" · ");

/**
 * @param {{set:string,name:string,owner?:string,size:number,landscapes:string[]}[]} presets  playable starter decks from GET /api/decks
 * @param {{id:string,name:string,hero:string,cards:string[],playable:boolean,problems?:string[]}[]} saved  from GET /api/my/decks
 * @param {object} cards  the catalog by id
 * @returns {{starters:Box[], mine:Box[]}}  Box: {value, name, sub, lands, size, disabled, title}
 */
export function shelf(presets, saved, cards) {
  return {
    starters: presets.map((d) => ({
      value: `${d.set} ${d.name}`,
      name: d.name,
      sub: subtitle("", cards[d.hero]),
      lands: d.landscapes,
      size: d.size,
      disabled: false,
      title: landscapeTitle(d.landscapes),
    })),
    // A deck still being built is on the shelf, so it can be found, but cannot be taken.
    mine: saved.map((d) => {
      const lands = landscapeRow(fromCards(d.cards), cards);
      return {
        value: `my:${d.id}`,
        name: d.name || "Untitled deck",
        sub: d.playable ? subtitle("Custom", cards[d.hero]) : "Unfinished",
        lands,
        size: d.cards.length,
        disabled: !d.playable,
        title: d.playable ? landscapeTitle(lands) : (d.problems ?? []).join("; "),
      };
    }),
  };
}

/** The value to select: the one remembered if it is still on the shelf and can be taken, else the first that can. */
export function pickDeck(boxes, remembered) {
  const all = [...boxes.starters, ...boxes.mine].filter((b) => !b.disabled);
  return all.find((b) => b.value === remembered)?.value ?? all[0]?.value ?? "";
}
