---
title: Capture clients
description: Every client posts to /api/v1/capture — the web app, an Apple Shortcut, a bookmarklet or curl.
---

Every client is a front-end over one endpoint: `POST /api/v1/capture`. Each one authenticates with its own bearer token. See [First run](/snagarr/start/first-run/#tokens) to issue one.

| Client | State | Runs on |
|--------|-------|---------|
| Web app | Works | Any browser |
| Apple Shortcut | Works, built by hand | iOS, iPadOS, macOS |
| Bookmarklet | Works from any origin | Desktop browser |
| `curl` and scripts | Works | Anywhere |
| Telegram bot | Not implemented | — |
| Command line | Not implemented | — |

## The capture request

```
POST /api/v1/capture
Authorization: Bearer sngr_…
Content-Type: application/json
```

| Field | Type | Effect |
|-------|------|--------|
| `query` | string | Free text, or a link |
| `url` | string | A link. Used when `query` is empty |
| `tmdb_id` | number | An exact TMDB ID. Needs `media_type` |
| `media_type` | string | `movie` or `tv`. Only with `tmdb_id` |
| `source` | string | `web`, `shortcut`, `telegram`, `bookmarklet`, `api` or `cli`. Defaults to `api` |
| `note` | string | Free text kept with the item |
| `source_url` | string | The page the capture came from |

Snagarr rejects any other field with `400 bad_request`.

Two paths:

| Input | Response | Behaviour |
|-------|----------|-----------|
| `tmdb_id` + `media_type` | `201 Created`, or `200 OK` if the title is already snagged | Resolves inline and returns the finished item |
| `query` or `url` | `202 Accepted` | Saves the raw input as a `needs_review` item and resolves in the background |

Snagarr stores whichever of `query` and `url` you send as `raw_input`, and treats a value that starts with `http://` or `https://` as a link. A capture is never rejected for being unidentifiable: anything the resolver cannot settle stays in **Needs Review** with its candidates and its raw input.

`source` appears in the item list and in the availability push.

## Web app

The binary serves the client at `/`. There is nothing to install.

1. Type into the search box. It is focused on load. Press `/` anywhere to return to it.
2. Read the results. Library matches rank first. Every row carries one state badge.
3. Select a result. The item saves at once, with an undo toast.

Paste a link into the same box to capture a page.

The **List** screen holds the poster grid, filtered by the chips **All**, **Ready**, **Pending**, **Reviewing** and **Archived**. Select an item for its detail sheet:

| Action | Who sees the button |
|--------|---------------------|
| **Send to Radarr** (movie) or **Send to Sonarr** (series) | Admin |
| **Request via Overseerr** | Admin |
| **Archive** or **Unarchive** | Everybody, own items only for a member |
| **Delete** | Admin |

## Apple Shortcut

No Shortcut is published in the gallery. Build it:

1. Open the **Shortcuts** app.
2. Select **+** to make a new shortcut.
3. Select the shortcut name. Choose **Rename**. Type `Snag`.
4. Add the action **Get Contents of URL**.
5. Set the URL to `https://snagarr.example.com/api/v1/capture`.
6. Select **Show More**.
7. Set **Method** to `POST`.
8. Select **Add new header**. Set the key to `Authorization` and the value to `Bearer sngr_your_token`.
9. Set **Request Body** to `JSON`.
10. Select **Add new field**. Choose **Text**. Set the key to `query` and the value to the **Shortcut Input** variable.
11. Select **Add new field**. Choose **Text**. Set the key to `source` and the value to `shortcut`.
12. Add the action **Show Notification**. Set the text to `Snagged`.
13. Select the details button (ⓘ).
14. Turn on **Show in Share Sheet**.
15. Set **Share Sheet Types** to **URLs** and **Text** only.
16. Select **Done**.

Run it from the share sheet, from selected text, or from the home screen. The response is `202 Accepted`, so the item can sit in **Needs Review** for a moment while it resolves.

A failing shortcut is almost always one of three things: the URL host, a missing `Bearer ` prefix in the header, or a **Request Body** that is not `JSON`.

## Bookmarklet

The bookmarklet posts the current page URL.

1. Open the settings page as an admin.
2. Go to **Household & tokens**.
3. Select **Generate bookmarklet**.
4. Copy the generated code. It carries a new token, readable once.
5. Make a browser bookmark. Paste the code into the address field.
6. Name the bookmark `Snag`.

```js
javascript:(function(){fetch('https://snagarr.example.com/api/v1/capture',{method:'POST',headers:{'Content-Type':'application/json','Authorization':'Bearer sngr_your_token'},body:JSON.stringify({url:location.href,source:'bookmarklet'})}).then(function(r){alert(r.ok?'Snagged':'Snagarr said '+r.status)}).catch(function(e){alert('Snagarr unreachable: '+e)})})()
```

The base URL comes from `general.public_url`, and falls back to the origin of the tab you generated it in. Set `general.public_url` before you generate the bookmarklet.

Snagarr allows every origin on `/api/v1`, so the bookmarklet works on any site.

:::caution
The bookmarklet code carries a working token in plain text. Do not share the bookmark. Revoke its token if you do.
:::

## curl

```sh
export SNAG_TOKEN=sngr_your_token

# free text
curl -X POST http://localhost:8080/api/v1/capture \
  -H "Authorization: Bearer $SNAG_TOKEN" -H "Content-Type: application/json" \
  -d '{"query":"sinners 2025","source":"api"}'

# a link
curl -X POST http://localhost:8080/api/v1/capture \
  -H "Authorization: Bearer $SNAG_TOKEN" -H "Content-Type: application/json" \
  -d '{"url":"https://www.themoviedb.org/movie/1233413","source":"api"}'

# an exact TMDB title
curl -X POST http://localhost:8080/api/v1/capture \
  -H "Authorization: Bearer $SNAG_TOKEN" -H "Content-Type: application/json" \
  -d '{"tmdb_id":1233413,"media_type":"movie","source":"api"}'

# search first, to see the state badges
curl -G http://localhost:8080/api/v1/search \
  -H "Authorization: Bearer $SNAG_TOKEN" --data-urlencode "q=sinners"

# list what needs review
curl -G http://localhost:8080/api/v1/items \
  -H "Authorization: Bearer $SNAG_TOKEN" --data-urlencode "status=needs_review"

# send to Radarr (admin token)
curl -X POST http://localhost:8080/api/v1/items/42/send \
  -H "Authorization: Bearer $SNAG_TOKEN" -H "Content-Type: application/json" \
  -d '{"target":"radarr"}'
```

## Posters

`poster_path` is a TMDB path such as `/abc.jpg`, not a URL. Build the URL from the fixed base and one size:

```
https://image.tmdb.org/t/p/<size><poster_path>
```

| Size | Use |
|------|-----|
| `w185` | List rows and small thumbnails |
| `w342` | Poster grids and detail sheets |

Example: `https://image.tmdb.org/t/p/w342/abc.jpg`. No setting changes the base. A `poster_path` of `null` means Snagarr has no artwork yet; render the title instead.
