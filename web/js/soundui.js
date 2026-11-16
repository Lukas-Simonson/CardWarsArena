// The sound control: a button in the lobby and in a match that opens a small dialog with the volume, the effects level,
// mute and "fewer sounds". The choices are kept in the browser (see soundsettings.js).
import { h, svg } from "./dom.js";
import { t } from "./i18n.js";
import { sound } from "./sound.js";

const SPEAKER = '<path d="M4 9v6h4l5 4V5L8 9H4z" fill="currentColor"/><path d="M16 9c1.5 1.5 1.5 4.5 0 6M18.5 6.5c3 3 3 8 0 11" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round"/>';
const MUTED = '<path d="M4 9v6h4l5 4V5L8 9H4z" fill="currentColor"/><path d="M16 9l5 6M21 9l-5 6" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round"/>';

/** The speaker button: shows whether sound is on, opens the dialog. */
export function soundButton() {
  const muted = sound.settings.muted;
  const btn = h("button", {
    type: "button", class: "btn-steel soundbtn", "aria-label": t("sound.button"), title: t("sound.button"), "aria-haspopup": "dialog",
    onclick: openSoundSettings,
  }, svg(muted ? MUTED : SPEAKER, { viewBox: "0 0 24 24" }));
  btn.hidden = !sound.available;
  return btn;
}

/** The settings dialog. */
export function openSoundSettings() {
  const s = sound.settings;
  const pct = (v) => String(Math.round(v * 100));
  const slider = (id, label, value, on) => {
    const out = h("output", { for: id }, `${pct(value)}%`);
    const input = h("input", { id, type: "range", min: "0", max: "100", step: "5", value: pct(value), "aria-valuetext": `${pct(value)}%`,
      oninput: () => { out.textContent = `${input.value}%`; input.setAttribute("aria-valuetext", `${input.value}%`); on(Number(input.value) / 100); } });
    return h("div", { class: "field soundrow" }, h("label", { for: id, class: "ink" }, label), input, out);
  };
  const check = (id, label, checked, on) => {
    const input = h("input", { id, type: "checkbox", checked, onchange: () => on(input.checked) });
    return h("label", { class: "ink soundcheck", for: id }, input, " ", label);
  };
  const dlg = h("dialog", { class: "modal parch deckdialog sounddialog", "aria-labelledby": "sound-title", "data-nosound": "" },
    h("h2", { id: "sound-title" }, t("sound.title")),
    check("snd-mute", t("sound.mute"), s.muted, (v) => { sound.update({ muted: v }); }),
    slider("snd-volume", t("sound.volume"), s.volume, (v) => sound.update({ volume: v })),
    slider("snd-effects", t("sound.effects"), s.effects, (v) => sound.update({ effects: v })),
    check("snd-quiet", t("sound.quiet"), s.quiet, (v) => sound.update({ quiet: v })),
    h("p", { class: "ink small" }, t("sound.note")),
    h("div", { class: "modalrow modalrow2" },
      h("button", { type: "button", class: "btn-steel", onclick: () => { sound.play("hit_medium"); sound.play("turn_yours", { delay: 250 }); } }, t("sound.test")),
      h("button", { type: "button", class: "btn-green", onclick: () => dlg.close() }, t("common.close"))));
  dlg.addEventListener("close", () => { dlg.remove(); for (const b of document.querySelectorAll(".soundbtn")) b.replaceWith(soundButton()); });
  dlg.addEventListener("click", (e) => { if (e.target === dlg) dlg.close(); });
  document.body.append(dlg);
  dlg.showModal();
}
