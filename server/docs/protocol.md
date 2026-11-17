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
| `GET /api/feedback` | `{"enabled":true,"links":[{"id":"bug","label":"Report a bug","url":"https://…"}]}`: the links the home screen shows. `enabled` is false, and `links` empty, unless the server runs with `-feedback` |
| `GET /api/decks` | decks: `set`, `name`, `owner`, `size`, `cards` (ids, one per copy), `landscapes`, `hero` (card id), `playable`. `?lang=<tag>` adds `title`, `owner_title` and `set_title` in that language where the card data has them; `name` and `set` stay English and are the deck's identity |
| `GET /api/cards` | every card: printed fields (`id`, `name`, `type`, `landscape`, `cost`, `attack`, `defense`, `text`, `image_url`) and `modeled`. `?lang=<tag>` gives `name`, `text`, `notes`, `clarification` and `image_url` in that language where the card data has them (`fr-CA` falls back to `fr`, then English), with `name_en` when the name differs. The answer has `Content-Language`. See [localization.md](localization.md) |
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
  reasons if it is unfinished). `bot_deck` takes the same: a starter deck, or `"my:<id>"` for one of the player's own (the same errors apply).
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
| `room_closed` | `reason` (`host_left`, `opponent_left`, `room_expired`), `message` (the same in English) |
| `error` | `code`, `reason`, `params`, `message` |
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

The server sends no prose a player has to read: it sends what went wrong as codes and values, and the client words it
in the player's language. An `error` has

- `code`: the kind: `bad_request`, `unknown_deck`, `deck_not_playable`, `room_not_found`, `room_full`, `not_in_room`,
  `already_in_room`, `bad_token`, `illegal_action`, `game_not_started`, `server_full`, `rate_limited`, `stale_state`.
- `reason` (when there is one): exactly what. For `illegal_action` it is the rule that was broken: `game_over`,
  `wrong_phase`, `not_your_turn`, `already_decided`, `no_mulligan`, `unknown_action`, `no_such_card`, `bad_lane`,
  `no_actions`, `landscapes`, `flooped`, `deck_empty`, `no_attacker`, `first_turn`, `not_playable`, `cannot_attack`,
  `no_target`, `not_ready`, `no_ability`, `pending`, `no_pending`, `bad_target`, `condition`, `choice`, `no_choice`,
  `limit`, `no_floop`, `frozen`, `not_frozen`, `no_replace`, `inert`. For `bad_request` it is `not_text`,
  `invalid_json` (`detail`), `action_required`, `unknown_message_type` (`type`), `unknown_bot` (`name`, `bots`). For
  `unknown_deck` it is `no_deck_matches` (`query`), `deck_ambiguous` (`query`, `decks`), `deck_not_found`,
  `sign_in_required` or `no_bot_deck`; for `deck_not_playable`, `problems` lists the deck's problems (see below).
- `params`: the values a message about it needs. A card is named by its id (`card`), never by its name, so a client
  can use the name in its own language. `stale_state` has `seq` (where the game is now).
- `message`: the same in English, for logs and for a client that does not know the code. It may change; do not match on it.

The deck API (`/api/my/decks`) answers a failure with `{"error": "<English>", "code": "...", "params": {...}}`: `sign_in_required`
(401), `wrong_origin` (403), `json_required` (415), `not_a_deck` (400, `detail`), `unknown_card`, `not_a_hero`,
`hero_as_card` (`card`), `too_many_copies` (`card`, `max`), `deck_too_large` (`max`), `landscape_count` (`lanes`),
`not_a_landscape` (`landscape`) (all 422), `deck_limit` (409, `max`), `deck_not_found` (404) and `store_unreachable` (500).

A *problem* is `{"code", "params", "text"}`: the same three things in one object. The deck problems below are a list of
them: `choose_hero`, `deck_too_small` (`n`, `min`), `choose_landscapes_rainbow`, `too_many_landscape_types` (`types`,
`lanes`) and `unmodelled` (`card`).

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
  "lanes": [ { "landscape":"blue_plains", "face_down":false, "frozen":false,
               "creature": { "uid":7,"card":"cool-dog","owner":0,
                             "damage":2,"status":"ready",   // ready | flooped | fought
                             "attack":2,"defense":7,         // current values, modifiers included
                             "protected":false,"stolen":false,"silenced":false,"lent":"","changed_lanes":false,
                             "types":["rainbow"],            // only when its Landscape types are not its card's own
                             "under":[ {"uid":12,"card":"fancy-zebracorn","owner":0} ] },  // only with cards under it (Carmel Camel)
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
| `play_card` | main | `card` (hand uid), `lane` (not for Spells), `cost_targets`, `targets`. A legal one also carries `labels`, `skip_labels` and `groups`, one per target slot, for display (what each choice is for, what leaving an optional slot empty does, and which slots share one target, as heal-or-damage does); they are ignored when played |
| `draw` | main | spend 1 Action to draw |
| `floop` | main | `lane`, `source` (`creature`/`building`), `index` (which of the card's Floop abilities, default 0: its own come first, then one its Hero grants it, as with Finn), `targets`. A legal `floop` also carries `via: "hero"` when the ability is granted (display only) |
| `activate` | main | `lane`, `source`, `index`, `targets`; `source: "hero"` for a Hero's ability (no `lane`) |
| `end_main` | main | go to the Fight phase (ends the turn on the first turn or with no one able to Fight) |
| `attack` | fight | `lane` |
| `play_pending` | after Silo of Truth | `lane`, `targets` |
| `thaw` | main | `lane` (one of your frozen Landscapes), `card` (the hand uid to discard): removes its frozen token for no Actions |
| `choose` | while a `choice` is open | see "Choices" |

`targets` is a list of target slots (an empty optional slot is `[]`, never `null`), each a list of references
`{"kind":"creature|building|landscape|lane|discard_card|hand_card|player","player":0,"lane":1,"card":0}`
(`player` = whose side, `lane` for board things, `card` = uid for card piles).

### Choices

Some cards stop the game until a player decides, and that player need not be
the active one. `view.choice` (sent to both seats) says who (`player`), what kind,
and which card asked (`card`). The decider is the only seat in `awaiting`, and
every other action is refused (`illegal_action`) until they answer.

| `kind` | question | answer |
|---|---|---|
| `discard` | discard `count` cards from your hand (Abraca Amadeus, Bone Wand, Mace Stump); with `only_cost`, only a card of that cost (Fly Swatter) | `choose` with `card` = the hand uid; repeat while `choice.count` remains |
| `upkeep` | discard `count` cards from your hand, or destroy the card that asks (Steakchop) | `choose` with `card` = a hand uid, or `choose` with no `card` to destroy it. After the first discard it becomes a `discard` choice |
| `reveal` | the top cards of your deck, `revealed`, are shown to both seats: pick one to put on the bottom, the rest are discarded (Ogre Gas) | `choose` with `card` = the revealed uid |
| `search` | the Buildings in your deck, `revealed` (only the decider is sent them): pick one to put into play, then the deck is shuffled (Deforestation) | `choose` with `card` = the revealed uid |
| `rebound` | discard a card from your hand to return the card that asks from your discard pile to your hand (Impossible Possum) | `choose` with `card` = a hand uid, or `choose` with no `card` to leave it |
| `redirect` | the Creature that attacked may be replaced by one of your Flooped Creatures, `revealed` (Pied Piper); the Fight waits for the answer | `choose` with `card` = the Flooped Creature's uid, or `choose` with no `card` to fight with the attacker |
| `move_damage` | move damage between two of your Creatures; `count` is the most you may move (Friendship Bracelet) | `choose` with `card` = how much, 1 to `count` |
| `move` | the Creature in your Lane `lane` (omitted for Lane 0) must move at the start of your turn to an empty Landscape on seat `move_side`'s side (omitted for seat 0, like any zero), one of `lanes` (Drained Cleric, Tired Wombat, Woadic Ring Leader) | `choose` with `lane` = one of `lanes` |
| `camel` | the Creature in `revealed` is leaving play; you may put it underneath your Camel instead of letting it go to your hand (`count` 1) or discard pile (`count` 0) (Carmel Camel). `lane` is the Lane it left, on seat `move_side`'s side | `choose` with `card` = the Camel's uid, or `choose` with no `card` to let it go |
| `landscape` | pick the type your Landscape in `lane` becomes until the start of your next turn (Silo of Freedom) | `choose` with `card` = 1 + the type's index in `cornfield, blue_plains, useless_swamp, nice_lands, sandy_lands, icy_lands` |
| `target` | a trigger of the card chose a target for you (Crystal Palace, Frozen Fish): `targets` lists the legal ones, `label` says what for, and `optional` that you may choose none. `lane` is the card's Lane, on seat `move_side`'s side | `choose` with `targets` = `[[ref]]` for one of the listed refs, or `choose` with no `targets` when `optional` |
| `ransom` | you may discard a card (of cost `min_cost` or more) or take what the card that asks says: it happens to you if you do not (Blood Bath, Bums Away!) | `choose` with `card` = a card in your hand, or `choose` with no `card` to take it |
| `amount` | pick how much: `options` are the numbers (The Big Hen House) | `choose` with `card` = one of the numbers |

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
`discard`, `steal`, `abilities`, `reveal` (a card in hand was shown), `remove` (the card `card`, owned by `player`, was removed from the game), `freeze` and `thaw` (a Landscape, `player` and `lane`, got or lost its frozen token), `under` (the Creature `card`, owned by `player`, went underneath the Camel in `lane`) and `retype` (a Landscape, `player` and `lane`, changed type: `amount` is 1 + its index in the list under the `landscape` choice). Events are for
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
120 cards, `landscapes` that are not four real ones), so a half-built deck can be kept. `playable` says whether it can be played, and `problems` is a list of problems (code, params, English text) saying
what stops it (no Hero, fewer than 40 cards, Landscapes the cards cannot decide and the player has not chosen, rules not implemented yet). A name
is trimmed to 40 characters (a blank one stays blank: the client says "Untitled deck" in the player's language); a player may keep 50 decks (`409` past that). A request that changes something
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
