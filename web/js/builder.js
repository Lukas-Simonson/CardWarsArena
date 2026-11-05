// The deck builder: a screen of its own, opened from the lobby by a signed-in
// player. Its look is the "Deck builder" board of the Card Wars Web design: a
// felt table of cards with plus and minus under each, and a steel panel that
// says how the deck is doing. The rules and the arithmetic are in deckmodel.js;
// this file is the screen and the conversation with the server
// (GET/POST/PUT/DELETE /api/my/decks).
import { h, svg } from "./dom.js";
import { cardEl } from "./card.js";
import { landscapeName } from "./describe.js";
import {
  emptyDeck, fromCards, toCards, total, copies, whyNot, add, remove, problems, landscapeRow, costCurve, heroes, pool, changed,
  MIN_CARDS, MAX_COPIES,
} from "./deckmodel.js";

// The colours of the Landscapes, as in the design: a tile colour and a darker diamond.
const TILE = { cornfield: "#9cc43a", blue_plains: "#7aa6d6", useless_swamp: "#9b7bbf", sandy_lands: "#e8b04a", nice_lands: "#f08fbc", icy_lands: "#bfe8f2", rainbow: "#e9ddf6" };
const DOT = { cornfield: "#5ea83a", blue_plains: "#3f8fd6", useless_swamp: "#7d5aa8", sandy_lands: "#e0a032", nice_lands: "#ea6fa6", icy_lands: "#6cc6e0", rainbow: "#ff6fae" };
const TYPES = [["", "All types"], ["creature", "Creatures"], ["spell", "Spells"], ["building", "Buildings"]];
const CURVE_HEIGHT = 96;
const kind = (c) => c.type[0].toUpperCase() + c.type.slice(1);

const CHEVRON = '<path d="M15 5 L8 12 L15 19" fill="none" stroke="#ffffff" stroke-width="3.5" stroke-linecap="round" stroke-linejoin="round"/>';

/**
 * @param {object} o
 * @param {HTMLElement} o.root          the #builder element
 * @param {()=>string} o.server         the game server's address
 * @param {()=>object} o.cards          the catalog by id
 * @param {()=>object[]} o.starters     the playable starter decks (GET /api/decks)
 * @param {()=>void} o.onClose          back to the lobby
 * @param {(decks:object[])=>void} o.onSaved   the player's decks changed: the new list
 */
export function createBuilder({ root, server, cards, starters, onClose, onSaved }) {
  let open = false;
  let myDecks = [];
  let deck = emptyDeck();
  let saved = null; // the deck as last saved; null for one not saved yet
  let filters = { text: "", type: "", landscape: "" };
  let busy = false;
  let note = { error: false, text: "" };

  const ui = {}; // the parts of the screen that change as the deck does

  async function api(method, path, body) {
    try {
      const res = await fetch(`${server()}/api/my/decks${path}`, {
        method, credentials: "include",
        headers: body ? { "Content-Type": "application/json" } : {},
        body: body ? JSON.stringify(body) : undefined,
      });
      const data = res.status === 204 ? null : await res.json().catch(() => null);
      return { ok: res.ok, status: res.status, data };
    } catch {
      return { ok: false, status: 0, data: { error: "Cannot reach the server." } };
    }
  }

  const dirty = () => changed(deck, saved);
  const leaveOk = () => !dirty() || confirm("You have changes that are not saved. Leave them behind?");
  const say = (text, error = false) => { note = { text, error }; paintDeck(); };

  // ---------- opening and choosing a deck

  async function refresh() {
    const r = await api("GET", "");
    if (r.ok) myDecks = r.data;
    return r.ok;
  }

  function show(next, savedAs) {
    deck = next;
    saved = savedAs;
    note = { text: "", error: false };
    ui.name.value = deck.name;
    ui.hero.value = deck.hero;
    ui.pick.value = deck.id ?? "";
    paintAll();
  }

  const fromSaved = (d) => fromCards(d.cards, { id: d.id, name: d.name, hero: d.hero });
  const clone = (d) => ({ ...d, counts: { ...d.counts } });

  function choose(value) {
    if (!leaveOk()) { ui.pick.value = deck.id ?? ""; return; }
    if (value.startsWith("starter:")) {
      const s = starters().find((d) => `starter:${d.set} ${d.name}` === value);
      if (s) show(fromCards(s.cards, { name: `${s.owner || s.name}'s deck`, hero: s.hero }), null);
      return;
    }
    const d = myDecks.find((x) => x.id === value);
    if (d) show(fromSaved(d), clone(fromSaved(d)));
    else show(emptyDeck(), null);
  }

  // ---------- saving

  async function save(asDraft) {
    if (busy) return;
    busy = true;
    paintDeck();
    const body = { name: deck.name, hero: deck.hero, cards: toCards(deck) };
    const r = deck.id ? await api("PUT", `/${encodeURIComponent(deck.id)}`, body) : await api("POST", "", body);
    busy = false;
    if (!r.ok) {
      say(r.status === 401 ? "You are signed out: sign in again to save." : r.data?.error ?? `The server said ${r.status}.`, true);
      return;
    }
    deck = fromSaved(r.data);
    saved = clone(deck);
    await refresh();
    onSaved(myDecks);
    fillPicker();
    ui.name.value = deck.name;
    ui.pick.value = deck.id;
    paintAll();
    say(r.data.playable ? "Saved. This deck is ready to play." : asDraft ? "Saved as a draft. Finish it to play it." : "Saved, but it cannot be played yet.");
  }

  async function destroy() {
    if (!deck.id || busy || !confirm(`Delete “${deck.name || "Untitled deck"}”? This cannot be undone.`)) return;
    busy = true;
    const r = await api("DELETE", `/${encodeURIComponent(deck.id)}`);
    busy = false;
    if (!r.ok && r.status !== 404) { say(r.data?.error ?? `The server said ${r.status}.`, true); return; }
    await refresh();
    onSaved(myDecks);
    fillPicker();
    show(emptyDeck(), null);
  }

  // ---------- the screen

  const diamond = (colour, size = 14) => h("span", { "aria-hidden": "true", class: "diamond", style: `width:${size}px;height:${size}px;background:${colour}` });

  /** A pill to press, with a Landscape's diamond if it has one. */
  function chip(label, pressed, onclick, dot) {
    return h("button", { type: "button", class: "chip", "aria-pressed": pressed ? "true" : "false", onclick }, dot && diamond(dot), label);
  }

  function build() {
    const field = (id, label, control) => h("div", { class: "field" }, h("label", { for: id, class: "ink" }, label), control);
    ui.search = h("input", { id: "bsearch", type: "search", class: "well", placeholder: "e.g. Floop", autocomplete: "off", oninput: () => { filters = { ...filters, text: ui.search.value }; paintPool(); } });
    ui.landChips = h("div", { role: "group", "aria-label": "Filter by Landscape", class: "chips" });
    ui.typeChips = h("div", { role: "group", "aria-label": "Filter by type", class: "chips" });
    ui.grid = h("div", { class: "cardgrid" });
    ui.poolCount = h("p", { class: "ink small", role: "status" });

    ui.pick = h("select", { id: "bpick", class: "well", onchange: () => choose(ui.pick.value) });
    ui.name = h("input", { id: "bname", class: "well big", maxlength: "40", placeholder: "Untitled deck", autocomplete: "off", oninput: () => { deck = { ...deck, name: ui.name.value }; paintDeck(); } });
    ui.hero = h("select", { id: "bhero", class: "well", onchange: () => { deck = { ...deck, hero: ui.hero.value }; paintDeck(); } });
    ui.tiles = h("div", { class: "landtiles" });
    ui.count = h("span", { class: "total" });
    ui.countNote = h("span", {});
    ui.bar = h("div", { class: "barfill" });
    ui.checks = h("ul", { class: "checks" });
    ui.save = h("button", { type: "button", class: "btn-green", onclick: () => save(false) }, "Save deck");
    ui.draft = h("button", { type: "button", class: "btn-steel", onclick: () => save(true) }, "Save as draft");
    ui.del = h("button", { type: "button", class: "btn-steel danger", onclick: destroy }, "Delete deck");
    ui.note = h("p", { class: "note", role: "status" });
    ui.curve = h("div", { class: "curve" });
    ui.curveLabels = h("div", { class: "curvelabels" });
    ui.list = h("ul", { class: "cardlist" });

    root.replaceChildren(h("div", { class: "bwrap" },
      h("header", { class: "bhead" },
        h("button", { type: "button", class: "btn-steel tolobby", onclick: close }, svg(CHEVRON, { viewBox: "0 0 24 24" }), "Lobby"),
        h("h1", { class: "h" }, "Deck builder")),
      h("div", { class: "bcols" },
        h("section", { class: "bleft", "aria-labelledby": "bcards-h" },
          h("div", { class: "steel bfilters" },
            h("div", { class: "bfilterhead" },
              h("h2", { id: "bcards-h", class: "h" }, "Your cards"),
              field("bsearch", "Search by name or text", ui.search)),
            ui.landChips, ui.typeChips, ui.poolCount),
          h("div", { class: "felt bfelt" }, ui.grid)),
        h("aside", { class: "bright", "aria-labelledby": "bdeck-h" },
          h("section", { class: "steel bdeck" },
            h("h2", { id: "bdeck-h", class: "h" }, "This deck"),
            field("bpick", "Deck", ui.pick),
            field("bname", "Deck name", ui.name),
            field("bhero", "Hero", ui.hero),
            h("div", { class: "field" },
              h("div", { class: "rowsplit" }, h("span", { class: "ink lead" }, "Landscapes"), h("span", { class: "ink small" }, "Set by the cards you pick")),
              ui.tiles),
            h("div", { class: "well meter" },
              h("div", { class: "rowsplit" }, h("span", { class: "totalwrap" }, ui.count, h("span", { class: "of" }, ` / ${MIN_CARDS}`)), ui.countNote),
              h("div", { class: "bar" }, ui.bar)),
            ui.checks,
            h("div", { class: "bactions" }, ui.save, ui.draft),
            h("div", { class: "bmore" }, ui.del, ui.note)),
          h("section", { class: "parch bcurve", "aria-labelledby": "bcurve-h" }, h("h2", { id: "bcurve-h" }, "Action cost curve"), ui.curve, ui.curveLabels),
          h("section", { class: "parch blist", "aria-labelledby": "blist-h" }, h("h2", { id: "blist-h" }, "Card list"), ui.list)))));

    ui.hero.append(h("option", { value: "" }, "(choose a Hero)"), ...heroes(cards()).map((c) => h("option", { value: c.id }, c.name)));
    paintChips();
  }

  /** "New deck", the player's decks, then the starters to copy. */
  function fillPicker() {
    const group = (label, opts) => { const g = h("optgroup", { label }); g.append(...opts); return g; };
    ui.pick.replaceChildren(h("option", { value: "" }, "New deck"));
    if (myDecks.length) ui.pick.append(group("My decks", myDecks.map((d) => h("option", { value: d.id }, d.playable ? d.name : `${d.name} (unfinished)`))));
    ui.pick.append(group("Start from a starter deck", starters().map((d) => h("option", { value: `starter:${d.set} ${d.name}` }, `${d.owner || d.name} — ${d.name}`))));
  }

  function paintChips() {
    const lands = [...new Set(pool(cards()).map((c) => c.landscape).filter(Boolean))].sort((a, b) => landscapeName(a).localeCompare(landscapeName(b)));
    const pickLand = (v) => () => { filters = { ...filters, landscape: v }; paintChips(); paintPool(); };
    const pickType = (v) => () => { filters = { ...filters, type: v }; paintChips(); paintPool(); };
    ui.landChips.replaceChildren(
      chip("All cards", filters.landscape === "", pickLand(""), "#f4e6bf"),
      ...lands.map((l) => chip(landscapeName(l), filters.landscape === l, pickLand(l), DOT[l])));
    ui.typeChips.replaceChildren(...TYPES.map(([v, l]) => chip(l, filters.type === v, pickType(v))));
  }

  function paintAll() {
    paintPool();
    paintDeck();
  }

  // The cards in the grid are made once per search; changing a deck only changes the counts and buttons under them.
  const cells = new Map();

  function paintPool() {
    const list = pool(cards(), filters);
    cells.clear();
    ui.poolCount.textContent = `${list.length} card${list.length === 1 ? "" : "s"} shown`;
    ui.grid.replaceChildren(...(list.length ? list.map((c) => {
      const num = h("span", { class: "ncopies" });
      const dec = h("button", { type: "button", class: "btn-steel step", "aria-label": `Remove one ${c.name}`, onclick: () => change(remove(deck, c.id)) }, "−");
      const inc = h("button", { type: "button", class: "btn-green step", onclick: () => change(add(deck, c.id)) }, "+");
      cells.set(c.id, { num, dec, inc, name: c.name });
      return h("div", { class: "cell" }, cardEl(c, null, { lazy: true }), h("div", { class: "well stepper" }, dec, num, inc));
    }) : [h("p", { class: "nocards" }, "No cards match. Try a different search or filter.")]));
    paintCells();
  }

  function paintCells() {
    for (const [id, { num, dec, inc, name }] of cells) {
      const n = copies(deck, id), why = whyNot(deck, id);
      num.textContent = `${n} of ${MAX_COPIES}`;
      dec.disabled = n === 0;
      inc.disabled = !!why;
      inc.setAttribute("aria-label", n >= MAX_COPIES ? `${name}: already at the ${MAX_COPIES}-copy limit` : why ? `${name}: ${why}` : `Add one ${name}`);
      inc.title = why;
    }
  }

  function paintDeck() {
    const cs = cards(), n = total(deck), list = problems(deck, cs), legal = list.length === 0;

    ui.tiles.replaceChildren(...[0, 1, 2, 3].map((i) => {
      const ls = landscapeRow(deck, cs)[i];
      return h("div", { class: "landtile", style: ls ? `background:${TILE[ls] ?? "#ddd"}` : "", role: "img", "aria-label": `Landscape ${i + 1}: ${ls ? landscapeName(ls) : "not decided yet"}` },
        h("span", {}, ls ? landscapeName(ls) : "?"));
    }));

    ui.count.textContent = n;
    ui.countNote.textContent = n >= MIN_CARDS ? "Minimum reached" : `${MIN_CARDS - n} to go`;
    ui.bar.style.width = `${Math.min(100, Math.round((n / MIN_CARDS) * 100))}%`;
    ui.bar.className = "barfill" + (n >= MIN_CARDS ? " full" : "");

    const check = (mark, tone, text) => h("li", { class: `check ${tone}` }, h("span", { class: "mark", "aria-hidden": "true" }, mark), h("span", {}, text));
    ui.checks.replaceChildren(
      ...list.map((p) => check("!", "warn", p[0].toUpperCase() + p.slice(1) + ".")),
      ...(legal ? [check("✓", "good", "Deck is legal. Ready to play!")] : []),
      check("i", "info", `Rainbow cards work with any Landscape. Max ${MAX_COPIES} copies of each card.`));

    const edited = dirty();
    ui.save.disabled = busy || !legal || !edited;
    ui.save.textContent = busy ? "Saving…" : "Save deck";
    ui.draft.disabled = busy || !edited;
    ui.del.hidden = !deck.id;
    ui.note.textContent = note.text;
    ui.note.className = "note" + (note.error ? " bad" : "");

    // How many cards of each cost.
    const curve = costCurve(deck, cs), top = Math.max(1, ...curve.map((b) => b.count));
    ui.curve.style.gridTemplateColumns = ui.curveLabels.style.gridTemplateColumns = `repeat(${curve.length}, minmax(0, 1fr))`;
    ui.curve.replaceChildren(...curve.map((b) => h("div", { class: "barcol" }, h("span", {}, b.count), h("div", { class: "curvebar", style: `height:${Math.round((b.count / top) * CURVE_HEIGHT)}px` }))));
    ui.curveLabels.replaceChildren(...curve.map((b) => h("span", {}, `Cost ${b.cost}`)));

    // The deck as a list.
    const ids = Object.keys(deck.counts).sort((a, b) => (cs[a]?.cost ?? 0) - (cs[b]?.cost ?? 0) || (cs[a]?.name ?? a).localeCompare(cs[b]?.name ?? b));
    ui.list.replaceChildren(...(ids.length ? ids.map((id) => {
      const c = cs[id] ?? { name: id, type: "creature" };
      return h("li", {},
        h("span", { class: "costorb", "aria-hidden": "true" }, c.cost ?? "–"),
        diamond(DOT[c.landscape] ?? "#bbb", 12),
        h("span", { class: "lname" }, c.name, h("span", { class: "lkind" }, ` · ${[c.landscape && landscapeName(c.landscape), kind(c)].filter(Boolean).join(" ")}`)),
        h("span", { class: "lcount" }, `×${deck.counts[id]}`));
    }) : [h("li", { class: "empty" }, "No cards yet: add some, or start from a starter deck.")]));

    paintCells();
  }

  function change(next) {
    deck = next;
    paintDeck();
  }

  // ---------- opening and closing

  async function openBuilder() {
    if (open) return;
    open = true;
    root.hidden = false;
    if (!ui.name) build();
    await refresh();
    fillPicker();
    show(emptyDeck(), null);
  }

  function close() {
    if (!leaveOk()) return;
    open = false;
    root.hidden = true;
    onClose();
  }

  return { open: openBuilder, close: () => { open = false; root.hidden = true; }, isOpen: () => open };
}
