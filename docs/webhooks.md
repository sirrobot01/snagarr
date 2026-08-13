# Webhooks

Webhooks tell Snagarr about a change at once. Without them, Snagarr finds the
same change on its next reconcile pass. The default interval is 15 minutes.

> Webhooks are an accelerator. The reconcile loop is the backstop. Snagarr
> works correctly with no webhook configured. It is only slower to react.

Snagarr acts on two kinds of event:

- **Import.** Radarr or Sonarr finished a download. Snagarr marks the item
  available. It sends the ntfy push. It adds the title to the collection.
- **Playback.** Somebody watched a title. Snagarr marks the item watched. It
  removes the title from the collection.

## URL shape

Every webhook uses the same shape:

```
POST <public url>/api/v1/webhooks/<service>?secret=<webhook secret>
```

`<service>` is one of:

| Service | Sender |
|---------|--------|
| `radarr` | Radarr |
| `sonarr` | Sonarr |
| `tautulli` | Tautulli, for Plex |
| `emby` | Emby |
| `jellyfin` | Jellyfin |

`emby` and `jellyfin` run the same handler. Either name works for either
server.

Example:

```
https://snagarr.example.com/api/v1/webhooks/radarr?secret=8f3c1a9e5d2b740c6e1f8a3d9b0c2e47
```

The webhook routes take no bearer token. Radarr, Tautulli and Emby cannot all
set request headers, so the secret travels in the query string.

## The webhook secret

Snagarr generates the secret on first start. It is 32 hexadecimal characters.
It is stored as `general.webhook_secret`.

> The settings UI does not show the secret. `GET /api/v1/settings` masks it
> like every other secret, so you see only the last four characters. Set your
> own value instead.

1. Make a value: `openssl rand -hex 16`.
2. Write it with an admin token:

   ```sh
   curl -X PUT http://localhost:8080/api/v1/settings \
     -H "Authorization: Bearer sngr_your_admin_token" \
     -H "Content-Type: application/json" \
     -d '{"general":{"webhook_secret":"8f3c1a9e5d2b740c6e1f8a3d9b0c2e47"}}'
   ```

3. Use that value in every webhook URL.

The new secret takes effect at once. No restart is needed.

Do not set the secret to an empty string. Snagarr then rejects every webhook.
It generates a replacement only at the next start-up.

The secret is not an environment variable. It has no `SNAGARR_*` override.

## Responses

| Status | Meaning |
|--------|---------|
| `204 No Content` | The payload was read. This is also the answer when nothing matched. |
| `401 Unauthorized` | The `secret` parameter is missing or wrong. |
| `404 Not Found` | The service name in the path is unknown. |

Snagarr answers `204` for a payload it cannot use. A sender that sees an error
retries, and none of these events is worth a retry storm. The reconcile loop
finds anything a webhook missed.

Read the Snagarr log at `debug` level to see why a payload did nothing. Set
`SNAGARR_LOG_LEVEL=debug`.

## Radarr

Radarr tells Snagarr that a movie file landed.

1. Open Radarr. Go to **Settings → Connect**.
2. Select **+**. Choose **Webhook**.
3. Set **Name** to `Snagarr`.
4. Turn on **On Import**. Turn on **On Import Complete** if your version has it.
5. Set **URL** to `<public url>/api/v1/webhooks/radarr?secret=<secret>`.
6. Set **Method** to `POST`.
7. Select **Test**. Radarr must report success.
8. Select **Save**.

Leave the other triggers off. Snagarr ignores them. They are harmless if you
turn them on.

## Sonarr

Sonarr tells Snagarr that an episode file landed.

Follow the Radarr steps. Change the URL to
`<public url>/api/v1/webhooks/sonarr?secret=<secret>`.

Sonarr must send `series.tmdbId` in the payload. Older versions send only
`series.tvdbId`. Snagarr then does nothing with the webhook, and the reconcile
loop marks the episode available on its next pass.

## Tautulli

Tautulli tells Snagarr that somebody watched a title on Plex.

1. Open Tautulli. Go to **Settings → Notification Agents**.
2. Select **Add a new notification agent**. Choose **Webhook**.
3. Set **Webhook URL** to `<public url>/api/v1/webhooks/tautulli?secret=<secret>`.
4. Set **Webhook Method** to `POST`.
5. Open the **Triggers** tab. Turn on **Watched**.
6. Open the **Data** tab. Open the **Watched** section.
7. Set **JSON Data** to the body below.
8. Select **Save**.

```json
{
  "media_type": "{media_type}",
  "tmdb_id": "{themoviedb_id}"
}
```

Check both parameter names against the list Tautulli shows under the JSON Data
field. The names change between Tautulli versions.

> Snagarr does not read the event name from a playback payload. Any payload it
> can match marks the title watched. Turn on the **Watched** trigger only. Do
> not turn on **Playback Start**.

## Emby

Emby tells Snagarr that somebody watched a title.

1. Open Emby. Go to **Settings → Notifications**.
2. Select **Add Notification**. Choose **Webhooks**.
3. Set **URL** to `<public url>/api/v1/webhooks/emby?secret=<secret>`.
4. Set **Request content type** to `application/json`.
5. Turn on the playback-stop event only.
6. Limit the events to your movie and show libraries.
7. Select **Save**.

Emby sends `Item.ProviderIds.Tmdb` and `Item.Type`, which is what Snagarr
reads. No template is needed.

A title with no TMDB provider ID in Emby produces no effect. Snagarr still
answers `204`.

## Jellyfin

Jellyfin needs the **Webhook** plugin. Its payload is built from a template, so
you must produce the fields Snagarr reads.

1. Install the Webhook plugin. Restart Jellyfin.
2. Go to **Dashboard → Plugins → Webhook**.
3. Select **Add Generic Destination**.
4. Set **Webhook URL** to `<public url>/api/v1/webhooks/jellyfin?secret=<secret>`.
5. Turn on **Playback Stop** only.
6. Turn on the **Movies** item type. Turn on the **Episodes** item type.
7. Set the template to the body below.
8. Select **Save**.

```json
{
  "media_type": "{{ItemType}}",
  "tmdb_id": "{{Provider_tmdb}}"
}
```

Check both variable names against the list the plugin shows. They change
between plugin versions.

## What Snagarr reads

Snagarr reads a small subset of each payload. It ignores every other field.

### Import payloads

| Field | Read from | Used for |
|-------|-----------|----------|
| `eventType` | Radarr, Sonarr | The event filter. |
| `movie.tmdbId` | Radarr | The title identity. |
| `series.tmdbId` | Sonarr | The title identity. |

Snagarr acts on four event types. It ignores every other one:

`Download` · `MovieFileImported` · `EpisodeFileImported` · `Import`

A Radarr payload is always treated as a movie. A Sonarr payload is always
treated as a series.

### Playback payloads

| Field | Read from | Used for |
|-------|-----------|----------|
| `tmdb_id` | Tautulli, Jellyfin template | The title identity. |
| `Item.ProviderIds.Tmdb` | Emby, Jellyfin | The title identity, if `tmdb_id` is absent. |
| `media_type` | Tautulli, Jellyfin template | The media type. |
| `Item.Type` | Emby, Jellyfin | The media type, if `media_type` is absent. |

Snagarr reads `tmdb_id` first. It falls back to `Item.ProviderIds`, where it
accepts the keys `Tmdb`, `tmdb` and `TMDB`.

The title is a series when `media_type` is `show` or `episode`, or when
`Item.Type` is `Episode` or `Series`. Otherwise it is a movie.

Snagarr does **not** read the `event` field or the `action` field. It applies
every payload it can match.

## What each webhook changes

### Import webhook

1. Snagarr finds the snagged item with that TMDB ID and media type.
2. If no item matches, nothing happens.
3. If the item is already `available` or `watched`, nothing happens.
4. Otherwise Snagarr sets the item to `available`. It stamps `available_at`.
5. Snagarr sends one ntfy push, if ntfy is configured. It never sends a second
   push for the same item.
6. Snagarr starts a reconcile pass. That pass adds the title to the collection.

### Playback webhook

1. Snagarr finds the snagged item with that TMDB ID and media type.
2. If no item matches, nothing happens.
3. Otherwise Snagarr records a watch event. It stores the sender name as the
   source.
4. Snagarr sets the item to `watched`.
5. Snagarr starts a reconcile pass. That pass removes the title from the
   collection.

The collection always holds `(snagged ∩ available) − watched`. It is live
state, never history.

## Test a webhook

Send a payload by hand:

```sh
curl -i -X POST \
  "http://localhost:8080/api/v1/webhooks/radarr?secret=YOUR_SECRET" \
  -H "Content-Type: application/json" \
  -d '{"eventType":"Download","movie":{"tmdbId":1233413}}'
```

A correct secret returns `204`. A wrong secret returns `401`.

Test a playback webhook the same way:

```sh
curl -i -X POST \
  "http://localhost:8080/api/v1/webhooks/emby?secret=YOUR_SECRET" \
  -H "Content-Type: application/json" \
  -d '{"Item":{"Type":"Movie","ProviderIds":{"Tmdb":"1233413"}}}'
```

Check the result with `GET /api/v1/items`. The item status must change.

## If a webhook does nothing

Work through this list:

1. Check the secret. A wrong secret returns `401`. Most senders log that status.
2. Check the service name in the path. An unknown name returns `404`.
3. Check that the title is snagged in Snagarr. Webhooks only update existing
   items. They never create one.
4. Check that the payload carries a TMDB ID. This is the most common cause.
5. Check that the media type matches. A movie snagged as a series never
   matches.
6. Set `SNAGARR_LOG_LEVEL=debug`. Restart Snagarr. Send the event again.

The reconcile loop corrects the state either way, within one interval.
