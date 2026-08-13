---
title: Radarr and Sonarr
description: Send targets — Radarr for movies, Sonarr for series, Overseerr for requests — as per-member services, and the fields each one uses.
---

Radarr, Sonarr and Overseerr are [services](/snagarr/configure/services/). Every member connects their own.

Send a resolved item with `POST /api/v1/items/{id}/send`. Any token may send. Snagarr picks the target from the media type in the UI, so a title has one send button, not two.

| Target | Media type | Item status after the send |
|--------|-----------|----------------------------|
| `radarr` | `movie` | `monitored` |
| `sonarr` | `tv` | `monitored` |
| `overseerr` | either | `requested` |

All three targets stay available at all times. No setting makes a request replace a direct push.

:::caution[A send spends your own service]
A member may only send to a service they own. With none of that kind, the send answers `503 not_configured`, and never borrows an admin's. An admin falls through: their own service, then the capturer's, then another admin's.
:::

## Radarr

| Field | Type | Default | Variable |
|-------|------|---------|----------|
| `url` | string | *(empty)* | `SNAGARR_RADARR_URL` |
| `api_key` | string | *(empty)* | `SNAGARR_RADARR_API_KEY` |
| `quality_profile_id` | integer | `0` | `SNAGARR_RADARR_QUALITY_PROFILE_ID` |
| `root_folder` | string | *(empty)* | `SNAGARR_RADARR_ROOT_FOLDER` |
| `search_on_add` | boolean | `true` | *(none)* |

Configured when the URL and the API key are both set.

`search_on_add` asks Radarr to search for a release at once.

## Sonarr

| Field | Type | Default | Variable |
|-------|------|---------|----------|
| `url` | string | *(empty)* | `SNAGARR_SONARR_URL` |
| `api_key` | string | *(empty)* | `SNAGARR_SONARR_API_KEY` |
| `quality_profile_id` | integer | `0` | `SNAGARR_SONARR_QUALITY_PROFILE_ID` |
| `root_folder` | string | *(empty)* | `SNAGARR_SONARR_ROOT_FOLDER` |
| `search_on_add` | boolean | `true` | *(none)* |
| `season_folder` | boolean | `true` | *(none)* |

Configured when the URL and the API key are both set.

Radarr and Sonarr share one config type. `season_folder` reaches Sonarr only; on a Radarr service it has no effect.

Sonarr keys its series on TVDB. Snagarr asks TMDB for the TVDB ID before it sends a series, so a Sonarr send fails without a TMDB key.

## Add one

```sh
curl -X POST http://localhost:8080/api/v1/services \
  -H "Authorization: Bearer sngr_your_token" \
  -H "Content-Type: application/json" \
  -d '{"kind":"radarr","config":{"url":"http://radarr.lan:7878","api_key":"your_key"}}'
```

The environment variables seed the first admin's services only. See [Environment variables](/snagarr/configure/environment/#service-seeding).

## Quality profiles and root folders

The card offers the real choices once the record is saved and complete. Read the same list from the API:

```sh
curl http://localhost:8080/api/v1/services/4/options \
  -H "Authorization: Bearer sngr_your_token"
```

```json
{ "quality_profiles": [ { "id": 4, "name": "HD-1080p" } ],
  "root_folders":     [ { "path": "/movies", "free_space": 812340000000 } ] }
```

A `radarr` or `sonarr` service answers with profiles and folders. A media server answers with `sections`. An `overseerr` or `ntfy` service has no options and answers `400 bad_request`.

## Overseerr

Optional. Jellyseerr uses the same API.

| Field | Type | Default | Variable |
|-------|------|---------|----------|
| `url` | string | *(empty)* | `SNAGARR_OVERSEERR_URL` |
| `api_key` | string | *(empty)* | `SNAGARR_OVERSEERR_API_KEY` |

Configured when the URL and the API key are both set.

Snagarr mirrors every enabled Overseerr request list into a local index on each reconcile pass. A title with a request shows the `REQUESTED` badge. One member's fulfilled request settles the title for the household.

## After a send

Each reconcile pass replaces the Radarr index and the Sonarr index with one list call per service, then recomputes item status from those indexes. An item stays `monitored` until the file lands, then becomes `available`.

Two members can both track a title. The household answer is the more advanced of the two: the `monitored` and `has_file` flags are ORed, never overwritten.

A Radarr or Sonarr webhook reports the import at once instead of waiting for the next pass. See [Webhooks](/snagarr/use/webhooks/).
