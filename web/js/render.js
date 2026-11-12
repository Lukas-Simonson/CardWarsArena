// Draws the battle scene from the current state. It rebuilds the DOM on every
// update (the scene is small) and reports clicks through `on`. Which cards may
// be clicked comes from the picker (js/picker.js); nothing here knows a card's
// rules.
import { APP_NAME } from "./brand.js";
import { h, svg } from "./dom.js";
import { avatarEl } from "./avatar.js";
import { cardEl } from "./card.js";
import { cardName, landscapeName, seatName, movesToTheirSide } from "./describe.js";
import { isMyDecision } from "./state.js";
import { sources, parseToken } from "./picker.js";
import { buildPrompt } from "./prompt.js";

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
        h("h2", { class: "h", style: "font-size:30px" }, state.waiting ? "Waiting for an opponent…" : "Joining…"),
        state.waiting && h("p", {}, "Send this room code to your friend; they type it under “Join”."),
        state.waiting && h("p", { class: "roomcode", "aria-label": "Room code" }, state.session?.room ?? ""))));
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
    choiceModal(model2, on)));
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
    h("div", { class: "l" }, leaveButton(on), h("h1", { class: "h" }, APP_NAME)),
    h("div", { class: "steel turnbox" },
      v.phase !== "mulligan" && h("span", { class: "ink" }, `Turn ${v.turn}`),
      v.phase !== "mulligan" && h("span", { class: `chip ${mine ? "" : "theirs"}` }, mine ? "Your turn" : `${seatName(ctx, v.active)}'s turn`),
      h("span", { class: "ink", style: "font-size:17px" }, phase)));
}

function playerBar(m, seat, on) {
  const { ctx, me, ui, tokens } = m;
  const v = ctx.view, p = v.players[seat], mine = seat === me;
  const hpw = Math.max(0, Math.min(100, Math.round((p.hp / MAX_HP) * 100)));
  const backs = !mine && Array.from({ length: Math.min(p.hand_count, 10) }, () => h("div", { class: "back", "aria-hidden": "true" }, h("i")));
  const pile = (label, n, clickable) => h("button", {
    type: "button", class: "well", disabled: !clickable, onclick: () => on.onDiscard(seat),
    "aria-label": `${label}: ${n} cards`,
  }, `${label} ${n}`);
  return h("section", { class: "steel pbar", "aria-label": mine ? "You" : "Opponent" },
    h("div", { class: "shield", "data-seat": seat, role: "img", "aria-label": `${mine ? "You have" : seatName(ctx, seat) + " has"} ${p.hp} of ${MAX_HP} Hit Points` },
      svg(SHIELD, { viewBox: "0 0 48 54" }), h("span", {}, p.hp)),
    avatarEl(seatName(ctx, seat), ctx.avatars?.[seat], { style: `background:${mine ? "#c98a1f" : "#2e5f9c"}` }),
    h("div", { class: "pname" },
      h("div", { class: "h" }, mine ? `${seatName(ctx, seat)} (you)` : seatName(ctx, seat)),
      h("div", { class: "sub" }, mine ? `${p.hand?.length ?? 0} in hand` : `${p.hand_count} in hand`)),
    heroChip(m, seat, on),
    backs && h("div", { class: "backs", "aria-hidden": "true" }, backs),
    !mine && h("div", { class: "counts" }, h("span", { class: "well", style: "padding:2px 10px;border-radius:999px" }, `Deck ${p.deck_count}`), pile("Discard", p.discard.length, true)),
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
      "aria-label": `${mine ? "Your" : "Opponent's"} Hero: ${name}`, title: ctx.cards[hero.card]?.text ?? "",
      onclick: () => on.onInspect(hero.card), onmouseenter: () => on.onInspect(hero.card, true),
    }, h("span", { class: "tag" }, "Hero"), h("span", { class: "nm" }, name)),
    abilities.map((n) => h("button", { type: "button", class: "btn-purple floop", "aria-label": `Use ${name}'s ability`, onclick: () => on.onHeroAbility(n) }, "Use ability")));
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
  const felt = h("section", { class: "felt", style: "padding:10px", "aria-label": "Battlefield" },
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
        abilities.map((n) => h("button", { type: "button", class: "btn-purple floop", "aria-label": `Use ${cardName(ctx, mc?.card)}'s ability`, onclick: () => on.onAbility(i, "creature", n) }, "Use ability")),
        canFloop && h("button", { type: "button", class: "btn-purple floop", "aria-label": `Floop ${cardName(ctx, mc?.card)}`, onclick: () => on.onFloop(i, "creature") }, "Floop >>>"),
        canThaw && h("button", { type: "button", class: "btn-ice floop thaw", "aria-label": `Discard a card to remove the frozen token on your lane ${i + 1} Landscape`, onclick: () => on.onThaw(i) }, "❄ Thaw"))),
    landName(ctx, me, i),
    buildingStrip(m, me, i, on));
}

/** The Landscape's name, shown beside the tile so it never covers a card. */
function landName(ctx, seat, i) {
  const lane = ctx.view.players[seat].lanes[i];
  return h("div", { class: "landname" }, lane.face_down ? "Face-down" : landscapeName(lane.landscape), lane.frozen && h("span", { class: "frozentag" }, " · Frozen"));
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
  return h("div", { class: "info" }, h("span", { class: "pill", title: `Lane ${i + 1}`, style: `background:${bg}` }, text));
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
  const label = `${mine ? "Your" : "Opponent"} Lane ${i + 1}, ${lane.face_down ? "face-down" : landscapeName(lane.landscape)}${lane.frozen ? ", frozen" : ""}${c ? `: ${cardName(ctx, c.card)}` : ", empty"}`;
  const stats = c && [c.types && c.types.map(landscapeName).join(" / "), c.protected && "protected", c.stolen && "stolen", c.silenced && (c.lent ? `lent: ${cardName(ctx, c.lent)}` : "silenced")].filter(Boolean);
  return h("button", {
    type: "button", class: classes.join(" "), "data-land": lane.face_down ? "" : lane.landscape, "aria-label": label,
    "data-seat": seat, "data-lane": i,
    onclick: click, onmouseenter: c ? () => on.onInspect(c.card, true) : null,
  },
    h("div", { class: "hex" }),
    lane.frozen && h("div", { class: "ice", "aria-hidden": "true" }, h("span", {}, "❄")),
    c && h("div", { class: `at ${c.status === "ready" ? "" : c.status}` }, cardEl(ctx.cards[c.card])),
    c && tileStats(ctx.cards[c.card], c),
    c && c.status !== "ready" && h("span", { class: "stat" }, c.status === "fought" ? "Fought" : "Flooped"),
    c && stats.length > 0 && c.status === "ready" && h("span", { class: "stat" }, stats.join(" · ")));
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
    "aria-label": `${mine ? "Your" : "Opponent"} Building slot, Lane ${i + 1}${b ? `: ${[b, ...extra].map((x) => cardName(ctx, x.card)).join(", ")}` : ", empty"}`,
    onmouseenter: b ? () => on.onInspect(b.card, true) : null,
  }, b
    ? h("span", { class: "row" }, h("span", { class: "orb" }, card?.cost ?? "–"), h("span", { class: "nm" }, cardName(ctx, b.card)), b.status === "flooped" && h("span", { class: "tag" }, "Flooped"))
    : (mine ? "Building slot" : "No Building"));
  if (extra.length === 0) return main;
  // Burly Lumberjack: the other Buildings in the Lane, each on a line of its own to read and inspect.
  return h("div", { class: "stripgroup" }, main, extra.map((x) => h("button", {
    type: "button", class: `strip extra ${mine ? "mine" : "theirs"}`, "data-seat": seat, "data-lane": i,
    "aria-label": `${mine ? "Your" : "Opponent"} extra Building, Lane ${i + 1}: ${cardName(ctx, x.card)}`,
    onclick: () => on.onInspect(x.card), onmouseenter: () => on.onInspect(x.card, true),
  }, h("span", { class: "row" }, h("span", { class: "orb" }, ctx.cards[x.card]?.cost ?? "–"), h("span", { class: "nm" }, cardName(ctx, x.card)),
    x.status === "flooped" && h("span", { class: "tag" }, "Flooped")))));
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
    return h("button", { type: "button", class: "modalopt cardopt", "aria-label": c?.name ?? "card", onclick: () => on.onAction(action) },
      cardEl(c ?? { id: inst.card, name: inst.card }), h("span", { class: "cap" }, c?.name ?? inst.card));
  };
  let title, text, body;
  if (ch.kind === "discard") {
    title = `Discard ${ch.count} card${ch.count === 1 ? "" : "s"}`;
    text = `${by} makes you discard. Choose from your hand:`;
    body = h("div", { class: "modalcards" }, (v.players[me].hand ?? []).map((inst) => {
      const action = choose.find((a) => a.card === inst.uid);
      return action && cardButton(inst, action);
    }));
  } else if (ch.kind === "reveal") {
    title = by;
    text = "These are the top cards of your deck. Choose one to put on the bottom; the rest are discarded.";
    body = h("div", { class: "modalcards" }, (ch.revealed ?? []).map((inst) => {
      const action = choose.find((a) => a.card === inst.uid);
      return action && cardButton(inst, action);
    }));
  } else if (ch.kind === "search") {
    title = by;
    text = "Choose a Building from your deck to put into play. Your deck is shuffled afterwards.";
    body = h("div", { class: "modalcards" }, (ch.revealed ?? []).map((inst) => {
      const action = choose.find((a) => a.card === inst.uid);
      return action && cardButton(inst, action);
    }));
  } else if (ch.kind === "rebound") {
    const keep = choose.find((a) => !a.card);
    title = `${by}: return it?`;
    text = `Discard a card from your hand to return ${by} to your hand, or leave it in the discard pile.`;
    body = h("div", {},
      h("div", { class: "modalcards" }, (v.players[me].hand ?? []).map((inst) => {
        const action = choose.find((a) => a.card === inst.uid);
        return action && cardButton(inst, action);
      })),
      keep && h("div", { class: "modalrow" }, h("button", { type: "button", class: "btn-steel modalopt", onclick: () => on.onAction(keep) }, "Leave it")));
  } else if (ch.kind === "redirect") {
    const stay = choose.find((a) => !a.card);
    title = `${by}: who fights?`;
    text = `${by} attacks. You may have one of your Flooped Creatures fight in its place.`;
    body = h("div", {},
      h("div", { class: "modalcards" }, (ch.revealed ?? []).map((inst) => {
        const action = choose.find((a) => a.card === inst.uid);
        return action && cardButton(inst, action);
      })),
      stay && h("div", { class: "modalrow" }, h("button", { type: "button", class: "btn-red modalopt", onclick: () => on.onAction(stay) }, `${by} fights`)));
  } else if (ch.kind === "move_damage") {
    title = by;
    text = `Move how much damage? (up to ${ch.count})`;
    body = h("div", { class: "modalrow" }, choose.map((a) =>
      h("button", { type: "button", class: "btn-purple modalopt", onclick: () => on.onAction(a) }, String(a.card))));
  } else if (ch.kind === "move") {
    const theirs = movesToTheirSide(ch, v.you);
    title = `${by}: where to?`;
    text = theirs ? `${by} has nowhere to go on your side: choose an empty Landscape on your opponent's side.` : `Choose an empty Landscape of yours to move ${by} to.`;
    body = h("div", { class: "modalrow modalrow2" }, choose.map((a) =>
      h("button", { type: "button", class: "btn-purple modalopt", onclick: () => on.onAction(a) }, `Lane ${(a.lane ?? 0) + 1}`)));
  } else if (ch.kind === "upkeep") {
    const destroy = choose.find((a) => !a.card);
    title = `${by}: upkeep`;
    text = `Discard ${ch.count} cards from your hand, or destroy ${by}.`;
    body = h("div", {},
      h("div", { class: "modalcards" }, (v.players[me].hand ?? []).map((inst) => {
        const action = choose.find((a) => a.card === inst.uid);
        return action && cardButton(inst, action);
      })),
      destroy && h("div", { class: "modalrow" }, h("button", { type: "button", class: "btn-red modalopt", onclick: () => on.onAction(destroy) }, `Destroy ${by}`)));
  } else {
    return null;
  }
  return h("div", { class: `modal-backdrop${ui.modalAgain ? " again" : ""}` },
    h("div", { class: "modal parch", role: "dialog", "aria-modal": "true", "aria-labelledby": "modal-h" },
      h("h2", { id: "modal-h" }, title),
      h("p", {}, text),
      body));
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
    const xf = sel ? "translateY(-30px) rotate(0deg)" : `translateY(${Math.abs(off) * 3}px) rotate(${off * 3}deg)`;
    const cls = ["hbtn", sel ? "sel" : "", pick ? "pick" : "", p.rainbow_hand?.includes(inst.uid) ? "rainbow" : "", playable && !ui.picker ? "go" : "", playable || pick || sel ? "" : "off"].join(" ");
    return h("button", {
      type: "button", class: cls, style: `transform:${xf}`, "data-uid": inst.uid, "aria-pressed": sel ? "true" : "false",
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
      h("div", { class: "actcount" },
        h("div", { class: "orb", role: "img", "aria-label": `${v.actions_left} of 2 Actions left` }, myMain || v.active === me ? v.actions_left : "–"),
        h("span", { class: "lbl" }, "Actions")),
      h("div", { class: "btns" },
        pending && h("button", { type: "button", class: "btn-purple", style: "min-height:40px;font-size:16px", onclick: on.onPending }, "Play the stolen card"),
        h("button", { type: "button", class: "btn-steel", disabled: !draw || !!ui.picker, onclick: () => draw && on.onAction(draw) }, "Draw · 1 Action"),
        end && !ui.picker && (canFight
          ? h("button", { type: "button", class: "btn-red", onclick: () => on.onAction(end) }, "Fight!")
          : h("button", { type: "button", class: "btn-green", onclick: () => on.onAction(end) }, "End turn")))));
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

/** Under "Card up close": the bug, card and feature forms, when the server offers them. They open in a new tab, so the game is left as it is. */
function feedbackPanel(links) {
  if (!links?.length) return null;
  return h("section", { class: "steel feedbackpanel", "aria-labelledby": "fb-h" },
    h("h2", { class: "h", id: "fb-h" }, "Something wrong?"),
    h("div", { class: "fbrow" }, links.map((l) => h("a", { class: "btn-steel", href: l.url, target: "_blank", rel: "noopener" }, l.label))));
}

function logPanel(state, ctx, on) {
  const lines = state.log.slice(-LOG_SHOWN).reverse();
  const names = nameMatcher(ctx.cards);
  return h("section", { class: "parch log", "aria-labelledby": "log-h" },
    h("div", { class: "loghead" },
      h("h2", { id: "log-h" }, "What just happened"),
      h("div", { class: "logbtns" },
        h("button", { type: "button", class: "btn-steel", onclick: (e) => on.onCopyLog(e.currentTarget), title: "Copy the whole game log as text" }, "Copy"),
        h("button", { type: "button", class: "btn-steel", onclick: () => on.onDownloadLog(), title: "Save the whole game log as a text file" }, "Save"))),
    lines.length === 0 ? h("p", { class: "muted", style: "margin:0" }, "Nothing yet.") :
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

/** Attack and Defense left, big and beside the card (not rotating with it). */
function tileStats(card, c) {
  const base = card?.attack ?? 0;
  const atk = c.attack ?? base;
  const left = (c.defense ?? card?.defense ?? 0) - (c.damage ?? 0);
  return h("div", { class: "tstats" },
    h("span", { class: `tstat atk ${atk > base ? "up" : ""}`, title: "Attack" }, h("small", {}, "ATK"), atk),
    h("span", { class: `tstat def ${c.damage > 0 ? "hurt" : ""}`, title: "Defense left" }, h("small", {}, "DEF"), left));
}
