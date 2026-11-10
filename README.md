# Card Wars Arena

Card Wars Arena is a web port of the Adventure Time Card Wars card game. Play the computer, or a friend with a room code, in your browser.

- Play against a bot, or against a friend by sharing a room code
- Nine ready-to-play starter decks, each with its Hero
- Optional sign-in with Discord or GitHub, to build and keep your own decks in a deck builder, and copy decks to and from plain text (Card Dweeb lists too)
- A Go rules engine that runs the real rules, and a browser client with animations and no build step

> **This is an unofficial fan project.** See [A fan project](#a-fan-project) below.

```
server/   Go rules engine, card data, game server and terminal client (see server/README.md)
web/      the browser client: plain ES modules, no build step (see web/README.md)
```

The two halves talk over the WebSocket protocol in [`server/docs/protocol.md`](server/docs/protocol.md);
a change to it changes both, which is why they live in one repository.

## Quick start

You need [Docker](https://docs.docker.com/get-docker/) with Compose.

```bash
docker compose up -d --build
```

Then open http://localhost:8088, pick a deck and press **Practice**. To play a friend, choose **Make a new room**
and send them the code. How to play is in [`web/README.md`](web/README.md#how-to-play).

## Running your own server

The images are published for each release to GitHub's container registry as
`ghcr.io/lukas-simonson/cardwarstcg-server` and `ghcr.io/lukas-simonson/cardwarstcg-web`
(`linux/amd64` and `linux/arm64`). `docker-compose.yml` uses them: pin a version, or keep `latest` and update with

```bash
docker compose pull && docker compose up -d
```

The first time, the packages may be private: make them public under the repository's Packages settings, or
`docker login ghcr.io` with a token that has `read:packages`. Merging to `main` publishes nothing;
see [CONTRIBUTING.md](CONTRIBUTING.md#releases).

### Configuration

Both images are configured through environment variables, so they run on any host that runs containers
(compose, Railway, Fly, a VPS). The server's settings are also command-line flags; the flag wins when both are set.

**Server** (`cardwarstcg-server`, listens on 8080):

| Variable | Flag | What it does |
|---|---|---|
| `CARDWARS_DB` | `-db` | The SQLite file for accounts and decks. Without it nothing is kept. In the image, put it on a volume, e.g. `/var/lib/cardwars/cardwars.db`. The compose file does this. |
| `CARDWARS_AUTH` | `-auth` | `none` (the default) or `oauth` for sign-in with Discord and GitHub. `oauth` needs `CARDWARS_DB` and `CARDWARS_PUBLIC_URL`. |
| `CARDWARS_PUBLIC_URL` | `-public-url` | Where browsers reach the site, e.g. `https://cards.example.com`: the address only, with no path. Providers send people back to it, and it decides whether cookies are `Secure`. |
| `CARDWARS_DISCORD_CLIENT_ID`, `CARDWARS_DISCORD_CLIENT_SECRET` | | The app's credentials at Discord. A provider without both is not offered. |
| `CARDWARS_GITHUB_CLIENT_ID`, `CARDWARS_GITHUB_CLIENT_SECRET` | | The same for GitHub. |

The other flags (`-addr`, `-data`, `-origins`, `-bot-delay`, `-dev`) are described in [`server/docs/protocol.md`](server/docs/protocol.md).

**Web** (`cardwarstcg-web`, Caddy, listens on 80):

| Variable | What it does |
|---|---|
| `CARDWARS_UPSTREAM` | Where the game server is, as `host:port`. Defaults to `server:8080`, the name compose gives it. |

### Signing in

By default the server is anonymous and everyone plays as a guest. To let people sign in:

1. Register an app with Discord and/or GitHub, with the callback `<public url>/api/auth/callback/discord` (or `/github`).
2. Copy `.env.example` to `.env` and fill in the address and the credentials.
3. `docker compose up -d`.

Only the provider's id for the person, a display name and a picture address are stored: no email and no password.
Signed-in players play under their account name and can build, save and play their own decks. Guests can still play.

### Things to know

- The browser must reach the web image and the server through **one address**: the web image serves the page and
  proxies `/ws` and `/api/*` to the server, and sign-in relies on the cookie being first-party. Expose the web image
  publicly and keep the server private.
- The server runs as a non-root user (uid 65532), so the directory behind `CARDWARS_DB` must be writable by it. A named
  volume is; a folder on the host needs `chown 65532:65532`. The card data lives in the image at `/data`: do not mount over it.
- Games live in the server's memory, so restarting it ends every game in progress. Accounts and decks are in the database
  and survive.
- A database file can be used by one server at a time: run a single instance.
- To publish the compose setup, point your own reverse proxy at port 8088 of the `web` container and keep the `Host`
  header (Caddy's `reverse_proxy` does by default), which the WebSocket origin check compares.

## Development

You need Go (the version in [`server/go.mod`](server/go.mod)) and Node 22 or newer. Run the server and the client
in two terminals:

```bash
cd server && go run ./cmd/cardwarsd -origins 'localhost:*'   # the game server on :8080
cd web && npm start                                          # the client on :5173
```

Then open http://localhost:5173/. Run the tests with:

```bash
cd server && go test ./...
cd web && npm test
```

Each directory's README has the details: [`server/README.md`](server/README.md) covers the engine, the card data and
the terminal client, and [`web/README.md`](web/README.md) covers the client's layout and tests. How we work, and how
to add or fix a card, is in [CONTRIBUTING.md](CONTRIBUTING.md).

## Contributing

Bug reports, card fixes and pull requests are welcome. Read [CONTRIBUTING.md](CONTRIBUTING.md) first. Planned work
is tracked in the [issues](https://github.com/Lukas-Simonson/CardWarsTCG/issues).

## A fan project

Card Wars Arena is an unofficial, non-commercial fan project. It is not affiliated with, authorized by or endorsed by
Cartoon Network, Warner Bros. Discovery, Cryptozoic Entertainment or anyone else who owns rights to Adventure Time or
Card Wars.

Adventure Time, Card Wars, and their characters, names, card text, artwork and other related material are the
property of their respective owners, and no ownership of them is claimed here. This repository's license covers the
code written for this project, not that material. If you are a rights holder and want something changed or removed,
please [open an issue](https://github.com/Lukas-Simonson/CardWarsTCG/issues) and it will be dealt with promptly.

## AI Disclosure

AI coding assistants were used to build this project. Much of the code, tests and documentation was written
with their help, directed and reviewed by the project's maintainers and contributors, who decide what is built
and what is merged. Commits and pull requests made with AI assistance say so in their attribution lines.

The game's rules, the card catalogue and the card images come from the Adventure Time Card Wars card game and
were not generated by AI. The code is tested (see [Development](#development)), but it may still have bugs,
including rules mistakes. If a card does the wrong thing, please
[open an issue](https://github.com/Lukas-Simonson/CardWarsTCG/issues).

Contributions made with AI assistance are welcome; please say so in the pull request.

## License

The code is released under the [MIT License](LICENSE). It applies to the source code only, as described above.
