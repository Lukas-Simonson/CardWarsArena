# Card Wars Arena web client

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
js/decktext.js   pure: a deck as plain text (and Card Dweeb lists), to copy, download and import
js/builder.js    the deck builder screen
js/avatar.js     a player's picture, or the first letter of their name
js/i18n.js       t(key, params), the message format, locale choice, pseudo-locales
js/errors.js     server error codes -> messages
js/sounds.js     pure: the sound cues, and which sounds the planned effects make
js/audio.js      the Web Audio engine (unlock, buffers, limits, variation), with the browser passed in
js/sound.js      the engine wired to the browser, the saved settings and the click sounds
js/soundui.js    the speaker button and the sound settings dialog
audio/           the sound files (two formats each) and CREDITS.md
locales/         message catalogues: en.json (the source) and index.json (the locales offered)
tools/           the checks on the catalogues (npm run i18n)
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

## Sound

Sound is driven from the same plan as the animation: `plan()` in `js/anim.js` lists what happens in an update, and
`soundsFor()` in `js/sounds.js` turns those effects into cues to play at the same delays, so what is heard stays in step with
what is seen and with the pacing. The cues (which files, how loud, how soon one may repeat, which are "core") are the table in
`js/sounds.js`; `js/audio.js` plays them with the Web Audio API.

- Nothing plays until the player has clicked or pressed a key (browsers insist); a sound asked for earlier is dropped quietly.
  With no audio support, or files that fail to load, it does nothing and nothing breaks. Nothing plays while the tab is hidden.
- A cue has a cooldown, hits share a limit, and at most 8 sounds play at once, so a busy board is not a wall of noise. Each
  play varies a little in pitch and loudness.
- The speaker button (lobby and match) opens the settings: volume, sound effects, mute, and "fewer sounds" (only turns, hits,
  defeats, errors and the end of the game; on by default when the system prefers reduced motion). They are kept in the browser.
- Files are in `audio/`, each as `.ogg` and `.m4a` (a browser that cannot play Ogg gets the `.m4a`). Every file is from a CC0 pack
  by Kenney and credited in `audio/CREDITS.md`; `npm test` fails on an uncredited, unused or missing file, or if they grow past
  2 MB. To use another sound, add it (original, CC0 or CC-BY: never from a show or game), credit it, and name it in `sounds.js`.
- There is no music yet: Kenney's packs have none, and nothing else we may use has been chosen.

## Localization

Every string the player reads is a message in `locales/<tag>.json`, looked up by a stable key: `t("lobby.join")`,
`t("shelf.cards", { n })`. English (`en.json`) is the source and the fallback for any key a language lacks.

Messages use a small part of ICU MessageFormat, so word order, plurals and choices live in the message and not in the code:

```
"Hello {name}"                                                  a parameter
"{n, plural, =0 {no cards} one {# card} other {# cards}}"       plural rules of the language (# is the number)
"{kind, select, creature {…} other {…}}"                        choose by a string
"{items, list}"                                                 "a, b and c" (pass an array)
'{' and '}' in quotes are literal; a lone ' is an apostrophe.
```

**Adding a string.** Add the key to `locales/en.json` and use it with `t()` (or `data-i18n="key"` / `data-i18n-attr="title:key"`
in `index.html`). Write whole sentences with parameters; never join phrases in code. `npm run i18n` (and `npm test`) fail on a key
the code uses that English lacks, a key nothing uses, a locale that differs from English, or text that looks hard-coded
(`node tools/i18n-check.mjs --strings` lists it; `// i18n-ignore` marks a word that is not for the player).

**Adding a language.** Copy `locales/en.json` to `locales/<tag>.json` (for example `fr` or `pt-BR`), translate the values, keep every
`{placeholder}` and add the plural forms the language needs, then list it in `locales/index.json`. A key left out falls back to English
while you work, but CI requires a finished file to have every key. The language picker appears once two languages are listed;
`?lang=fr` tries one without saving it, and a saved choice (`cw.locale`) wins over the browser's languages. `fr-CA` falls back to `fr`, then English.

**Testing layout.** `?lang=en-XA` shows every message accented and about 40% longer (anything still in plain English is hard-coded);
`?lang=en-XB` also lays the page out right to left.

**Card data.** Card names and rules text come from the server in the language asked for (`?lang=`): the translations live in `server/data/i18n/<tag>/` and are never overwritten by the card importer; see `server/docs/localization.md` for how to start a language (`go run ./cmd/cardi18n init <tag>`). A card with no translation is English, and `name_en` lets a deck list be written and read in English whatever language is shown. The names a card gives its targets
("Heal", "Freeze") are looked up as `label.<the text in lower case>`; `node tools/i18n-check.mjs --add-labels` adds any a new card brings.
The server sends no prose for the client to translate: an error arrives as a `code` (the kind), a `reason` (exactly what: a rule
of the game, a deck problem, why a room closed) and `params` (cards by id), and the client words it from `reason.<reason>` or
`error.<code>` in the catalogue (`js/errors.js`); the English `message` shows only for a code the client has no message for.
See "Error codes" in `server/docs/protocol.md`. Adding a rule or a deck problem on the server means adding its `reason.<code>` message here.

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
