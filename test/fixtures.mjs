// Hand-built protocol objects shaped like the server's.
export const cards = {
  "the-pig": { id: "the-pig", name: "The Pig", type: "creature", landscape: "rainbow", cost: 1, attack: 1, defense: 4, text: "FLOOP >>> Flip target Cornfield Landscape in this Lane face down." },
  "cool-dog": { id: "cool-dog", name: "Cool Dog", type: "creature", landscape: "blue_plains", cost: 2, attack: 2, defense: 7, text: "Your Creatures on adjacent Lanes may not be Attacked." },
  "corn-scepter": { id: "corn-scepter", name: "Corn Scepter", type: "spell", landscape: "rainbow", cost: 1, text: "Deal 1 Damage to target Creature for each Cornfield Landscape you control." },
  "blood-fortress": { id: "blood-fortress", name: "Blood Fortress", type: "building", landscape: "rainbow", cost: 1, text: "Your Creature in this Lane has +1 ATK." },
  "field-reaper": { id: "field-reaper", name: "Field Reaper", type: "creature", landscape: "cornfield", cost: 2, attack: 1, defense: 4, text: "…" },
  "uni-knight": { id: "uni-knight", name: "Uni-Knight", type: "creature", landscape: "blue_plains", cost: 1, attack: 3, defense: 4, text: "Pay 1 Action >>> Target Creature in this Lane has -10 ATK this turn." },
};

const lane = (landscape, creature = null, building = null, face_down = false) => ({ landscape, face_down, creature, building });
const creature = (uid, card, over = {}) => ({ uid, card, owner: 0, damage: 0, status: "ready", attack: 1, defense: 4, ...over });

/** A mid-game view for seat 0 (Finn, blue plains) against seat 1 (Jake, cornfield). */
export function view(over = {}) {
  return {
    you: 0, phase: "main", turn: 4, active: 0, first: 1, actions_left: 2, spells_this_turn: 0,
    over: false, awaiting: [0], legal: [],
    players: [
      {
        hp: 25, deck_count: 30, hand_count: 3, kept: true, mulliganed: false,
        hand: [{ uid: 10, card: "cool-dog", owner: 0 }, { uid: 11, card: "corn-scepter", owner: 0 }, { uid: 12, card: "blood-fortress", owner: 0 }],
        discard: [{ uid: 13, card: "uni-knight", owner: 0 }],
        lanes: [
          lane("blue_plains", creature(20, "the-pig")),
          lane("blue_plains", null, { uid: 21, card: "blood-fortress", owner: 0, status: "ready" }),
          lane("blue_plains", creature(22, "uni-knight", { attack: 3 })),
          lane("blue_plains"),
        ],
      },
      {
        hp: 22, deck_count: 28, hand_count: 4, kept: true, mulliganed: false, discard: [],
        lanes: [
          lane("cornfield", creature(30, "cool-dog", { owner: 1, damage: 2 })),
          lane("cornfield", null, null, true),
          lane("cornfield"),
          lane("cornfield"),
        ],
      },
    ],
    ...over,
  };
}

export const ctxFor = (v = view(), extra = {}) => ({
  view: v, cards, names: ["Finn", "Jake"],
  known: { 10: "cool-dog", 11: "corn-scepter", 12: "blood-fortress", 13: "uni-knight", 20: "the-pig", 21: "blood-fortress", 22: "uni-knight", 30: "cool-dog" },
  ...extra,
});
