// The gallery (gallery/index.html) is the list of every question, and these tests are what make it complete: an engine kind
// with no entry fails here, and so does a pop-up built outside the shared components.
import { test } from "node:test";
import assert from "node:assert/strict";
import { readFileSync, readdirSync } from "node:fs";
import { scenarios, cards } from "../gallery/data.js";
import { fromQuestion, SURFACE } from "../js/question.js";
import { t, has } from "../js/i18n.js";

const read = (rel) => readFileSync(new URL(rel, import.meta.url), "utf8");
const engineKinds = [...read("../../server/internal/engine/state.go").matchAll(/^\tChoice\w+\s*=\s*"(\w+)"/gm)].map((m) => m[1]);
const asked = (s) => fromQuestion({ view: { you: s.q.player, over: false }, cards }, s.q, true);

test("every kind of engine choice has an entry in the gallery", () => {
  assert.ok(engineKinds.length >= 15, `found only ${engineKinds}`);
  const shown = new Set(scenarios.map((s) => s.kind));
  assert.deepEqual(engineKinds.filter((k) => !shown.has(k)), [], "add these to web/gallery/data.js (server/docs/effects.md, \"Adding a question\")");
  assert.deepEqual([...shown].filter((k) => !engineKinds.includes(k)), [], "a gallery entry for a kind the engine does not have");
});

test("the gallery shows the states that stress a layout", () => {
  const qs = scenarios.map((s) => s.q);
  assert.ok(qs.some((q) => q.skip), "a question with a skip");
  assert.ok(qs.some((q) => !q.skip && q.options.length), "a question with no skip");
  assert.ok(qs.some((q) => q.options.length === 1), "a question with one option");
  assert.ok(qs.some((q) => q.options.length >= 6), "a question with many options");
  assert.ok(qs.some((q) => q.answer === "cards" && q.options.length === 0), "an empty pile");
  assert.ok(qs.some((q) => q.options.some((o) => o.card && cards[o.card.card].name.length > 40)), "a long card name");
  assert.ok(scenarios.some((s) => s.mine === false), "what the opponent sees");
});

test("every question in the gallery has the standard parts", () => {
  for (const s of scenarios) {
    const q = asked(s), where = `${s.kind}: ${s.title}`;
    assert.ok(q, `${where}: not drawn`);
    assert.ok(q.source?.name, `${where}: no source card (A1)`);
    assert.ok(q.text && !/^question\.|\{/.test(q.text), `${where}: no sentence, or one that is not worded (A2): ${q.text}`);
    assert.ok(Object.values(SURFACE).includes(q.surface), `${where}: no surface`);
    if (s.q.skip) assert.equal(q.skip.label.includes("question."), false, `${where}: skip not worded`);
    if (s.mine !== false) {
      const answers = q.options.length + Object.keys(q.boardPicks).length + Object.keys(q.handPicks).length + (q.skip ? 1 : 0);
      if (s.q.options.length) assert.ok(answers > 0, `${where}: nothing to answer with`);
    }
    for (const o of q.options) assert.ok(o.label && !o.label.includes("question."), `${where}: an option without words (A3)`);
  }
});

test("only cards that are not on the table are shown in a pop-up (W4)", () => {
  for (const s of scenarios) {
    const q = asked(s);
    if (q.surface === SURFACE.popup) assert.equal(s.q.answer, "cards", `${s.title}: a pop-up for a question that is not answered with cards`);
    if (s.q.answer === "cards") assert.equal(q.surface, SURFACE.popup, s.title);
  }
});

test("every sentence the gallery uses is in the catalogue", () => {
  const walk = (m) => { if (m?.key) assert.ok(has(m.key), `missing message ${m.key}`); };
  for (const s of scenarios) for (const o of [...s.q.options, s.q.skip].filter(Boolean)) { walk(o.label); walk(o.result); walk(s.q.text); }
});

// ---- the shared components: a surface is built in one place

test("a pop-up or a button row is built only by the shared question components", () => {
  const js = new URL("../js/", import.meta.url);
  const allowed = { "questionui.js": ["modal-backdrop", "qpopup", "qbar", "qopt"], "render.js": ["modal-backdrop"] }; // render.js: the pile viewer, which is not a question
  for (const f of readdirSync(js).filter((f) => f.endsWith(".js"))) {
    const src = read(`../js/${f}`);
    for (const word of ["modal-backdrop", "qpopup", "qbar", "qopt"]) {
      if (src.includes(`"${word}`) || src.includes(`class: \`${word}`) || src.includes(`${word}"`)) {
        const ok = (allowed[f] ?? []).includes(word);
        assert.ok(ok, `${f} builds "${word}" itself: draw the question with questionBar/questionPopup (web/docs/interactions.md)`);
      }
    }
  }
});

test("the game draws its questions with the shared components", () => {
  const render = read("../js/render.js");
  assert.match(render, /questionBar\(/);
  assert.match(render, /questionPopup\(/);
  assert.doesNotMatch(render, /choiceModal/);
});
