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
  emptyDeck, fromCards, toCards, total, copies, whyNot, add, remove, problems, landscapesOf, chooseLandscape, autoLandscapes,
  unplayable, whyUnplayable, costCurve, heroes, pool, buckets, bucketOf, changed, LANDSCAPES, LANES, MIN_CARDS, MAX_COPIES,
} from "./deckmodel.js";
import { formatDeck, importDeck, fileName, FORMATS } from "./decktext.js";
import { t, locale, cardText } from "./i18n.js";
import { errorText, apiError } from "./errors.js";
import { labelText } from "./describe.js";

// The colours of the Landscapes, as in the design: a tile colour and a darker diamond.
const TILE = { cornfield: "#9cc43a", blue_plains: "#7aa6d6", useless_swamp: "#9b7bbf", sandy_lands: "#e8b04a", nice_lands: "#f08fbc", icy_lands: "#bfe8f2", lava_flats: "#f2a27a", rainbow: "#e9ddf6" };
const DOT = { cornfield: "#5ea83a", blue_plains: "#3f8fd6", useless_swamp: "#7d5aa8", sandy_lands: "#e0a032", nice_lands: "#ea6fa6", icy_lands: "#6cc6e0", lava_flats: "#e0562a", rainbow: "#ff6fae" };
const TYPES = [["", "builder.type_all"], ["creature", "builder.type_creature"], ["spell", "builder.type_spell"], ["building", "builder.type_building"]];
const CURVE_HEIGHT = 96;
const kind = (c) => t(`type.${c.type}`);

const CHEVRON = '<path d="M15 5 L8 12 L15 19" fill="none" stroke="#ffffff" stroke-width="3.5" stroke-linecap="round" stroke-linejoin="round"/>';

/**
 * @param {object} o
 * @param {HTMLElement} o.root          the #builder element
 * @param {()=>string} o.server         the game server's address
 * @param {()=>object} o.cards          the catalog by id
 * @param {()=>object[]} o.starters     the playable starter decks (GET /api/decks)
 * @param {()=>void} o.onClose          back to the lobby
 * @param {(decks:object[])=>void} o.onSaved   the player's decks changed: the new list
 * @param {()=>string|null} o.remembered   the id of the saved deck last worked on, to open on
 * @param {(id:string|null)=>void} o.remember  note the saved deck now being worked on (null: none)
 */
export function createBuilder({ root, server, cards, starters, onClose, onSaved, remembered = () => null, remember = () => {} }) {
  let open = false;
  let myDecks = [];
  let deck = emptyDeck();
  let saved = null; // the deck as last saved; null for one not saved yet
  let origin = null; // the name of the starter deck this unsaved deck is a copy of, if it is one
  let filters = { text: "", type: "", landscape: "", members: "", from: "", cost: [], atk: [], def: [] }; // members: "", "in" or "out" of this deck; from: another deck's value
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
      return { ok: false, status: 0, data: { error: t("builder.unreachable"), code: "unreachable" } };
    }
  }

  /** What to tell the player when the deck API said no: its code worded here, else its English text, else the status. */
  const failure = (r) => (apiError(r.data) ? errorText(apiError(r.data), cards()) : r.data?.error ?? t("builder.server_said", { status: r.status }));

  const dirty = () => changed(deck, saved);
  const leaveOk = () => !dirty() || confirm(t("builder.unsaved"));
  const say = (text, error = false) => { note = { text, error }; paintDeck(); };

  // ---------- opening and choosing a deck

  async function refresh() {
    const r = await api("GET", "");
    if (r.ok) myDecks = r.data;
    return r.ok;
  }

  function show(next, savedAs, copyOf = null) {
    deck = next;
    saved = savedAs;
    origin = copyOf;
    if (next.id) remember(next.id);
    note = { text: "", error: false };
    ui.name.value = deck.name;
    ui.hero.value = deck.hero;
    ui.pick.value = deck.id ?? "";
    filters = { ...filters, members: "", from: "" }; // another deck, so a fresh view of the pool
    ui.from.value = "";
    paintAll();
  }

  const fromSaved = (d) => fromCards(d.cards, { id: d.id, name: d.name, hero: d.hero, landscapes: d.landscapes_chosen ? d.landscapes : null });
  const clone = (d) => ({ ...d, counts: { ...d.counts }, landscapes: d.landscapes && [...d.landscapes] });

  /** Load what the drop-down names: "" a blank deck, "starter:<Set Name>" a copy of a starter, else a saved deck's id. */
  function load(value) {
    if (value.startsWith("starter:")) {
      const s = starters().find((d) => `starter:${d.set} ${d.name}` === value);
      if (s) {
        show(fromCards(s.cards, { name: `${s.owner || s.name}'s deck`, hero: s.hero }), null, cardText(s.title || s.name));
        ui.pick.value = value;
        return;
      }
    }
    const d = myDecks.find((x) => x.id === value);
    if (d) show(fromSaved(d), clone(fromSaved(d)));
    else show(emptyDeck(), null);
  }

  function choose(value) {
    if (!leaveOk()) { ui.pick.value = deck.id ?? ""; return; }
    load(value);
  }

  // ---------- saving

  /** A saved deck's copy is a new deck: it keeps the name only with "(copy)" added, so the two can be told apart. */
  const copyName = (name) => t("builder.copy_name", { name: name || t("deck.untitled") }).slice(0, 40);

  async function save(asDraft, asCopy = false) {
    if (busy) return;
    const copy = asCopy && !!deck.id; // a new deck needs no copy: Save already makes one
    busy = true;
    paintDeck();
    const name = copy && deck.name === saved?.name ? copyName(deck.name) : deck.name;
    const body = { name, hero: deck.hero, cards: toCards(deck), landscapes: deck.landscapes ?? [] };
    const r = deck.id && !copy ? await api("PUT", `/${encodeURIComponent(deck.id)}`, body) : await api("POST", "", body);
    busy = false;
    if (!r.ok) {
      say(r.status === 401 ? t("builder.signed_out") : failure(r), true);
      return;
    }
    deck = fromSaved(r.data);
    saved = clone(deck);
    remember(deck.id);
    await refresh();
    onSaved(myDecks);
    fillPicker();
    ui.name.value = deck.name;
    ui.pick.value = deck.id;
    paintAll();
    origin = null;
    say(copy ? t("builder.saved_copy", { name: deck.name }) : t(r.data.playable ? "builder.saved_ready" : asDraft ? "builder.saved_draft" : "builder.saved_unplayable"));
  }

  async function destroy() {
    if (!deck.id || busy || !confirm(t("builder.delete_confirm", { name: deck.name || t("deck.untitled") }))) return;
    busy = true;
    const r = await api("DELETE", `/${encodeURIComponent(deck.id)}`);
    busy = false;
    if (!r.ok && r.status !== 404) { say(failure(r), true); return; }
    await refresh();
    onSaved(myDecks);
    fillPicker();
    remember(null); // the deck just deleted is not the one to come back to
    show(emptyDeck(), null);
  }

  // ---------- deck text: copy, download, import

  const listed = () => formatDeck(deck, cards());

  /** A dialog over the builder; closes on Esc, and is removed when it does. Returns it, already open. */
  function dialog(title, ...body) {
    const dlg = h("dialog", { class: "modal parch deckdialog", "aria-label": title },
      h("h2", {}, title), ...body);
    dlg.addEventListener("close", () => dlg.remove());
    dlg.addEventListener("click", (e) => { if (e.target === dlg) dlg.close(); });
    root.append(dlg);
    dlg.showModal();
    return dlg;
  }

  async function copyList() {
    const text = listed();
    try {
      await navigator.clipboard.writeText(text);
      say(t("builder.copied"));
    } catch {
      // No clipboard here (or it was refused): show the text, selected, to copy by hand.
      const box = h("textarea", { class: "well decktext", readonly: true, rows: "14", spellcheck: "false", "aria-label": t("builder.deck_list") });
      box.value = text;
      const dlg = dialog(t("builder.deck_list"), h("p", {}, t("log.dialog_text")), box,
        h("div", { class: "modalrow" }, h("button", { type: "button", class: "btn-green modalopt", onclick: () => dlg.close() }, t("common.close"))));
      box.select();
    }
  }

  function download() {
    const url = URL.createObjectURL(new Blob([listed()], { type: "text/plain;charset=utf-8" }));
    const a = h("a", { href: url, download: fileName(deck) });
    document.body.append(a);
    a.click();
    a.remove();
    setTimeout(() => URL.revokeObjectURL(url), 1000);
    say(t("builder.downloaded", { file: fileName(deck) }));
  }

  function importList() {
    const box = h("textarea", { class: "well decktext", rows: "14", spellcheck: "false", "aria-label": t("builder.deck_list"), placeholder: t("builder.import_placeholder") });
    const fmt = h("select", { id: "bformat", class: "well" }, FORMATS.map(([v, l]) => h("option", { value: v }, t(l))));
    const why = h("p", { class: "bad", role: "alert" });
    const done = () => dlg.close(); // dlg is set by the time a button is pressed
    const load = () => {
      if (!box.value.trim()) { why.textContent = t("builder.paste_first"); return; }
      const r = importDeck(box.value, cards(), fmt.value);
      if (!r.any) {
        why.textContent = t("builder.nothing_read");
        why.append(report(r.issues));
        return;
      }
      if (!leaveOk()) return;
      show(r.deck, null); // an unsaved draft: nothing is saved until the player presses Save
      const n = total(r.deck);
      say(t("builder.imported", { n, notes: r.issues.length }), r.issues.some((i) => i.level === "error"));
      if (!r.issues.length) { dlg.close(); return; }
      dlg.replaceChildren(
        h("h2", {}, t("builder.imported_title")),
        h("p", {}, t("builder.imported_text", { n })),
        report(r.issues),
        h("div", { class: "modalrow" }, h("button", { type: "button", class: "btn-green modalopt", onclick: done }, t("common.close"))));
    };
    const dlg = dialog(t("builder.import_title"),
      h("p", {}, t("builder.import_text")),
      h("div", { class: "field" }, h("label", { for: "bformat", class: "ink" }, t("builder.format")), fmt),
      box, why,
      h("div", { class: "modalrow modalrow2" },
        h("button", { type: "button", class: "btn-steel modalopt", onclick: done }, t("prompt.cancel")),
        h("button", { type: "button", class: "btn-green modalopt", onclick: load }, t("builder.load"))));
    box.focus();
  }

  /** The lines that could not be used, with their numbers. */
  const report = (issues) => h("ul", { class: "issues" }, issues.map((i) =>
    h("li", { class: i.level }, h("span", { class: "ln" }, t("builder.line", { n: i.line })), h("code", {}, i.text), h("span", {}, i.reason))));

  // ---------- the screen

  const diamond = (colour, size = 14) => h("span", { "aria-hidden": "true", class: "diamond", style: `width:${size}px;height:${size}px;background:${colour}` });

  /** A pill to press, with a Landscape's diamond if it has one. */
  function chip(label, pressed, onclick, dot) {
    return h("button", { type: "button", class: "chip", "aria-pressed": pressed ? "true" : "false", onclick }, dot && diamond(dot), label);
  }

  function build() {
    const field = (id, label, control) => h("div", { class: "field" }, h("label", { for: id, class: "ink" }, label), control);
    ui.search = h("input", { id: "bsearch", type: "search", class: "well", placeholder: t("builder.search_placeholder"), autocomplete: "off", oninput: () => { filters = { ...filters, text: ui.search.value }; paintPool(); } });
    ui.landChips = h("div", { role: "group", "aria-label": t("builder.filter_land"), class: "chips" });
    ui.typeChips = h("div", { role: "group", "aria-label": t("builder.filter_type"), class: "chips" });
    ui.memberChips = h("div", { role: "group", "aria-label": t("builder.filter_deck"), class: "chips" });
    ui.from = h("select", { id: "bfrom", class: "well", onchange: () => { filters = { ...filters, from: ui.from.value }; paintPool(); } });
    ui.costChips = h("div", { role: "group", "aria-label": t("builder.filter_cost"), class: "chips" });
    ui.atkChips = h("div", { role: "group", "aria-label": t("builder.filter_atk"), class: "chips" });
    ui.defChips = h("div", { role: "group", "aria-label": t("builder.filter_def"), class: "chips" });
    ui.statNote = h("p", { class: "ink small", role: "status" });
    ui.statClear = h("button", { type: "button", class: "btn-steel small", onclick: () => { filters = { ...filters, cost: [], atk: [], def: [] }; paintStats(); paintPool(); } }, t("builder.clear_stats"));
    ui.more = h("details", { class: "bmorefilters" },
      h("summary", {}),
      h("div", { class: "field" }, h("span", { class: "ink lead" }, t("builder.filter_cost")), ui.costChips),
      h("div", { class: "field" }, h("span", { class: "ink lead" }, t("builder.filter_atk")), ui.atkChips),
      h("div", { class: "field" }, h("span", { class: "ink lead" }, t("builder.filter_def")), ui.defChips),
      ui.statNote, ui.statClear);
    ui.fromClear = h("button", { type: "button", class: "btn-steel small", onclick: () => { ui.from.value = ""; filters = { ...filters, from: "" }; paintPool(); } }, t("builder.clear"));
    ui.fromRow = h("div", { class: "field fromrow" }, h("label", { for: "bfrom", class: "ink" }, t("builder.cards_from")), h("div", { class: "fromsel" }, ui.from, ui.fromClear));
    ui.grid = h("div", { class: "cardgrid" });
    ui.poolCount = h("p", { class: "ink small", role: "status" });

    ui.pick = h("select", { id: "bpick", class: "well", "aria-describedby": "bpick-hint", onchange: () => choose(ui.pick.value) });
    ui.name = h("input", { id: "bname", class: "well big", maxlength: "40", placeholder: t("deck.untitled"), autocomplete: "off", oninput: () => { deck = { ...deck, name: ui.name.value }; paintDeck(); } });
    ui.hero = h("select", { id: "bhero", class: "well", onchange: () => { deck = { ...deck, hero: ui.hero.value }; paintDeck(); } });
    ui.landSel = Array.from({ length: LANES }, (_, i) => {
      const sel = h("select", { class: "landsel", "aria-label": t("builder.land_for_lane", { lane: i + 1 }), onchange: () => change(chooseLandscape(deck, cards(), i, sel.value)) },
        h("option", { value: "", disabled: true }, "?"),
        ...LANDSCAPES.map((l) => h("option", { value: l }, landscapeName(l))));
      return sel;
    });
    ui.tiles = h("div", { class: "landtiles" }, ui.landSel.map((sel) => h("div", { class: "landtile" }, sel)));
    ui.landNote = h("span", { class: "ink small" });
    ui.landAuto = h("button", { type: "button", class: "btn-steel small", onclick: () => change(autoLandscapes(deck)) }, t("builder.auto_lands"));
    ui.count = h("span", { class: "total" });
    ui.countNote = h("span", {});
    ui.bar = h("div", { class: "barfill" });
    ui.checks = h("ul", { class: "checks" });
    ui.save = h("button", { type: "button", class: "btn-green", onclick: () => save(false) }, t("builder.save"));
    ui.saveCopy = h("button", { type: "button", class: "btn-steel", onclick: () => save(true, true) }, t("builder.save_copy"));
    ui.draft = h("button", { type: "button", class: "btn-steel", onclick: () => save(true) }, t("builder.save_draft"));
    ui.del = h("button", { type: "button", class: "btn-steel danger", onclick: destroy }, t("builder.delete"));
    ui.copy = h("button", { type: "button", class: "btn-steel", onclick: copyList }, t("builder.copy"));
    ui.download = h("button", { type: "button", class: "btn-steel", onclick: download }, t("builder.download"));
    ui.import = h("button", { type: "button", class: "btn-steel", onclick: importList }, t("builder.import"));
    ui.note = h("p", { class: "note", role: "status" });
    ui.title = h("h1", { class: "h" }, t("builder.title_new"));
    ui.state = h("p", { class: "bstate", role: "status" });
    ui.curve = h("div", { class: "curve" });
    ui.curveLabels = h("div", { class: "curvelabels" });
    ui.list = h("ul", { class: "cardlist" });

    root.replaceChildren(h("div", { class: "bwrap" },
      h("header", { class: "bhead" },
        h("button", { type: "button", class: "btn-steel tolobby", onclick: close }, svg(CHEVRON, { viewBox: "0 0 24 24" }), t("builder.lobby")),
        ui.title),
      h("div", { class: "bcols" },
        h("section", { class: "bleft", "aria-labelledby": "bcards-h" },
          h("div", { class: "steel bfilters" },
            h("div", { class: "bfilterhead" },
              h("h2", { id: "bcards-h", class: "h" }, t("builder.your_cards")),
              field("bsearch", t("builder.search"), ui.search)),
            ui.landChips, ui.typeChips, ui.memberChips, ui.more, ui.fromRow, ui.poolCount),
          h("div", { class: "felt bfelt" }, ui.grid)),
        h("aside", { class: "bright", "aria-labelledby": "bdeck-h" },
          h("section", { class: "steel bdeck" },
            h("h2", { id: "bdeck-h", class: "h" }, t("builder.this_deck")),
            ui.state,
            h("p", { class: "ink small bhint", id: "bpick-hint" }, t("builder.pick_hint")),
            field("bpick", t("builder.deck"), ui.pick),
            field("bname", t("builder.deck_name"), ui.name),
            field("bhero", t("hero.tag"), ui.hero),
            h("div", { class: "field" },
              h("div", { class: "rowsplit" }, h("span", { class: "ink lead" }, t("builder.landscapes")), ui.landNote),
              ui.tiles, ui.landAuto),
            h("div", { class: "well meter" },
              h("div", { class: "rowsplit" }, h("span", { class: "totalwrap" }, ui.count, h("span", { class: "of" }, t("builder.of_min", { min: MIN_CARDS }))), ui.countNote),
              h("div", { class: "bar" }, ui.bar)),
            ui.checks,
            h("div", { class: "bactions" }, ui.save, ui.draft, ui.saveCopy),
            h("div", { class: "bshare", role: "group", "aria-label": t("builder.share") }, ui.copy, ui.download, ui.import),
            h("div", { class: "bmore" }, ui.del, ui.note)),
          h("section", { class: "parch bcurve", "aria-labelledby": "bcurve-h" }, h("h2", { id: "bcurve-h" }, t("builder.curve")), ui.curve, ui.curveLabels),
          h("section", { class: "parch blist", "aria-labelledby": "blist-h" }, h("h2", { id: "blist-h" }, t("builder.list")), ui.list)))));

    ui.hero.append(h("option", { value: "" }, t("builder.choose_hero")), ...heroes(cards()).map((c) => h("option", { value: c.id }, c.name)));
    paintChips();
    paintStats();
  }

  /** "New deck", the player's decks, then the starters to copy. */
  /** "Any deck", then the player's decks and the starters: what the pool can be narrowed to. */
  function fillFrom() {
    const group = (label, opts) => { const g = h("optgroup", { label }); g.append(...opts); return g; };
    ui.from.replaceChildren(h("option", { value: "" }, t("builder.any_deck")));
    if (myDecks.length) ui.from.append(group(t("builder.my_decks"), myDecks.map((d) => h("option", { value: `my:${d.id}` }, d.playable ? d.name : t("builder.unfinished_deck", { name: d.name })))));
    ui.from.append(group(t("builder.starter_decks"), starters().map((d) => h("option", { value: `starter:${d.set} ${d.name}` }, t("builder.starter_option", { owner: cardText(d.owner_title || d.owner || d.name), name: cardText(d.title || d.name) })))));
    if (![...ui.from.options].some((o) => o.value === filters.from)) filters = { ...filters, from: "" };
    ui.from.value = filters.from;
  }

  /** The other deck chosen under "Cards from": its name and card counts, or null for any deck. */
  function otherDeck() {
    const v = filters.from;
    if (!v) return null;
    const d = v.startsWith("my:") ? myDecks.find((x) => `my:${x.id}` === v) : starters().find((x) => `starter:${x.set} ${x.name}` === v);
    return d ? { name: d.name, counts: fromCards(d.cards).counts } : null;
  }

  function fillPicker() {
    fillFrom();
    const group = (label, opts) => { const g = h("optgroup", { label }); g.append(...opts); return g; };
    ui.pick.replaceChildren(h("option", { value: "" }, t("builder.new_deck")));
    if (myDecks.length) ui.pick.append(group(t("builder.edit_a_deck"), myDecks.map((d) => h("option", { value: d.id }, d.playable ? d.name : t("builder.unfinished_deck", { name: d.name })))));
    ui.pick.append(group(t("builder.start_from"), starters().map((d) => h("option", { value: `starter:${d.set} ${d.name}` }, t("builder.starter_option", { owner: cardText(d.owner_title || d.owner || d.name), name: cardText(d.title || d.name) })))));
  }

  function paintChips() {
    const lands = [...new Set(pool(cards()).map((c) => c.landscape).filter(Boolean))].sort((a, b) => landscapeName(a).localeCompare(landscapeName(b), locale()));
    const pickLand = (v) => () => { filters = { ...filters, landscape: v }; paintChips(); paintPool(); };
    // ATK and DEF only apply to Creatures, so choosing another kind of card lets them go.
    const pickType = (v) => () => { filters = { ...filters, type: v, ...(v && v !== "creature" ? { atk: [], def: [] } : {}) }; paintChips(); paintStats(); paintPool(); };
    ui.landChips.replaceChildren(
      chip(t("builder.all_cards"), filters.landscape === "", pickLand(""), "#f4e6bf"),
      ...lands.map((l) => chip(landscapeName(l), filters.landscape === l, pickLand(l), DOT[l])));
    ui.typeChips.replaceChildren(...TYPES.map(([v, l]) => chip(t(l), filters.type === v, pickType(v))));
  }

  /** How many cards of each cost. A bar is a button: it shows the cards of that cost in the pool (again to undo). */
  function paintCurve() {
    const cs = cards();
    const curve = costCurve(deck, cs), top = Math.max(1, ...curve.map((b) => b.count));
    const groups = statBuckets().cost;
    ui.curve.style.gridTemplateColumns = ui.curveLabels.style.gridTemplateColumns = `repeat(${curve.length}, minmax(0, 1fr))`;
    ui.curve.replaceChildren(...curve.map((b) => {
      const group = bucketOf(groups, b.cost);
      const on = !!group && filters.cost.length === 1 && filters.cost[0].key === group.key;
      return h("button", {
        type: "button", class: "barcol", "aria-pressed": on ? "true" : "false", title: t("builder.curve_filter", { cost: b.cost }),
        "aria-label": t("builder.curve_filter_count", { cost: b.cost, n: b.count }),
        onclick: () => { filters = { ...filters, cost: on || !group ? [] : [group] }; paintStats(); paintPool(); },
      }, h("span", {}, b.count), h("div", { class: "curvebar", style: `height:${Math.round((b.count / top) * CURVE_HEIGHT)}px` }));
    }));
    ui.curveLabels.replaceChildren(...curve.map((b) => h("span", {}, t("detail.cost", { cost: b.cost }))));
  }

  /** The numbers the pool's Creatures and cards have, for the cost, ATK and DEF chips: the groups come from the catalogue. */
  function statBuckets() {
    const list = pool(cards());
    const creatures = list.filter((c) => c.type === "creature");
    return {
      cost: buckets(list.map((c) => c.cost), { step: 1, cap: 4 }),
      atk: buckets(creatures.map((c) => c.attack), { step: 1, cap: 4 }),
      def: buckets(creatures.map((c) => c.defense), { step: 3, cap: 9 }),
    };
  }

  const bucketLabel = (b) => (b.max === Infinity ? t("builder.bucket_plus", { n: b.min }) : b.min === b.max ? String(b.min) : t("builder.bucket_range", { a: b.min, b: b.max }));

  /** Cost, ATK and DEF: chips that toggle, none pressed meaning any. ATK and DEF also leave Creatures only. */
  function paintStats() {
    const all = statBuckets();
    const pressed = (kind, b) => filters[kind].some((x) => x.key === b.key);
    const toggle = (kind, b) => () => {
      const next = pressed(kind, b) ? filters[kind].filter((x) => x.key !== b.key) : [...filters[kind], b];
      // Only Creatures have ATK and DEF: asking for one shows Creatures, and says so.
      filters = { ...filters, [kind]: next, ...(kind !== "cost" && next.length && filters.type !== "creature" ? { type: "creature" } : {}) };
      paintChips();
      paintStats();
      paintPool();
    };
    const row = (kind, el) => el.replaceChildren(
      chip(t("builder.any_value"), filters[kind].length === 0, () => { filters = { ...filters, [kind]: [] }; paintStats(); paintPool(); }),
      ...all[kind].map((b) => chip(bucketLabel(b), pressed(kind, b), toggle(kind, b))));
    row("cost", ui.costChips);
    row("atk", ui.atkChips);
    row("def", ui.defChips);
    const active = [filters.cost, filters.atk, filters.def].filter((f) => f.length).length;
    ui.more.querySelector("summary").textContent = active ? t("builder.more_filters_on", { n: active }) : t("builder.more_filters");
    if (active) ui.more.open = true;
    ui.statNote.textContent = filters.atk.length || filters.def.length ? t("builder.creatures_only") : "";
    ui.statClear.hidden = !active;
    paintCurve();
  }

  /** "All / In deck (n) / Not in deck": n is how many different cards this deck has. */
  function paintMembers() {
    const mine = Object.values(deck.counts).filter((n) => n > 0).length;
    const set = (v) => () => { filters = { ...filters, members: v }; paintMembers(); paintPool(); };
    const inChip = chip(t("builder.in_deck", { n: mine }), filters.members === "in", set(filters.members === "in" ? "" : "in"));
    inChip.title = t("builder.in_deck_title", { n: mine });
    ui.memberChips.replaceChildren(
      chip(t("builder.any_card"), filters.members === "", set("")), inChip,
      chip(t("builder.not_in_deck"), filters.members === "out", set(filters.members === "out" ? "" : "out")));
  }

  function paintAll() {
    paintPool();
    paintDeck();
  }

  // The cards in the grid are made once per search; changing a deck only changes the counts and buttons under them.
  const cells = new Map();

  function paintPool() {
    const other = otherDeck();
    const list = pool(cards(), { ...filters, counts: deck.counts, from: other?.counts ?? null });
    cells.clear();
    ui.fromClear.hidden = !filters.from;
    ui.poolCount.textContent = t("builder.shown", { n: list.length });
    ui.grid.replaceChildren(...(list.length ? list.map((c) => {
      const num = h("span", { class: "ncopies" });
      const dec = h("button", { type: "button", class: "btn-steel step", "aria-label": t("builder.remove_one", { name: c.name }), onclick: () => change(remove(deck, c.id)) }, "−");
      const inc = h("button", { type: "button", class: "btn-green step", onclick: () => change(add(deck, c.id)) }, "+");
      const there = other && h("span", { class: "ncopies there" }, `${other.counts[c.id]} in ${other.name}`);
      cells.set(c.id, { num, dec, inc, name: c.name });
      return h("div", { class: "cell" }, cardEl(c, null, { lazy: true }), there, h("div", { class: "well stepper" }, dec, num, inc));
    }) : [h("p", { class: "nocards" }, filters.members === "in" && !total(deck) && !other ? t("builder.no_cards_deck") : t("builder.no_cards_match"))]));
    paintCells();
  }

  function paintCells() {
    for (const [id, { num, dec, inc, name }] of cells) {
      const n = copies(deck, id), why = whyNot(deck, id);
      num.textContent = t("builder.n_of_max", { n, max: MAX_COPIES });
      dec.disabled = n === 0;
      inc.disabled = !!why;
      inc.setAttribute("aria-label", n >= MAX_COPIES ? t("builder.at_limit", { name, max: MAX_COPIES }) : why ? t("builder.cannot_add", { name, why }) : t("builder.add_one", { name }));
      inc.title = why;
    }
  }

  function paintDeck() {
    const cs = cards(), n = total(deck), list = problems(deck, cs), legal = list.length === 0;
    paintMembers(); // the count on "In deck" follows the deck

    // One Landscape for each Lane: the player's choice, or what the cards suggest.
    const row = landscapesOf(deck, cs);
    ui.landSel.forEach((sel, i) => {
      sel.value = row[i] ?? "";
      sel.parentElement.style.background = row[i] ? TILE[row[i]] ?? "#ddd" : "";
    });
    ui.landNote.textContent = t(deck.landscapes ? "builder.lands_chosen" : row.length ? "builder.lands_auto" : "builder.lands_none");
    ui.landAuto.hidden = !deck.landscapes;
    const maybe = unplayable(deck, cs);
    const maybeIds = new Map(maybe.map((u) => [u.id, u]));

    ui.count.textContent = n;
    ui.countNote.textContent = n >= MIN_CARDS ? t("builder.min_reached") : t("builder.to_go", { n: MIN_CARDS - n });
    ui.bar.style.width = `${Math.min(100, Math.round((n / MIN_CARDS) * 100))}%`;
    ui.bar.className = "barfill" + (n >= MIN_CARDS ? " full" : "");

    const check = (mark, tone, text) => h("li", { class: `check ${tone}` }, h("span", { class: "mark", "aria-hidden": "true" }, mark), h("span", {}, text));
    ui.checks.replaceChildren(
      ...list.map((p) => check("!", "warn", p)),
      ...(legal ? [check("✓", "good", t("builder.legal"))] : []),
      ...(maybe.length ? [h("li", { class: "check maybe" }, h("span", { class: "mark", "aria-hidden": "true" }, "!"),
        h("span", {}, t("builder.maybe", { n: maybe.length }),
          h("ul", { class: "maybelist" }, maybeLines(maybe))))] : []),
      check("i", "info", t("builder.rainbow_note", { max: MAX_COPIES })));

    const edited = dirty();
    ui.save.disabled = busy || !legal || !edited;
    ui.save.textContent = t(busy ? "builder.saving" : deck.id ? "builder.save_changes" : "builder.save");
    ui.saveCopy.hidden = !deck.id;
    ui.saveCopy.disabled = busy;
    // Which deck this is, said in words: new, a saved deck being edited, or an unsaved copy of a starter.
    const named = (deck.id && saved?.name) || deck.name || t("deck.untitled");
    ui.title.textContent = deck.id ? t("builder.title_editing", { name: named }) : origin ? t("builder.title_copy", { name: origin }) : t("builder.title_new");
    ui.state.textContent = deck.id ? t("builder.state_editing", { changed: edited ? "yes" : "no" }) : origin ? t("builder.state_copy", { name: origin }) : t("builder.state_new");
    ui.draft.disabled = busy || !edited;
    ui.del.hidden = !deck.id;
    ui.copy.disabled = ui.download.disabled = total(deck) === 0 && !deck.hero && !deck.name.trim();
    ui.note.textContent = note.text;
    ui.note.className = "note" + (note.error ? " bad" : "");

    paintCurve();

    // The deck as a list.
    const ids = Object.keys(deck.counts).sort((a, b) => (cs[a]?.cost ?? 0) - (cs[b]?.cost ?? 0) || (cs[a]?.name ?? a).localeCompare(cs[b]?.name ?? b));
    ui.list.replaceChildren(...(ids.length ? ids.map((id) => {
      const c = cs[id] ?? { name: id, type: "creature" };
      const u = maybeIds.get(id);
      return h("li", { class: u ? "maybe" : "", title: u ? t("builder.maybe_title", { why: whyUnplayable(u) }) : "" },
        h("span", { class: "costorb", "aria-hidden": "true" }, c.cost ?? "–"),
        diamond(DOT[c.landscape] ?? "#bbb", 12),
        h("span", { class: "lname" }, u ? h("span", { class: "warnmark", "aria-label": t("builder.maybe_label") }, "! ") : null, c.name, h("span", { class: "lkind" }, t("builder.kind_line", { kind: [c.landscape && landscapeName(c.landscape), kind(c)].filter(Boolean).join(" ") }))),
        h("span", { class: "lcount" }, t("builder.times", { n: deck.counts[id] })));
    }) : [h("li", { class: "empty" }, t("builder.empty_list"))]));

    paintCells();
  }

  /** The cards that may be hard to play, one line for each Landscape with none, then one for each that has too few. */
  function maybeLines(list) {
    const none = new Map();
    for (const u of list) if (u.never) none.set(u.type, [...(none.get(u.type) ?? []), u.name]);
    return [
      ...[...none].map(([type, names]) => h("li", {}, h("b", {}, t("builder.no_land", { type: landscapeName(type) })), ` ${names.join(t("strip.sep"))}`)),
      ...list.filter((u) => !u.never).map((u) => h("li", {}, h("b", {}, u.name), ` ${whyUnplayable(u)}`)),
    ];
  }

  function change(next) {
    deck = next;
    paintDeck();
  }

  // ---------- opening and closing

  /** Opens on the deck last worked on, if it is still there, else a blank one. */
  async function openBuilder() {
    if (open) return;
    open = true;
    root.hidden = false;
    if (!ui.name) build();
    await refresh();
    fillPicker();
    const last = remembered();
    if (last && myDecks.some((d) => d.id === last)) load(last);
    else show(emptyDeck(), null);
  }

  function close() {
    if (!leaveOk()) return;
    open = false;
    root.hidden = true;
    onClose();
  }

  return { open: openBuilder, close: () => { open = false; root.hidden = true; }, isOpen: () => open };
}
