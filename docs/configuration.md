# Configuration

Snagarr has two layers of configuration.

**Start-up options** tell the process where to listen. They also tell it where
to keep its files. Snagarr reads them before it opens the database. Set them
with environment variables or command-line flags only.

**Settings** hold the integration credentials and the loop options. Snagarr
keeps them in the database. Edit them in the settings UI. Environment variables
can override them.

## Start-up options

Snagarr reads these once, at start-up. A change needs a restart.

| Variable | Type | Default | What it does |
|----------|------|---------|--------------|
| `SNAGARR_ADDR` | string | `:8080` | The address the HTTP server listens on. |
| `SNAGARR_DATA_DIR` | string | `data` | The directory for the database and the secret key. |
| `SNAGARR_LOG_LEVEL` | string | `info` | One of `debug`, `info`, `warn`, `error`. |

An unknown log level falls back to `info`. An empty variable falls back to the
default.

Two flags override the first two variables:

```sh
snagarr serve --addr :9090 --data /var/lib/snagarr
```

The Docker image sets `SNAGARR_DATA_DIR=/data`.

## Where settings live

Snagarr keeps all settings in one row of the `settings` table in
`<data dir>/snagarr.db`. The value is a JSON document.

The settings UI reads and writes this document through `GET /api/v1/settings`
and `PUT /api/v1/settings`. Only an admin token may call these routes. See
[api.md](api.md) for the request shapes.

`PUT` accepts a partial document. Any field you leave out keeps its stored
value.

## Secrets at rest

Snagarr encrypts every settings value with AES-256-GCM before it writes to the
database. The key is 32 random bytes in `<data dir>/secret.key`.

Snagarr generates the key on first start. The file mode is `0600`.

The key does not protect a compromised host. It keeps the credentials out of
database backups and copies.

> **Back up `secret.key` with the database.** Snagarr cannot read any stored
> setting without it. A restored database with a new key loses every credential.

These eight fields are treated as secrets:

`tmdb.api_key` · `library.token` · `radarr.api_key` · `sonarr.api_key` ·
`overseerr.api_key` · `ntfy.token` · `telegram.bot_token` ·
`general.webhook_secret`

`GET /api/v1/settings` masks each one. It sends `••••` plus the last four
characters. Send a masked value back unchanged to keep the stored secret.

## Environment overrides

An environment variable wins over the stored value. Snagarr applies the
overrides at start-up. It applies them again after every save.

A save still writes your value to the database. The environment value then
goes back on top. The stored value takes effect only when you remove the
variable.

Snagarr ignores an empty variable. It ignores a number it cannot parse. It
ignores a duration it cannot parse.

### Locking in the UI

An override locks the whole settings card, not one field. The UI renders the
card read-only. This stops an operator from editing a value that the next
restart overwrites.

Example: `SNAGARR_RADARR_URL` locks the URL, the API key, the quality profile
and the root folder on the Radarr card.

## TMDB

TMDB is required. Without it, Snagarr saves captures but cannot resolve them.

| Field | Type | Default | Environment variable |
|-------|------|---------|----------------------|
| `tmdb.api_key` | string | *(empty)* | `SNAGARR_TMDB_API_KEY` |

Use a TMDB API key (v3). The section counts as configured when the key is set.

## Media server

Snagarr reads one media server. It mirrors the contents into a local index. It
also keeps the "Snagged" collection in sync there.

| Field | Type | Default | Environment variable |
|-------|------|---------|----------------------|
| `library.provider` | string | *(empty)* | `SNAGARR_LIBRARY_PROVIDER` |
| `library.url` | string | *(empty)* | `SNAGARR_LIBRARY_URL` |
| `library.token` | string | *(empty)* | `SNAGARR_LIBRARY_TOKEN` |
| `library.section_ids` | string list | *(empty)* | *(none)* |
| `library.collection_name` | string | `Snagged` | `SNAGARR_LIBRARY_COLLECTION` |

`library.provider` must be `plex`, `emby` or `jellyfin`. Any other value leaves
the media server unconfigured.

`library.url` is the base URL of the server, for example
`http://plex.lan:32400` or `http://emby.lan:8096`.

`library.token` holds the credential for the provider:

| Provider | Credential | Header Snagarr sends |
|----------|-----------|----------------------|
| `plex` | Plex token | `X-Plex-Token` |
| `emby` | Emby API key | `X-Emby-Token` |
| `jellyfin` | Jellyfin API key | `X-Emby-Token` |

`library.section_ids` limits the sync to named libraries. An empty list means
every movie library and every show library. Snagarr always drops music and
photo libraries.

The settings UI lists the real sections after you save the URL and the token.
It falls back to a free-text field of comma-separated IDs.

The section counts as configured when the provider, the URL and the token are
all set.

`library.collection_name` names the collection Snagarr maintains on the media
server. An empty name turns the collection sync off.

## Radarr

| Field | Type | Default | Environment variable |
|-------|------|---------|----------------------|
| `radarr.url` | string | *(empty)* | `SNAGARR_RADARR_URL` |
| `radarr.api_key` | string | *(empty)* | `SNAGARR_RADARR_API_KEY` |
| `radarr.quality_profile_id` | integer | `0` | `SNAGARR_RADARR_QUALITY_PROFILE_ID` |
| `radarr.root_folder` | string | *(empty)* | `SNAGARR_RADARR_ROOT_FOLDER` |
| `radarr.search_on_add` | boolean | `true` | *(none)* |
| `radarr.season_folder` | boolean | `false` | *(none)* |

Snagarr uses the quality profile and the root folder when you send a movie.
`search_on_add` asks Radarr to search for the release at once.

`season_folder` has no effect on Radarr. Snagarr sends it to Sonarr only.

The section counts as configured when the URL and the API key are both set.

## Sonarr

| Field | Type | Default | Environment variable |
|-------|------|---------|----------------------|
| `sonarr.url` | string | *(empty)* | `SNAGARR_SONARR_URL` |
| `sonarr.api_key` | string | *(empty)* | `SNAGARR_SONARR_API_KEY` |
| `sonarr.quality_profile_id` | integer | `0` | `SNAGARR_SONARR_QUALITY_PROFILE_ID` |
| `sonarr.root_folder` | string | *(empty)* | `SNAGARR_SONARR_ROOT_FOLDER` |
| `sonarr.search_on_add` | boolean | `true` | *(none)* |
| `sonarr.season_folder` | boolean | `true` | *(none)* |

Sonarr keys its series on TVDB. Snagarr asks TMDB for the TVDB ID before it
sends a series. Keep the TMDB key set for this reason.

The section counts as configured when the URL and the API key are both set.

## Overseerr

Overseerr is optional. Jellyseerr uses the same API.

| Field | Type | Default | Environment variable |
|-------|------|---------|----------------------|
| `overseerr.url` | string | *(empty)* | `SNAGARR_OVERSEERR_URL` |
| `overseerr.api_key` | string | *(empty)* | `SNAGARR_OVERSEERR_API_KEY` |
| `overseerr.prefer` | boolean | `false` | *(none)* |

When configured, Snagarr mirrors the request list into a local index. A title
with a request shows the `REQUESTED` badge.

`overseerr.prefer` is stored but not used yet. The clients offer all three
send targets at all times.

The section counts as configured when the URL and the API key are both set.

## ntfy

ntfy is optional. Snagarr sends one push when a snagged title becomes
available.

| Field | Type | Default | Environment variable |
|-------|------|---------|----------------------|
| `ntfy.url` | string | `https://ntfy.sh` | `SNAGARR_NTFY_URL` |
| `ntfy.topic` | string | *(empty)* | `SNAGARR_NTFY_TOPIC` |
| `ntfy.token` | string | *(empty)* | `SNAGARR_NTFY_TOKEN` |
| `ntfy.priority` | integer | `3` | *(none)* |

Set `ntfy.token` for a server that needs authentication. Leave it empty for an
open server.

`ntfy.priority` must be 1 to 5. Any other value makes Snagarr leave the header
out, so the ntfy server default applies.

The section counts as configured when the topic is set.

The push carries the capture context, for example
`Sinners is ready — snagged by Amina, 12 Jul, from telegram`. It carries a click
link when `general.public_url` is set.

## Telegram

| Field | Type | Default | Environment variable |
|-------|------|---------|----------------------|
| `telegram.bot_token` | string | *(empty)* | `SNAGARR_TELEGRAM_BOT_TOKEN` |

The Telegram bot is not implemented yet. The field is stored and reported as
configured. Nothing reads it. See [clients.md](clients.md).

## General

| Field | Type | Default | Environment variable |
|-------|------|---------|----------------------|
| `general.reconcile_interval` | duration string | `15m` | `SNAGARR_RECONCILE_INTERVAL` |
| `general.public_url` | string | *(empty)* | `SNAGARR_PUBLIC_URL` |
| `general.webhook_secret` | string | *(generated)* | *(none)* |
| `general.stale_days` | integer | `90` | *(none)* |
| `general.image_base` | string | `https://image.tmdb.org/t/p` | *(none)* |

`general.reconcile_interval` is a Go duration string, for example `15m`, `1h`
or `90s`. A value of zero or less falls back to 15 minutes.

> Snagarr reads the interval once, at start-up. Restart the process after you
> change it.

`general.public_url` is the URL an outside client uses to reach Snagarr, for
example `https://snagarr.example.com`. Snagarr uses it for the ntfy click link.
The settings UI uses it to build the bookmarklet.

`general.webhook_secret` authenticates every inbound webhook. Snagarr generates
32 hexadecimal characters on first start. See [webhooks.md](webhooks.md).

`general.stale_days` is stored but not used yet. Stale-item triage is not
implemented.

`general.image_base` is stored but not used yet. The web client uses
`https://image.tmdb.org/t/p` directly.

## All environment variables

| Variable | Setting it overrides |
|----------|----------------------|
| `SNAGARR_ADDR` | *(start-up option)* |
| `SNAGARR_DATA_DIR` | *(start-up option)* |
| `SNAGARR_LOG_LEVEL` | *(start-up option)* |
| `SNAGARR_TMDB_API_KEY` | `tmdb.api_key` |
| `SNAGARR_LIBRARY_PROVIDER` | `library.provider` |
| `SNAGARR_LIBRARY_URL` | `library.url` |
| `SNAGARR_LIBRARY_TOKEN` | `library.token` |
| `SNAGARR_LIBRARY_COLLECTION` | `library.collection_name` |
| `SNAGARR_RADARR_URL` | `radarr.url` |
| `SNAGARR_RADARR_API_KEY` | `radarr.api_key` |
| `SNAGARR_RADARR_QUALITY_PROFILE_ID` | `radarr.quality_profile_id` |
| `SNAGARR_RADARR_ROOT_FOLDER` | `radarr.root_folder` |
| `SNAGARR_SONARR_URL` | `sonarr.url` |
| `SNAGARR_SONARR_API_KEY` | `sonarr.api_key` |
| `SNAGARR_SONARR_QUALITY_PROFILE_ID` | `sonarr.quality_profile_id` |
| `SNAGARR_SONARR_ROOT_FOLDER` | `sonarr.root_folder` |
| `SNAGARR_OVERSEERR_URL` | `overseerr.url` |
| `SNAGARR_OVERSEERR_API_KEY` | `overseerr.api_key` |
| `SNAGARR_NTFY_URL` | `ntfy.url` |
| `SNAGARR_NTFY_TOPIC` | `ntfy.topic` |
| `SNAGARR_NTFY_TOKEN` | `ntfy.token` |
| `SNAGARR_TELEGRAM_BOT_TOKEN` | `telegram.bot_token` |
| `SNAGARR_PUBLIC_URL` | `general.public_url` |
| `SNAGARR_RECONCILE_INTERVAL` | `general.reconcile_interval` |

No other `SNAGARR_*` variable exists.

## Settings the UI does not show

The settings UI has one card per integration. It does not render every field.
Set the fields below with `PUT /api/v1/settings` or with an environment
variable.

| Field | How to set it |
|-------|---------------|
| `library.collection_name` | API, or `SNAGARR_LIBRARY_COLLECTION` |
| `radarr.search_on_add` | API |
| `sonarr.search_on_add` | API |
| `sonarr.season_folder` | API |
| `overseerr.prefer` | API |
| `ntfy.token` | API, or `SNAGARR_NTFY_TOKEN` |
| `ntfy.priority` | API |
| every `general.*` field | API, or the variables listed above |

Example:

```sh
curl -X PUT http://localhost:8080/api/v1/settings \
  -H "Authorization: Bearer sngr_your_admin_token" \
  -H "Content-Type: application/json" \
  -d '{"general":{"reconcile_interval":"30m"},"library":{"collection_name":"Watch Later"}}'
```

## The reconcile loop

The loop keeps the local indexes fresh. It derives every item's state from
them. No request to Snagarr waits on Plex, Radarr or Sonarr.

Snagarr runs one pass at start-up. It then runs a pass every
`general.reconcile_interval`.

Each pass does this work in order:

1. Replace the Radarr index and the Sonarr index. One list call per service.
2. Replace the Overseerr request index, if Overseerr is configured.
3. Sync the media server index. See below.
4. Recompute the status of every snagged item from the local indexes.
5. Send one ntfy push for each item that became available.
6. Make the collection hold `(snagged ∩ available) − watched`.
7. Refresh cached TMDB metadata older than 7 days.
8. Delete expired rows from the HTTP response cache.

The media server sync is incremental on most passes. It is full once every 24
hours. Only a full sweep can find a deleted title, so only a full sweep removes
rows from the index.

A failed sync is logged and skipped. The pass continues. The stale index keeps
answering. The next pass retries.

Force a pass at any time with `POST /api/v1/admin/sync`. The settings UI has a
**Force reconcile now** button.

Inbound webhooks are an accelerator on top of the loop. See
[webhooks.md](webhooks.md).

## Test a connection

Use the **Test** button on each settings card. It calls
`POST /api/v1/settings/test`.

The card shows the upstream message without change, for example
`OK · 611 items` or `401 — check token`.

There is no test for Telegram.
