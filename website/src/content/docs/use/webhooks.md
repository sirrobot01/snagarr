---
title: Webhooks
description: Point Radarr, Sonarr, Tautulli, Emby and Jellyfin at Snagarr so imports and watches apply at once instead of on the next reconcile pass.
---

A webhook applies a change at once. Without one, the reconcile loop finds the same change within `general.reconcile_interval`, 15 minutes by default. Snagarr is correct with no webhook configured, only slower.

Snagarr acts on two kinds of event:

| Kind | Sender | Effect |
|------|--------|--------|
| Import | Radarr, Sonarr | Marks the item `available`, sends the ntfy push, adds the title to every collection whose server holds it |
| Playback | Tautulli, Emby, Jellyfin | Marks the item `watched`, removes the title from the collections |

## URL

```
POST <public url>/api/v1/webhooks/<service>
```

| `<service>` | Sender |
|-------------|--------|
| `radarr` | Radarr |
| `sonarr` | Sonarr |
| `tautulli` | Tautulli, for Plex |
| `emby` | Emby |
| `jellyfin` | Jellyfin |

`emby` and `jellyfin` run the same handler; either name works for either server.

```
https://snagarr.example.com/api/v1/webhooks/radarr
```

| Status | Meaning |
|--------|---------|
| `204 No Content` | The payload was read. Also the answer when nothing matched. |
| `401 Unauthorized` | The credential is missing, wrong, or revoked. |
| `404 Not Found` | The service name in the path is unknown. |

Snagarr answers `204` for a payload it cannot use, so a sender never retries.

## Authentication

A webhook authenticates as a household member. There is no separate webhook secret. Snagarr accepts three forms:

| Form | What to send | Senders |
|------|--------------|---------|
| Username and password | The member's sign-in details, as HTTP basic authentication | Radarr, Sonarr |
| Token | `Authorization: Bearer sngr_…` | Tautulli, Jellyfin, and any sender that sets headers |
| Token | `?token=sngr_…` on the URL | Emby, and any sender that sets no header |

Give the webhooks their own token. You then revoke that one token to stop them, and the member keeps their other clients. See [Tokens](/snagarr/start/first-run/#tokens).

A revoked token, a wrong password and an unknown username all return `401`. Ten failed username-and-password attempts in fifteen minutes pause password checks for webhooks; further attempts answer `401` until the window clears. Tokens are not limited.

:::caution[Use HTTPS]
Basic authentication and a bearer token both travel in clear text over HTTP. On a local network that is the same exposure as the old shared secret. Across the internet, put Snagarr behind TLS.
:::

## Radarr

1. Open Radarr. Go to **Settings → Connect**.
2. Select **+**. Choose **Webhook**.
3. Set **Name** to `Snagarr`.
4. Turn on **On Import**, and **On Import Complete** if your version has it.
5. Set **URL** to `<public url>/api/v1/webhooks/radarr`.
6. Set **Method** to `POST`.
7. Set **Username** to a household username.
8. Set **Password** to that member's password.
9. Select **Test**. Radarr must report success.
10. Select **Save**.

Leave the other triggers off. Snagarr ignores them.

## Sonarr

Follow the Radarr steps with the URL `<public url>/api/v1/webhooks/sonarr`.

Sonarr must send `series.tmdbId`. Older versions send only `series.tvdbId`; Snagarr then does nothing with the webhook and the reconcile loop marks the episode available on its next pass.

## Tautulli

1. Open Tautulli. Go to **Settings → Notification Agents**.
2. Select **Add a new notification agent**. Choose **Webhook**.
3. Set **Webhook URL** to `<public url>/api/v1/webhooks/tautulli`.
4. Set **Webhook Method** to `POST`.
5. Set **Webhook Headers** to `{"Authorization": "Bearer sngr_your_token"}`.
6. Open the **Triggers** tab. Turn on **Watched**, and nothing else.
7. Open the **Data** tab. Open the **Watched** section.
8. Set **JSON Data** to the body below.
9. Select **Save**.

```json
{
  "media_type": "{media_type}",
  "tmdb_id": "{themoviedb_id}"
}
```

Check both parameter names against the list Tautulli shows under the JSON Data field. They change between versions.

:::caution
The template above carries no event name, so any payload Snagarr can match marks the title watched. Turn on **Watched** only. Snagarr ignores a payload that names a start, pause, resume or progress event, but that backstop only works when the payload carries the event name.
:::

## Emby

1. Open Emby. Go to **Settings → Notifications**.
2. Select **Add Notification**. Choose **Webhooks**.
3. Set **URL** to `<public url>/api/v1/webhooks/emby?token=sngr_your_token`.
4. Set **Request content type** to `application/json`.
5. Turn on the playback-stop event only.
6. Limit the events to your movie and show libraries.
7. Select **Save**.

Emby sends `Item.ProviderIds.Tmdb` and `Item.Type`, which is what Snagarr reads. No template is needed. A title with no TMDB provider ID has no effect, and still answers `204`.

## Jellyfin

Jellyfin needs the **Webhook** plugin, which builds its payload from a template.

1. Install the Webhook plugin. Restart Jellyfin.
2. Go to **Dashboard → Plugins → Webhook**.
3. Select **Add Generic Destination**.
4. Set **Webhook URL** to `<public url>/api/v1/webhooks/jellyfin`. Add the header `Authorization: Bearer sngr_your_token`, or put `?token=sngr_your_token` on the URL if your plugin version sets no headers.
5. Turn on **Playback Stop** only.
6. Turn on the **Movies** and **Episodes** item types.
7. Set the template to the body below.
8. Select **Save**.

```json
{
  "media_type": "{{ItemType}}",
  "tmdb_id": "{{Provider_tmdb}}"
}
```

Check both variable names against the list the plugin shows. They change between plugin versions.

## Fields Snagarr reads

Every other field is ignored.

### Import payloads

| Field | Sender | Use |
|-------|--------|-----|
| `eventType` | Radarr, Sonarr | The event filter |
| `movie.tmdbId` | Radarr | Title identity, always a movie |
| `series.tmdbId` | Sonarr | Title identity, always a series |

Snagarr acts on four event types and ignores the rest: `Download` · `MovieFileImported` · `EpisodeFileImported` · `Import`.

### Playback payloads

| Field | Sender | Use |
|-------|--------|-----|
| `tmdb_id` | Tautulli, Jellyfin template | Title identity, read first |
| `Item.ProviderIds.Tmdb` | Emby, Jellyfin | Title identity, if `tmdb_id` is absent |
| `media_type` | Tautulli, Jellyfin template | Media type |
| `Item.Type` | Emby, Jellyfin | Media type, if `media_type` is absent |
| `event`, `action`, `NotificationType` | Emby, Jellyfin plugin, and any sender that names its events | The event filter |

Under `Item.ProviderIds`, Snagarr accepts the keys `Tmdb`, `tmdb` and `TMDB`. The title is a series when `media_type` is `show` or `episode`, or when `Item.Type` is `Episode` or `Series`. Otherwise it is a movie.

Snagarr ignores a payload whose `event`, `action` or `NotificationType` names a start, pause, resume or progress event, in any sender's spelling (`playback.start`, `PlaybackStart`, `media.play`, `pause`). Any other event name, or no event name at all, counts as a watch.

## What each webhook changes

Import:

1. Find the snagged item with that TMDB ID and media type. No match, no effect.
2. Already `available` or `watched`, no effect.
3. Otherwise set the item to `available` and stamp `available_at`.
4. Send one ntfy push to the capturer's topic, or to an admin's when the capturer has none. Never a second push for the same item.
5. Start a reconcile pass, which adds the title to every collection whose server holds it.

Playback:

1. Find the snagged item with that TMDB ID and media type. No match, no effect.
2. Record a watch event with the sender name as the source.
3. Set the item to `watched`.
4. Start a reconcile pass, which removes the title from every collection.

## Test

```sh
curl -i -X POST \
  "http://localhost:8080/api/v1/webhooks/radarr" \
  -u "your-username:your-password" \
  -H "Content-Type: application/json" \
  -d '{"eventType":"Download","movie":{"tmdbId":1233413}}'
```

```sh
curl -i -X POST \
  "http://localhost:8080/api/v1/webhooks/emby?token=sngr_your_token" \
  -H "Content-Type: application/json" \
  -d '{"Item":{"Type":"Movie","ProviderIds":{"Tmdb":"1233413"}}}'
```

A correct credential returns `204`, a wrong one `401`. Check the effect with `GET /api/v1/items`. A webhook that returns `204` and changes nothing is covered in [Troubleshooting](/snagarr/reference/troubleshooting/#a-webhook-does-nothing).
