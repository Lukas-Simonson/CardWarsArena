// Plays the cues with the Web Audio API: one AudioContext, files decoded once into buffers, so a sound starts at once
// and sounds overlap. Everything the browser supplies is passed in (createEngine), so the rules here are tested with
// stubs; sound.js wires the real ones.
//
// What it does for the player:
//  - Nothing plays until the browser allows it: the context is made on the first click or key press (unlock), and
//    a sound asked for before that is dropped quietly, never an error.
//  - A cue may not repeat within its cooldown, cues of a group share a limit, and only so many sounds play at once
//    (the least important go first), so a board with several hits is not a wall of noise.
//  - Each play varies a little in pitch and loudness.
//  - Nothing plays while the page is hidden.
//  - With no AudioContext, or files that will not load or decode, it does nothing at all.
import { CUES, GROUPS } from "./sounds.js";
import { loudness } from "./soundsettings.js";

export const MAX_VOICES = 8;
const PITCH = 0.04; // each play is up to this much faster or slower
const VOLUME = 0.08; // ...and this much louder or quieter

/**
 * @param {object} io  what the browser provides
 * @param {new () => AudioContext} [io.AudioContextClass]
 * @param {(url:string) => Promise<ArrayBuffer>} io.fetchBytes
 * @param {(type:string) => boolean} io.canPlay          can the browser play this audio type?
 * @param {() => boolean} [io.visible]                   is the page shown?
 * @param {() => number} [io.random]
 * @param {string} [io.base]                             where the files are ("audio/")
 * @param {Record<string, import("./sounds.js").Cue>} [io.cues]
 */
export function createEngine(io) {
  const { AudioContextClass, fetchBytes, canPlay, visible = () => true, random = Math.random, base = "audio/", cues = CUES } = io;
  const formats = canPlay('audio/ogg; codecs="vorbis"') ? ["ogg", "m4a"] : ["m4a", "ogg"];
  let ctx = null, master = null;
  let settings = { volume: 0.6, effects: 1, muted: false, quiet: false };
  const bytes = new Map();    // file -> its bytes, fetched before the context exists
  const buffers = new Map();  // file -> decoded
  const lastStart = new Map(); // cue -> ms of its last start
  const groupStarts = new Map(); // group -> [ms of recent starts]
  let voices = [];             // { end, priority, stop() }
  let unlocking = null;

  const files = () => [...new Set(Object.values(cues).flatMap((c) => c.files))];

  /** Fetch every file's bytes (no context needed): done when the page loads, so they are ready when it may play. */
  async function preload() {
    await Promise.all(files().map(async (f) => {
      if (bytes.has(f)) return;
      try { bytes.set(f, await fetchBytes(`${base}${f}.${formats[0]}`)); } catch { /* it will not play */ }
    }));
  }

  async function decode(file) {
    for (const [i, fmt] of formats.entries()) {
      try {
        // decoding uses the bytes up, so a second format is fetched afresh
        const data = i === 0 && bytes.has(file) ? bytes.get(file) : await fetchBytes(`${base}${file}.${fmt}`);
        bytes.delete(file);
        return await ctx.decodeAudioData(data.slice ? data.slice(0) : data);
      } catch { /* try the other format */ }
    }
    return null;
  }

  /** Make the context (on a user's click or key press) and decode the files. Safe to call again. */
  function unlock() {
    if (unlocking) return unlocking;
    if (!AudioContextClass) return (unlocking = Promise.resolve(false));
    unlocking = (async () => {
      try {
        ctx = new AudioContextClass();
        master = ctx.createGain();
        master.connect(ctx.destination);
        if (ctx.state === "suspended") await ctx.resume();
        await Promise.all(files().map(async (f) => { const b = await decode(f); if (b) buffers.set(f, b); }));
        return ctx.state === "running";
      } catch {
        ctx = null;
        unlocking = null; // not allowed yet (no click or key press): the next one tries again
        return false;
      }
    })();
    return unlocking;
  }

  const ready = () => !!ctx && ctx.state === "running";

  /**
   * Play a cue. `delay` (ms) starts it later, in step with an animation; `gain` scales it. Returns whether it was
   * started, which is false for anything dropped: before the browser allows sound, while muted or hidden, for a
   * non-core cue when the player wants fewer, and for one that repeats too soon or is too much at once.
   */
  function play(cue, { delay = 0, gain = 1 } = {}) {
    const def = cues[cue];
    if (!def || !ready() || settings.muted || !visible()) return false;
    if (settings.quiet && !def.core) return false;
    const have = def.files.filter((f) => buffers.has(f));
    if (!have.length) return false;

    const t0 = ctx.currentTime * 1000 + delay;
    const cooldown = def.cooldown ?? 60;
    if (t0 - (lastStart.get(cue) ?? -Infinity) < cooldown) return false;
    const group = def.group && GROUPS[def.group];
    const recent = group ? (groupStarts.get(def.group) ?? []).filter((t) => Math.abs(t0 - t) < group.window) : [];
    if (group && recent.length >= group.max) return false;

    // only so many at once: the least important makes way, or this one is dropped
    voices = voices.filter((v) => v.end > ctx.currentTime * 1000);
    const priority = def.priority ?? 1;
    if (voices.length >= MAX_VOICES) {
      const weakest = voices.reduce((a, b) => (b.priority < a.priority ? b : a));
      if (weakest.priority >= priority) return false;
      weakest.stop();
      voices = voices.filter((v) => v !== weakest);
    }

    const buffer = buffers.get(have[Math.floor(random() * have.length)]);
    const rate = 1 + (random() * 2 - 1) * PITCH;
    const level = loudness(settings, (def.gain ?? 0.7) * gain) * (1 + (random() * 2 - 1) * VOLUME);
    try {
      const src = ctx.createBufferSource();
      const vol = ctx.createGain();
      src.buffer = buffer;
      src.playbackRate.value = rate;
      vol.gain.value = Math.max(0, Math.min(1, level));
      src.connect(vol);
      vol.connect(master);
      src.start(ctx.currentTime + Math.max(0, delay) / 1000);
      voices.push({ end: t0 + (buffer.duration / rate) * 1000, priority, stop: () => { try { src.stop(); } catch { /* already over */ } } });
    } catch {
      return false;
    }
    lastStart.set(cue, t0);
    if (group) groupStarts.set(def.group, [...recent, t0]);
    return true;
  }

  return {
    preload, unlock, play, ready,
    /** The settings to play by (see soundsettings.js). */
    configure(next) { settings = { ...settings, ...next }; },
    get settings() { return settings; },
    /** For tests and the settings dialog: how many files are decoded. */
    get loaded() { return buffers.size; },
  };
}
