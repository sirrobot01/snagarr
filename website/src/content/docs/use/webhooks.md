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
POST <public url>/api/v1/webhooks/<service>?secret=<webhook secret>
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
https://snagarr.example.com/api/v1/webhooks/radarr?secret=8f3c1a9e5d2b740c6e1f8a3d9b0c2e47
```

These routes take no bearer token. The secret travels in the query string, because Radarr, Tautulli and Emby cannot all set request headers.

| Status | Meaning |
|--------|---------|
| `204 No Content` | The payload was read. Also the answer when nothing matched. |
| `401 Unauthorized` | The `secret` parameter is missing or wrong. |
| `404 Not Found` | The service name in the path is unknown. |

Snagarr answers `204` for a payload it cannot use, so a sender never retries.

## The secret

Snagarr generates 32 hexadecimal characters on first start and stores them as `general.webhook_secret`. It returns in clear text, unlike every other secret. It has no `SNAGARR_*` override.

Read it in **Settings → General**, which also prints the finished webhook URL beside it. Or read it from the API:

```sh
curl -s http://localhost:8080/api/v1/settings \
  -H "Authorization: Bearer sngr_your_admin_token"
```

```json
{ "general": { "reconcile_interval": "15m0s", "public_url": "",
               "webhook_secret": "8f3c1a9e5d2b740c6e1f8a3d9b0c2e47" } }
```

To replace it:

1. Make a value: `openssl rand -hex 16`.
2. Write it:

   ```sh
   curl -X PUT http://localhost:8080/api/v1/settings \
     -H "Authorization: Bearer sngr_your_admin_token" \
     -H "Content-Type: application/json" \
     -d '{"general":{"webhook_secret":"8f3c1a9e5d2b740c6e1f8a3d9b0c2e47"}}'
   ```

3. Update every webhook URL.

The new secret applies at once. Do not set it to an empty string: Snagarr then rejects every webhook until the next start-up generates a replacement.

## Radarr

1. Open Radarr. Go to **Settings → Connect**.
2. Select **+**. Choose **Webhook**.
3. Set **Name** to `Snagarr`.
4. Turn on **On Import**, and **On Import Complete** if your version has it.
5. Set **URL** to `<public url>/api/v1/webhooks/radarr?secret=<secret>`.
6. Set **Method** to `POST`.
7. Select **Test**. Radarr must report success.
8. Select **Save**.

Leave the other triggers off. Snagarr ignores them.

## Sonarr

Follow the Radarr steps with the URL `<public url>/api/v1/webhooks/sonarr?secret=<secret>`.

Sonarr must send `series.tmdbId`. Older versions send only `series.tvdbId`; Snagarr then does nothing with the webhook and the reconcile loop marks the episode available on its next pass.

## Tautulli

1. Open Tautulli. Go to **Settings → Notification Agents**.
2. Select **Add a new notification agent**. Choose **Webhook**.
3. Set **Webhook URL** to `<public url>/api/v1/webhooks/tautulli?secret=<secret>`.
4. Set **Webhook Method** to `POST`.
5. Open the **Triggers** tab. Turn on **Watched**, and nothing else.
6. Open the **Data** tab. Open the **Watched** section.
7. Set **JSON Data** to the body below.
8. Select **Save**.

```json
{
  "media_type": "{media_type}",
  "tmdb_id": "{themoviedb_id}"
}
```

Check both parameter names against the list Tautulli shows under the JSON Data field. They change between versions.

:::caution
Snagarr reads no event name from a playback payload. Any payload it can match marks the title watched. Do not turn on **Playback Start**.
:::

## Emby

1. Open Emby. Go to **Settings → Notifications**.
2. Select **Add Notification**. Choose **Webhooks**.
3. Set **URL** to `<public url>/api/v1/webhooks/emby?secret=<secret>`.
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
4. Set **Webhook URL** to `<public url>/api/v1/webhooks/jellyfin?secret=<secret>`.
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

Under `Item.ProviderIds`, Snagarr accepts the keys `Tmdb`, `tmdb` and `TMDB`. The title is a series when `media_type` is `show` or `episode`, or when `Item.Type` is `Episode` or `Series`. Otherwise it is a movie. Snagarr reads neither the `event` field nor the `action` field.

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
  "http://localhost:8080/api/v1/webhooks/radarr?secret=YOUR_SECRET" \
  -H "Content-Type: application/json" \
  -d '{"eventType":"Download","movie":{"tmdbId":1233413}}'
```

```sh
curl -i -X POST \
  "http://localhost:8080/api/v1/webhooks/emby?secret=YOUR_SECRET" \
  -H "Content-Type: application/json" \
  -d '{"Item":{"Type":"Movie","ProviderIds":{"Tmdb":"1233413"}}}'
```

A correct secret returns `204`, a wrong one `401`. Check the effect with `GET /api/v1/items`. A webhook that returns `204` and changes nothing is covered in [Troubleshooting](/snagarr/reference/troubleshooting/#a-webhook-does-nothing).
