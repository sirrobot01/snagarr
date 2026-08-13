---
title: Notifications
description: Configure ntfy pushes for titles that become available, plus the state of the Telegram fields.
---

## ntfy

Optional. Snagarr sends one push when a snagged title becomes available, and never a second push for the same item.

| Field | Type | Default | Variable |
|-------|------|---------|----------|
| `ntfy.url` | string | `https://ntfy.sh` | `SNAGARR_NTFY_URL` |
| `ntfy.topic` | string | *(empty)* | `SNAGARR_NTFY_TOPIC` |
| `ntfy.token` | string | *(empty)* | `SNAGARR_NTFY_TOKEN` |
| `ntfy.priority` | integer | `3` | *(none)* |

Configured when the topic is set.

Set `ntfy.token` for a server that needs authentication. Leave it empty for an open server.

`ntfy.priority` must be 1 to 5. Any other value makes Snagarr leave the header out, so the ntfy server default applies.

The push carries the capture context:

```
Sinners is ready — snagged by Amina, 12 Jul, from telegram
```

It carries a click link when `general.public_url` is set.

Two events send a push: a Radarr or Sonarr import webhook, and a reconcile pass that finds an item became available.

`ntfy.token` and `ntfy.priority` have no card in the settings UI. Set them through the API:

```sh
curl -X PUT http://localhost:8080/api/v1/settings \
  -H "Authorization: Bearer sngr_your_admin_token" \
  -H "Content-Type: application/json" \
  -d '{"ntfy":{"url":"https://ntfy.example.com","topic":"snagarr-home","priority":4}}'
```

## Telegram

| Field | Type | Default | Variable |
|-------|------|---------|----------|
| `telegram.bot_token` | string | *(empty)* | `SNAGARR_TELEGRAM_BOT_TOKEN` |

The Telegram bot is not implemented. Nothing reads this field. Snagarr runs no bot, polls nothing and answers no message. User records accept a `telegram_user_id` that nothing reads either.

`GET /api/v1/settings` still reports the section as configured when a bot token is stored, and the **Telegram** row on the setup screen turns green. That state means only that the value is saved. `GET /api/v1/status` does not list Telegram.

There is no connection test for Telegram.
