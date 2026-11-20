// Draws the battle scene from the current state. It rebuilds the DOM on every
// update (the scene is small) and reports clicks through `on`. Which cards may
// be clicked comes from the picker (js/picker.js); nothing here knows a card's
// rules.
import { tokenChips } from "./tokens.js";
import { APP_NAME } from "./brand.js";
import { isCrowded, fanPose } from "./handlayout.js";
import { h, svg } from "./dom.js";
import { avatarEl } from "./avatar.js";
import { cardEl } from "./card.js";
import { cardName, describeRef, landscapeName, LANDSCAPE_ORDER, seatName, movesToTheirSide, labelText } from "./describe.js";
import { t, fmtList, fmtNumber } from "./i18n.js";
import { isMyDecision } from "./state.js";
import { sources, parseToken, discardSeats } from "./picker.js";
import { buildPrompt } from "./prompt.js";

const fmtJoin = (parts) => parts.join(t("tile.stats_sep"));
const SHIELD = `<path d="M24 3 L44 9 V26 C44 40 35 48 24 51 C13 48 4 40 4 26 V9 Z" fill="#3f8fd6" stroke="#16212b" stroke-width="3.5" stroke-linejoin="round"/><path d="M24 8 L39 12.5 V26 C39 37 32 43.5 24 46" fill="none" stroke="#a8d6ff" stroke-width="2.5" opacity="0.7"/>`;
const MAX_HP = 25;
const LOG_SHOWN = 40;

/**
 * @param {HTMLElement} root  the #game element
 * @param {{state:object, cards:object, ui:{source:string, picker:object|null, awaitingKind:boolean, inspect:string|null, discard:number|null}}} model
 * @param {object} on  handlers: onHand onLane onFloop onAbility onToken onKind onSkip onConfirm onCancel
 *                     onAction onPending onInspect onDiscard onLeave
 */
export function renderGame(root, model, on) {
  const { state, cards, ui } = model;
  const v = state.view;
  if (!v) {
    root.replaceChildren(h("div", { class: "table" },
      h("div", { class: "topbar" }, h("div", { class: "l" }, leaveButton(on), h("h1", { class: "h" }, APP_NAME))),
      h("div", { class: "steel", style: "padding:24px 28px" },
        h("h2", { class: "h", style: "font-size:30px" }, t(state.waiting ? "game.waiting" : "game.joining")),
        state.waiting && h("p", {}, t("game.send_code")),
        state.waiting && h("p", { class: "roomcode", "aria-label": t("game.room_code") }, state.session?.room ?? ""))));
    return;
  }
  const ctx = { view: v, cards, names: state.names, avatars: state.avatars, known: state.known };
  const me = v.you, opp = 1 - me;
  const legal = v.legal ?? [];
  const myDecision = isMyDecision(state);
  const srcs = myDecision ? sources(legal) : {};
  const picker = ui.picker;
  const tokens = new Set(picker && !ui.awaitingKind ? picker.options().tokens : []);
  const prompt = buildPrompt(ctx, myDecision, ui.source, picker, ui.awaitingKind);
  const srcCard = sourceCard(ctx, ui.source);
  const model2 = { ctx, me, opp, legal, myDecision, srcs, tokens, ui, srcCard, prompt };

  root.replaceChildren(h("div", { class: "table" },
    topbar(model2, on),
    h("div", { class: "cols" },
      h("div", { class: "main" },
        battlefield(model2, on),
        promptBar(model2, on),
        handSection(model2, on),
        logPanel(state, ctx, on)),
      h("div", { class: "side" }, playerBar(model2, opp, on), playerBar(model2, me, on), detailPanel(ctx, ui.inspect), feedbackPanel(ui.feedback))),
    choiceModal(model2, on),
    pileModal(model2, on)));
}

/** Replace just the "Card up close" panel (used on hover, so the scene is not rebuilt). */
export function updateDetail(root, ctx, cardId) {
  const old = root.querySelector('[data-role="detail"]');
  if (old) old.replaceWith(detailPanel(ctx, cardId));
}

const leaveButton = (on) => h("button", { type: "button", class: "btn-steel", style: "min-height:44px;font-size:19px", onclick: on.onLeave }, t("game.lobby"));

function sourceCard(ctx, source) {
  const v = ctx.view;
  const p = (source ?? "").split(":");
  let id = null;
  if (p[0] === "hand") id = v.players[v.you].hand?.find((c) => c.uid === Number(p[1]))?.card;
  else if (p[0] === "pending") id = v.pending?.card;
  return id ? ctx.cards[id] : null;
}

// ---------- top bar and players

function topbar({ ctx, me }, on) {
  const v = ctx.view;
  const mine = v.active === me;
  const phase = t(`phase.${v.phase}`);
  return h("div", { class: "topbar" },
    h("div", { class: "l" }, leaveButton(on), h("h1", { class: "h" }, APP_NAME)),
    h("div", { class: "steel turnbox" },
      v.phase !== "mulligan" && h("span", { class: "ink" }, t("game.turn", { turn: v.turn })),
      v.phase !== "mulligan" && h("span", { class: `chip ${mine ? "" : "theirs"}` }, mine ? t("game.your_turn") : t("game.their_turn", { who: seatName(ctx, v.active) })),
      h("span", { class: "ink", style: "font-size:17px" }, phase)));
}

function playerBar(m, seat, on) {
  const { ctx, me, ui, tokens } = m;
  const v = ctx.view, p = v.players[seat], mine = seat === me;
  const hpw = Math.max(0, Math.min(100, Math.round((p.hp / MAX_HP) * 100)));
  const backs = !mine && Array.from({ length: Math.min(p.hand_count, 10) }, () => h("div", { class: "back", "aria-hidden": "true" }, h("i")));
  const pile = (label, n, clickable) => h("button", {
    type: "button", class: "well", disabled: !clickable, onclick: () => on.onDiscard(seat),
    "aria-label": t("player.pile_label", { label, n }),
  }, `${label} ${n}`);
  const aim = `player:${seat}:0`; // a player may be what a card is aimed at (Frosty Frolic)
  const aimed = tokens.has(aim);
  return h("section", { class: `steel pbar${aimed ? " pick" : ""}${aimed && !mine ? " foe" : ""}`, "aria-label": t(mine ? "player.you" : "player.opponent") },
    aimed && h("button", { type: "button", class: "btn-purple aim", onclick: () => on.onToken(aim) }, t(mine ? "player.choose_self" : "player.choose_opponent")),
    h("div", { class: "shield", "data-seat": seat, role: "img", "aria-label": t(mine ? "player.hp_you" : "player.hp_them", { who: seatName(ctx, seat), hp: p.hp, max: MAX_HP }) },
      svg(SHIELD, { viewBox: "0 0 48 54" }), h("span", {}, p.hp)),
    avatarEl(seatName(ctx, seat), ctx.avatars?.[seat], { style: `background:${mine ? "#c98a1f" : "#2e5f9c"}` }),
    h("div", { class: "pname" },
      h("div", { class: "h" }, mine ? t("player.name_you", { who: seatName(ctx, seat) }) : seatName(ctx, seat)),
      h("div", { class: "sub" }, t("player.in_hand", { n: mine ? p.hand?.length ?? 0 : p.hand_count }))),
    heroChip(m, seat, on),
    backs && h("div", { class: "backs", "aria-hidden": "true" }, backs),
    !mine && h("div", { class: "counts" }, h("span", { class: "well", style: "padding:2px 10px;border-radius:999px" }, t("player.deck", { n: p.deck_count })), pile(t("player.discard"), p.discard.length, true)),
    h("div", { class: "well hpbar", "data-seat": seat, "aria-hidden": "true" }, h("div", { style: `width:${hpw}%` })));
}

/** The player's Hero: click to read it; on my side, its abilities show as buttons while I may use them. */
function heroChip(m, seat, on) {
  const { ctx, me, srcs, ui } = m;
  const hero = ctx.view.players[seat].hero;
  if (!hero) return null;
  const mine = seat === me;
  const name = cardName(ctx, hero.card);
  const acts = mine && !ui.picker ? srcs.hero ?? [] : [];
  const abilities = [...new Set(acts.filter((a) => a.kind === "activate").map((a) => a.index ?? 0))].sort();
  return h("div", { class: "herochip" },
    h("button", {
      type: "button", class: `well hero ${abilities.length ? "go" : ""} ${mine && ui.source === "hero" ? "sel" : ""}`,
      "aria-label": t(mine ? "hero.yours" : "hero.theirs", { name }), title: ctx.cards[hero.card]?.text ?? "",
      onclick: () => on.onInspect(hero.card), onmouseenter: () => on.onInspect(hero.card, true),
    }, h("span", { class: "tag" }, t("hero.tag")), h("span", { class: "nm" }, name)),
    abilities.map((n) => h("button", { type: "button", class: "btn-purple floop", "aria-label": t("ability.use_named", { name }), onclick: () => on.onHeroAbility(n) }, t("ability.use"))));
}

function promptBar({ prompt, myDecision, ctx }, on) {
  const kids = [h("span", {}, prompt.text)];
  for (const [key, label] of prompt.kinds) kids.push(h("button", { type: "button", class: "btn-purple", onclick: () => on.onKind(key) }, label));
  for (const [token, label] of prompt.modes) kids.push(h("button", { type: "button", class: "btn-purple", onclick: () => on.onToken(token) }, label));
  if (prompt.confirm) kids.push(h("button", { type: "button", class: "btn-green", onclick: on.onConfirm }, prompt.confirm));
  if (prompt.skip) kids.push(h("button", { type: "button", class: "btn-steel", onclick: on.onSkip }, prompt.skip));
  if (prompt.cancel) kids.push(h("button", { type: "button", class: "btn-steel", onclick: on.onCancel }, t("prompt.cancel")));
  return h("div", { role: "status", class: `well prompt ${myDecision && !ctx.view.over ? "you" : ""}` }, kids);
}

// ---------- battlefield

function battlefield(m, on) {
  const { ctx, me, opp } = m;
  const v = ctx.view;
  const felt = h("section", { class: "felt", style: "padding:10px", "aria-label": t("game.battlefield") },
    h("div", { class: "lanes" }, [0, 1, 2, 3].map((i) => laneColumn(m, i, on))));
  const veil = overlay(m, on);
  if (veil) felt.append(veil);
  return felt;
}

const has = (m, t) => m.tokens.has(t);

function laneColumn(m, i, on) {
  const { ctx, me, opp, srcs, ui } = m;
  const v = ctx.view;
  const mc = v.players[me].lanes[i].creature, oc = v.players[opp].lanes[i].creature;
  const fight = m.myDecision && v.phase === "fight";
  const mine = ui.picker ? [] : srcs[`lane:${i}:creature`] ?? [];
  const canFloop = mine.some((a) => a.kind === "floop");
  const abilities = [...new Set(mine.filter((a) => a.kind === "activate").map((a) => a.index ?? 0))].sort();
  const canThaw = !ui.picker && !!srcs[`thaw:${i}`]; // a frozen Landscape I can clear by discarding a card
  return h("div", { class: "lane" },
    buildingStrip(m, opp, i, on),
    landName(ctx, opp, i),
    tile(m, opp, i, on),
    laneInfo(m, i, mc, oc),
    h("div", { class: "tilewrap" },
      tile(m, me, i, on),
      (canFloop || abilities.length > 0 || canThaw) && h("div", { class: "tileacts" },
        abilities.map((n) => h("button", { type: "button", class: "btn-purple floop", "aria-label": t("ability.use_named", { name: cardName(ctx, mc?.card) }), onclick: () => on.onAbility(i, "creature", n) }, t("ability.use"))),
        canFloop && h("button", { type: "button", class: "btn-purple floop", "aria-label": t("ability.floop_named", { name: cardName(ctx, mc?.card) }), onclick: () => on.onFloop(i, "creature") }, t("ability.floop")),
        canThaw && h("button", { type: "button", class: "btn-ice floop thaw", "aria-label": t("ability.thaw_label", { lane: i + 1 }), onclick: () => on.onThaw(i) }, t("ability.thaw")))),
    landName(ctx, me, i),
    buildingStrip(m, me, i, on));
}

/** The Landscape's name, shown beside the tile so it never covers a card. */
function landName(ctx, seat, i) {
  const lane = ctx.view.players[seat].lanes[i];
  return h("div", { class: "landname" }, lane.face_down ? t("land.face_down") : landscapeName(lane.landscape), lane.frozen && h("span", { class: "frozentag" }, t("land.frozen_tag")));
}

function laneInfo(m, i, mc, oc) {
  const { ctx } = m;
  const v = ctx.view;
  const myTurn = v.active === v.you && v.phase !== "mulligan";
  let text = t("lane.quiet"), bg = "#f4e6bf";
  const atk = mc?.attack ?? 0;
  if (mc && mc.status === "ready") {
    if (myTurn && oc) { text = t("lane.fights", { name: cardName(ctx, oc.card) }); bg = "#ffe08a"; }
    else if (myTurn) { text = t("lane.hits_hero", { n: atk }); bg = "#c4ee9f"; }
    else text = t("lane.ready");
  } else if (mc && mc.status === "flooped") { text = t("lane.flooped"); bg = "#dccbf3"; }
  else if (mc && mc.status === "fought") { text = t("lane.fought"); bg = "#d9cfb8"; }
  else if (!mc && oc) { text = t(myTurn ? "lane.open_mine" : "lane.open"); bg = "#ffb39e"; }
  return h("div", { class: "info" }, h("span", { class: "pill", title: t("lane.title", { lane: i + 1 }), style: `background:${bg}` }, text));
}

/** One half of a Lane: Landscape background with the Creature on it. */
function tile(m, seat, i, on) {
  const { ctx, me, srcs, ui, srcCard } = m;
  const v = ctx.view, mine = seat === me;
  const lane = v.players[seat].lanes[i];
  const c = lane.creature;
  const key = `lane:${i}:creature`;

  const pickToken = [
    ...(mine && srcCard && srcCard.type !== "building" ? [`dest:${i}`] : []),
    `creature:${seat}:${i}`, `lane:${seat}:${i}`, `landscape:${seat}:${i}`,
  ].find((t) => has(m, t));
  const selectable = mine && srcs[key];
  const classes = ["tile", mine ? "mine" : "theirs", lane.face_down ? "down" : "up"];
  if (lane.frozen) classes.push("frozen");
  if (pickToken) classes.push("pick", mine ? "" : "foe");
  else if (selectable && !ui.picker) classes.push("go");
  if (selectable && ui.source === key) classes.push("sel");

  const click = () => {
    if (pickToken) return on.onToken(pickToken);
    if (selectable) return on.onLane(i, "creature", c?.card);
    if (c) on.onInspect(c.card);
  };
  const label = t("tile.label", { mine: mine ? "yes" : "no", lane: i + 1, down: lane.face_down ? "yes" : "no", type: landscapeName(lane.landscape), frozen: lane.frozen ? "yes" : "no", empty: c ? "no" : "yes", card: c ? cardName(ctx, c.card) : "" });
  const stats = c && [c.types && c.types.map(landscapeName).join(t("tile.types_sep")), c.protected && t("tile.protected"), c.stolen && t("tile.stolen"), c.silenced && (c.lent ? t("tile.lent", { name: cardName(ctx, c.lent) }) : t("tile.silenced")), c.under?.length > 0 && t("tile.under", { n: c.under.length }), c.gobbled?.length > 0 && t("tile.gobbled", { n: c.gobbled.length })].filter(Boolean);
  return h("button", {
    type: "button", class: classes.join(" "), "data-land": lane.face_down ? "" : lane.landscape, "aria-label": label,
    "data-seat": seat, "data-lane": i,
    onclick: click, onmouseenter: c ? () => on.onInspect(c.card, true) : null,
  },
    h("div", { class: "hex" }),
    lane.frozen && h("div", { class: "ice", "aria-hidden": "true" }, h("span", {}, "❄")),
    c && h("div", { class: `at ${c.status === "ready" ? "" : c.status}` }, cardEl(ctx.cards[c.card])),
    c && tileStats(ctx.cards[c.card], c),
    c && tokenRow(c.tokens),
    c && c.status !== "ready" && h("span", { class: "stat" }, t(c.status === "fought" ? "lane.fought" : "tile.flooped")),
    c && stats.length > 0 && c.status === "ready" && h("span", { class: "stat" }, fmtJoin(stats)));
}

function buildingStrip(m, seat, i, on) {
  const { ctx, me, srcs, ui, srcCard } = m;
  const mine = seat === me;
  const b = ctx.view.players[seat].lanes[i].building;
  const extra = ctx.view.players[seat].lanes[i].extra ?? [];
  const key = `lane:${i}:building`;
  const pickToken = [...(mine && srcCard?.type === "building" ? [`dest:${i}`] : []), `building:${seat}:${i}`].find((t) => has(m, t));
  const selectable = mine && srcs[key];
  const classes = ["strip", mine ? "mine" : "theirs", b ? "" : "empty"];
  if (pickToken) classes.push("pick", mine ? "" : "foe");
  else if (selectable && !ui.picker) classes.push("go");
  if (selectable && ui.source === key) classes.push("sel");
  const click = () => {
    if (pickToken) return on.onToken(pickToken);
    if (selectable) return on.onLane(i, "building", b?.card);
    if (b) on.onInspect(b.card);
  };
  const card = b && ctx.cards[b.card];
  const main = h("button", {
    type: "button", class: classes.join(" "), onclick: click, "data-seat": seat, "data-lane": i,
    "aria-label": t("strip.label", { mine: mine ? "yes" : "no", lane: i + 1, empty: b ? "no" : "yes", cards: [b, ...extra].filter(Boolean).map((x) => cardName(ctx, x.card)).join(t("strip.sep")) }),
    onmouseenter: b ? () => on.onInspect(b.card, true) : null,
  }, b
    ? h("span", { class: "row" }, h("span", { class: "orb" }, card?.cost ?? "–"), h("span", { class: "nm" }, cardName(ctx, b.card)), b.status === "flooped" && h("span", { class: "tag" }, t("tile.flooped")))
    : t(mine ? "strip.mine_empty" : "strip.theirs_empty"));
  if (extra.length === 0) return main;
  // Burly Lumberjack: the other Buildings in the Lane, each on a line of its own to read and inspect.
  return h("div", { class: "stripgroup" }, main, extra.map((x) => h("button", {
    type: "button", class: `strip extra ${mine ? "mine" : "theirs"}`, "data-seat": seat, "data-lane": i,
    "aria-label": t("strip.extra_label", { mine: mine ? "yes" : "no", lane: i + 1, name: cardName(ctx, x.card) }),
    onclick: () => on.onInspect(x.card), onmouseenter: () => on.onInspect(x.card, true),
  }, h("span", { class: "row" }, h("span", { class: "orb" }, ctx.cards[x.card]?.cost ?? "–"), h("span", { class: "nm" }, cardName(ctx, x.card)),
    x.status === "flooped" && h("span", { class: "tag" }, t("tile.flooped"))))));
}

// ---------- the modal for a Choice

/**
 * A Choice stops the game until someone decides (an opponent's spell makes
 * you discard, Ogre Gas shows cards). The player who decides
 * gets a modal dialog over everything; nothing else can be done until it is
 * answered, so it has no way to be dismissed.
 */
function choiceModal(m, on) {
  const { ctx, me, myDecision, legal, ui } = m;
  const v = ctx.view;
  const ch = v.choice;
  if (!ch || ch.player !== me || !myDecision || v.over) return null;
  const by = cardName(ctx, ch.card);
  const choose = legal.filter((a) => a.kind === "choose");
  const cardButton = (inst, action) => {
    const c = ctx.cards[inst.card];
    return h("button", { type: "button", class: "modalopt cardopt", "aria-label": c?.name ?? t("noun.card"), onclick: () => on.onAction(action) },
      cardEl(c ?? { id: inst.card, name: inst.card }), h("span", { class: "cap" }, c?.name ?? inst.card));
  };
  let title, text, body;
  if (ch.kind === "discard") {
    title = t("modal.discard_title", { n: ch.count });
    text = t("modal.discard_text", { name: by });
    body = h("div", { class: "modalcards" }, (v.players[me].hand ?? []).map((inst) => {
      const action = choose.find((a) => a.card === inst.uid);
      return action && cardButton(inst, action);
    }));
  } else if (ch.kind === "reveal") {
    title = by;
    text = t("modal.reveal_text");
    body = h("div", { class: "modalcards" }, (ch.revealed ?? []).map((inst) => {
      const action = choose.find((a) => a.card === inst.uid);
      return action && cardButton(inst, action);
    }));
  } else if (ch.kind === "search") {
    title = by;
    text = t("modal.search_text");
    body = h("div", { class: "modalcards" }, (ch.revealed ?? []).map((inst) => {
      const action = choose.find((a) => a.card === inst.uid);
      return action && cardButton(inst, action);
    }));
  } else if (ch.kind === "rebound") {
    const keep = choose.find((a) => !a.card);
    title = t("modal.rebound_title", { name: by });
    text = t("modal.rebound_text", { name: by });
    body = h("div", {},
      h("div", { class: "modalcards" }, (v.players[me].hand ?? []).map((inst) => {
        const action = choose.find((a) => a.card === inst.uid);
        return action && cardButton(inst, action);
      })),
      keep && h("div", { class: "modalrow" }, h("button", { type: "button", class: "btn-steel modalopt", onclick: () => on.onAction(keep) }, t("modal.leave_it"))));
  } else if (ch.kind === "redirect") {
    const stay = choose.find((a) => !a.card);
    title = t("modal.redirect_title", { name: by });
    text = t("modal.redirect_text", { name: by });
    body = h("div", {},
      h("div", { class: "modalcards" }, (ch.revealed ?? []).map((inst) => {
        const action = choose.find((a) => a.card === inst.uid);
        return action && cardButton(inst, action);
      })),
      stay && h("div", { class: "modalrow" }, h("button", { type: "button", class: "btn-red modalopt", onclick: () => on.onAction(stay) }, t("modal.redirect_same", { name: by }))));
  } else if (ch.kind === "defend") {
    const stay = choose.find((a) => !a.card);
    const attacker = v.players[ch.move_side]?.lanes?.[ch.lane]?.creature;
    title = t("modal.defend_title", { name: by });
    text = t("modal.defend_text", { name: by, attacker: attacker ? cardName(ctx, attacker.card) : "?" });
    body = h("div", {},
      h("div", { class: "modalcards" }, (ch.revealed ?? []).map((inst) => {
        const action = choose.find((a) => a.card === inst.uid);
        return action && cardButton(inst, action);
      })),
      stay && h("div", { class: "modalrow" }, h("button", { type: "button", class: "btn-steel modalopt", onclick: () => on.onAction(stay) }, t("modal.defend_none"))));
  } else if (ch.kind === "move_damage") {
    title = by;
    text = t("modal.move_damage_text", { n: ch.count });
    body = h("div", { class: "modalrow" }, choose.map((a) =>
      h("button", { type: "button", class: "btn-purple modalopt", onclick: () => on.onAction(a) }, String(a.card))));
  } else if (ch.kind === "move") {
    const theirs = movesToTheirSide(ch, v.you);
    title = t("modal.move_title", { name: by });
    text = t(theirs ? "modal.move_theirs" : "modal.move_mine", { name: by });
    body = h("div", { class: "modalrow modalrow2" }, choose.map((a) =>
      h("button", { type: "button", class: "btn-purple modalopt", onclick: () => on.onAction(a) }, t("modal.lane_n", { lane: (a.lane ?? 0) + 1 }))));
  } else if (ch.kind === "camel") {
    const gone = ch.revealed?.[0];
    const put = choose.find((a) => a.card);
    const let_go = choose.find((a) => !a.card);
    title = t("modal.camel_title", { name: by });
    text = t("modal.camel_text", { card: cardName(ctx, gone?.card), name: by });
    body = h("div", {},
      gone && h("div", { class: "modalcards" }, h("div", { class: "modalopt cardopt" }, cardEl(ctx.cards[gone.card] ?? { id: gone.card, name: gone.card }), h("span", { class: "cap" }, cardName(ctx, gone.card)))),
      h("div", { class: "modalrow" },
        put && h("button", { type: "button", class: "btn-purple modalopt", onclick: () => on.onAction(put) }, t("modal.camel_put", { name: by })),
        let_go && h("button", { type: "button", class: "btn-steel modalopt", onclick: () => on.onAction(let_go) }, t("modal.let_go"))));
  } else if (ch.kind === "landscape") {
    title = by;
    text = t("modal.landscape_text", { lane: ch.lane + 1 });
    body = h("div", { class: "modalrow modalrow2" }, choose.map((a) =>
      h("button", { type: "button", class: "btn-purple modalopt", onclick: () => on.onAction(a) }, landscapeName(LANDSCAPE_ORDER[a.card - 1]))));
  } else if (ch.kind === "target") {
    const none = choose.find((a) => !a.targets?.[0]?.length);
    title = t("modal.target_title", { name: by, label: ch.label ? labelText(ch.label) : t("modal.target_default") });
    text = t(ch.optional ? "modal.target_optional" : "modal.target_text", { name: by });
    body = h("div", {},
      h("div", { class: "modalrow modalrow2" }, choose.filter((a) => a.targets?.[0]?.length).map((a) =>
        h("button", { type: "button", class: "btn-purple modalopt", onclick: () => on.onAction(a) }, describeRef(ctx, a.targets[0][0])))),
      none && h("div", { class: "modalrow" }, h("button", { type: "button", class: "btn-steel modalopt", onclick: () => on.onAction(none) }, t("modal.choose_nothing"))));
  } else if (ch.kind === "ransom") {
    const decline = choose.find((a) => !a.card);
    title = t("modal.ransom_title", { name: by });
    text = t("modal.ransom_text", { name: by, min: ch.min_cost ?? 0 });
    body = h("div", {},
      h("div", { class: "modalcards" }, (v.players[me].hand ?? []).map((inst) => {
        const action = choose.find((a) => a.card === inst.uid);
        return action && cardButton(inst, action);
      })),
      decline && h("div", { class: "modalrow" }, h("button", { type: "button", class: "btn-red modalopt", onclick: () => on.onAction(decline) }, t("modal.take_it"))));
  } else if (ch.kind === "amount") {
    title = by;
    text = t("modal.amount_text", { options: ch.options.map(String) });
    body = h("div", { class: "modalrow" }, choose.map((a) =>
      h("button", { type: "button", class: "btn-purple modalopt", onclick: () => on.onAction(a) }, String(a.card))));
  } else if (ch.kind === "upkeep") {
    const destroy = choose.find((a) => !a.card);
    title = t("modal.upkeep_title", { name: by });
    text = t("modal.upkeep_text", { n: ch.count, name: by });
    body = h("div", {},
      h("div", { class: "modalcards" }, (v.players[me].hand ?? []).map((inst) => {
        const action = choose.find((a) => a.card === inst.uid);
        return action && cardButton(inst, action);
      })),
      destroy && h("div", { class: "modalrow" }, h("button", { type: "button", class: "btn-red modalopt", onclick: () => on.onAction(destroy) }, t("modal.destroy", { name: by }))));
  } else {
    return null;
  }
  return h("div", { class: `modal-backdrop${ui.modalAgain ? " again" : ""}` },
    h("div", { class: "modal parch", role: "dialog", "aria-modal": "true", "aria-labelledby": "modal-h" },
      h("h2", { id: "modal-h" }, title),
      h("p", {}, text),
      body));
}

/**
 * The discard viewer, one scrollable dialog: opened on request, or by a choice that needs a card from a pile. A card
 * that may take from either player's pile shows both, each under its own header, so nothing needs hunting for.
 */
function pileModal(m, on) {
  const { ctx, me, tokens, ui } = m;
  const v = ctx.view;
  if (v.over) return null;
  const seats = discardSeats(tokens, ui.discard, me, ui.picker?.options().piles);
  if (!seats.length) return null;
  const pileOf = (seat) => {
    const pile = v.players[seat].discard;
    const title = t(seat === me ? "pile.title_yours" : "pile.title_theirs", { who: seatName(ctx, seat), n: pile.length });
    return h("section", { class: "pilebox", "aria-label": title },
      h("h3", { class: "pilehead" }, title),
      pile.length === 0 ? h("p", { class: "pileempty" }, t("pile.empty")) :
        h("div", { class: "pilegrid" }, pile.map((x) => {
          const token = `discard_card:${seat}:${x.uid}`;
          const pick = tokens.has(token);
          return h("button", {
            type: "button", class: pick ? "pick" : "", "aria-label": cardName(ctx, x.card),
            onclick: () => (pick ? on.onToken(token) : on.onInspect(x.card)), onmouseenter: () => on.onInspect(x.card, true),
          }, cardEl(ctx.cards[x.card]));
        })));
  };
  const wantsCard = tokens.size > 0 && [...tokens].some((tk) => tk.startsWith("discard_card:"));
  // Opened by hand: Close. Opened by the choice: nothing to close, the card is what is wanted, but it can be called off.
  const done = ui.discard != null
    ? h("button", { type: "button", class: "btn-steel", onclick: () => on.onDiscard(null) }, t("common.close"))
    : h("button", { type: "button", class: "btn-steel", onclick: on.onCancel }, t("prompt.cancel"));
  return h("div", { class: "modal-backdrop again" },
    h("div", { class: "modal parch pilemodal", role: "dialog", "aria-modal": "true", "aria-labelledby": "pile-h" },
      h("h2", { id: "pile-h" }, t(wantsCard ? (seats.length > 1 ? "pile.pick_either" : "pile.pick") : "pile.view")),
      h("div", { class: "piles" }, seats.map(pileOf)),
      h("div", { class: "pilefoot" }, done)));
}

// ---------- overlays on the felt

function overlay(m, on) {
  const { ctx, me, myDecision, legal, tokens } = m;
  const v = ctx.view;
  if (v.over) {
    const won = v.winner === me;
    return h("div", { class: "veil" }, h("div", { class: "parch" },
      h("h2", {}, won ? t("over.you_win") : t("over.they_win", { who: seatName(ctx, v.winner) })),
      h("p", {}, t(won ? "over.you_text" : "over.they_text")),
      h("div", { class: "row" }, h("button", { type: "button", class: "btn-green", onclick: on.onLeave }, t("over.back")))));
  }
  if (v.phase === "mulligan" && myDecision) {
    return h("div", { class: "veil" }, h("div", { class: "parch" },
      h("h2", {}, t("mull.title")),
      h("p", {}, t("mull.text")),
      h("div", { class: "row" },
        legal.filter((a) => a.kind === "keep_hand").map((a) => h("button", { type: "button", class: "btn-green", onclick: () => on.onAction(a) }, t("mull.keep"))),
        legal.filter((a) => a.kind === "mulligan").map((a) => h("button", { type: "button", class: "btn-red", onclick: () => on.onAction(a) }, t("mull.mulligan"))))));
  }
  return null;
}

// ---------- hand and actions

function handSection(m, on) {
  const { ctx, me, srcs, ui, tokens, legal, myDecision } = m;
  const v = ctx.view, p = v.players[me];
  const hand = p.hand ?? [];
  const n = hand.length;
  const crowded = isCrowded(n); // more than a row's worth: the hand wraps and lies flat
  const cardsEl = hand.map((inst, k) => {
    const card = ctx.cards[inst.card];
    const key = `hand:${inst.uid}`;
    const token = `hand_card:${inst.uid}`;
    const pick = tokens.has(token);
    const playable = !!srcs[key];
    const sel = ui.source === key;
    const pose = fanPose(k, n);
    const xf = sel ? "translateY(-30px) rotate(0deg)" : crowded ? "none" : `translateY(${pose.drop}px) rotate(${pose.rotate}deg)`;
    const cls = ["hbtn", sel ? "sel" : "", pick ? "pick" : "", p.rainbow_hand?.includes(inst.uid) ? "rainbow" : "", playable && !ui.picker ? "go" : "", playable || pick || sel ? "" : "off"].join(" ");
    return h("button", {
      type: "button", class: cls, style: `transform:${xf}`, "data-uid": inst.uid, "aria-pressed": sel ? "true" : "false",
      "aria-label": t("hand.card_label", { name: card?.name ?? inst.card, cost: card?.cost ?? "?", playable: playable ? "yes" : "no" }),
      onclick: () => (pick ? on.onToken(token) : playable ? on.onHand(inst.uid, inst.card) : on.onInspect(inst.card)),
      onmouseenter: () => on.onInspect(inst.card, true),
    }, cardEl(card));
  });

  const draw = legal.find((a) => a.kind === "draw");
  const end = legal.find((a) => a.kind === "end_main");
  const canFight = v.turn > 1 && p.lanes.some((l) => l.creature?.status === "ready");
  const myMain = myDecision && v.phase === "main";
  const pending = srcs.pending && !ui.picker;
  return h("section", { class: "steel handbox", "aria-label": t("hand.region") },
    h("div", { class: "piles" },
      h("div", { class: "well" }, h("span", {}, t("hand.deck")), h("b", {}, p.deck_count)),
      h("button", { type: "button", class: "well", onclick: () => on.onDiscard(me), "aria-label": t("hand.discard_label", { n: p.discard.length }) }, h("span", {}, t("player.discard")), h("b", {}, p.discard.length))),
    h("div", { class: `well hand${crowded ? " crowded" : ""}`, style: `--n:${Math.max(n, 1)}` }, n === 0 ? h("span", { class: "empty" }, t("hand.empty")) : cardsEl),
    h("div", { class: "acts" },
      h("div", { class: "actcount" },
        h("div", { class: "orb", role: "img", "aria-label": t("hand.actions_label", { n: v.actions_left }) }, myMain || v.active === me ? v.actions_left : "–"),
        h("span", { class: "lbl" }, t("hand.actions"))),
      h("div", { class: "btns" },
        pending && h("button", { type: "button", class: "btn-purple", style: "min-height:40px;font-size:16px", onclick: on.onPending }, t("hand.play_stolen")),
        h("button", { type: "button", class: "btn-steel", disabled: !draw || !!ui.picker, onclick: () => draw && on.onAction(draw) }, t("hand.draw")),
        end && !ui.picker && (canFight
          ? h("button", { type: "button", class: "btn-red", onclick: () => on.onAction(end) }, t("confirm.attack"))
          : h("button", { type: "button", class: "btn-green", onclick: () => on.onAction(end) }, t("hand.end_turn"))))));
}

// ---------- sidebar

function detailPanel(ctx, cardId) {
  const c = cardId && ctx.cards[cardId];
  return h("section", { class: "steel detail", "data-role": "detail", "aria-labelledby": "det-h" },
    h("h2", { class: "h", id: "det-h" }, t("detail.title")),
    h("div", { class: "well" }, c
      ? [cardEl(c),
        h("div", { class: "tl" }, [c.type && t(`type.${c.type}`), c.landscape && landscapeName(c.landscape), c.cost != null && t("detail.cost", { cost: c.cost })].filter(Boolean).join(t("detail.sep"))),
        c.image_url && c.text && h("div", { class: "txt" }, c.text),
        c.modeled === false && h("div", { class: "txt", style: "color:#ffb39e" }, t("detail.unmodelled"))]
      : h("p", {}, t("detail.hint"))));
}

/** Under "Card up close": the bug, card and feature forms, when the server offers them. They open in a new tab, so the game is left as it is. */
function feedbackPanel(links) {
  if (!links?.length) return null;
  return h("section", { class: "steel feedbackpanel", "aria-labelledby": "fb-h" },
    h("h2", { class: "h", id: "fb-h" }, t("feedback.title")),
    h("div", { class: "fbrow" }, links.map((l) => h("a", { class: "btn-steel", href: l.url, target: "_blank", rel: "noopener" }, l.label))));
}

function logPanel(state, ctx, on) {
  const lines = state.log.slice(-LOG_SHOWN).reverse();
  const names = nameMatcher(ctx.cards);
  return h("section", { class: "parch log", "aria-labelledby": "log-h" },
    h("div", { class: "loghead" },
      h("h2", { id: "log-h" }, t("log.title")),
      h("div", { class: "logbtns" },
        h("button", { type: "button", class: "btn-steel", onclick: (e) => on.onCopyLog(e.currentTarget), title: t("log.copy_title") }, t("log.copy")),
        h("button", { type: "button", class: "btn-steel", onclick: () => on.onDownloadLog(), title: t("log.save_title") }, t("log.save")))),
    lines.length === 0 ? h("p", { class: "muted", style: "margin:0" }, t("log.empty")) :
      h("ul", {}, lines.map((l) => h("li", { class: l.startsWith("──") ? "turn" : l.startsWith("***") ? "big" : "" },
        ...logParts(l.replaceAll("──", "").replaceAll("***", "").trim(), names, on)))));
}

/** A regex (longest names first) and name -> card id, or null with no catalog. */
function nameMatcher(cards) {
  const byName = new Map();
  for (const c of Object.values(cards ?? {})) if (c.name && !byName.has(c.name)) byName.set(c.name, c.id);
  if (byName.size === 0) return null;
  const alts = [...byName.keys()].sort((a, b) => b.length - a.length).map((n) => n.replace(/[.*+?^${}()|[\]\\]/g, "\\$&"));
  return { re: new RegExp(alts.join("|"), "g"), byName };
}

/** A log line as text with each card name made hoverable. */
function logParts(line, names, on) {
  if (!names) return [line];
  const out = [];
  let at = 0;
  for (const m of line.matchAll(names.re)) {
    if (m.index > at) out.push(line.slice(at, m.index));
    const id = names.byName.get(m[0]);
    out.push(h("span", { class: "cardref", tabindex: "0", onmouseenter: () => on.onInspect(id, true), onfocus: () => on.onInspect(id, true), onclick: () => on.onInspect(id) }, m[0]));
    at = m.index + m[0].length;
  }
  if (at < line.length) out.push(line.slice(at));
  return out;
}

/** The tokens on a Creature as a column of chips under its ATK: an icon and a count for each kind it has. */
function tokenRow(tokens) {
  const chips = tokenChips(tokens);
  if (!chips.length) return null;
  return h("div", { class: "ttokens" }, chips.map((k) =>
    h("span", { class: `ttoken ${k.kind}`, title: t(k.title, { n: k.n }), "aria-label": t(k.title, { n: k.n }) }, h("i", { "aria-hidden": "true" }, k.icon), k.n)));
}

/** Attack and Defense left, big and beside the card (not rotating with it). */
function tileStats(card, c) {
  const base = card?.attack ?? 0;
  const atk = c.attack ?? base;
  const left = (c.defense ?? card?.defense ?? 0) - (c.damage ?? 0);
  return h("div", { class: "tstats" },
    h("span", { class: `tstat atk ${atk > base ? "up" : ""}`, title: t("stat.attack") }, h("small", {}, t("stat.atk")), atk),
    h("span", { class: `tstat def ${c.damage > 0 ? "hurt" : ""}`, title: t("stat.defense") }, h("small", {}, t("stat.def")), left));
}
