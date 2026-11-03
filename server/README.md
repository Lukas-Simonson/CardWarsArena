# Card Wars server

Go rules engine, card data and tooling for the Adventure Time Card Wars web port.

```
cmd/cardimport   convert the card catalogue into data/cards
cmd/cardwars     interactive terminal client (play and test)
cmd/cardwarsd    WebSocket game server (rooms, bot opponent)
internal/card    card model, effect schema, catalog loader
internal/engine  the rules engine (pure, deterministic)
internal/bot     weighted-random computer player
internal/decks   finds preconstructed decks in the card data
internal/view    what one seat may see of a game (hidden information)
internal/server  rooms, protocol, WebSocket handling
data/            the catalogue, generated cards, hand-written effects
docs/            design notes (effects.md) and the client protocol (protocol.md)
```

## Playing in the terminal

```
go run ./cmd/cardwars                      # Finn vs Jake; you are Finn, a bot is Jake
go run ./cmd/cardwars --bot none           # hot-seat: both seats are you
go run ./cmd/cardwars --bot both           # watch two bots
go run ./cmd/cardwars --seed 42 --first 0  # replay a specific game
go run ./cmd/cardwars --list-decks         # decks and whether they are playable
go run ./cmd/cardwars --p0 "jake" --p1 "finn"
```

Every turn lists the legal actions as a numbered menu; type a number. Other
commands: `b` board, `h` hand, `a [text]` actions (filtered), `i <card|#>`
inspect (shows how the engine models it), `d` discard pile, `log`, `u` undo,
`json` full state, `bot ...`, `r` reveal hidden info, `?` help, `q` quit.

### Sandbox

`--sandbox` enables `/` commands that bypass the rules so you can test one card
without playing to it:

```
/give <card> [player]        put a card in a hand
/put <player> <lane> <card>  place a Creature or Building directly
/clear <player> <lane>
/hp <player> <n>             /actions <n>
/land <player> <lane> <type> [down]
```

Players are `0`, `1`, `me`, `opp`; lanes are 1-4; card names match by
substring. `u` undoes sandbox edits too.

Every card of all eight decks is modelled (Fionna's and Cake's decks list 39 cards each in the catalogue, one short
of the 40 a game needs, so the server does not offer them yet). A card without modelled rules (a future deck) is refused
unless `--allow-unmodelled` is given, which plays it with its text ignored.

## Running the server

```
go run ./cmd/cardwarsd                 # then open http://localhost:8080/
```

The page at `/` is a debug client: play against the bot, or open it in two
browsers to play yourself (Create room / Join). The client protocol is in
`docs/protocol.md`.

## Development

```
go test ./...                    # everything
go test -short ./...             # fewer random games
go run ./cmd/cardimport          # regenerate data/cards from data/CardWarsCatalogue.json
                                 # (-image-base sets the URL the card scans are served from)
```

Card rules live in `data/effects/*.json` (see `docs/effects.md`); never edit
`data/cards/` by hand.
