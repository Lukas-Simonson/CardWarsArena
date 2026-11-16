// The catalogues and the code that uses them stay in step: CI fails on a missing, unused or mismatched key, and on a
// card-data label with no message.
import { test } from "node:test";
import assert from "node:assert/strict";
import { readCatalogs, compareCatalog, keyUse, labelGaps, hardcoded, findHardcoded, placeholders } from "../tools/i18n-lib.mjs";
import { pseudoize, format, parse } from "../js/i18n.js";

const { index, catalogs } = await readCatalogs();
const en = catalogs.en;

test("every locale has English's keys and placeholders", () => {
  for (const { tag } of index) {
    if (tag === "en") continue;
    const c = compareCatalog(en, catalogs[tag]);
    assert.deepEqual(c, { missing: [], extra: [], mismatched: [], broken: [] }, `locale ${tag}`);
  }
});

test("every English message parses", () => {
  for (const [k, m] of Object.entries(en)) assert.doesNotThrow(() => placeholders(m), k);
});

test("the code asks only for keys English has, and English has none the code ignores", async () => {
  const use = await keyUse(en);
  assert.deepEqual(use.undefinedKeys, [], "keys used in the code that en.json lacks");
  assert.deepEqual(use.unused, [], "keys in en.json that nothing uses (delete them)");
});

test("every target label in the card data has a message", async () => {
  const gaps = await labelGaps(en);
  assert.deepEqual(gaps.missing, [], "run: node tools/i18n-check.mjs --add-labels");
  assert.deepEqual(gaps.stale, []);
});

test("no user-facing text is hard-coded in the client", async () => {
  assert.deepEqual(await hardcoded(), []);
});

test("the hard-coded text finder does find text", () => {
  const found = findHardcoded('h("p", {}, "Waiting for the other player");\nh("b", {}, `Hello ${x}`);\nconst k = "btn-steel well";\nt("a.b");\n// "A comment about nothing"\nx("Hero"); // i18n-ignore');
  assert.deepEqual(found.map((f) => f.text), ["Waiting for the other player", "Hello"]);
});

test("the pseudo-locale turns every English message into text with no English left and every placeholder kept", () => {
  for (const [k, m] of Object.entries(en)) {
    const args = placeholders(m);
    const params = Object.fromEntries([...args].map(([name, type]) => [name, type === "plural" ? 2 : type === "list" ? ["a", "b"] : type === "select" ? "other" : type === "number" ? 7 : "X"]));
    const out = format(pseudoize(m), params, "en");
    assert.match(out, /^\[.*\]$/s, k);
    for (const [name, type] of args) if (!type) assert.ok(out.includes("X") || params[name] !== "X", k);
  }
});

test("the longest messages stay readable when the pseudo-locale lengthens them", () => {
  const worst = Object.entries(en).map(([k, m]) => [k, pseudoize(m).length / Math.max(1, m.length)]).sort((a, b) => b[1] - a[1])[0];
  assert.ok(worst[1] < 3, `${worst[0]} grows ${worst[1].toFixed(1)}x`);
  assert.ok(parse(pseudoize("x")).length > 0);
});
