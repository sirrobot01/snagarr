<div align="center">

# Snagarr

**Snag it now, watch it later.**

Capture a film or show the moment you hear about it. Find it waiting on your
media server when you sit down to watch.

[![CI](https://github.com/sirrobot01/snagarr/actions/workflows/ci.yml/badge.svg)](https://github.com/sirrobot01/snagarr/actions/workflows/ci.yml)
[![Release](https://img.shields.io/github/v/release/sirrobot01/snagarr)](https://github.com/sirrobot01/snagarr/releases)
[![License](https://img.shields.io/badge/license-MIT-blue)](LICENSE)

</div>

---

Snagarr closes the gap between two moments: when you hear about a film, and when
you decide what to watch. It captures the title in seconds, identifies it
against TMDB, hands it to Radarr or Sonarr, and then puts it in a **Snagged**
collection on your media server when the file lands.

Snagarr does not replace Overseerr, Radarr or your watchlist. It **works with
them**. Overseerr manages requests. Radarr and Sonarr acquire files. Snagarr
handles the two parts nobody else covers: the capture, and the reminder.

## The problem

1. **Capture friction loses intentions.** You open a notes app, you find the add
   screen, you type the title. The moment has gone. The film is forgotten.
2. **State is scattered.** "Do I already have this?" means checking Plex, then
   Radarr, then Overseerr. Duplicates happen.
3. **Lists rot.** Watch-later lists live in apps you never open at the moment you
   choose what to watch.

## What Snagarr does

| | |
|---|---|
| **Capture** | Type or paste into one box. Free text, or a link from TMDB, IMDB, Letterboxd or a review page. Nothing is ever lost: an unclear capture waits in Needs Review with its best guesses. |
| **Identify** | TMDB search with confidence scoring. A clear winner resolves on its own. Anything doubtful asks you once, and one tap settles it. |
| **Show state** | Every result carries one badge before you add it: In Library, Monitored, Requested or new. You see the duplicate before you make it. |
| **Act** | One tap sends the title to Radarr, Sonarr or Overseerr, with your quality profile and root folder. |
| **Remind** | When the file lands, the title appears in a **Snagged** collection on Plex, Emby or Jellyfin. You get an ntfy push that says why it is there: *"Sinners is ready — snagged by Amina, 12 Jul, from Telegram."* |
| **Clean up** | Watch it, and it leaves the collection. The collection is live state, never history. |

Snagarr answers every state question from **local indexes**. It mirrors your
library and your *arr lists into SQLite. Search and badges never wait on an
external API. A service that goes down degrades Snagarr; it does not stop it.

## Install

Snagarr is one static binary with the web UI inside it. There is no database
server, no Redis and no reverse proxy to set up.

### Docker Compose

```yaml
services:
  snagarr:
    image: ghcr.io/sirrobot01/snagarr:latest
    container_name: snagarr
    ports:
      - "8080:8080"
    volumes:
      - ./data:/data
    restart: unless-stopped
```

Start it, then read the first-run token from the logs:

```bash
docker compose up -d
docker compose logs snagarr
```

The log prints a setup URL that carries the token. Open it. Snagarr stores the
token in your browser.

### Binary

Download a build for your platform from the
[releases page](https://github.com/sirrobot01/snagarr/releases), then run it:

```bash
./snagarr serve
```

Snagarr listens on `:8080`. It writes to `./data`.

## Set up

The setup wizard asks for four things. You can skip any of them and add them
later in Settings.

1. **TMDB API key.** Required. Get a free key from
   [themoviedb.org](https://www.themoviedb.org/settings/api).
2. **Media server.** Plex, Emby or Jellyfin. This gives you the library badges
   and the Snagged collection.
3. **Radarr and Sonarr.** These let you send titles with one tap.
4. **ntfy.** Optional. This sends the push when a title becomes ready.

Each card has a **Test connection** button. Use it before you save.

See [docs/configuration.md](docs/configuration.md) for every setting, and
[docs/deployment.md](docs/deployment.md) for backups and upgrades.

## Capture from anywhere

The API is the product. The web app is one client.

- **Web** — the add box is the home page. It is focused when the page opens.
  Press `/` from anywhere to return to it.
- **Apple Shortcut** — share a link or type a title from iOS.
- **Bookmarklet** — send the page you are reading. Generate it in Settings.
- **curl or any script** — one `POST` to `/api/v1/capture`.

```bash
curl -X POST https://snagarr.example.com/api/v1/capture \
  -H "Authorization: Bearer $SNAGARR_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"query": "sinners", "source": "cli"}'
```

See [docs/clients.md](docs/clients.md) to build each one.

## Household

Snagarr uses one shared list with attribution. It does not use accounts or
passwords.

Add a household member in Settings. Give them a token. Their captures show
their name, and the push that announces a film says who wanted it.

Members can capture, browse and resolve. Only admins can send to Radarr or
Sonarr, delete other people's items, and change settings.

## Documentation

| Document | Contents |
|---|---|
| [Configuration](docs/configuration.md) | Every setting and environment variable |
| [Deployment](docs/deployment.md) | Docker, binaries, backup and upgrade |
| [API](docs/api.md) | The full HTTP API |
| [Webhooks](docs/webhooks.md) | Radarr, Sonarr, Tautulli and Emby events |
| [Clients](docs/clients.md) | Shortcut, bookmarklet and scripts |
| [Design](docs/design.md) | The UI specification |

## Develop

Snagarr uses [Task](https://taskfile.dev) and [air](https://github.com/air-verse/air).

```bash
task install    # Go tools and web dependencies
task dev        # API with live reload, plus the Vite dev server
task test       # Go and web tests
task build      # one binary with the UI inside it, into bin/
```

The Go API runs on `:8080`. The Vite dev server proxies `/api` to it.

**Layout.** `cmd/snagarr` is the entry point. `internal/store` owns SQLite.
`internal/api` is the HTTP surface. `internal/reconcile` keeps the indexes
fresh and derives state. `internal/tmdb`, `internal/arr`, `internal/library`,
`internal/overseerr` and `internal/notify` are the external clients, each behind
a narrow interface. `web` is the React client, built into `internal/web/dist`
and embedded at compile time.

## Non-goals

Snagarr is deliberately small.

- It is not a watched-history tracker. Trakt and Watcharr do that.
- It is not a request system with approval flows. Overseerr does that.
- It is not a media server, a downloader or an indexer.
- It is not multi-tenant SaaS.

## Credits

This product uses the TMDB API but is not endorsed or certified by TMDB.

Licensed under the [MIT License](LICENSE).
