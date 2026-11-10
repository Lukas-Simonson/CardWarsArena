# Client protocol (version 1)

This is what a client (the web client, the debug page, a bot) speaks to
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
- **A `rate_limited` message is dropped and nothing else happens**, so no
  update follows: wait a moment and send it again. Likewise a `stale_state` error
  means the next update is on its way; choose again from that one.
- The server pings every 30 s; use any WebSocket library's default pong.

Plain HTTP endpoints:

| path | |
|---|---|
| `GET /api/bots` | `{"default":"heuristic","bots":["heuristic","random"]}`: the bots `create` may ask for |
| `GET /api/decks` | decks: `set`, `name`, `owner`, `size`, `cards` (ids, one per copy), `landscapes`, `hero` (card id), `playable` |
| `GET /api/cards` | every card: printed fields (`id`, `name`, `type`, `landscape`, `cost`, `attack`, `defense`, `text`, `image_url`) and `modeled` |
| `GET /api/auth/config` | `{"mode":"none"\|"oauth","providers":[{"name":"github","label":"GitHub"}]}`: what the client may offer |
| `GET /api/auth/me` | `{"user":{"id","name","avatar"}}`, or `{"user":null}` when not signed in |
| `GET /api/auth/login/{provider}` | start signing in: redirects to the provider (`-auth oauth` only). `?return=<page>` is where to come back to: this site, or a host allowed by `-origins` |
| `GET /api/auth/callback/{provider}` | where the provider sends people back; sets the session cookie and redirects to `/` (or to `/?login_error=denied\|state\|failed`) |
| `POST /api/auth/logout` | ends the session and clears the cookie |
| `GET /api/my/decks` | the signed-in player's saved decks (needs `-db`; 401 when not signed in) |
| `POST /api/my/decks` | save a deck: `{"name","hero","cards":[ids, one per copy]}` as `application/json`; 201 with the deck |
| `PUT /api/my/decks/{id}` | replace one of your decks; 404 if it is not yours |
| `DELETE /api/my/decks/{id}` | delete one of your decks; 204 |
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
{"type":"create","name":"Lukas","deck":"finn","vs_bot":true,"bot_deck":"jake"}
{"type":"join","room":"B69K","name":"Ann","deck":"jake"}
{"type":"rejoin","room":"B69K","token":"0f59…"}
{"type":"action","seq":12,"action":{"kind":"play_card","card":57,"lane":2}}
{"type":"leave"}
{"type":"ping"}
```

- `deck` is words matched against `"SET Deck name"` (`"finn"`, or `"Finn Vs Jake Finn"`); words that name one deck on its own win over the set's name. The match must
  be unique and playable. `bot_deck` defaults to the next playable deck. `bot` (with `vs_bot`) names the bot
  from `/api/bots`; the default is `heuristic`, and an unknown name is a `bad_request`.
- A signed-in player may ask for one of their saved decks as `"my:<id>"` (from `GET /api/my/decks`) in `deck`;
  it must be playable (`unknown_deck` if it is not theirs or the player is a guest, `deck_not_playable` with the
  reasons if it is unfinished). `bot_deck` still names a starter deck.
- **Signed-in players** (`-auth oauth`; the session cookie is sent with the WebSocket upgrade) play under
  their account name: `name` is ignored. Guests choose their own. Who a connection is is fixed when it opens,
  so reconnect after signing in or out.
- `seed` (create only) is honoured only when the server runs with `-dev`.
- **Always send `seq`** with an action: the `seq` of the update you chose it from.
  If anything has happened since (the bot moved, the opponent acted) the server
  replies `stale_state` and changes nothing; wait for the next update and choose
  again. Without `seq` the action is applied to whatever the position is now.

## Server → client

| `type` | fields |
|---|---|
| `hello` | `protocol` |
| `joined` | `room`, `seat` (0/1), `token`, `vs_bot`, `names`, `avatars` |
| `waiting` | `room`: share this code |
| `update` | `seq`, `view`, `events`, `names`, `avatars` |
| `opponent` | `connected`, `name` |
| `room_closed` | `message` (reason) |
| `error` | `code`, `message` |
| `pong` | |

`avatars` is in the same order as `names` (seat 0, seat 1): the address of each player's picture at their sign-in
provider (Discord or GitHub), or `""` for a guest or the bot. A client shows the picture, and falls back to the
name's first letter when it is empty or does not load. The addresses are on the providers' own image hosts, so the
viewer's browser loads them from there.

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
  "preload": ["cool-dog", "the-pig"],   // card ids whose art to fetch early (see "Hidden information")
  "awaiting": [0],                       // seats that must decide now (both, during mulligan)
  "pending": {"uid":9,"card":"cool-dog","owner":1},  // Silo of Truth: must be played first
  "choice": {"player":1,"kind":"discard","card":"abraca-amadeus","count":2},  // see "Choices"
  "legal": [ …actions… ],                // only when you are in `awaiting`
  "players": [ { …seat 0… }, { …seat 1… } ]
}
```

A player:

```jsonc
{ "hp": 25, "deck_count": 35, "hand_count": 5,
  "hero": {"uid":2,"card":"finn","owner":0},         // the deck's Hero, public; its abilities are in `legal`
  "hand": [{"uid":3,"card":"the-pig","owner":0}],   // ONLY your own seat
  "discard": [ … ],                                  // public, both seats
  "kept": true, "mulliganed": false,
  "lanes": [ { "landscape":"blue_plains", "face_down":false,
               "creature": { "uid":7,"card":"cool-dog","owner":0,
                             "damage":2,"status":"ready",   // ready | flooped | fought
                             "attack":2,"defense":7,         // current values, modifiers included
                             "protected":false,"stolen":false,"silenced":false,"lent":"","changed_lanes":false,
                             "types":["rainbow"] },          // only when its Landscape types are not its card's own
               "building": { "uid":8,"card":"blood-fortress","owner":0,"status":"ready" },
               "extra": [ { "uid":9,"card":"farmhouse","owner":0,"status":"ready" } ] } ] }  // 4 lanes; extra only with Burly Lumberjack
```

`rainbow_hand` (your seat only) lists the uids in `hand` that are Rainbow cards until the end of the turn (Woadic Weirdo).

Lane index 0–3 is the same Lane on both sides: `players[0].lanes[2]` faces
`players[1].lanes[2]`. **Hidden information is absent, not hidden**: the
opponent has `hand_count` but no `hand`, and neither deck's contents are sent.
Opponent draws arrive as events with `card: 0`.

### Hidden information: `preload`

The one place hidden cards are named is `preload`, a list of card ids that lets the client
fetch their art before it is needed: the opponent's hand and your next 3 draws, together, with
repeats removed and sorted, so nothing says which card is which or in what order. Whoever reads
the traffic still learns that those cards are somewhere in the opponent's hand or on top of
their own deck. This is a deliberate trade for faster-loading art, not an oversight. Without
it nothing would be known until a card is played or drawn. Drop the field to remove the leak.

## Actions

An action is `{ "kind": …, "card"?, "lane"?, "source"?, "index"?, "cost_targets"?, "targets"? }`.
**Send back exactly an object from `view.legal`.** Do not construct actions.

| `kind` | when | fields |
|---|---|---|
| `keep_hand`, `mulligan` | mulligan phase | |
| `play_card` | main | `card` (hand uid), `lane` (not for Spells), `cost_targets`, `targets` |
| `draw` | main | spend 1 Action to draw |
| `floop` | main | `lane`, `source` (`creature`/`building`), `targets` |
| `activate` | main | `lane`, `source`, `index`, `targets`; `source: "hero"` for a Hero's ability (no `lane`) |
| `end_main` | main | go to the Fight phase (ends the turn on the first turn or with no one able to Fight) |
| `attack` | fight | `lane` |
| `play_pending` | after Silo of Truth | `lane`, `targets` |
| `choose` | while a `choice` is open | see "Choices" |

`targets` is a list of target slots (an empty optional slot is `[]`, never `null`), each a list of references
`{"kind":"creature|building|landscape|lane|discard_card|hand_card","player":0,"lane":1,"card":0}`
(`player` = whose side, `lane` for board things, `card` = uid for card piles).

### Choices

Some cards stop the game until a player decides, and that player need not be
the active one. `view.choice` (sent to both seats) says who (`player`), what kind,
and which card asked (`card`). The decider is the only seat in `awaiting`, and
every other action is refused (`illegal_action`) until they answer.

| `kind` | question | answer |
|---|---|---|
| `discard` | discard `count` cards from your hand (Abraca Amadeus, Bone Wand, Mace Stump) | `choose` with `card` = the hand uid; repeat while `choice.count` remains |
| `upkeep` | discard `count` cards from your hand, or destroy the card that asks (Steakchop) | `choose` with `card` = a hand uid, or `choose` with no `card` to destroy it. After the first discard it becomes a `discard` choice |
| `reveal` | the top cards of your deck, `revealed`, are shown to both seats: pick one to put on the bottom, the rest are discarded (Ogre Gas) | `choose` with `card` = the revealed uid |
| `search` | the Buildings in your deck, `revealed` (only the decider is sent them): pick one to put into play, then the deck is shuffled (Deforestation) | `choose` with `card` = the revealed uid |
| `rebound` | discard a card from your hand to return the card that asks from your discard pile to your hand (Impossible Possum) | `choose` with `card` = a hand uid, or `choose` with no `card` to leave it |
| `redirect` | the Creature that attacked may be replaced by one of your Flooped Creatures, `revealed` (Pied Piper); the Fight waits for the answer | `choose` with `card` = the Flooped Creature's uid, or `choose` with no `card` to fight with the attacker |
| `move_damage` | move damage between two of your Creatures; `count` is the most you may move (Friendship Bracelet) | `choose` with `card` = how much, 1 to `count` |

As always, send one of the objects in `view.legal`.

**Limit:** `legal` lists at most 128 target choices per card, so a card with a very
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
`attack`, `damage`, `hp_loss`, `heal` (a Creature healed `amount`), `hp_gain` (a player healed `amount`), `die`, `game_over`, `move`, `flip`, `ready`,
`bounce`, `destroy`, `return`, `deck_top`, `actions`, `floop`, `activate`,
`discard`, `steal`, `abilities`, `reveal` (a card in hand was shown). Events are for
animation and the game log only: the view is always authoritative.

## Running

```
go run ./cmd/cardwarsd -addr :8080          # production-like
go run ./cmd/cardwarsd -dev                 # also: client-chosen seeds, unmodelled decks
```

Flags: `-data`, `-bot-delay` (pause between bot moves, default 700 ms),
`-origins`, `-db` (a SQLite file for accounts and decks; the schema is created and brought up to date
at start, and nothing is kept without it), `-auth none|oauth` and `-public-url` (see "Signing in").

### Saved decks

A saved deck is `{id, name, hero, cards, landscapes, landscapes_chosen, created, updated, playable, problems}`.
`cards` has one id per copy, the way the game takes a deck. `landscapes` are the four Landscapes it is played
with, one for each Lane in order (`[]` when the cards do not say), and `landscapes_chosen` says whether the
player picked them. To choose, send `landscapes` with exactly four of `cornfield`, `blue_plains`,
`useless_swamp`, `nice_lands`, `sandy_lands`, `icy_lands`; leave it out (or send `[]`) to let the cards decide,
as the starter decks do. A card whose Landscape is not among the four can be hard or impossible to play (it
needs as many face-up Landscapes of its type as it costs), but that does not stop the deck being played; the
client warns about it. Saving accepts work in progress: only a deck that makes no sense is refused
(`422`: an unknown card, a Hero among the cards, a Hero that is not one, more than 3 copies of a card, more than
120 cards, `landscapes` that are not four real ones), so a half-built deck can be kept. `playable` says whether it can be played, and `problems` says in
words what stops it (no Hero, fewer than 40 cards, Landscapes the cards cannot decide and the player has not chosen, rules not implemented yet). A name
is trimmed to 40 characters; a player may keep 50 decks (`409` past that). A request that changes something
must carry `application/json` and, if it has an `Origin`, be from this site or an allowed web client.

### Signing in

`-auth oauth` needs `-db`, `-public-url` (where browsers reach the server, e.g. `https://cards.example.com`)
and credentials for at least one provider in the environment: `CARDWARS_DISCORD_CLIENT_ID` and
`CARDWARS_DISCORD_CLIENT_SECRET`, `CARDWARS_GITHUB_CLIENT_ID` and `CARDWARS_GITHUB_CLIENT_SECRET`. Register
the app with the provider and give it the callback `<public-url>/api/auth/callback/discord` (or `/github`).
Only the provider's id for the person and a display name are kept: no email, no password. Sessions last 30
days; the cookie is `HttpOnly` and `SameSite=Lax`, and over https `Secure` with a `__Host-` name. Idle rooms are removed after 30 minutes; a room survives a dropped
connection until then.

Not yet built: accounts, matchmaking, spectators, turn timers, resign (use
`leave`), persistence of games across a server restart.
