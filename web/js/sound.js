// The game's sound, wired to the browser: the engine (audio.js) with the real AudioContext, fetch and page visibility,
// the player's settings kept in localStorage, and the rules for when to unlock. Everything else asks this module to
// play a cue or a list of planned effects; it never throws, and it does nothing without audio support.
import { createEngine } from "./audio.js";
import { soundsFor } from "./sounds.js";
import * as saved from "./soundsettings.js";

const storage = () => { try { return globalThis.localStorage; } catch { return null; } };
const reducedMotion = () => !!globalThis.matchMedia?.("(prefers-reduced-motion: reduce)").matches;

const engine = createEngine({
  AudioContextClass: globalThis.AudioContext ?? globalThis.webkitAudioContext,
  fetchBytes: async (url) => {
    const res = await fetch(url);
    if (!res.ok) throw new Error(`${url}: ${res.status}`);
    return res.arrayBuffer();
  },
  canPlay: (type) => { try { return document.createElement("audio").canPlayType(type) !== ""; } catch { return false; } },
  visible: () => !globalThis.document?.hidden,
});

let settings = saved.load(storage(), saved.defaults({ reducedMotion: reducedMotion() }));
engine.configure(settings);
const listeners = new Set();

export const sound = {
  /** Play one cue (see sounds.js). Quiet and safe whatever the browser allows. */
  play(cue, opts) { try { return engine.play(cue, opts); } catch { return false; } },
  /** Play the sounds for the effects anim.js planned for an update. */
  playEffects(fx, ctx) { for (const s of soundsFor(fx, ctx)) this.play(s.cue, { delay: s.delay, gain: s.gain }); },
  get settings() { return settings; },
  /** Change the settings (a partial object), remember them, and tell the listeners. */
  update(partial) {
    settings = saved.normalize({ ...settings, ...partial }, settings);
    engine.configure(settings);
    saved.save(storage(), settings);
    for (const f of listeners) f(settings);
  },
  onChange(fn) { listeners.add(fn); return () => listeners.delete(fn); },
  /** Is there any way to play sound here? (A settings dialog is pointless without.) */
  get available() { return !!(globalThis.AudioContext ?? globalThis.webkitAudioContext); },
};

if (typeof document !== "undefined") {
  engine.preload();
  // Browsers keep a page silent until the player has done something on it: the first click or key press unlocks it.
  const unlock = () => { engine.unlock().then((ok) => { if (ok) { for (const type of ["pointerdown", "keydown"]) document.removeEventListener(type, unlock, true); } }); };
  for (const type of ["pointerdown", "keydown"]) document.addEventListener(type, unlock, true);

  // Buttons click; cards you point at select; a dialog opening or closing has its own sound.
  document.addEventListener("click", (e) => {
    const el = e.target?.closest?.("button, a.btn-steel, a.btn-purple, [role=radio]");
    if (!el || el.disabled || el.closest("[data-nosound]")) return;
    sound.play(el.matches(".hbtn, .tile, .strip, .deckbox") ? "ui_select" : "ui_click");
  });
  const showModal = globalThis.HTMLDialogElement?.prototype?.showModal;
  if (showModal) {
    globalThis.HTMLDialogElement.prototype.showModal = function (...args) {
      sound.play("ui_open");
      return showModal.apply(this, args);
    };
    document.addEventListener("close", (e) => { if (e.target?.tagName === "DIALOG") sound.play("ui_close"); }, true);
  }
}
