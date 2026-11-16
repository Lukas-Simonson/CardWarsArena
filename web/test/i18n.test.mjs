import { test } from "node:test";
import assert from "node:assert/strict";
import { format, parse, pseudoize, fallbackChain, pickLocale, direction, addCatalog, setLocale, t, locale } from "../js/i18n.js";

test("a message fills in its parameters, and leaves an unknown one visible", () => {
  assert.equal(format("Hello {name}!", { name: "Finn" }, "en"), "Hello Finn!");
  assert.equal(format("Hello {name}!", {}, "en"), "Hello {name}!");
});

test("apostrophes are literal unless they quote a brace", () => {
  assert.equal(format("Opp's Lane", {}, "en"), "Opp's Lane");
  assert.equal(format("It''s here", {}, "en"), "It's here");
  assert.equal(format("Use '{braces}' like this", {}, "en"), "Use {braces} like this");
  assert.equal(format("A lone } stays", {}, "en"), "A lone } stays");
});

test("plurals follow the locale's rules, with exact matches and #", () => {
  const m = "{n, plural, =0 {no cards} one {# card} other {# cards}}";
  assert.equal(format(m, { n: 0 }, "en"), "no cards");
  assert.equal(format(m, { n: 1 }, "en"), "1 card");
  assert.equal(format(m, { n: 1234 }, "en"), "1,234 cards");
  // Polish has few and many; the message supplies them.
  const pl = "{n, plural, one {# karta} few {# karty} many {# kart} other {# karty}}";
  assert.equal(format(pl, { n: 3 }, "pl"), "3 karty");
  assert.equal(format(pl, { n: 5 }, "pl"), "5 kart");
  assert.equal(format("{n, plural, offset:1 =0 {nobody} =1 {just you} one {you and # other} other {you and # others}}", { n: 3 }, "en"), "you and 2 others");
});

test("select chooses by a string and falls to other", () => {
  const m = "{kind, select, creature {a Creature} building {a Building} other {a card}}";
  assert.equal(format(m, { kind: "building" }, "en"), "a Building");
  assert.equal(format(m, { kind: "spell" }, "en"), "a card");
});

test("selects and plurals nest, and a plural can hold placeholders", () => {
  const m = "{who} {n, plural, one {draws # card} other {draws # cards}} {when, select, now {now} other {later}}";
  assert.equal(format(m, { who: "Ann", n: 2, when: "now" }, "en"), "Ann draws 2 cards now");
});

test("numbers and lists use Intl", () => {
  assert.equal(format("{n, number}", { n: 1234567.5 }, "en"), "1,234,567.5");
  assert.equal(format("{n, number}", { n: 1234567.5 }, "de"), "1.234.567,5");
  assert.equal(format("{items, list}", { items: ["a", "b", "c"] }, "en"), "a, b, and c");
});

test("a broken message comes back as written instead of throwing", () => {
  assert.equal(format("{n, plural, one {x}", { n: 1 }, "en"), "{n, plural, one {x}");
});

test("locale tags fall back from the specific to the general, then English", () => {
  assert.deepEqual(fallbackChain("fr-CA"), ["fr-CA", "fr", "en"]);
  assert.deepEqual(fallbackChain("en"), ["en"]);
  assert.deepEqual(fallbackChain("zh-Hant-TW"), ["zh-Hant-TW", "zh-Hant", "zh", "en"]);
});

test("the locale is chosen from what was asked for, then the browser's languages", () => {
  const offered = [{ tag: "en" }, { tag: "fr" }, { tag: "pt-BR" }];
  assert.equal(pickLocale("fr-CA", offered, ["de"]), "fr");
  assert.equal(pickLocale("", offered, ["de", "pt-BR", "fr"]), "pt-BR");
  assert.equal(pickLocale("", offered, ["de"]), "en");
  assert.equal(pickLocale("en-XA", offered, []), "en-XA");
  assert.equal(pickLocale("", offered, ["en-GB", "fr"]), "en");
});

test("right-to-left locales are told apart", () => {
  assert.equal(direction("ar"), "rtl");
  assert.equal(direction("he-IL"), "rtl");
  assert.equal(direction("fr"), "ltr");
  assert.equal(direction("en-XB"), "rtl");
});

test("pseudo-localizing keeps placeholders working and makes text longer", () => {
  const en = "{n, plural, one {# card left} other {# cards left}} for {name}";
  const px = pseudoize(en);
  assert.match(px, /^\[.*\]$/);
  assert.ok(px.length > en.length * 1.1);
  const out = format(px, { n: 2, name: "Finn" }, "en");
  assert.match(out, /2/);
  assert.match(out, /Finn/);
  assert.doesNotMatch(out, /cards left/); // the English words are gone
  assert.equal(parse(pseudoize("Don't {x}")).length > 0, true);
});

test("t falls back to English for a missing key, and to the key itself when English lacks it", async () => {
  addCatalog("en", { "test.hello": "Hello {name}", "test.only_en": "English only" });
  addCatalog("fr", { "test.hello": "Bonjour {name}" });
  await setLocale("fr-CA");
  assert.equal(locale(), "fr");
  assert.equal(t("test.hello", { name: "Finn" }), "Bonjour Finn");
  assert.equal(t("test.only_en"), "English only");
  assert.equal(t("test.nothing"), "test.nothing");
  await setLocale("en-XA");
  assert.match(t("test.hello", { name: "Finn" }), /^\[.*Finn.*\]$/);
  await setLocale("en");
  assert.equal(t("test.hello", { name: "Finn" }), "Hello Finn");
});
