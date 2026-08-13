---
title: First run
description: Read the admin token, open the setup URL, complete the wizard, then issue and revoke tokens.
---

## The admin token

At first start Snagarr creates the admin user, issues one token and prints both to standard output:

```
  Snagarr is ready. Open the setup URL to finish:

    http://localhost:8080/#token=sngr_a1b2c3d4e5f6g7h8i9j0k1l2m3n4o5p6

  Admin token: sngr_a1b2c3d4e5f6g7h8i9j0k1l2m3n4o5p6
  Keep it safe — it is not shown again.
```

| Deployment | Read the token with |
|------------|---------------------|
| Compose | `docker compose logs snagarr` |
| Plain Docker | `docker logs snagarr` |
| systemd | `sudo journalctl -u snagarr` |

Snagarr stores only the SHA-256 digest of a token. The raw value is printed once and cannot be recovered. Issue a replacement from the settings UI if you lose it.

The setup URL uses `general.public_url` when that setting is set. Otherwise it uses the listen address, with `localhost` in place of an empty host.

## Open the web app

1. Open the setup URL. The client reads the token from the URL fragment, saves it and removes it from the URL.
2. The app opens on the **Snag** screen.

Without a token, the app shows a token box. Paste a token there. It is kept in browser local storage under the key `snagarr.token`.

To change the token later, open `<public url>/#token=<new token>`.

## Setup wizard

Open `/setup` for the four-step wizard. Every step can be skipped and done later in Settings.

| Step | What you do | Required |
|------|-------------|----------|
| 1 | Paste a TMDB API key (v3) from [themoviedb.org](https://www.themoviedb.org/settings/api) | Yes. Without it, captures save but never resolve. |
| 2 | Add a media server: Plex, Emby or Jellyfin | No. Gives the library badges and the `Snagged` collection. |
| 3 | Add Radarr and Sonarr | No. Needed to send titles. |
| 4 | Take a household token | No. |

Step 1 writes a setting. Steps 2 and 3 create [services](/snagarr/configure/services/) you own. A service must exist before it can be tested, so each card saves itself.

Each card has a **Test connection** button. It shows the upstream message unchanged, for example `OK · 611 items` or `401 — check token`.

Step 4 lists what the household as a whole can reach. It reads `GET /api/v1/status`, so a service another member connected also counts.

ntfy is not a wizard step. Add it in Settings. See [Notifications](/snagarr/configure/notifications/).

## Household members

Add a member in the settings page, or through the API:

```sh
curl -X POST http://localhost:8080/api/v1/users \
  -H "Authorization: Bearer sngr_your_admin_token" \
  -H "Content-Type: application/json" \
  -d '{"display_name":"Amina","role":"member"}'
```

Roles are `admin` and `member`. See the role table in the [Introduction](/snagarr/#roles). The last admin cannot be deleted or demoted; the API answers `409 conflict`. Deleting a user keeps their items and nulls the attribution, and removes their services with every index row those services own.

Every member connects their own Radarr, Sonarr, Overseerr, media server and ntfy:

1. Give the member a token.
2. The member opens the app and goes to **Settings → My services**.
3. The member adds what they own.

A member with no service of a kind cannot send to that kind. Your services are not shared. See [Services](/snagarr/configure/services/).

## Tokens

A token belongs to one user. Every capture records that user, so the item list and the ntfy push name who snagged a title. Give each client its own token so you can revoke one client alone.

Four ways to get one:

| Source | Steps |
|--------|-------|
| First run | Read the admin token from the log. |
| Setup wizard | Last step, **Create a household token**. |
| Settings page | **Generate bookmarklet** issues a token named `Bookmarklet` inside the generated code. |
| API | `POST /api/v1/users/{id}/tokens` |

```sh
curl -X POST http://localhost:8080/api/v1/users/1/tokens \
  -H "Authorization: Bearer sngr_your_admin_token" \
  -H "Content-Type: application/json" \
  -d '{"name":"iPhone Shortcut"}'
```

```json
{ "id": 7, "name": "iPhone Shortcut", "token": "sngr_…", "created_at": "…" }
```

This response is the only time the raw token is readable.

### Revoke

```sh
curl -X DELETE http://localhost:8080/api/v1/tokens/7 \
  -H "Authorization: Bearer sngr_your_admin_token"
```

To revoke every token of one member, open the settings page, find the member in the household table and select **Revoke**.

`GET /api/v1/users/{id}/tokens` lists the name, the prefix and the last use. It never returns the token.
