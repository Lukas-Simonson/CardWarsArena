// The tokens a Creature can hold, in the order they are drawn. A kind hangs under one of the two stat badges on a tile
// (`under`: "atk" or "def"), shows a pip for each token (up to MAX_PIPS) and says how many in `text`. The server sends
// the counts by kind in `tokens`. Adding a kind, such as burn tokens, is a line here, its message in the catalogue,
// and a colour in the style sheet.
export const MAX_PIPS = 6;

export const TOKEN_KINDS = [
  { kind: "attack", under: "atk", text: (n) => `+${n}`, title: "token.attack" },
  { kind: "burn", under: "def", text: (n) => `🔥${n}`, title: "token.burn" },
];

/** What to draw for a Creature's tokens: only kinds it has some of, in the usual order, ignoring kinds this client does not know. */
export function tokenChips(tokens) {
  return TOKEN_KINDS.filter((k) => tokens?.[k.kind] > 0).map((k) => ({ ...k, n: tokens[k.kind], pips: Math.min(tokens[k.kind], MAX_PIPS), label: k.text(tokens[k.kind]) }));
}
