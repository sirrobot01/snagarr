---
title: Media servers
description: Connect Plex, Emby or Jellyfin as your own service, choose the libraries to index and name the collection Snagarr maintains there.
---

A media server is a [service](/snagarr/configure/services/). Every member connects their own. Snagarr mirrors each one into a local index and keeps one collection in sync on it.

| Kind | Credential | Header Snagarr sends | Example URL |
|------|-----------|----------------------|-------------|
| `plex` | Plex token | `X-Plex-Token` | `http://plex.lan:32400` |
| `emby` | Emby API key | `X-Emby-Token` | `http://emby.lan:8096` |
| `jellyfin` | Jellyfin API key | `X-Emby-Token` | `http://jellyfin.lan:8096` |

## Config fields

| Field | Type | Default | Variable |
|-------|------|---------|----------|
| `url` | string | *(empty)* | `SNAGARR_LIBRARY_URL` |
| `token` | string | *(empty)* | `SNAGARR_LIBRARY_TOKEN` |
| `section_ids` | string list | *(empty)* | *(none)* |
| `collection_name` | string | `Snagged` | `SNAGARR_LIBRARY_COLLECTION` |

The service counts as configured when the URL and the token are both set.

The environment variables seed the first admin's service only, and `SNAGARR_LIBRARY_PROVIDER` picks the kind. See [Environment variables](/snagarr/configure/environment/#service-seeding).

## Add one

1. Open **Settings**.
2. Choose `plex`, `emby` or `jellyfin` under **Add service**. Select **Add**.
3. Fill in the URL and the token.
4. Select **Test connection**.

```sh
curl -X POST http://localhost:8080/api/v1/services \
  -H "Authorization: Bearer sngr_your_token" \
  -H "Content-Type: application/json" \
  -d '{"kind":"plex","config":{"url":"http://plex.lan:32400","token":"your_plex_token"}}'
```

## Sign in with Plex

A `plex` card offers **Sign in with Plex**, so you never have to hunt for an `X-Plex-Token`.

1. Select **Sign in with Plex**. A plex.tv window opens.
2. Approve the code shown in the dialog.
3. Choose one of your servers. Snagarr fills in the token.

Snagarr fills in the token only. The **URL** stays what you typed, because plex.tv reports the addresses your Plex account knows, not the one this Snagarr can reach. Each server is listed once, whatever the number of accounts it is shared with.

The flow uses three endpoints:

| Method | Path | Effect |
|--------|------|--------|
| `POST` | `/api/v1/plex/pin` | Starts a sign-in and returns `id`, `code`, `auth_url` and `expires_at` |
| `GET` | `/api/v1/plex/pin/{id}` | `202` while pending, `200` with the token once approved, `410` when expired |
| `GET` | `/api/v1/plex/servers?token=` | Lists the servers that token reaches, connections fastest first |

Pasting a token into the **Token** field does the same job. Emby and Jellyfin have no sign-in flow; paste an API key.

## Libraries

`section_ids` limits the sync to named libraries. An empty list indexes every movie library and every show library. Snagarr always drops music and photo libraries.

After you save the URL and the token, the card lists the real sections. It falls back to a free-text field of comma-separated IDs. Read the same list from the API:

```sh
curl http://localhost:8080/api/v1/services/3/options \
  -H "Authorization: Bearer sngr_your_token"
```

```json
{ "sections": [ { "id": "1", "title": "Movies", "type": "movie" } ] }
```

An empty `section_ids` serialises as `null`, not `[]`.

## Collection

`collection_name` names the collection Snagarr maintains on that server. The default is `Snagged`. An empty name turns the collection sync off for that service alone.

Collections are personal. Each media server gets its own collection, holding `(snagged ∩ available) − watched` limited to the titles **that** server has. A title only somebody else owns never appears in yours.

An import event adds a title; a playback event removes it. The collection is live state, not history.

## Household state

State is the union. A title reads **IN LIBRARY** when any enabled media server in the household holds it, whoever owns that server.

Disable a service to stop it answering. Delete one to drop its index rows with it. See [Services](/snagarr/configure/services/#union-state-personal-action).

## Sync behaviour

Each media server syncs incrementally on most reconcile passes and fully once every 24 hours. The two stamps are per service, so a server you connect today gets its own first full sweep. Only a full sweep removes rows, because only a full sweep can see a deleted title.

Playback events reach Snagarr through webhooks: Tautulli for Plex, the native notifier for Emby, the Webhook plugin for Jellyfin. See [Webhooks](/snagarr/use/webhooks/).
