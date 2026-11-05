# Card Wars web client

A browser client for the Card Wars server (`../server`): no build step, no
dependencies. The look comes from the "Card Wars Web — Game Design" Claude
Design project: wood table, felt battlefield, riveted steel panels.

It plays a match against the bot, or against a friend with a room code. Card art comes from the
catalog's `image_url`; a card without a scan is drawn from its data in a frame
coloured for its Landscape.

## Run it

```
# 1. the game server (in ../server); -origins lets this page talk to it
go run ./cmd/cardwarsd -origins 'localhost:*'

# 2. this client
npm start            # or: node serve.mjs 5173
```

Open http://localhost:5173/. Pick a deck from the shelf and choose **Practice**, or **Make a new room** and send the code to a friend, who types it under **Join**. (To try it alone, use two browser tabs: each keeps its own seat.) The
server address (default `http://localhost:8080`, or `?server=http://host:port`)
is under "Server" at the foot of the home screen. Reloading the page puts you back in your seat.

## How to play

You act on the cards themselves; there is no list of actions.

* Cards that can act glow green. Click one to select it.
* **Play a card:** click it in your hand, then a lane. Lanes light up gold
  (replacing a Creature is allowed). Buildings go in the slot under a lane.
* **Spells and abilities:** click the card (or your Creature), then each
  target it asks for. **None** skips an optional choice. A spell with no
  choices shows a **Confirm** button so you never cast by accident.
* **Floop:** the purple button under a Creature. **Fight!** moves to the Fight
  phase, where you click each Creature to attack. **End turn** appears when
  nobody can Fight.
* `Esc` or **Cancel** drops the selection. Hover any card to read it big.
* Things that happen are animated: cards dropping into lanes, lunges and hits with
  damage numbers, destroyed cards fading out, draws, spells being cast, and a
  banner when the turn changes. `prefers-reduced-motion` turns them off.

## Layout

```
index.html       home screen markup and the #builder and #game mount points
css/style.css    materials (wood, felt, steel, parchment), buttons, card frames
js/net.js        reconnecting WebSocket, server address helpers
js/state.js      pure reducer: server messages -> client state
js/describe.js   pure: actions, events and targets -> text
js/picker.js     pure: legal actions -> the clicks a player makes (no card rules)
js/prompt.js     pure: what the prompt bar says for the current selection
js/dom.js        tiny element builder
js/card.js       one card: the scan, or a frame built from its data
js/render.js     draws the battle scene (DOM)
js/anim.js       pure plan() of what to animate between two views, plus the Web Animations that play it
js/main.js       wiring: home screen, connection, rejoin, click handlers
js/decklist.js   pure: what the home screen's deck shelf offers
js/deckmodel.js  pure: a deck being built and the rules it is judged by
js/builder.js    the deck builder screen
test/            node --test unit tests, fixtures, and a smoke test
serve.mjs        tiny static server for development
```

## Tests

```
npm test                                       # unit tests, no server needed
node test/smoke.mjs http://localhost:8080 10   # plays whole games against a server
```

The smoke test chooses every action through the picker, as a player would, and
fails if the picker ever offers an action the server did not list or leaves
the player with nothing to click. Start the server with `-bot-delay 1ms` to
make it fast.

## Docker

`docker-compose.yml` in the repository root runs the game server and this client behind one
port, so the browser sees a single origin (no CORS or `-origins` needed):

```bash
docker compose up -d --build   # then open http://localhost:8088
```

The `web` container (Caddy) serves the static files and proxies `/ws`, `/api/*`
and `/healthz` to the `server` container. To publish it, point your own reverse
proxy at port 8088 of the `web` container and keep the `Host` header (Caddy's
`reverse_proxy` does by default), which the WebSocket origin check compares:

```
cardwars.example.com {
	reverse_proxy host:8088
}
```

Games live in memory, so restarting the server ends any match in progress.
