---
title: Capture clients
description: Every client posts to /api/v1/capture — the web app, an Apple Shortcut, a bookmarklet or curl.
---

Every client is a front-end over one endpoint: `POST /api/v1/capture`. Each one authenticates with its own bearer token. See [First run](/snagarr/start/first-run/#tokens) to issue one.

| Client | State | Runs on |
|--------|-------|---------|
| Web app | Works | Any browser |
| Apple Shortcut | Import from the operator's iCloud link | iOS, iPadOS, macOS |
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

Snagarr does not generate a shortcut file. The operator builds one shortcut, shares it as an iCloud link, and stores that link in `general.shortcut_url`. Everybody in the household imports from the same link.

### Why an iCloud link

An unsigned `.shortcut` file makes iOS demand **Settings → Shortcuts → Allow Untrusted Shortcuts**. That toggle stays hidden until the device has run a shortcut once. Signing a file needs macOS, which a Linux container does not have.

Apple signs every shortcut shared as an iCloud link. It imports with no prompt.

An iCloud link is public, so it cannot carry a token. The shortcut asks for the server URL and the token as **import questions** instead. Two prompts, once, at import time.

### Build the shortcut

Add the actions first. An import question binds to a field inside an existing action. An empty shortcut answers `This shortcut has no actions. Please add some actions to your shortcut before setting up Import Questions.`

1. Open the **Shortcuts** app on a Mac. Click **+**. Name the shortcut `Snag`.
2. Add a **Text** action. Set its content to `https://snagarr.example.com`. The URL question targets this field.
3. Add a second **Text** action. Set its content to `sngr_your_token`. The token question targets this field.
4. Add **Get Contents of URL**. Click **Show More**. Fill it in from this table.

| Field | Value |
|-------|-------|
| URL | The **Text** variable from step 2, then `/api/v1/capture` |
| Method | `POST` |
| Header `Authorization` | `Bearer `, then the **Text** variable from step 3 |
| Header `Content-Type` | `application/json` |
| Request Body | `JSON` |
| JSON field `query` | The **Shortcut Input** variable |
| JSON field `source` | `shortcut` |

The request it sends:

```
POST https://snagarr.example.com/api/v1/capture
Authorization: Bearer sngr_your_token
Content-Type: application/json

{"query": "<Shortcut Input>", "source": "shortcut"}
```

5. Add **Show Notification**. Set its text to the **Contents of URL** variable.
6. Run the shortcut once. Both **Text** actions still hold real values, so this proves the request works. Fix any failure before you share the link.
7. Click the details icon in the toolbar. Click **Setup**.
8. Click **+**. Choose the **Text** action from step 2. Type `What is your Snagarr URL?` in **Question Text**.
9. Click **+**. Choose the **Text** action from step 3. Type `What is your Snagarr token?` in **Question Text**.
10. Click **Done**.
11. Click **Details**. Select **Show in Share Sheet**. A **Receive** action appears at the top of the shortcut.
12. Click **Any** in that **Receive** action. Limit the input types to **URLs** and **Text**.
13. Click the share button. Choose **Copy iCloud Link**.
14. Paste the link into Snagarr under **Settings → General → iOS Shortcut link**.

Leave the real values in the two **Text** actions. Sharing the link clears both fields, because an import question is set on each.

### Import the shortcut

The Snag screen shows an **Install the iOS Shortcut** button once `general.shortcut_url` is set. The button stays hidden while the setting is empty.

1. Tap the link.
2. Tap **Get Shortcut**.
3. Answer `What is your Snagarr URL?`.
4. Answer `What is your Snagarr token?`.

Give each member their own token. Every capture records the member behind the token. See [First run](/snagarr/start/first-run/#tokens).

### Run it

Run it from the share sheet, from selected text, or from the home screen.

Send everything as `query`. Snagarr treats a value that starts with `http://` or `https://` as a link and runs it through the same resolver. The shortcut needs no branch on the input.

The response is `202 Accepted`. The item can sit in **Needs Review** for a moment while it resolves.

| Symptom | Cause |
|---------|-------|
| `401 unauthorized` | The header value lost its `Bearer ` prefix, or the token is revoked |
| `400 bad_request` | **Request Body** is not `JSON` |
| `404`, or nothing arrives | The URL answer is wrong, or it ends in `/` |

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
