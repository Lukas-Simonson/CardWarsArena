# Client protocol (version 1)

This is what a client (the Godot game, the debug page, a bot) speaks to
`cardwarsd`. The server owns every rule; a client only renders the view it is
sent and asks for actions the view says are legal.

## Transport

- WebSocket at `/ws`; every message is one JSON **text** frame (max 16 KiB).
- Browsers must connect from the server's own origin, or from one listed in
  `-origins` (e.g. `localhost:*`). The same list controls which origins may read
  the JSON API (CORS). Non-browser clients send no `Origin` and are always allowed.
- Each connection is rate limited (default burst 40, 20 messages/s). A
  `rate_limited` error means the message was dropped: resend it. A client that
  keeps flooding (25 refused messages in a row) is disconnected.
- The server pings every 30 s; use any WebSocket library's default pong.

Plain HTTP endpoints:

| path | |
|---|---|
| `GET /api/decks` | decks: `set`, `name`, `owner`, `size`, `landscapes`, `playable` |
| `GET /api/cards` | every card: printed fields (`id`, `name`, `type`, `landscape`, `cost`, `attack`, `defense`, `text`, `image_url`) and `modeled` |
| `GET /healthz` | `ok` |
| `GET /` | the debug client |

Card data never changes during a game, so fetch `/api/cards` once and look cards
up by `id`. Views refer to cards only by `id` and instance `uid`.

## Lifecycle

```
connect                      ← hello
create {deck, vs_bot}        ← joined, then (vs bot) update…   or (PvP) waiting
join {room, deck}            ← joined, update…                 (host also gets opponent)
action {action, seq}         ← update to every seat            or error to you
leave                        → the room closes for both        ← room_closed to the other seat
(connection drops)           → seat stays reserved; opponent gets opponent{connected:false}
rejoin {room, token}         ← joined, update (current state)
```

`create` seats you in seat 0; the player who joins is seat 1. The server picks
who goes first. Keep the `token` from `joined` (the debug page keeps it in
`sessionStorage`): it is the only way to get your seat back.

## Client → server

```jsonc
{"type":"create","name":"Lukas","deck":"cp1 finn","vs_bot":true,"bot_deck":"cp1 jake"}
{"type":"join","room":"B69K","name":"Ann","deck":"cp1 jake"}
{"type":"rejoin","room":"B69K","token":"0f59…"}
{"type":"action","seq":12,"action":{"kind":"play_card","card":57,"lane":2}}
{"type":"leave"}
{"type":"ping"}
```

- `deck` is words matched against `"SET Deck name"` (`"cp1 finn"`); the match must
  be unique and playable. `bot_deck` defaults to the next playable deck.
- `seed` (create only) is honoured only when the server runs with `-dev`.
- **Always send `seq`** with an action: the `seq` of the update you chose it from.
  If anything has happened since (the bot moved, the opponent acted) the server
  replies `stale_state` and changes nothing; wait for the next update and choose
  again. Without `seq` the action is applied to whatever the position is now.

## Server → client

| `type` | fields |
|---|---|
| `hello` | `protocol` |
| `joined` | `room`, `seat` (0/1), `token`, `vs_bot`, `names` |
| `waiting` | `room`: share this code |
| `update` | `seq`, `view`, `events`, `names` |
| `opponent` | `connected`, `name` |
| `room_closed` | `message` (reason) |
| `error` | `code`, `message` |
| `pong` | |

`update.seq` increases by exactly 1 for every change to the game. `events` is
what just happened, as *this seat* may see it (animate these); `view` is the full
resulting position (render this). After a `rejoin` you get one `update` with the
current `seq` and no events.

### Error codes

`bad_request`, `unknown_deck`, `deck_not_playable`, `room_not_found`,
`room_full`, `not_in_room`, `already_in_room`, `bad_token`, `illegal_action`
(the rules refuse it; `message` says why), `game_not_started`, `server_full`,
`rate_limited`, `stale_state`.

## The view

```jsonc
{
  "you": 0, "phase": "main", "turn": 4, "active": 0, "first": 1,
  "actions_left": 2, "spells_this_turn": 0,
  "over": false, "winner": 1,            // winner only when over
  "awaiting": [0],                       // seats that must decide now (both, during mulligan)
  "pending": {"uid":9,"card":"cool-dog","owner":1},  // Silo of Truth: must be played first
  "legal": [ …actions… ],                // only when you are in `awaiting`
  "players": [ { …seat 0… }, { …seat 1… } ]
}
```

A player:

```jsonc
{ "hp": 25, "deck_count": 35, "hand_count": 5,
  "hand": [{"uid":3,"card":"the-pig","owner":0}],   // ONLY your own seat
  "discard": [ … ],                                  // public, both seats
  "kept": true, "mulliganed": false,
  "lanes": [ { "landscape":"blue_plains", "face_down":false,
               "creature": { "uid":7,"card":"cool-dog","owner":0,
                             "damage":2,"status":"ready",   // ready | flooped | fought
                             "attack":2,"defense":7,         // current values, modifiers included
                             "protected":false,"stolen":false,"silenced":false,"lent":"","changed_lanes":false },
               "building": { "uid":8,"card":"blood-fortress","owner":0,"status":"ready" } } ] }  // 4 lanes
```

Lane index 0–3 is the same Lane on both sides: `players[0].lanes[2]` faces
`players[1].lanes[2]`. **Hidden information is absent, not hidden**: the
opponent has `hand_count` but no `hand`, and neither deck's contents are sent.
Opponent draws arrive as events with `card: 0`.

## Actions

An action is `{ "kind": …, "card"?, "lane"?, "source"?, "index"?, "cost_targets"?, "targets"? }`.
**Send back exactly an object from `view.legal`.** Do not construct actions.

| `kind` | when | fields |
|---|---|---|
| `keep_hand`, `mulligan` | mulligan phase | |
| `play_card` | main | `card` (hand uid), `lane` (not for Spells), `cost_targets`, `targets` |
| `draw` | main | spend 1 Action to draw |
| `floop` | main | `lane`, `source` (`creature`/`building`), `targets` |
| `activate` | main | `lane`, `source`, `index`, `targets` |
| `end_main` | main | go to the Fight phase (ends the turn on the first turn or with no one able to Fight) |
| `attack` | fight | `lane` |
| `play_pending` | after Silo of Truth | `lane`, `targets` |

`targets` is a list of target slots (an empty optional slot is `[]`, never `null`), each a list of references
`{"kind":"creature|building|landscape|lane|discard_card|hand_card","player":0,"lane":1,"card":0}`
(`player` = whose side, `lane` for board things, `card` = uid for card piles).

**Limit:** `legal` lists at most 64 target choices per card, so a card with a very
wide choice (Patchy the Pumpkin with many Creatures) may offer only some of its
legal plays. The server still *accepts* any legal choice, so a client may
construct such a play from the same shape; a server-side "list all targets"
query is planned.

## Events

`events[]` entries are `{ "kind", "player", "card"?, "card_id"?, "lane"?, "amount"? }`.
`card` is the instance uid and `card_id` names it, so you can describe a card you
never see in a view (a Creature that entered play and returned to a hand in the
same action). An opponent's `draw` has neither.
Kinds: `mulligan`, `keep_hand`, `turn_start`, `draw`, `play`, `replace`,
`attack`, `damage`, `hp_loss`, `die`, `game_over`, `move`, `flip`, `ready`,
`bounce`, `destroy`, `return`, `deck_top`, `actions`, `floop`, `activate`,
`discard`, `steal`, `abilities`. Events are for
animation and the game log only: the view is always authoritative.

## Running

```
go run ./cmd/cardwarsd -addr :8080          # production-like
go run ./cmd/cardwarsd -dev                 # also: client-chosen seeds, unmodelled decks
```

Flags: `-data`, `-bot-delay` (pause between bot moves, default 700 ms),
`-origins`. Idle rooms are removed after 30 minutes; a room survives a dropped
connection until then.

Not yet built: accounts, matchmaking, spectators, turn timers, resign (use
`leave`), persistence of games across a server restart.
