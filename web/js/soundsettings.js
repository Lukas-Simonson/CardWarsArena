// The player's sound choices, kept in the browser (not on the account). Pure: storage is passed in.

export const KEY = "cw.sound";

/**
 * volume  0..1, everything     effects 0..1, the sound effects     muted, all sound off
 * quiet   only the core sounds (turns, hits, deaths, errors, the end of the game)
 */
export const defaults = ({ reducedMotion = false } = {}) => ({ volume: 0.6, effects: 1, muted: false, quiet: reducedMotion });

const level = (v, fallback) => (typeof v === "number" && Number.isFinite(v) ? Math.min(1, Math.max(0, v)) : fallback);

/** A settings object made safe: unknown or broken values fall back to the defaults. */
export function normalize(raw, base = defaults()) {
  const s = raw && typeof raw === "object" ? raw : {};
  return {
    volume: level(s.volume, base.volume),
    effects: level(s.effects, base.effects),
    muted: typeof s.muted === "boolean" ? s.muted : base.muted,
    quiet: typeof s.quiet === "boolean" ? s.quiet : base.quiet,
  };
}

/** What is saved, or the defaults when nothing is (or storage is blocked or holds rubbish). */
export function load(storage, base = defaults()) {
  try {
    return normalize(JSON.parse(storage?.getItem(KEY) ?? "null"), base);
  } catch {
    return base;
  }
}

/** Save the settings; a browser that will not keep them is not an error. */
export function save(storage, settings) {
  try { storage?.setItem(KEY, JSON.stringify(settings)); } catch { /* private window, blocked storage */ }
}

/** The loudness a cue is played at: the master volume, the effects level and the cue's own, or 0 when muted. */
export const loudness = (s, cueGain = 1) => (s.muted ? 0 : s.volume * s.effects * cueGain);
