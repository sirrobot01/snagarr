---
title: Radarr and Sonarr
description: Send targets — Radarr for movies, Sonarr for series, Overseerr for requests — and the fields each one uses.
---

An admin sends a resolved item to one of three targets with `POST /api/v1/items/{id}/send`. Snagarr picks the target from the media type in the UI, so a title has one send button, not two.

| Target | Media type | Item status after the send |
|--------|-----------|----------------------------|
| `radarr` | `movie` | `monitored` |
| `sonarr` | `tv` | `monitored` |
| `overseerr` | either | `requested` |

All three targets stay available at all times. No setting makes a request replace a direct push.

## Radarr

| Field | Type | Default | Variable |
|-------|------|---------|----------|
| `radarr.url` | string | *(empty)* | `SNAGARR_RADARR_URL` |
| `radarr.api_key` | string | *(empty)* | `SNAGARR_RADARR_API_KEY` |
| `radarr.quality_profile_id` | integer | `0` | `SNAGARR_RADARR_QUALITY_PROFILE_ID` |
| `radarr.root_folder` | string | *(empty)* | `SNAGARR_RADARR_ROOT_FOLDER` |
| `radarr.search_on_add` | boolean | `true` | *(none)* |
| `radarr.season_folder` | boolean | `false` | *(none)* |

Configured when the URL and the API key are both set.

`search_on_add` asks Radarr to search for a release at once. `radarr.season_folder` has no effect: Radarr and Sonarr share one settings type, and Snagarr sends the field to Sonarr only.

## Sonarr

| Field | Type | Default | Variable |
|-------|------|---------|----------|
| `sonarr.url` | string | *(empty)* | `SNAGARR_SONARR_URL` |
| `sonarr.api_key` | string | *(empty)* | `SNAGARR_SONARR_API_KEY` |
| `sonarr.quality_profile_id` | integer | `0` | `SNAGARR_SONARR_QUALITY_PROFILE_ID` |
| `sonarr.root_folder` | string | *(empty)* | `SNAGARR_SONARR_ROOT_FOLDER` |
| `sonarr.search_on_add` | boolean | `true` | *(none)* |
| `sonarr.season_folder` | boolean | `true` | *(none)* |

Configured when the URL and the API key are both set.

Sonarr keys its series on TVDB. Snagarr asks TMDB for the TVDB ID before it sends a series, so a Sonarr send fails without a TMDB key.

## Quality profiles and root folders

The settings UI offers the real choices. Read the same list from the API:

```sh
curl -G http://localhost:8080/api/v1/settings/options \
  -H "Authorization: Bearer sngr_your_admin_token" \
  --data-urlencode "service=radarr"
```

```json
{ "quality_profiles": [ { "id": 4, "name": "HD-1080p" } ],
  "root_folders":     [ { "path": "/movies", "free_space": 812340000000 } ] }
```

`service` accepts `radarr`, `sonarr` and `library`.

## Overseerr

Optional. Jellyseerr uses the same API.

| Field | Type | Default | Variable |
|-------|------|---------|----------|
| `overseerr.url` | string | *(empty)* | `SNAGARR_OVERSEERR_URL` |
| `overseerr.api_key` | string | *(empty)* | `SNAGARR_OVERSEERR_API_KEY` |

Configured when the URL and the API key are both set.

Snagarr mirrors the Overseerr request list into a local index on each reconcile pass. A title with a request shows the `REQUESTED` badge.

## After a send

Each reconcile pass replaces the Radarr index and the Sonarr index with one list call per service, then recomputes item status from those indexes. An item stays `monitored` until the file lands, then becomes `available`.

A Radarr or Sonarr webhook reports the import at once instead of waiting for the next pass. See [Webhooks](/snagarr/use/webhooks/).
