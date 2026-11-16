// Localization: every string the player reads comes from a message catalogue (locales/<tag>.json), looked up by a
// stable key with t(key, params). Messages use a small subset of ICU MessageFormat, so word order, plurals and
// choices live in the message, not in the code that fills it:
//
//   "Hello {name}"                                   a parameter
//   "{n, number}"                                    a number, in the locale's format
//   "{n, plural, =0 {nothing} one {# card} other {# cards}}"   plural rules (# is the number)
//   "{kind, select, creature {…} other {…}}"         choose by a string
//   "{names, list}"                                  a list: "a, b and c" (an array parameter)
//   '{' and '}' in quotes ('{x}') are literal; a lone ' is an apostrophe.
//
// English is always loaded and is the fallback for a missing key. Pseudo-locales (en-XA, en-XB) are made from the
// English messages to show hard-coded strings and layouts that break on longer text. No build step, no dependency.

const FALLBACK = "en";
const RTL = new Set(["ar", "he", "fa", "ur", "yi", "ps", "sd", "ug", "dv"]);

const catalogs = new Map(); // tag -> { key: message }
let chain = [FALLBACK]; // tags to look through, most specific first
let current = FALLBACK;
let pseudo = null; // "en-XA" | "en-XB" while a pseudo-locale is active

async function readJson(rel) {
  const url = new URL(rel, import.meta.url);
  if (typeof window === "undefined") {
    const { readFile } = await import("node:fs/promises");
    return JSON.parse(await readFile(url, "utf8"));
  }
  const res = await fetch(url);
  if (!res.ok) throw new Error(`${rel}: ${res.status}`);
  return res.json();
}

// English is needed before anything is drawn, so loading this module waits for it.
catalogs.set(FALLBACK, await readJson("../locales/en.json"));

/** Register a catalogue by hand (tests, or a locale that is already in memory). */
export function addCatalog(tag, messages) { catalogs.set(tag, messages); }

/** The locales the client offers: [{ tag, name, dir }], from locales/index.json. */
export async function availableLocales() {
  try { return await readJson("../locales/index.json"); } catch { return [{ tag: FALLBACK, name: "English", dir: "ltr" }]; }
}

export const isPseudo = (tag) => tag === "en-XA" || tag === "en-XB";

/** fr-CA -> ["fr-CA", "fr", "en"] */
export function fallbackChain(tag) {
  const out = [];
  for (let t = tag; t; t = t.includes("-") ? t.slice(0, t.lastIndexOf("-")) : "") out.push(t);
  if (!out.includes(FALLBACK)) out.push(FALLBACK);
  return out;
}

/** Choose a locale: a saved or requested tag if we have it, else the first of the browser's languages we have. */
export function pickLocale(wanted, offered, languages = []) {
  const tags = new Set(offered.map((l) => l.tag));
  for (const tag of [wanted, ...languages]) {
    if (!tag) continue;
    if (isPseudo(tag)) return tag;
    const hit = fallbackChain(tag).find((c) => c !== FALLBACK && tags.has(c));
    if (hit) return hit;
    if (tag === FALLBACK || tag.startsWith(`${FALLBACK}-`)) return FALLBACK;
  }
  return FALLBACK;
}

/** Make tag the active locale: loads its catalogue (and the ones it falls back through). */
export async function setLocale(tag) {
  const wanted = tag || FALLBACK;
  if (isPseudo(wanted)) {
    pseudo = wanted;
    chain = [FALLBACK];
    current = wanted;
    return current;
  }
  pseudo = null;
  const tags = fallbackChain(wanted);
  for (const t of tags) {
    if (catalogs.has(t)) continue;
    try { catalogs.set(t, await readJson(`../locales/${t}.json`)); } catch { /* no such catalogue: fall through */ }
  }
  chain = tags.filter((t) => catalogs.has(t));
  current = chain[0] ?? FALLBACK;
  return current;
}

export const locale = () => current;
export const direction = (tag = current) => (tag === "en-XB" || RTL.has(tag.split("-")[0]) ? "rtl" : "ltr");

/** The message for key, before it is filled in: the active locale's, else English's, else null. */
function lookup(key) {
  for (const tag of chain) {
    const m = catalogs.get(tag)?.[key];
    if (typeof m === "string") return m;
  }
  return null;
}

// ---------- the message format

const cache = new Map();

/** Parse a message into text and placeholders. */
export function parse(src) {
  let hit = cache.get(src);
  if (!hit) cache.set(src, (hit = parseNodes(src, { i: 0 }, false)));
  return hit;
}

function parseNodes(s, st, inPlural, close = false) {
  const nodes = [];
  let text = "";
  const flush = () => { if (text) { nodes.push(text); text = ""; } };
  while (st.i < s.length) {
    const c = s[st.i];
    if (c === "'") {
      if (s[st.i + 1] === "'") { text += "'"; st.i += 2; continue; }
      if (s[st.i + 1] === "{" || s[st.i + 1] === "}") { // quoted: literal until the closing quote
        let j = st.i + 1;
        while (j < s.length && !(s[j] === "'" && s[j + 1] !== "'")) { text += s[j] === "'" ? "'" : s[j]; j += s[j] === "'" ? 2 : 1; }
        st.i = j + 1;
        continue;
      }
      text += c; st.i++;
    } else if (c === "{") {
      flush();
      st.i++;
      nodes.push(parseArg(s, st, inPlural));
    } else if (c === "}") {
      if (close) { flush(); return nodes; }
      text += c; st.i++;
    } else if (c === "#" && inPlural) {
      flush(); nodes.push({ hash: true }); st.i++;
    } else {
      text += c; st.i++;
    }
  }
  flush();
  return nodes;
}

function parseArg(s, st, inPlural) {
  const end = (re) => { const m = re.exec(s.slice(st.i)); const out = s.slice(st.i, st.i + (m ? m.index : s.length - st.i)); st.i += out.length; return out.trim(); };
  const name = end(/[,}]/);
  if (s[st.i] === "}") { st.i++; return { arg: name }; }
  st.i++; // the comma
  const type = end(/[,}]/);
  if (s[st.i] === "}") { st.i++; return { arg: name, type }; }
  st.i++;
  if (type !== "plural" && type !== "select") throw new Error(`unknown argument type "${type}" in: ${s}`);
  const node = { arg: name, type, offset: 0, cases: {} };
  for (;;) {
    while (/\s/.test(s[st.i] ?? "")) st.i++;
    if (s[st.i] === "}") { st.i++; return node; }
    if (st.i >= s.length) throw new Error(`unterminated ${type} in: ${s}`);
    const sel = end(/[\s{]/);
    if (type === "plural" && sel.startsWith("offset:")) { node.offset = Number(sel.slice(7)); continue; }
    while (/\s/.test(s[st.i] ?? "")) st.i++;
    if (s[st.i] !== "{") throw new Error(`expected { after "${sel}" in: ${s}`);
    st.i++;
    node.cases[sel] = parseNodes(s, st, type === "plural", true);
    st.i++; // the closing brace
  }
}

function render(nodes, params, tag, hashValue) {
  let out = "";
  for (const n of nodes) {
    if (typeof n === "string") { out += n; continue; }
    if (n.hash) { out += new Intl.NumberFormat(tag).format(hashValue); continue; }
    const v = params?.[n.arg];
    if (v === undefined) { out += `{${n.arg}}`; continue; }
    switch (n.type) {
      case undefined: out += String(v); break;
      case "number": out += new Intl.NumberFormat(tag).format(v); break;
      case "list": out += new Intl.ListFormat(tag, { style: "long", type: "conjunction" }).format(v.map(String)); break;
      case "select": out += render(n.cases[String(v)] ?? n.cases.other ?? [], params, tag, hashValue); break;
      case "plural": {
        const num = Number(v) - n.offset;
        const branch = n.cases[`=${Number(v)}`] ?? n.cases[new Intl.PluralRules(tag).select(num)] ?? n.cases.other ?? [];
        out += render(branch, params, tag, num);
        break;
      }
      default: out += String(v);
    }
  }
  return out;
}

/** Fill in a message written in the format above. */
export function format(message, params = {}, tag = current) {
  try { return render(parse(message), params, isPseudo(tag) ? FALLBACK : tag); } catch { return message; }
}

// ---------- pseudo-locales

const ACCENTS = { a: "á", b: "ƀ", c: "ç", d: "ð", e: "é", f: "ƒ", g: "ĝ", h: "ĥ", i: "í", j: "ĵ", k: "ķ", l: "ļ", m: "ɱ", n: "ñ", o: "ö", p: "þ", q: "ǫ", r: "ŕ", s: "š", t: "ţ", u: "û", v: "ṽ", w: "ŵ", x: "ẋ", y: "ý", z: "ž",
  A: "Á", B: "Ɓ", C: "Ç", D: "Ð", E: "É", F: "Ƒ", G: "Ĝ", H: "Ĥ", I: "Í", J: "Ĵ", K: "Ķ", L: "Ļ", M: "Ṁ", N: "Ñ", O: "Ö", P: "Þ", Q: "Ǫ", R: "Ŕ", S: "Š", T: "Ţ", U: "Û", V: "Ṽ", W: "Ŵ", X: "Ẋ", Y: "Ý", Z: "Ž" };

/** Accent and lengthen plain text, bracketed, for a pseudo-locale. */
export function pseudoText(text) {
  const lengthen = (s) => [...s].map((ch) => ACCENTS[ch] ?? ch).join("") + "~".repeat(Math.ceil([...s.replace(/\s/g, "")].length * 0.4));
  return `[${lengthen(String(text))}]`;
}

/** Accent and lengthen the text of an English message (about 40% longer), keeping its placeholders, and bracket it. */
export function pseudoize(message) {
  const wrap = (nodes) => {
    let out = "";
    for (const n of nodes) {
      if (typeof n === "string") {
        const letters = [...n].map((ch) => ACCENTS[ch] ?? ch).join("");
        out += letters.replace(/'/g, "''").replace(/[{}]/g, (b) => `'${b}'`) + "~".repeat(Math.ceil([...n.replace(/\s/g, "")].length * 0.4));
      } else if (n.hash) {
        out += "#";
      } else if (!n.type || n.type === "number" || n.type === "list") {
        out += `{${n.arg}${n.type ? `, ${n.type}` : ""}}`;
      } else {
        out += `{${n.arg}, ${n.type},${n.offset ? ` offset:${n.offset}` : ""}${Object.entries(n.cases).map(([k, v]) => ` ${k} {${wrap(v)}}`).join("")}}`;
      }
    }
    return out;
  };
  return `[${wrap(parse(message))}]`;
}

// ---------- the API

/** The text for a message key, in the active locale, filled in with params. A missing key shows English, then the key. */
export function t(key, params) {
  const english = catalogs.get(FALLBACK)[key];
  const own = pseudo ? (typeof english === "string" ? pseudoize(english) : null) : lookup(key);
  const message = own ?? english;
  if (typeof message !== "string") return key;
  return format(message, params, own === english && !pseudo ? FALLBACK : current);
}

/** The English text of a message, whatever the locale: for what is exchanged with other players (a pasted deck list). */
export function tEn(key, params) {
  const m = catalogs.get(FALLBACK)[key];
  return typeof m === "string" ? format(m, params, FALLBACK) : key;
}

/**
 * Words that are not a message: the name or rules text of a card, which comes from the card data in the language
 * the server was asked for. In a pseudo-locale it is accented and lengthened like the messages are.
 */
export function cardText(text) {
  return pseudo && text ? pseudoText(text) : text;
}

/** Does a catalogue (any locale) have this key? Used by the tooling and tests. */
export const has = (key, tag = FALLBACK) => typeof catalogs.get(tag)?.[key] === "string";
export const catalog = (tag = FALLBACK) => catalogs.get(tag);

export const fmtNumber = (n, opts) => new Intl.NumberFormat(pseudo ? FALLBACK : current, opts).format(n);
export const fmtList = (items, opts) => new Intl.ListFormat(pseudo ? FALLBACK : current, { style: "long", type: "conjunction", ...opts }).format(items);
export const fmtDate = (d, opts) => new Intl.DateTimeFormat(pseudo ? FALLBACK : current, { dateStyle: "medium", ...opts }).format(d);

/** Translate the static HTML under root: data-i18n="key" sets the text; data-i18n-attr="title:key;aria-label:key" attributes. */
export function translateDom(root = document) {
  for (const el of root.querySelectorAll("[data-i18n]")) el.textContent = t(el.dataset.i18n);
  for (const el of root.querySelectorAll("[data-i18n-attr]")) {
    for (const pair of el.dataset.i18nAttr.split(";")) {
      const [attr, key] = pair.split(":").map((x) => x.trim());
      if (attr && key) el.setAttribute(attr, t(key));
    }
  }
}

/** Point the document at the active locale: <html lang> and dir. */
export function applyDocument(doc = document) {
  doc.documentElement.lang = current === "en-XA" || current === "en-XB" ? "en" : current;
  doc.documentElement.dir = direction(current);
}
