---
title: Services
description: Every integration belongs to one household member. State is the household union; sending, collections and pushes are personal.
---

A service is one member's connection to one external server. Every integration except TMDB is a service.

Each row in the `services` table carries an owner, a kind, a name, an encrypted config document and an `enabled` flag. The owner never changes: a service belongs to whoever created it.

TMDB is not a service. One key serves the whole install. See [Settings](/snagarr/configure/settings/).

## Kinds

| Kind | Configured when | Page |
|------|-----------------|------|
| `plex` | `url` and `token` are set | [Media servers](/snagarr/configure/media-servers/) |
| `emby` | `url` and `token` are set | [Media servers](/snagarr/configure/media-servers/) |
| `jellyfin` | `url` and `token` are set | [Media servers](/snagarr/configure/media-servers/) |
| `radarr` | `url` and `api_key` are set | [Radarr and Sonarr](/snagarr/configure/radarr-sonarr/) |
| `sonarr` | `url` and `api_key` are set | [Radarr and Sonarr](/snagarr/configure/radarr-sonarr/) |
| `overseerr` | `url` and `api_key` are set | [Radarr and Sonarr](/snagarr/configure/radarr-sonarr/#overseerr) |
| `ntfy` | `topic` is set | [Notifications](/snagarr/configure/notifications/) |

Any other kind is `400 bad_request`.

An unconfigured service is a normal state, not a fault. Snagarr leaves it out of the household and logs nothing.

## Who owns what

| Rule | Effect |
|------|--------|
| A member creates a service | The caller owns it |
| A member reads, edits, tests or deletes a service | Their own only. Anything else is `403 forbidden` |
| An admin reads, edits, tests or deletes a service | Anybody's |
| Two services of one kind, one member | Allowed. Give them different names |
| Two services of one kind and one name, one member | `409 conflict` |

A new service is named `Default` when you send no name.

## Union state, personal action

State is the household union. A title reads **IN LIBRARY** when *any* member's media server holds it. The same rule covers Radarr, Sonarr and Overseerr.

| Question | Answer comes from |
|----------|-------------------|
| Is this title in the library? | Every enabled media server in the household |
| Is this title monitored? | Every enabled Radarr and Sonarr. The `monitored` and `has_file` flags are ORed |
| Is this title requested? | Every enabled Overseerr. One fulfilled request settles the title |

Actions are personal.

| Action | Service it uses |
|--------|-----------------|
| Send to Radarr, Sonarr or Overseerr | The caller's own |
| Snagged collection | Each media server gets its own, holding only titles that server has |
| Availability push | The capturer's ntfy. An admin's ntfy when the capturer has none |

:::caution[A member may only send to their own services]
Snagarr blocks the fall-through to an admin's Radarr on purpose. A member with no Radarr of their own gets `503 not_configured`, not the admin's. Anybody holding a token could otherwise push to it.
:::

An admin sending falls through in this order: their own service, then the capturer's, then another admin's.

## Disable and delete

Disable a service to stop it answering for the household. Every index read joins `services` and filters on `enabled = 1`. The settings UI still lists it.

Delete a service to remove its index rows with it. Both actions start a reconcile pass at once.

## Endpoints

| Method | Path | Who |
|--------|------|-----|
| `GET` | `/api/v1/services` | The caller's own services, disabled ones included |
| `POST` | `/api/v1/services` | Creates one owned by the caller |
| `PATCH` | `/api/v1/services/{id}` | The owner, or any admin |
| `DELETE` | `/api/v1/services/{id}` | The owner, or any admin |
| `POST` | `/api/v1/services/{id}/test` | The owner, or any admin |
| `GET` | `/api/v1/services/{id}/options` | The owner, or any admin |
| `GET` | `/api/v1/users/{id}/services` | Admin only |

```sh
curl -X POST http://localhost:8080/api/v1/services \
  -H "Authorization: Bearer sngr_your_token" \
  -H "Content-Type: application/json" \
  -d '{"kind":"radarr","name":"Mine","config":{"url":"http://radarr.lan:7878","api_key":"your_key"}}'
```

`POST` merges your config over the kind's defaults. `PATCH` merges it over the stored document. An omitted field keeps its value.

Full request and response shapes: [HTTP API](/snagarr/reference/api/#services).

## Secrets

Snagarr encrypts every service config with `<data dir>/secret.key`, the same key that protects the settings blob.

Two field names are masked in every response: `api_key` and `token`. They come back as `••••` plus the last four characters. Send a masked value back unchanged to keep the stored secret.

## In the web app

Open **Settings**. The **My services** section lists your own stack.

1. Choose a kind under **Add service**. Select **Add**.
2. Fill in the card.
3. Select **Test connection**. Snagarr saves your edits first, then tests the stored record.

The card shows the upstream message unchanged, for example `OK · 611 items` or `401 — check token`.

### See another member's services

1. Open **Settings** as an admin.
2. Go to **Household & tokens**.
3. Select **Services** on the member's row.

The panel lists the kind, the name, the address and the state. It calls `GET /api/v1/users/{id}/services`.

## Services the environment owns

`SNAGARR_RADARR_URL` and its siblings seed the **first admin's** services on every start. Those services come back with `"locked": true`, and the UI renders their fields read-only.

See [Environment variables](/snagarr/configure/environment/#service-seeding).
