---
title: Media servers
description: Connect Plex, Emby or Jellyfin, choose the libraries to index and name the collection Snagarr maintains.
---

Snagarr reads one media server. It mirrors the contents into a local index and keeps one collection in sync there.

| Field | Type | Default | Variable |
|-------|------|---------|----------|
| `library.provider` | `plex` \| `emby` \| `jellyfin` | *(empty)* | `SNAGARR_LIBRARY_PROVIDER` |
| `library.url` | string | *(empty)* | `SNAGARR_LIBRARY_URL` |
| `library.token` | string | *(empty)* | `SNAGARR_LIBRARY_TOKEN` |
| `library.section_ids` | string list | *(empty)* | *(none)* |
| `library.collection_name` | string | `Snagged` | `SNAGARR_LIBRARY_COLLECTION` |

The section counts as configured when the provider, the URL and the token are all set. Any `provider` value outside the three names leaves it unconfigured.

## Credentials

| Provider | Credential | Header Snagarr sends | Example URL |
|----------|-----------|----------------------|-------------|
| `plex` | Plex token | `X-Plex-Token` | `http://plex.lan:32400` |
| `emby` | Emby API key | `X-Emby-Token` | `http://emby.lan:8096` |
| `jellyfin` | Jellyfin API key | `X-Emby-Token` | `http://jellyfin.lan:8096` |

## Libraries

`library.section_ids` limits the sync to named libraries. An empty list indexes every movie library and every show library. Snagarr always drops music and photo libraries.

After you save the URL and the token, the settings UI lists the real sections. It falls back to a free-text field of comma-separated IDs. Read the same list from the API:

```sh
curl -G http://localhost:8080/api/v1/settings/options \
  -H "Authorization: Bearer sngr_your_admin_token" \
  --data-urlencode "service=library"
```

```json
{ "sections": [ { "id": "1", "title": "Movies", "type": "movie" } ] }
```

An empty `library.section_ids` serialises as `null`, not `[]`.

## Collection

`library.collection_name` names the collection Snagarr maintains on the media server. The default is `Snagged`. An empty name turns the collection sync off.

Each reconcile pass makes the collection hold `(snagged ∩ available) − watched`. An import event adds a title; a playback event removes it. The collection is live state, not history.

## Sync behaviour

The media server sync is incremental on most reconcile passes and full once every 24 hours. Only a full sweep removes rows from the index, because only a full sweep can see a deleted title.

Playback events reach Snagarr through webhooks: Tautulli for Plex, the native notifier for Emby, the Webhook plugin for Jellyfin. See [Webhooks](/snagarr/use/webhooks/).
