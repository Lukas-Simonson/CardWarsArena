// What the lobby's deck menu offers: the starter decks, then (when signed in) the player's own.
// Pure: main.js fetches the lists and builds the <select>.

/**
 * @param {{set:string,name:string,owner?:string}[]} presets  playable starter decks from GET /api/decks
 * @param {{id:string,name:string,playable:boolean,problems?:string[]}[]} saved  from GET /api/my/decks
 * @returns {{starters:{label:string,value:string}[], mine:{label:string,value:string,disabled:boolean,title:string}[]}}
 */
export function deckOptions(presets, saved) {
  return {
    starters: presets.map((d) => ({ label: `${d.owner || d.name} — ${d.name}`, value: `${d.set} ${d.name}` })),
    // A deck still being built is listed, so it can be found, but cannot be chosen.
    mine: saved.map((d) => ({
      label: d.playable ? d.name : `${d.name} (unfinished)`,
      value: `my:${d.id}`,
      disabled: !d.playable,
      title: d.playable ? "" : (d.problems ?? []).join("; "),
    })),
  };
}

/** The value to select: the one remembered if it is still on offer and can be chosen, else the first that can. */
export function pickDeck(options, remembered) {
  const all = [...options.starters, ...options.mine].filter((o) => !o.disabled);
  return all.find((o) => o.value === remembered)?.value ?? all[0]?.value ?? "";
}
