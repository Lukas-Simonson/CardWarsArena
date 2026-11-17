// What is heard for what happens. Pure: no audio, no DOM. The cues are named after what happens in a match or in the
// interface; each lists the files it may use (a variant is picked at random, so a repeated sound is less tiring) and how
// it behaves (loudness, how soon it may repeat, how it ranks when many sound at once, and whether it is a core sound
// that stays when the player asks for fewer). soundsFor() turns the effects anim.js plans for an update into the
// cues to play and when, so what is heard stays in step with what is seen and with the pacing.
//
// The files are in audio/ (two formats each, .ogg and .m4a) and every one is credited in audio/CREDITS.md.

/**
 * @typedef {object} Cue
 * @property {string[]} files   audio/<name>.ogg|.m4a, one is picked at random
 * @property {number} [gain]    loudness, 0..1 (default 0.7)
 * @property {number} [cooldown] the least time between two plays of this cue, in ms (default 60)
 * @property {string} [group]   cues of a group share a limit: at most `max` starts within `window` ms
 * @property {number} [priority] when too many sounds play at once, higher ones stay (default 1)
 * @property {boolean} [core]   kept when the player asks for fewer sounds
 */

/** @type {Record<string, Cue>} */
export const CUES = {
  // interface
  ui_click: { files: ["ui-click1", "ui-click2"], gain: 0.5, cooldown: 40 },
  ui_select: { files: ["ui-rollover4", "ui-rollover5"], gain: 0.5, cooldown: 40 },
  ui_back: { files: ["interface-back_001", "interface-back_002"], gain: 0.5, cooldown: 80 },
  ui_error: { files: ["interface-error_001", "interface-error_007"], gain: 0.6, cooldown: 250, core: true, priority: 3 },
  ui_open: { files: ["interface-open_001", "interface-open_002"], gain: 0.5, cooldown: 120 },
  ui_close: { files: ["interface-close_001", "interface-close_002"], gain: 0.5, cooldown: 120 },
  ui_confirm: { files: ["interface-confirmation_001", "interface-confirmation_002"], gain: 0.6, cooldown: 200 },
  room_joined: { files: ["interface-maximize_004"], gain: 0.6, cooldown: 500 },
  opponent_joined: { files: ["interface-pluck_001", "interface-pluck_002"], gain: 0.55, cooldown: 500 },
  // the match
  game_start: { files: ["casino-card-shuffle"], gain: 0.7, cooldown: 1000, core: true, priority: 3 },
  draw: { files: ["casino-card-slide-1", "casino-card-slide-2", "casino-card-slide-3", "casino-card-slide-4"], gain: 0.6, cooldown: 90 },
  draw_opp: { files: ["casino-card-slide-5", "casino-card-slide-6", "casino-card-slide-7", "casino-card-slide-8"], gain: 0.4, cooldown: 90 },
  shuffle: { files: ["casino-card-fan-1", "casino-card-fan-2"], gain: 0.6, cooldown: 300 },
  reveal: { files: ["casino-cards-pack-take-out-1", "casino-cards-pack-take-out-2"], gain: 0.6, cooldown: 300 },
  play_creature: { files: ["casino-card-place-1", "casino-card-place-2", "casino-card-place-3", "casino-card-place-4"], gain: 0.7, cooldown: 120, priority: 2 },
  play_building: { files: ["interface-drop_002", "interface-drop_003", "interface-drop_004"], gain: 0.6, cooldown: 150, priority: 2 },
  cast: { files: ["casino-cards-pack-open-1", "casino-cards-pack-open-2"], gain: 0.6, cooldown: 400, priority: 2 },
  discard: { files: ["casino-card-shove-1", "casino-card-shove-2", "casino-card-shove-3", "casino-card-shove-4"], gain: 0.5, cooldown: 120 },
  destroy: { files: ["impact-impactGlass_medium_000", "impact-impactGlass_medium_001", "impact-impactGlass_medium_002"], gain: 0.7, cooldown: 100, group: "hits", core: true, priority: 3 },
  attack: { files: ["rpg-knifeSlice", "rpg-knifeSlice2", "rpg-drawKnife1", "rpg-drawKnife2"], gain: 0.6, cooldown: 150, priority: 2 },
  hit_small: { files: ["impact-impactSoft_medium_000", "impact-impactSoft_medium_001", "impact-impactSoft_medium_002"], gain: 0.7, cooldown: 60, group: "hits", core: true, priority: 2 },
  hit_medium: { files: ["impact-impactPunch_medium_000", "impact-impactPunch_medium_001", "impact-impactPunch_medium_002"], gain: 0.75, cooldown: 60, group: "hits", core: true, priority: 3 },
  hit_big: { files: ["impact-impactPunch_heavy_000", "impact-impactPunch_heavy_001", "impact-impactPunch_heavy_002"], gain: 0.85, cooldown: 60, group: "hits", core: true, priority: 3 },
  hero_hit: { files: ["impact-impactPlate_medium_000", "impact-impactPlate_medium_001", "impact-impactPlate_medium_002"], gain: 0.7, cooldown: 120, core: true, priority: 3 },
  heal: { files: ["interface-confirmation_003", "interface-select_003"], gain: 0.5, cooldown: 150, priority: 2 },
  heal_hero: { files: ["interface-select_004", "interface-select_005"], gain: 0.5, cooldown: 200, priority: 2 },
  floop: { files: ["ui-switch1", "ui-switch2", "ui-switch3"], gain: 0.55, cooldown: 200, priority: 2 },
  ability: { files: ["casino-chip-lay-1", "casino-chip-lay-2", "casino-chip-lay-3"], gain: 0.55, cooldown: 200, priority: 2 },
  freeze: { files: ["interface-glass_001", "interface-glass_004"], gain: 0.55, cooldown: 150 },
  turn_yours: { files: ["interface-confirmation_004"], gain: 0.65, cooldown: 800, core: true, priority: 3 },
  turn_theirs: { files: ["interface-question_001"], gain: 0.55, cooldown: 800, core: true, priority: 3 },
  fight: { files: ["interface-maximize_006"], gain: 0.6, cooldown: 800, core: true, priority: 2 },
  win: { files: ["casino-chips-handle-5", "casino-chips-handle-1"], gain: 0.75, cooldown: 2000, core: true, priority: 4 },
  lose: { files: ["interface-error_005", "interface-error_006"], gain: 0.75, cooldown: 2000, core: true, priority: 4 },
};

/** At most this many sounds of one group may start within `window` ms: several hits at once are not a wall of noise. */
export const GROUPS = { hits: { max: 3, window: 160 } };

/** The least a hit that big (in damage) sounds heavier than the one below it. */
export const HIT_MEDIUM = 3;
export const HIT_BIG = 6;

/** Which hit sound a blow of this many points makes. */
export const hitCue = (amount) => (amount >= HIT_BIG ? "hit_big" : amount >= HIT_MEDIUM ? "hit_medium" : "hit_small");

/** Every file the cues use, once each. */
export const allFiles = () => [...new Set(Object.values(CUES).flatMap((c) => c.files))].sort();

/**
 * The sounds for one planned effect (see plan() in anim.js), as [{ cue, delay, gain? }]: gain scales the cue's own.
 * Effects with nothing to hear give [].
 */
export function soundsOf(f, ctx = {}) {
  const at = f.delay ?? 0;
  const one = (cue, extra = {}) => [{ cue, delay: at, ...extra }];
  switch (f.t) {
    case "drop": return one("play_creature");
    case "build": return one("play_building");
    case "cast": return one("cast");
    case "lunge": return one("attack");
    case "hit": return one(hitCue(f.amount ?? 0));
    case "hp": return one("hero_hit");
    case "heal": return one("heal");
    case "hpgain": return one("heal_hero");
    case "ghost": return one("destroy");
    case "deal": return one("draw");
    case "oppdraw": return one("draw_opp");
    case "turn": return one(f.mine ? "turn_yours" : "turn_theirs");
    case "discard": return one("discard");
    case "floop": return one("floop");
    case "ability": return one("ability");
    case "shuffle": return one("shuffle");
    case "reveal": return one("reveal");
    case "freeze": return one("freeze");
    case "fight": return one("fight");
    case "over": return one(f.won ? "win" : "lose");
    default: return [];
  }
}

/** The sounds for a list of planned effects, in the order they were planned. */
export const soundsFor = (fx, ctx = {}) => fx.flatMap((f) => soundsOf(f, ctx));
