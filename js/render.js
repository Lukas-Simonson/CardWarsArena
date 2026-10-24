// Draws the game screen from the current state. It rebuilds the DOM on every
// update (the UI is small), and reports clicks through `handlers`.
import {
  describeAction, instName, landscapeName, seatName, cardName,
  handSelection, laneSelection, sameSelection, filterActions,
} from "./describe.js";
import { isMyDecision } from "./state.js";

export function h(tag, attrs = {}, ...kids) {
  const el = document.createElement(tag);
  for (const [k, v] of Object.entries(attrs)) {
    if (v == null || v === false) continue;
    if (k === "class") el.className = v;
    else if (k.startsWith("on")) el.addEventListener(k.slice(2), v);
    else el.setAttribute(k, v === true ? "" : v);
  }
  for (const kid of kids.flat()) if (kid != null && kid !== false) el.append(kid);
  return el;
}

const GENERAL = new Set(["keep_hand", "mulligan", "draw", "end_main"]);

const typeLine = (c) => [c.type && c.type[0].toUpperCase() + c.type.slice(1), c.landscape && landscapeName(c.landscape)].filter(Boolean).join(" · ");

/**
 * @param {HTMLElement} root  the #game element
 * @param {{state:object, cards:object, ui:object}} model
 * @param {{onHand:Function,onLane:Function,onInspect:Function,onAction:Function,onDiscard:Function,onClearSelection:Function,onLeave:Function}} on
 */
export function renderGame(root, model, on) {
  const { state, cards, ui } = model;
  const v = state.view;
  if (!v) {
    root.replaceChildren(h("section", {}, state.waiting
      ? [h("h2", {}, `Waiting for an opponent — room ${state.session?.room ?? ""}`), h("p", {}, "Send this code to your friend; they choose “Join”. You can leave this page open.")]
      : h("p", { class: "muted" }, "Joining…")));
    return;
  }
  const ctx = { view: v, cards, names: state.names, known: state.known };
  const me = v.you, opp = 1 - me;
  const legal = v.legal ?? [];

  root.replaceChildren(
    statusBar(state, ctx),
    playerBar(ctx, opp, false, ui, on),
    board(ctx, legal, ui, on),
    playerBar(ctx, me, true, ui, on),
    h("div", { class: "lower" },
      h("div", { class: "right" }, handPanel(ctx, legal, ui, on)),
      h("div", { class: "right" }, actionsPanel(state, ctx, legal, ui, on), detailsPanel(ctx, ui, on), logPanel(state)),
    ),
  );
  const log = root.querySelector(".log");
  if (log) log.scrollTop = log.scrollHeight;
}

function statusBar(state, ctx) {
  const v = ctx.view;
  const turn = v.phase === "mulligan"
    ? "Opening hands"
    : `Turn ${v.turn} · ${seatName(ctx, v.active)}'s turn · ${v.phase[0].toUpperCase() + v.phase.slice(1)} phase${v.phase === "main" ? ` · Actions: ${v.actions_left}` : ""}`;
  return h("div", { class: "statusbar" },
    h("span", { class: "turn" }, turn),
    v.over && h("span", { class: "chip ok" }, `Game over — ${seatName(ctx, v.winner)} wins`),
    v.pending && h("span", { class: "chip warn" }, `Stolen card must be played: ${cardName(ctx, v.pending.card)}`),
    state.opponent.connected === false && h("span", { class: "chip bad" }, `${state.opponent.name || "Opponent"} disconnected`),
  );
}

function playerBar(ctx, seat, mine, ui, on) {
  const p = ctx.view.players[seat];
  const toggle = (label, n) => h("button", { class: "linkish", onclick: () => on.onDiscard(seat) }, `${label} ${n}`);
  return h("div", { class: `playerbar ${mine ? "me" : "opp"}` },
    h("strong", {}, `${seatName(ctx, seat)} ${mine ? "(you)" : ""}`),
    h("span", { class: "hp" }, `♥ ${p.hp}`),
    !mine && h("span", {}, `Hand ${p.hand_count}`),
    h("span", {}, `Deck ${p.deck_count}`),
    toggle("Discard", p.discard.length),
    ctx.view.awaiting.includes(seat) && !ctx.view.over && h("span", { class: "chip warn" }, "to act"),
  );
}

function board(ctx, legal, ui, on) {
  const v = ctx.view, me = v.you, opp = 1 - me;
  const kids = [h("div", { class: "rl" }), ...[0, 1, 2, 3].map((i) => h("div", { class: "lh" }, `Lane ${i + 1}`))];
  const row = (label, seat, kind) => {
    kids.push(h("div", { class: "rl" }, label));
    for (let i = 0; i < 4; i++) kids.push(cell(ctx, legal, ui, on, seat, i, kind));
  };
  row("Land", opp, "land");
  row("Bldg", opp, "bld");
  row("Crea", opp, "crea");
  kids.push(h("div", { class: "sep" }));
  row("Crea", me, "crea");
  row("Bldg", me, "bld");
  row("Land", me, "land");
  return h("div", { class: "board" }, kids);
}

function cell(ctx, legal, ui, on, seat, lane, kind) {
  const v = ctx.view, mine = seat === v.you;
  const l = v.players[seat].lanes[lane];
  const side = mine ? "mine" : "theirs";
  if (kind === "land") {
    return h("div", { class: `cell land ${l.face_down ? "facedown" : ""}`, title: l.face_down ? "Face-down Landscape: provides no type" : landscapeName(l.landscape) },
      l.face_down ? "▼ face down" : landscapeName(l.landscape));
  }
  if (kind === "bld") {
    const b = l.building;
    if (!b) return h("div", { class: "cell bld empty" }, "·");
    const sel = laneSelection(lane, "building");
    const actionable = mine && legal.some((a) => a.source === "building" && (a.lane ?? 0) === lane);
    return h("div", {
      class: `cell bld ${side} selectable ${actionable ? "actionable" : ""} ${mine && sameSelection(ui.selection, sel) ? "selected" : ""}`,
      title: ctx.cards[b.card]?.text ?? "",
      onclick: () => (mine ? on.onLane(lane, "building", b.card) : on.onInspect(b.card)),
    }, h("div", { class: "name" }, cardName(ctx, b.card)), b.status === "flooped" && h("div", { class: "chips" }, h("span", { class: "chip" }, "flooped")));
  }
  const c = l.creature;
  if (!c) return h("div", { class: "cell crea empty" }, "·");
  const sel = laneSelection(lane, "creature");
  const actionable = mine && legal.some((a) => (a.source ?? "creature") === "creature" && (a.kind === "attack" || a.kind === "floop" || a.kind === "activate") && (a.lane ?? 0) === lane);
  const chips = [
    h("span", { class: `chip ${c.status === "ready" ? "ok" : ""}` }, c.status === "fought" ? "tired" : c.status),
    c.damage > 0 && h("span", { class: "chip bad" }, `dmg ${c.damage}`),
    c.protected && h("span", { class: "chip" }, "protected"),
    c.stolen && h("span", { class: "chip warn" }, "stolen"),
    c.silenced && h("span", { class: "chip warn" }, c.lent ? `lent: ${cardName(ctx, c.lent)}` : "silenced"),
  ];
  return h("div", {
    class: `cell crea ${side} selectable ${actionable ? "actionable" : ""} ${mine && sameSelection(ui.selection, sel) ? "selected" : ""}`,
    title: ctx.cards[c.card]?.text ?? "",
    onclick: () => (mine ? on.onLane(lane, "creature", c.card) : on.onInspect(c.card)),
  }, h("div", { class: "name" }, cardName(ctx, c.card)), h("div", { class: "stats" }, `${c.attack}/${c.defense}`), h("div", { class: "chips" }, chips));
}

function handPanel(ctx, legal, ui, on) {
  const hand = ctx.view.players[ctx.view.you].hand ?? [];
  return h("div", { class: "panel" },
    h("h3", {}, `Your hand (${hand.length})`),
    hand.length === 0 ? h("div", { class: "muted" }, "empty") :
      h("div", { class: "hand" }, hand.map((inst) => {
        const c = ctx.cards[inst.card] ?? { name: inst.card };
        const playable = legal.some((a) => a.kind === "play_card" && a.card === inst.uid);
        const stats = c.attack != null || c.defense != null ? `${c.attack ?? 0}/${c.defense ?? 0}` : "";
        return h("div", {
          class: `hcard selectable ${playable ? "playable" : ""} ${sameSelection(ui.selection, handSelection(inst.uid)) ? "selected" : ""}`,
          onclick: () => on.onHand(inst.uid, inst.card),
          title: c.text ?? "",
        },
          h("div", { class: "top" }, h("span", {}, c.name), h("span", { class: "cost" }, c.cost ?? "–")),
          h("div", { class: "type" }, `${typeLine(c)}${stats ? " · " + stats : ""}`),
          h("div", { class: "text" }, c.text ?? ""));
      })));
}

function actionsPanel(state, ctx, legal, ui, on) {
  const v = ctx.view;
  const title = h("h3", {}, "Your actions");
  if (v.over) {
    return h("div", { class: "panel" }, title,
      h("p", {}, `Game over — ${seatName(ctx, v.winner)} wins.`),
      h("button", { class: "primary", onclick: on.onLeave }, "Back to the lobby"));
  }
  if (!isMyDecision(state)) {
    return h("div", { class: "panel" }, title, h("p", { class: "muted" }, `Waiting for ${v.awaiting.map((s) => seatName(ctx, s)).join(" and ")}…`));
  }
  const general = legal.filter((a) => GENERAL.has(a.kind));
  const specific = legal.filter((a) => !GENERAL.has(a.kind));
  const shown = filterActions(specific, ui.selection);
  const btn = (a) => h("button", { onclick: () => on.onAction(a) }, describeAction(ctx, a));

  let hint = null;
  if (ui.selection) {
    const name = ui.selection.type === "hand" ? instName(ctx, ui.selection.uid) : "that card";
    hint = h("div", { class: "muted", style: "margin-bottom:6px" },
      `${shown.length} of ${specific.length} actions for ${name} · `, h("button", { class: "linkish", onclick: on.onClearSelection }, "show all"));
  } else if (specific.length > 8) {
    hint = h("div", { class: "muted", style: "margin-bottom:6px" }, "Click a card in your hand or on your side of the board to narrow this list.");
  }
  return h("div", { class: "panel" }, title,
    general.length > 0 && h("div", { class: "general" }, general.map((a) => h("button", { class: a.kind === "end_main" ? "primary" : "", onclick: () => on.onAction(a) }, describeAction(ctx, a)))),
    hint,
    shown.length > 0 ? h("div", { class: "actions" }, shown.map(btn))
      : specific.length > 0 || ui.selection ? h("div", { class: "muted" }, ui.selection ? "No legal actions for that card right now." : "") : null);
}

function detailsPanel(ctx, ui, on) {
  const v = ctx.view;
  const pileSeat = ui.discard;
  const pile = pileSeat != null && v.players[pileSeat].discard;
  const c = ui.inspect && ctx.cards[ui.inspect];
  const stats = c && (c.attack != null || c.defense != null) ? ` · ${c.attack ?? 0}/${c.defense ?? 0}` : "";
  return h("div", { class: "panel" },
    pile && h("div", { style: "margin-bottom:8px" },
      h("h3", {}, `${seatName(ctx, pileSeat)}'s discard pile (${pile.length})`),
      pile.length
        ? h("div", {}, pile.map((x) => h("div", {}, h("button", { class: "linkish", onclick: () => on.onInspect(x.card) }, cardName(ctx, x.card)))))
        : h("div", { class: "muted" }, "empty")),
    h("h3", {}, "Card details"),
    !c ? h("div", { class: "muted" }, "Click any card to read it.") :
      h("div", { class: "details" },
        c.image_url && h("img", { src: c.image_url, alt: c.name, onerror: (e) => e.target.remove() }),
        h("div", {},
          h("strong", {}, c.name),
          h("div", { class: "muted" }, `Cost ${c.cost ?? "–"} · ${typeLine(c)}${stats}`),
          h("div", { class: "text" }, c.text ?? ""),
          c.modeled === false && h("div", { class: "chip warn", style: "margin-top:6px" }, "rules not modelled by the engine yet"))));
}

function logPanel(state) {
  return h("div", { class: "panel" }, h("h3", {}, "Game log"),
    h("div", { class: "log" }, state.log.map((line) => h("div", { class: line.startsWith("──") ? "turn" : "" }, line))));
}
