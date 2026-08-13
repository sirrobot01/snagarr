# Capture clients

The API is the product. Every client is a thin front-end over one endpoint:
`POST /api/v1/capture`.

This page shows how to use each client that exists today. See
[api.md](api.md) for the full API.

| Client | State | Where it runs |
|--------|-------|---------------|
| Web app | Works | Any browser. |
| Apple Shortcut | Works, built by hand | iOS, iPadOS, macOS. |
| Bookmarklet | Works on the Snagarr origin only | Desktop browser. |
| `curl` and scripts | Works | Anywhere. |
| Telegram bot | Not implemented | — |
| Command line | Not implemented | — |

## Tokens

Every client authenticates with a bearer token:

```
Authorization: Bearer sngr_a1b2c3d4e5f6g7h8i9j0k1l2m3n4o5p6
```

A token belongs to one user. Every capture records that user, so the list and
the notifications can say who snagged a title.

A token starts with `sngr_`. Snagarr stores only its SHA-256 digest. A raw
token is readable once, when it is issued.

An `admin` may do everything. A `member` may capture, read, and act on their
own items. See the roles section of [api.md](api.md).

### Get a token

There are four ways.

**From the first run.** Snagarr prints one admin token to standard output at
first start. See [deployment.md](deployment.md).

**From the setup wizard.** Open `/setup`. Go to the last step. Select **Create
a household token**. Copy the value.

**From the settings page.** Select **Generate bookmarklet**. Snagarr issues a
token named `Bookmarklet`. The token is inside the generated code.

**From the API.** Ask for a token for a user. This needs an admin token:

```sh
curl -X POST http://localhost:8080/api/v1/users/1/tokens \
  -H "Authorization: Bearer sngr_your_admin_token" \
  -H "Content-Type: application/json" \
  -d '{"name":"iPhone Shortcut"}'
```

```json
{ "id": 7, "name": "iPhone Shortcut", "token": "sngr_…", "created_at": "…" }
```

This response is the only time the raw token is readable. Copy it now.

Give each client its own token. You can then revoke one client without
touching the others.

### Revoke a token

Revoke one token by ID:

```sh
curl -X DELETE http://localhost:8080/api/v1/tokens/7 \
  -H "Authorization: Bearer sngr_your_admin_token"
```

To revoke every token of one household member, open the settings page. Find
the member in the household table. Select **Revoke**.

List a user's tokens with `GET /api/v1/users/{id}/tokens`. The response shows
the name, the prefix and the last use. It never shows the token.

## The capture request

All clients send the same request:

```
POST /api/v1/capture
Authorization: Bearer sngr_…
Content-Type: application/json
```

The body accepts these fields, and no others:

| Field | Type | What it does |
|-------|------|--------------|
| `query` | string | Free text, or a link. |
| `url` | string | A link. Used when `query` is empty. |
| `tmdb_id` | number | An exact TMDB ID. Needs `media_type`. |
| `media_type` | string | `movie` or `tv`. Only with `tmdb_id`. |
| `source` | string | Which client captured the item. Defaults to `api`. |
| `note` | string | Free text kept with the item. |
| `source_url` | string | The page the capture came from. |

> Snagarr rejects an unknown field with `400 bad_request`. Send only the fields
> above.

`source` should be one of `web`, `shortcut`, `telegram`, `bookmarklet`, `api`
or `cli`. Snagarr shows the value in the item list. It also shows it in the
availability push.

There are two paths.

**Exact identity.** Send `tmdb_id` and `media_type`. Snagarr resolves the title
inline. It answers `201 Created` with the finished item. It answers `200 OK`
with the existing item when that title is already snagged.

**Everything else.** Send `query` or `url`. Snagarr saves the raw input at
once. It answers `202 Accepted` with a `needs_review` item. Resolution then
runs in the background.

Snagarr stores whichever of `query` and `url` you send as `raw_input`. A value
that starts with `http://` or `https://` is treated as a link. So one field
carries both free text and links.

A capture is never rejected for being unidentifiable. Anything the resolver
cannot settle stays in **Needs Review** with its candidates and its raw input.

## Web app

The Go binary serves the web app at `/`. There is nothing to install.

### First use

1. Open the setup URL Snagarr printed at first start.
2. The app takes the token from the URL fragment. It saves the token. It then
   removes the token from the URL.
3. The app opens on the **Snag** screen.

If you open Snagarr without a token, it shows a token box. Paste a token there.
The app keeps it in browser local storage under the key `snagarr.token`.

To change the token later, open `<public url>/#token=<new token>`.

### Capture

1. Type into the search box. It is focused on load.
2. Wait for the results. Library matches rank first. Every row carries one
   state badge.
3. Select a result. The item is saved at once, with an undo toast.

Paste a link into the same box to capture a page.

Press `/` anywhere to return to the search box.

The **List** screen holds the poster grid. Filter it with the chips: **All**,
**Ready**, **Pending**, **Reviewing** and **Archived**.

Select an item to open its detail sheet. The sheet holds the actions:

| Action | Who sees the button |
|--------|---------------------|
| **Send to Radarr** for a movie, **Send to Sonarr** for a series | Admin |
| **Request via Overseerr** | Admin |
| **Archive** or **Unarchive** | Everybody |
| **Delete** | Admin |

Snagarr picks the send target from the media type. There is one send button,
not two.

A member may archive only the items they captured. The API answers `403` for
another member's item.

## Apple Shortcut

There is no published Shortcut in the gallery yet. Build it with the steps
below. It takes about two minutes.

The shortcut sends the share-sheet input, or the text you type, to the capture
endpoint.

### Build it

1. Open the **Shortcuts** app.
2. Select **+** to make a new shortcut.
3. Select the shortcut name. Choose **Rename**. Type `Snag`.
4. Add the action **Get Contents of URL**.
5. Set the URL to `https://snagarr.example.com/api/v1/capture`.
6. Select **Show More** to expand the action.
7. Set **Method** to `POST`.
8. Select **Add new header**. Set the key to `Authorization`. Set the value to
   `Bearer sngr_your_token`.
9. Set **Request Body** to `JSON`.
10. Select **Add new field**. Choose **Text**. Set the key to `query`. Set the
    value to the **Shortcut Input** variable.
11. Select **Add new field**. Choose **Text**. Set the key to `source`. Set the
    value to `shortcut`.
12. Add the action **Show Notification**. Set the text to `Snagged`.
13. Select the details button (ⓘ) at the top.
14. Turn on **Show in Share Sheet**.
15. Set **Share Sheet Types** to **URLs** and **Text** only.
16. Select **Done**.

### Use it

- **From a web page.** Select **Share**. Choose **Snag**.
- **From selected text.** Select the text. Select **Share**. Choose **Snag**.
- **By hand.** Run the shortcut from the home screen. Type a title when it
  asks.

The same `query` field carries both a title and a link. Snagarr decides which
it is.

### Check it

Snagarr answers `202 Accepted` for this path. The item appears in the web app
right away. It may sit in **Needs Review** for a moment while it resolves.

An ambiguous capture stays in **Needs Review**. Resolve it in the web app with
one tap. Nothing is lost.

If the shortcut fails, check three things: the URL host, the `Bearer ` prefix
in the header, and the **Request Body** setting of `JSON`.

## Bookmarklet

The bookmarklet posts the current page URL to the capture endpoint. It is the
desktop capture path with nothing to install.

### Generate it

1. Open the settings page as an admin.
2. Go to **Household & tokens**.
3. Select **Generate bookmarklet**.
4. Copy the generated code. It carries a new token, readable once.
5. Make a new browser bookmark. Paste the code into the address field.
6. Name the bookmark `Snag`.

The code has this shape, with your own base URL and token:

```js
javascript:(function(){fetch('https://snagarr.example.com/api/v1/capture',{method:'POST',headers:{'Content-Type':'application/json','Authorization':'Bearer sngr_your_token'},body:JSON.stringify({url:location.href,source:'bookmarklet'})}).then(function(r){alert(r.ok?'Snagged':'Snagarr said '+r.status)}).catch(function(e){alert('Snagarr unreachable: '+e)})})()
```

The base URL comes from `general.public_url`. Snagarr falls back to the origin
of the browser tab you generated it in. Set `general.public_url` before you
generate the bookmarklet.

### Use it

Open a page about a film. Select the `Snag` bookmark. An alert says `Snagged`.

### Known limitation

Snagarr sends no CORS headers. A browser therefore blocks this request from any
page that is not on the Snagarr origin, and the alert reports
`Snagarr unreachable`.

The bookmarklet works today only on pages served from the Snagarr host. Use the
Apple Shortcut or `curl` for capture from other sites.

> The bookmarklet code carries a working token in plain text. Do not share the
> bookmark. Revoke its token if you do.

## curl and scripts

Capture free text:

```sh
curl -X POST http://localhost:8080/api/v1/capture \
  -H "Authorization: Bearer sngr_your_token" \
  -H "Content-Type: application/json" \
  -d '{"query":"sinners 2025","source":"api"}'
```

Capture a link:

```sh
curl -X POST http://localhost:8080/api/v1/capture \
  -H "Authorization: Bearer sngr_your_token" \
  -H "Content-Type: application/json" \
  -d '{"url":"https://www.themoviedb.org/movie/1233413","source":"api"}'
```

Capture an exact TMDB title:

```sh
curl -X POST http://localhost:8080/api/v1/capture \
  -H "Authorization: Bearer sngr_your_token" \
  -H "Content-Type: application/json" \
  -d '{"tmdb_id":1233413,"media_type":"movie","source":"api"}'
```

Search before you capture, to see the state badges:

```sh
curl -G http://localhost:8080/api/v1/search \
  -H "Authorization: Bearer sngr_your_token" \
  --data-urlencode "q=sinners"
```

List what needs review:

```sh
curl -G http://localhost:8080/api/v1/items \
  -H "Authorization: Bearer sngr_your_token" \
  --data-urlencode "status=needs_review"
```

Send a resolved item to Radarr. This needs an admin token:

```sh
curl -X POST http://localhost:8080/api/v1/items/42/send \
  -H "Authorization: Bearer sngr_your_admin_token" \
  -H "Content-Type: application/json" \
  -d '{"target":"radarr"}'
```

Put your token in an environment variable to keep it out of your shell history:

```sh
export SNAG_TOKEN=sngr_your_token
curl -X POST http://localhost:8080/api/v1/capture \
  -H "Authorization: Bearer $SNAG_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"query":"sinners","source":"api"}'
```

## Not yet implemented

These clients appear in the product plan. No code for them exists today. Do not
plan around them.

### Telegram bot

The settings UI has a Telegram card. It accepts a bot token. The user records
accept a Telegram user ID.

Nothing reads either value. Snagarr runs no bot. It does not poll Telegram. It
answers no Telegram message.

The **Telegram** row on the setup screen turns green when a bot token is
stored. This only means the value is saved.

### Command line

The binary has two commands: `snagarr serve` and `snagarr version`. There is no
`snagarr add` and no `snagarr list`.

Use `curl` against a running instance instead.

### Generic webhook ingest

There is no endpoint that turns arbitrary JSON into a capture. The routes under
`/api/v1/webhooks/` accept import and playback events only. See
[webhooks.md](webhooks.md).
