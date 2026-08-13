---
title: Notifications
description: Connect an ntfy topic as your own service and get one push when a snagged title becomes available.
---

ntfy is a [service](/snagarr/configure/services/). Every member connects their own topic. Snagarr sends one push when a snagged title becomes available, and never a second push for the same item.

## Config fields

| Field | Type | Default | Variable |
|-------|------|---------|----------|
| `url` | string | `https://ntfy.sh` | `SNAGARR_NTFY_URL` |
| `topic` | string | *(empty)* | `SNAGARR_NTFY_TOPIC` |
| `token` | string | *(empty)* | `SNAGARR_NTFY_TOKEN` |
| `priority` | integer | `3` | *(none)* |

Configured when the topic is set. An ntfy server needs no credential, so the topic is the whole requirement.

Set `token` for a server that needs authentication. Leave it empty for an open server.

`priority` must be 1 to 5. Any other value makes Snagarr leave the header out, so the ntfy server default applies. The card offers **Send at high priority**, which writes `4`. Set any other value with the API.

## Add one

1. Open **Settings**.
2. Choose `ntfy` under **Add service**. Select **Add**.
3. Fill in the topic.
4. Select **Test connection**. Snagarr calls `/v1/health` on the server.

```sh
curl -X POST http://localhost:8080/api/v1/services \
  -H "Authorization: Bearer sngr_your_token" \
  -H "Content-Type: application/json" \
  -d '{"kind":"ntfy","config":{"url":"https://ntfy.example.com","topic":"snagarr-amina","priority":4}}'
```

The environment variables seed the first admin's topic only. See [Environment variables](/snagarr/configure/environment/#service-seeding).

## Who gets the push

The push goes to the **capturer's** own ntfy. When the capturer has none, it goes to an admin's, because an unowned push still has to reach somebody.

Connect your own topic to be told about the titles you snagged.

## The push

It carries the capture context:

```
Sinners is ready — snagged by Amina, 12 Jul, from shortcut
```

The title is `Ready to watch`. It carries a click link when `general.public_url` is set.

Two events send a push: a Radarr or Sonarr import webhook, and a reconcile pass that finds an item became available.
