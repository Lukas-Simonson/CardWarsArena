// The tokens a Creature can hold, in the order they are shown. Each hangs under one of the two stat badges (`under`). Each kind says how it is drawn (a picture, or an icon
// until it has one, and a colour by its class) and how it is read out; the server sends the counts by kind in `tokens`. Adding a kind, such as burn
// tokens, is a line here, its message in the catalogue, and a colour in the style sheet.
export const TOKEN_KINDS = [
  { kind: "attack", icon: "⚔", img: "img/token-attack.webp", under: "atk", title: "token.attack" },
  { kind: "burn", icon: "🔥", img: "img/token-burn.webp", under: "def", title: "token.burn" },
];

/** The chips to draw for a Creature's tokens: only kinds it has some of, in the usual order, ignoring kinds this client does not know. */
export function tokenChips(tokens) {
  return TOKEN_KINDS.filter((k) => tokens?.[k.kind] > 0).map((k) => ({ ...k, n: tokens[k.kind] }));
}
