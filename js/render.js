// Draws the battle scene from the current state. It rebuilds the DOM on every
// update (the scene is small) and reports clicks through `on`. Which cards may
// be clicked comes from the picker (js/picker.js); nothing here knows a card's
// rules.
import { h, svg } from "./dom.js";
import { cardEl } from "./card.js";
import { cardName, landscapeName, seatName } from "./describe.js";
import { isMyDecision } from "./state.js";
import { sources, parseToken } from "./picker.js";
import { buildPrompt } from "./prompt.js";

const SHIELD = `<path d="M24 3 L44 9 V26 C44 40 35 48 24 51 C13 48 4 40 4 26 V9 Z" fill="#3f8fd6" stroke="#16212b" stroke-width="3.5" stroke-linejoin="round"/><path d="M24 8 L39 12.5 V26 C39 37 32 43.5 24 46" fill="none" stroke="#a8d6ff" stroke-width="2.5" opacity="0.7"/>`;
const MAX_HP = 25;
const LOG_SHOWN = 40;

/**
 * @param {HTMLElement} root  the #game element
 * @param {{state:object, cards:object, ui:{source:string, picker:object|null, awaitingKind:boolean, inspect:string|null, discard:number|null}}} model
 * @param {object} on  handlers: onHand onLane onFloop onToken onKind onSkip onConfirm onCancel
 *                     onAction onPending onInspect onDiscard onLeave
 */
export function renderGame(root, model, on) {
  const { state, cards, ui } = model;
  const v = state.view;
  if (!v) {
    root.replaceChildren(h("div", { class: "table" },
      h("div", { class: "topbar" }, h("div", { class: "l" }, leaveButton(on), h("h1", { class: "h" }, "Card Wars"))),
      h("div", { class: "steel", style: "padding:24px 28px" },
        h("h2", { class: "h", style: "font-size:30px" }, state.waiting ? `Waiting for an opponent — room ${state.session?.room ?? ""}` : "Joining…"),
        state.waiting && h("p", {}, "Send this code to your friend; they choose “Join”."))));
    return;
  }
  const ctx = { view: v, cards, names: state.names, known: state.known };
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
        playerBar(model2, opp, on),
        promptBar(model2, on),
        battlefield(model2, on),
        playerBar(model2, me, on),
        handSection(model2, on)),
      h("div", { class: "side" }, detailPanel(ctx, ui.inspect), logPanel(state)))));
}

/** Replace just the "Card up close" panel (used on hover, so the scene is not rebuilt). */
export function updateDetail(root, ctx, cardId) {
  const old = root.querySelector('[data-role="detail"]');
  if (old) old.replaceWith(detailPanel(ctx, cardId));
}

const leaveButton = (on) => h("button", { type: "button", class: "btn-steel", style: "min-height:44px;font-size:19px", onclick: on.onLeave }, "‹ Lobby");

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
  const phase = v.phase === "mulligan" ? "Opening hands" : `${v.phase[0].toUpperCase()}${v.phase.slice(1)} phase`;
  return h("div", { class: "topbar" },
    h("div", { class: "l" }, leaveButton(on), h("h1", { class: "h" }, "Card Wars")),
    h("div", { class: "steel turnbox" },
      v.phase !== "mulligan" && h("span", { class: "ink" }, `Turn ${v.turn}`),
      v.phase !== "mulligan" && h("span", { class: `chip ${mine ? "" : "theirs"}` }, mine ? "Your turn" : `${seatName(ctx, v.active)}'s turn`),
      h("span", { class: "ink", style: "font-size:17px" }, phase)));
}

function playerBar({ ctx, me, ui, tokens }, seat, on) {
  const v = ctx.view, p = v.players[seat], mine = seat === me;
  const hpw = Math.max(0, Math.min(100, Math.round((p.hp / MAX_HP) * 100)));
  const backs = !mine && Array.from({ length: Math.min(p.hand_count, 10) }, () => h("div", { class: "back", "aria-hidden": "true" }, h("i")));
  const pile = (label, n, clickable) => h("button", {
    type: "button", class: "well", disabled: !clickable, onclick: () => on.onDiscard(seat),
    "aria-label": `${label}: ${n} cards`,
  }, `${label} ${n}`);
  return h("section", { class: "steel pbar", "aria-label": mine ? "You" : "Opponent" },
    h("div", { class: "shield", role: "img", "aria-label": `${mine ? "You have" : seatName(ctx, seat) + " has"} ${p.hp} of ${MAX_HP} Hit Points` },
      svg(SHIELD, { viewBox: "0 0 48 54" }), h("span", {}, p.hp)),
    h("div", { class: "well avatar", "aria-hidden": "true", style: `background:${mine ? "#c98a1f" : "#2e5f9c"}` }, seatName(ctx, seat)[0]?.toUpperCase() ?? "?"),
    h("div", { class: "pname" },
      h("div", { class: "h" }, mine ? `${seatName(ctx, seat)} (you)` : seatName(ctx, seat)),
      h("div", { class: "sub" }, mine ? `${p.hand?.length ?? 0} in hand` : `${p.hand_count} in hand`)),
    backs && h("div", { class: "backs", "aria-hidden": "true" }, backs),
    !mine && h("div", { class: "counts" }, h("span", { class: "well", style: "padding:2px 10px;border-radius:999px" }, `Deck ${p.deck_count}`), pile("Discard", p.discard.length, true)),
    h("div", { class: "well hpbar", "aria-hidden": "true" }, h("div", { style: `width:${hpw}%` })));
}

function promptBar({ prompt, myDecision, ctx }, on) {
  const kids = [h("span", {}, prompt.text)];
  for (const [key, label] of prompt.kinds) kids.push(h("button", { type: "button", class: "btn-purple", onclick: () => on.onKind(key) }, label));
  if (prompt.confirm) kids.push(h("button", { type: "button", class: "btn-green", onclick: on.onConfirm }, prompt.confirm));
  if (prompt.skip) kids.push(h("button", { type: "button", class: "btn-steel", onclick: on.onSkip }, prompt.skip));
  if (prompt.cancel) kids.push(h("button", { type: "button", class: "btn-steel", onclick: on.onCancel }, "Cancel"));
  return h("div", { role: "status", class: `well prompt ${myDecision && !ctx.view.over ? "you" : ""}` }, kids);
}

// ---------- battlefield

function battlefield(m, on) {
  const { ctx, me, opp } = m;
  const v = ctx.view;
  const felt = h("section", { class: "felt", style: "padding:16px", "aria-label": "Battlefield" },
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
  const canFloop = (srcs[`lane:${i}:creature`] ?? []).some((a) => a.kind === "floop") && !ui.picker;
  return h("div", { class: "lane" },
    buildingStrip(m, opp, i, on),
    tile(m, opp, i, on),
    laneInfo(m, i, mc, oc),
    tile(m, me, i, on),
    h("div", { class: "floop" }, canFloop && h("button", { type: "button", class: "btn-purple", "aria-label": `Floop ${cardName(ctx, mc?.card)}`, onclick: () => on.onFloop(i, "creature") }, "Floop >>>")),
    buildingStrip(m, me, i, on));
}

function laneInfo(m, i, mc, oc) {
  const { ctx } = m;
  const v = ctx.view;
  const myTurn = v.active === v.you && v.phase !== "mulligan";
  let text = "Quiet lane", bg = "#f4e6bf";
  const atk = mc?.attack ?? 0;
  if (mc && mc.status === "ready") {
    if (myTurn && oc) { text = `Fights ${cardName(ctx, oc.card)}`; bg = "#ffe08a"; }
    else if (myTurn) { text = `Hits Hero for ${atk}`; bg = "#c4ee9f"; }
    else text = "Ready";
  } else if (mc && mc.status === "flooped") { text = "Flooped, no Fight"; bg = "#dccbf3"; }
  else if (mc && mc.status === "fought") { text = "Fought"; bg = "#d9cfb8"; }
  else if (!mc && oc) { text = myTurn ? "Open lane!" : "Open lane"; bg = "#ffb39e"; }
  return h("div", { class: "info" }, h("small", {}, `Lane ${i + 1}`), h("span", { class: "pill", style: `background:${bg}` }, text));
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
  if (pickToken) classes.push("pick", mine ? "" : "foe");
  else if (selectable && !ui.picker) classes.push("go");
  if (selectable && ui.source === key) classes.push("sel");

  const click = () => {
    if (pickToken) return on.onToken(pickToken);
    if (selectable) return on.onLane(i, "creature", c?.card);
    if (c) on.onInspect(c.card);
  };
  const label = `${mine ? "Your" : "Opponent"} Lane ${i + 1}, ${lane.face_down ? "face-down" : landscapeName(lane.landscape)}${c ? `: ${cardName(ctx, c.card)}` : ", empty"}`;
  const stats = c && [c.protected && "protected", c.stolen && "stolen", c.silenced && (c.lent ? `lent: ${cardName(ctx, c.lent)}` : "silenced"), c.damage > 0 && `dmg ${c.damage}`].filter(Boolean);
  return h("button", {
    type: "button", class: classes.join(" "), "data-land": lane.face_down ? "" : lane.landscape, "aria-label": label,
    onclick: click, onmouseenter: c ? () => on.onInspect(c.card, true) : null,
  },
    h("div", { class: "hex" }),
    h("span", { class: "land" }, lane.face_down ? "Face-down" : landscapeName(lane.landscape)),
    c && h("div", { class: `at ${c.status === "ready" ? "" : c.status}` }, cardEl(ctx.cards[c.card], { atk: c.attack, def: c.defense, damage: c.damage })),
    c && c.status !== "ready" && h("span", { class: "stat" }, c.status === "fought" ? "Fought" : "Flooped"),
    c && stats.length > 0 && c.status === "ready" && h("span", { class: "stat" }, stats.join(" · ")));
}

function buildingStrip(m, seat, i, on) {
  const { ctx, me, srcs, ui, srcCard } = m;
  const mine = seat === me;
  const b = ctx.view.players[seat].lanes[i].building;
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
  return h("button", {
    type: "button", class: classes.join(" "), onclick: click,
    "aria-label": `${mine ? "Your" : "Opponent"} Building slot, Lane ${i + 1}${b ? `: ${cardName(ctx, b.card)}` : ", empty"}`,
    onmouseenter: b ? () => on.onInspect(b.card, true) : null,
  }, b
    ? h("span", { class: "row" }, h("span", { class: "orb" }, card?.cost ?? "–"), h("span", { class: "nm" }, cardName(ctx, b.card)), b.status === "flooped" && h("span", { class: "tag" }, "Flooped"))
    : (mine ? "Building slot" : "No Building"));
}

// ---------- overlays on the felt

function overlay(m, on) {
  const { ctx, me, myDecision, legal, tokens } = m;
  const v = ctx.view;
  if (v.over) {
    const won = v.winner === me;
    return h("div", { class: "veil" }, h("div", { class: "parch" },
      h("h2", {}, won ? "You win!" : `${seatName(ctx, v.winner)} wins`),
      h("p", {}, won ? "You are the Cool Guy." : "Better luck next time."),
      h("div", { class: "row" }, h("button", { type: "button", class: "btn-green", onclick: on.onLeave }, "Back to lobby"))));
  }
  if (v.phase === "mulligan" && myDecision) {
    return h("div", { class: "veil" }, h("div", { class: "parch" },
      h("h2", {}, "Your opening hand"),
      h("p", {}, "Keep it, or mulligan once: shuffle it back and draw a new five."),
      h("div", { class: "row" },
        legal.filter((a) => a.kind === "keep_hand").map((a) => h("button", { type: "button", class: "btn-green", onclick: () => on.onAction(a) }, "Keep")),
        legal.filter((a) => a.kind === "mulligan").map((a) => h("button", { type: "button", class: "btn-red", onclick: () => on.onAction(a) }, "Mulligan")))));
  }
  // The discard viewer: opened on request, or by a choice that needs a card from a pile.
  const wanted = [...tokens].map(parseToken).find((t) => t.kind === "discard_card");
  const seat = m.ui.discard ?? wanted?.player ?? null;
  if (seat != null) {
    const pile = v.players[seat].discard;
    return h("div", { class: "veil pile" }, h("div", { class: "parch" },
      h("h2", {}, `${seatName(ctx, seat)}'s discard pile (${pile.length})`),
      pile.length === 0 ? h("p", {}, "Empty.") :
        h("div", { class: "pilegrid" }, pile.map((x) => {
          const token = `discard_card:${seat}:${x.uid}`;
          const pick = tokens.has(token);
          return h("button", {
            type: "button", class: pick ? "pick" : "", "aria-label": cardName(ctx, x.card),
            onclick: () => (pick ? on.onToken(token) : on.onInspect(x.card)), onmouseenter: () => on.onInspect(x.card, true),
          }, cardEl(ctx.cards[x.card]));
        })),
      h("div", { class: "row" }, h("button", { type: "button", class: "btn-steel", style: "min-height:44px;font-size:19px", onclick: () => on.onDiscard(seat) }, "Close"))));
  }
  return null;
}

// ---------- hand and actions

function handSection(m, on) {
  const { ctx, me, srcs, ui, tokens, legal, myDecision } = m;
  const v = ctx.view, p = v.players[me];
  const hand = p.hand ?? [];
  const n = hand.length;
  const cardsEl = hand.map((inst, k) => {
    const card = ctx.cards[inst.card];
    const key = `hand:${inst.uid}`;
    const token = `hand_card:${inst.uid}`;
    const pick = tokens.has(token);
    const playable = !!srcs[key];
    const sel = ui.source === key;
    const off = k - (n - 1) / 2;
    const xf = sel ? "translateY(-30px) rotate(0deg)" : `translateY(${Math.abs(off) * 7}px) rotate(${off * 4}deg)`;
    const cls = ["hbtn", sel ? "sel" : "", pick ? "pick" : "", playable && !ui.picker ? "go" : "", playable || pick || sel ? "" : "off"].join(" ");
    return h("button", {
      type: "button", class: cls, style: `transform:${xf}`, "aria-pressed": sel ? "true" : "false",
      "aria-label": `${card?.name ?? inst.card}, cost ${card?.cost ?? "?"}${playable ? "" : ", cannot be played now"}`,
      onclick: () => (pick ? on.onToken(token) : playable ? on.onHand(inst.uid, inst.card) : on.onInspect(inst.card)),
      onmouseenter: () => on.onInspect(inst.card, true),
    }, cardEl(card));
  });

  const draw = legal.find((a) => a.kind === "draw");
  const end = legal.find((a) => a.kind === "end_main");
  const canFight = v.turn > 1 && p.lanes.some((l) => l.creature?.status === "ready");
  const myMain = myDecision && v.phase === "main";
  const pending = srcs.pending && !ui.picker;
  return h("section", { class: "steel handbox", "aria-label": "Your hand and actions" },
    h("div", { class: "piles" },
      h("div", { class: "well" }, h("span", {}, "Deck"), h("b", {}, p.deck_count)),
      h("button", { type: "button", class: "well", onclick: () => on.onDiscard(me), "aria-label": `Your discard pile, ${p.discard.length} cards` }, h("span", {}, "Discard"), h("b", {}, p.discard.length))),
    h("div", { class: "well hand" }, n === 0 ? h("span", { class: "empty" }, "Your hand is empty") : cardsEl),
    h("div", { class: "acts" },
      h("div", { style: "display:flex;flex-direction:column;align-items:center;gap:12px" },
        h("div", { class: "orb", role: "img", "aria-label": `${v.actions_left} of 2 Actions left` }, myMain || v.active === me ? v.actions_left : "–"),
        h("span", { class: "lbl" }, "Actions left")),
      pending && h("button", { type: "button", class: "btn-purple", style: "min-height:48px;font-size:18px", onclick: on.onPending }, "Play the stolen card"),
      h("button", { type: "button", class: "btn-steel", disabled: !draw || !!ui.picker, onclick: () => draw && on.onAction(draw) }, "Draw a card · 1 Action"),
      end && !ui.picker && (canFight
        ? h("button", { type: "button", class: "btn-red", onclick: () => on.onAction(end) }, "Fight!")
        : h("button", { type: "button", class: "btn-green", onclick: () => on.onAction(end) }, "End turn"))));
}

// ---------- sidebar

function detailPanel(ctx, cardId) {
  const c = cardId && ctx.cards[cardId];
  return h("section", { class: "steel detail", "data-role": "detail", "aria-labelledby": "det-h" },
    h("h2", { class: "h", id: "det-h" }, "Card up close"),
    h("div", { class: "well" }, c
      ? [cardEl(c),
        h("div", { class: "tl" }, [c.type && c.type[0].toUpperCase() + c.type.slice(1), c.landscape && landscapeName(c.landscape), c.cost != null && `Cost ${c.cost}`].filter(Boolean).join(" · ")),
        c.image_url && c.text && h("div", { class: "txt" }, c.text),
        c.modeled === false && h("div", { class: "txt", style: "color:#ffb39e" }, "Rules not modelled by the engine yet")]
      : h("p", {}, "Point at or click a card to read it big.")));
}

function logPanel(state) {
  const lines = state.log.slice(-LOG_SHOWN).reverse();
  return h("section", { class: "parch log", "aria-labelledby": "log-h" },
    h("h2", { id: "log-h" }, "What just happened"),
    lines.length === 0 ? h("p", { class: "muted", style: "margin:0" }, "Nothing yet.") :
      h("ul", {}, lines.map((l) => h("li", { class: l.startsWith("──") ? "turn" : l.startsWith("***") ? "big" : "" }, l.replaceAll("──", "").replaceAll("***", "").trim()))));
}
