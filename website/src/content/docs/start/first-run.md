---
title: First run
description: Create the first administrator in the browser, complete the setup wizard, then invite your household.
---

## Create the administrator

Start Snagarr, then open its web address. The default local address is:

```
http://localhost:8080
```

An empty installation opens the registration screen. Choose a username and
password. The first account is always an administrator. Registration closes as
soon as that account exists, so another visitor cannot create a second initial
administrator.

After registration, the browser receives a private session and takes you into
Snagarr. There is no setup secret in the server log and no credential in the
URL. On another browser, use the sign-in screen with the username and password
you created.

The browser session is stored locally on that device. Use **Sign out** from the
account menu before leaving a shared device.

## Setup wizard

Open `/setup` for the four-step wizard. Every step can be skipped and done later in Settings.

| Step | What you do | Required |
|------|-------------|----------|
| 1 | Paste a TMDB API key (v3) from [themoviedb.org](https://www.themoviedb.org/settings/api) | Yes. Without it, captures save but never resolve. |
| 2 | Add a media server: Plex, Emby or Jellyfin | No. Gives the library badges and the `Snagged` collection. |
| 3 | Add Radarr and Sonarr | No. Needed to send titles. |
| 4 | Review the connections and optionally create a client token | No. |

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
  -d '{"username":"amina","password":"temporary-password","role":"member"}'
```

Roles are `admin` and `member`. See the role table in the [Introduction](/snagarr/#roles). The last admin cannot be deleted or demoted; the API answers `409 conflict`. Deleting a user keeps their items and nulls the attribution, and removes their services with every index row those services own.

Every member connects their own Radarr, Sonarr, Overseerr, media server and ntfy:

1. Give the member the username and password you created for them.
2. The member signs in and goes to **Settings → My services**.
3. The member adds what they own.

A member with no service of a kind cannot send to that kind. Your services are not shared. See [Services](/snagarr/configure/services/).

## Tokens

A token belongs to one user. Every capture records that user, so the item list and the ntfy push name who snagged a title. Give each client its own token so you can revoke one client alone.

Four ways to get one:

| Source | Steps |
|--------|-------|
| Browser sign-in | Snagarr creates a browser session automatically; its secret is never displayed. |
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
