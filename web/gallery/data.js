// Every question the engine can ask, in the shapes the server sends it (see server/docs/protocol.md, "Questions"), in
// the states that stress its layout: with and without a skip, one option and many, a long name, an empty pile. The
// gallery page (gallery/index.html) draws them; test/gallery.test.mjs fails when an engine kind has no entry here, so a new
// question has to be added to this list (docs: server/docs/effects.md, "Adding a question").
//
// `kind` is the engine's Choice kind (server/internal/engine/state.go). `mine: false` shows what the opponent sees.

const card = (id, name, type = "creature", landscape = "rainbow", over = {}) => ({ id, name, type, landscape, cost: 1, attack: 1, defense: 3, text: "", ...over });
export const cards = Object.fromEntries([
  card("impossible-possum", "Impossible Possum"), card("steakchop", "Steakchop"), card("blood-bath", "Blood Bath", "spell"),
  card("ogre-gas", "Ogre Gas", "spell"), card("deforestation", "Deforestation", "spell"), card("pied-piper", "Pied Piper"),
  card("genteel-armourdillo", "Genteel Armourdillo"), card("friendship-bracelet", "Friendship Bracelet", "building"),
  card("tired-wombat", "Tired Wombat"), card("carmel-camel", "Carmel Camel"), card("silo-of-freedom", "Silo of Freedom", "building"),
  card("frozen-fish", "Frozen Fish"), card("the-big-hen-house", "The Big Hen House", "building"), card("abraca-amadeus", "Abraca Amadeus"),
  card("the-pig", "The Pig"), card("cool-dog", "Cool Dog"), card("corn-scepter", "Corn Scepter", "spell"),
  card("blood-fortress", "Blood Fortress", "building"), card("field-reaper", "Field Reaper"),
  card("long-name", "Her Royal Highness the Magnificent Princess Bubblegum of the Candy Kingdom"),
].map((c) => [c.id, c]));

const msg = (key, params) => ({ key, ...(params && { params }) });
const opt = (id, rest) => ({ id, ...rest });
const hand = (id, uid) => opt(id, { ref: { kind: "hand_card", player: 0, card: uid } });
const creature = (id, lane) => opt(id, { ref: { kind: "creature", player: 0, lane } });
const pick = (id, cardId, uid = id + 40) => opt(id, { card: { uid, card: cardId, owner: 0 } });
const q = (card, text, answer, options, rest = {}) => ({ player: 0, card, text, answer, options, ...rest });

export const scenarios = [
  { kind: "discard", title: "Discard cards", note: "Click the hand; the count goes down as cards are discarded.",
    q: q("abraca-amadeus", msg("question.discard", { n: 2 }), "hand", [hand(0, 10), hand(1, 11), hand(2, 12)], { left: 2 }) },
  { kind: "discard", title: "Discard, one card left to pick", q: q("abraca-amadeus", msg("question.discard", { n: 1 }), "hand", [hand(0, 10), hand(1, 11)], { left: 1 }) },
  { kind: "ransom", title: "Ransom: discard or take the effect", note: "A5: the red button says what it costs.",
    q: q("blood-bath", msg("question.ransom", { min: 2 }), "hand", [hand(0, 10), opt(1, { label: msg("question.take_it"), role: "danger", result: msg("result.damage_hero", { n: 5 }) })]) },
  { kind: "ransom", title: "Ransom, no cost to discard", q: q("blood-bath", msg("question.ransom", { min: 0 }), "hand", [hand(0, 10), hand(1, 11), opt(2, { label: msg("question.take_it"), role: "danger", result: msg("result.damage_creatures", { n: 2 }) })]) },
  { kind: "ransom", title: "Ransom, effect not described", q: q("blood-bath", msg("question.ransom", { min: 0 }), "hand", [hand(0, 10), opt(1, { label: msg("question.take_it"), role: "danger" })]) },
  { kind: "upkeep", title: "Upkeep: discard or destroy it", q: q("steakchop", msg("question.upkeep", { n: 1 }), "hand", [hand(0, 10), hand(1, 11), opt(2, { label: msg("question.destroy"), role: "danger" })]) },
  { kind: "rebound", title: "Rebound: discard to return it", note: "The skip is neutral, not red.",
    q: q("impossible-possum", msg("question.rebound"), "hand", [hand(0, 10), hand(1, 11)], { skip: opt(2, { label: msg("question.leave_it") }) }) },
  { kind: "reveal", title: "Reveal: pick one of the top cards", q: q("ogre-gas", msg("question.reveal"), "cards", [pick(0, "the-pig"), pick(1, "cool-dog"), pick(2, "corn-scepter")]) },
  { kind: "reveal", title: "Reveal, one card", q: q("ogre-gas", msg("question.reveal"), "cards", [pick(0, "the-pig")]) },
  { kind: "reveal", title: "Reveal, long card name", q: q("ogre-gas", msg("question.reveal"), "cards", [pick(0, "long-name"), pick(1, "cool-dog"), pick(2, "field-reaper")]) },
  { kind: "search", title: "Search the deck for a Building", q: q("deforestation", msg("question.search"), "cards", [pick(0, "blood-fortress"), pick(1, "friendship-bracelet"), pick(2, "silo-of-freedom")]) },
  { kind: "search", title: "Search, nothing to find", note: "An empty pile: the pop-up has no cards. The engine does not ask this, but the pop-up must not break.", q: q("deforestation", msg("question.search"), "cards", []) },
  { kind: "place", title: "Place a Building in a lane", q: q("deforestation", msg("question.place"), "board", [0, 1, 2, 3].map((l) => opt(l, { ref: { kind: "lane", player: 0, lane: l } })), { subject: { uid: 60, card: "blood-fortress", owner: 0 } }) },
  { kind: "redirect", title: "Redirect an attack", q: q("pied-piper", msg("question.redirect"), "board", [creature(0, 0), creature(1, 2)], { skip: opt(2, { label: msg("question.redirect_skip") }) }) },
  { kind: "defend", title: "Take an attack instead", q: q("genteel-armourdillo", msg("question.defend"), "board", [creature(0, 1)], { subject: { uid: 61, card: "cool-dog", owner: 1 }, skip: opt(1, { label: msg("question.defend_skip") }) }) },
  { kind: "move_damage", title: "Move damage between two Creatures", note: "Each amount shows both Creatures' damage after.",
    q: q("friendship-bracelet", msg("question.move_damage", { from_card: "the-pig", to_card: "cool-dog" }), "buttons", [1, 2, 3].map((n) => opt(n - 1, { label: msg("question.move_n", { n }), result: msg("question.move_result", { from_card: "the-pig", a1: 3, a2: 3 - n, to_card: "cool-dog", b1: 0, b2: n }) }))) },
  { kind: "move", title: "Move to an empty Landscape (mine)", q: q("tired-wombat", msg("question.move_mine"), "board", [1, 3].map((l, i) => opt(i, { ref: { kind: "landscape", player: 0, lane: l } }))) },
  { kind: "move", title: "Move to an empty Landscape (theirs)", q: q("tired-wombat", msg("question.move_theirs"), "board", [0, 1, 2, 3].map((l) => opt(l, { ref: { kind: "landscape", player: 1, lane: l } }))) },
  { kind: "camel", title: "Put a leaving Creature under a Camel", q: q("carmel-camel", msg("question.camel"), "buttons", [opt(0, { label: msg("question.camel_put") })], { subject: { uid: 62, card: "the-pig", owner: 0 }, skip: opt(1, { label: msg("question.camel_skip") }) }) },
  { kind: "camel", title: "Camel, two Camels", q: q("carmel-camel", msg("question.camel"), "buttons", [opt(0, { label: msg("question.camel_put_lane", { lane: 1 }) }), opt(1, { label: msg("question.camel_put_lane", { lane: 3 }) })], { subject: { uid: 62, card: "the-pig", owner: 0 }, skip: opt(2, { label: msg("question.camel_skip") }) }) },
  { kind: "landscape", title: "Choose a Landscape type", q: q("silo-of-freedom", msg("question.landscape", { lane: 2 }), "buttons", ["cornfield", "blue_plains", "useless_swamp", "nice_lands", "sandy_lands", "icy_lands", "lava_flats"].map((t, i) => opt(i, { label: msg(`landscape.${t}`) }))) },
  { kind: "target", title: "Trigger target, optional, with its own skip words", q: q("frozen-fish", msg("question.target", { optional: true, what: "landscape", label: "Freeze" }), "board", [0, 1, 2].map((l) => opt(l, { ref: { kind: "landscape", player: 1, lane: l } })), { skip: opt(3, { label: { text: "Don't freeze a Landscape" } }) }) },
  { kind: "target", title: "Trigger target, mandatory", q: q("frozen-fish", msg("question.target", { optional: false, what: "creature" }), "board", [creature(0, 0), creature(1, 2)]) },
  { kind: "target", title: "Trigger target in a pile", q: q("frozen-fish", msg("question.target", { optional: true, what: "discard_card" }), "board", [opt(0, { ref: { kind: "discard_card", player: 0, card: 13 } })], { skip: opt(1, { label: { text: "Don't return a Creature" } }) }) },
  { kind: "amount", title: "Choose an amount", q: q("the-big-hen-house", msg("question.amount", { options: [1, 2] }), "buttons", [1, 2].map((n, i) => opt(i, { label: msg("question.amount_n", { n, unit: "damage" }) }))) },
  { kind: "discard", title: "What the opponent sees", note: "Only the seat that decides gets the options; the other sees the sentence.", mine: false,
    q: { ...q("abraca-amadeus", msg("question.discard", { n: 2 }), "hand", []), player: 1 } },
];
