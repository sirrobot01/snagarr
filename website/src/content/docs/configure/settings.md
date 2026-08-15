---
title: Settings
description: The two install-wide settings sections, how Snagarr encrypts them, how environment overrides lock a card, and what the reconcile loop does.
---

Settings are what stays global once every integration belongs to a member: the TMDB key and the install-wide knobs. Everything else is a [service](/snagarr/configure/services/).

Settings live in one JSON row of the `settings` table in `<data dir>/snagarr.db`, encrypted with `<data dir>/secret.key`. Read and write them at `GET`/`PUT /api/v1/settings` (admin only). `PUT` accepts a partial document; an omitted field keeps its stored value.

Start-up options — `SNAGARR_ADDR`, `SNAGARR_DATA_DIR`, `SNAGARR_LOG_LEVEL` — are not settings. They are read once, before the database opens. See [Environment variables](/snagarr/configure/environment/).

## Sections

| Section | Counts as configured when |
|---------|---------------------------|
| `tmdb` | `api_key` is set |
| `general` | always |

Radarr, Sonarr, Overseerr, Plex, Emby, Jellyfin and ntfy are gone from this endpoint. Each member connects their own at `/api/v1/services`.

## TMDB

Release builds carry a shared TMDB key, so this section works out of the box. Set `tmdb.api_key` to use your own key instead — it always wins over the shared one. A source build without an embedded key needs one; without any key, Snagarr saves captures but never resolves them.

| Field | Type | Default | Variable |
|-------|------|---------|----------|
| `tmdb.api_key` | string | *(empty — the shared key answers)* | `SNAGARR_TMDB_API_KEY` |

Use a TMDB API key (v3).

## General

| Field | Type | Default | Variable |
|-------|------|---------|----------|
| `general.reconcile_interval` | duration | `15m0s` | `SNAGARR_RECONCILE_INTERVAL` |
| `general.public_url` | string | *(empty)* | `SNAGARR_PUBLIC_URL` |
| `general.auto_send` | boolean | `true` | *(none)* |

`reconcile_interval` is a Go duration string (`15m`, `1h`, `90s`). Zero or less falls back to 15 minutes. The value round-trips in long form: send `15m`, read `15m0s`. The loop reads the interval before each wait, so a change applies after the current wait ends without a restart.

`public_url` is the URL an outside client uses, for example `https://snagarr.example.com`. It builds the ntfy click link and the bookmarklet.

`auto_send` hands a resolved capture to the capturer's own Radarr or Sonarr, with no second action. It is on by default.

| Condition | Result |
|-----------|--------|
| The title resolved and nobody owns it yet | Sent. The item reads `monitored` |
| The title is already in a library, monitored or requested | Nothing happens |
| The capturer owns no Radarr or Sonarr | Nothing happens. The **Send** button stays |
| The download manager rejects it or cannot be reached | Nothing happens. Snagarr logs it and the capture still stands |

Snagarr sends to the capturer's own service only. It never spends another member's, exactly as the **Send** button never does. A series goes to Sonarr, a film to Radarr; Overseerr is a manual send.

There is no webhook secret. An inbound webhook authenticates as a household member, with a username and password or with a token. See [Webhooks](/snagarr/use/webhooks/).

## Secrets at rest

Snagarr encrypts the settings blob and every service config with AES-256-GCM before it writes to the database. The key is 32 random bytes in `<data dir>/secret.key`, mode `0600`, generated on first start.

:::danger[Back up `secret.key` with the database]
Snagarr cannot read any stored setting or service without it. A restored database beside a new key loses every credential.
:::

| Value | Masked in the API |
|-------|-------------------|
| `tmdb.api_key` | Yes |
| A service config `api_key` or `token` | Yes |

A masked value comes back as `••••` plus the last four characters. Send it back unchanged to keep the stored secret.

## Environment overrides

An environment variable wins over the stored value. Snagarr applies the overrides at start-up and again after every save. A save still writes your value to the database, and the environment value goes back on top; the stored value takes effect only when you remove the variable.

Snagarr ignores an empty variable and an unparsable duration.

An override locks the whole settings card, not one field: the UI renders the card read-only, because an edit there is overwritten on the next restart. `SNAGARR_PUBLIC_URL` locks both the public URL and the reconcile interval on the General card.

`GET /api/v1/settings` reports `locked` per section. Services carry their own `locked` flag per record.

```sh
curl -X PUT http://localhost:8080/api/v1/settings \
  -H "Authorization: Bearer sngr_your_admin_token" \
  -H "Content-Type: application/json" \
  -d '{"general":{"reconcile_interval":"30m"}}'
```

## The reconcile loop

Snagarr runs one pass at start-up, then one every `general.reconcile_interval`. Each pass walks every enabled service in the household and does this work in order:

1. Replace the Radarr index and the Sonarr index. One list call per service.
2. Replace the Overseerr request index. One list call per service.
3. Sync each media server index.
4. Recompute the status of every snagged item from the local indexes.
5. Send one ntfy push for each item that became available.
6. Make each media server's collection hold `(snagged ∩ available) − watched`.
7. Refresh cached TMDB metadata older than 7 days.
8. Delete expired rows from the HTTP response cache.

The media server sync is incremental on most passes and full once every 24 hours, per service. Only a full sweep removes rows, because only a full sweep can see a deleted title.

A failed sync is logged and skipped. The pass continues, the stale index keeps answering and the next pass retries.

Force a pass with `POST /api/v1/admin/sync`, or with **Force reconcile now** in the settings UI. Creating, changing or deleting a service also starts one.

Inbound webhooks accelerate the loop. They never replace it. See [Webhooks](/snagarr/use/webhooks/).

## Test a connection

`POST /api/v1/settings/test` accepts one service name:

```json
{ "service": "tmdb" }
```

Any other name is `400 bad_request`. Test every other integration with `POST /api/v1/services/{id}/test`.
