---
title: Troubleshooting
description: Symptoms, causes and fixes for capture, webhook, settings, collection and deployment failures.
---

Set `SNAGARR_LOG_LEVEL=debug` and restart before you investigate anything. Debug logging names the payload or the upstream call that failed.

## Symptom index

| Symptom | Cause | Fix |
|---------|-------|-----|
| Captures stay `needs_review` forever | No TMDB key | Set `tmdb.api_key` |
| A send answers `503 not_configured` | You own no service of that kind | Add one under **Settings → My services** |
| API answers `403 forbidden` | A `member` token on an admin route, or another member's service | Use an admin token |
| API answers `400 bad_request` | An unknown field in the body | Send only the documented fields |
| `POST /api/v1/services` answers `409 conflict` | You already own that kind under that name | Give it a different name |
| A settings card is read-only | An environment variable pins a value in that section | Remove the variable and restart |
| A service card is read-only | An environment variable seeds that record on every start | Remove the variable and restart |
| A saved value reverts after a restart | The same environment override | Remove the variable |
| The UI returns 404, the API works | The binary was built without `internal/web/dist` | Run `task build`, not `go build` |
| The browser blocks every API call | Duplicate CORS headers at the proxy | Remove the CORS headers from the proxy |
| The app breaks under `/snagarr/` | The client calls `/api/v1` from the domain root | Serve Snagarr on its own host |
| `permission denied` on the data directory | The image runs as UID 65532 | `sudo chown -R 65532:65532 ./data` |
| A Sonarr send fails | Sonarr keys on TVDB, which Snagarr reads from TMDB | Set `tmdb.api_key` |
| No ntfy push | The capturer owns no ntfy and no admin does either, or the item already sent one | One push per item, ever |
| The collection stays empty | That service's `collection_name` is empty, or the server holds none of the titles | Set the name; a collection only names titles that server has |
| A title reads `IN LIBRARY` but is not in your library | Another member's server holds it | State is the household union |

## A webhook does nothing

A webhook that returns `204` and changes nothing has one of these causes. Work down the list.

1. **Wrong secret.** A wrong or missing `?secret=` returns `401`. Most senders log the status.
2. **Wrong service name.** An unknown name in the path returns `404`.
3. **The title is not snagged.** Webhooks only update existing items. They never create one.
4. **The payload carries no TMDB ID.** The most common cause. Sonarr must send `series.tmdbId`; older versions send only `series.tvdbId`.
5. **The media type does not match.** A movie snagged as a series never matches.
6. **The item is already `available` or `watched`.** An import webhook makes no change in that state.

Reproduce it by hand:

```sh
curl -i -X POST \
  "http://localhost:8080/api/v1/webhooks/radarr?secret=YOUR_SECRET" \
  -H "Content-Type: application/json" \
  -d '{"eventType":"Download","movie":{"tmdbId":1233413}}'
```

The reconcile loop corrects the state within one interval either way.

## A capture never resolves

1. Check `GET /api/v1/settings` for `"tmdb": {"configured": true}`.
2. Read the item: `GET /api/v1/items/{id}`. A `needs_review` item carries `candidates` with a `score` from 0 to 1.
3. Resolve it by hand: `POST /api/v1/items/{id}/resolve` with `tmdb_id` and `media_type`.

`raw_input` is never discarded, so nothing is lost while an item waits.

## State is stale

Item status comes from the local indexes, not from a live call. Force a pass:

```sh
curl -X POST http://localhost:8080/api/v1/admin/sync \
  -H "Authorization: Bearer sngr_your_admin_token"
```

Watch `sync.running` and the `sync` timestamps in `GET /api/v1/status`. A timestamp of `null` means that sync has never run.

A deleted title survives in the index until the next full media server sweep, which runs once every 24 hours.

## A connection test fails

`POST /api/v1/services/{id}/test` shows the upstream message unchanged. `POST /api/v1/settings/test` does the same for TMDB, and accepts no other name.

| Message | Cause |
|---------|-------|
| `401 — check token` | Wrong API key or media server token |
| A connection error | Wrong URL, wrong port, or no route from the container |
| `404` | A URL with a trailing path, or the wrong service on that port |

The test reads the **stored** record. Save the card before you test it; the UI does that for you.

Use the address the Snagarr container can reach, not the one your browser uses. `http://localhost:7878` inside a container is the container itself.

## A send goes to the wrong server

Sending is personal. Snagarr uses the service the **caller** owns.

1. Read your own stack: `GET /api/v1/services`.
2. An admin reads a member's stack: `GET /api/v1/users/{id}/services`.
3. Check `enabled` and the config on the row you expected.

A member never falls through to an admin's service. An admin falls through to their own, then the capturer's, then another admin's.

## A lost client token

Snagarr stores only the SHA-256 digest of a token, so a lost value cannot be
recovered. Issue a replacement from the household settings or with an admin
token: `POST /api/v1/users/{id}/tokens`. Browser sessions are created by signing
in again and are not copied into client configuration.

## A lost `secret.key`

Every stored setting and every service config is encrypted with `<data dir>/secret.key`. Without that file, no credential can be read. Set every service again, or restore the key from a backup. Items, users and tokens are not encrypted and survive.

## A settings value will not stay

Environment variables are re-applied at start-up and after every save. `PUT /api/v1/settings` writes your value to the database, and the variable goes back on top. Remove the variable to make the stored value take effect.

Snagarr ignores an empty variable, an unparsable integer and an unparsable duration, so a typo in `SNAGARR_RECONCILE_INTERVAL` silently keeps the stored interval. Check the resolved value with `GET /api/v1/settings`.

## Health and version

```sh
curl http://localhost:8080/api/v1/health
# {"status":"ok","version":"0.1.0"}
```

The image is distroless and holds no shell and no `curl`, so an in-container `HEALTHCHECK` is not possible. Probe from outside the container.
