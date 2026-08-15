---
title: Capture clients
description: Capture from the web app, the Snagarr CLI, the published Apple Shortcut, the Telegram bot, a bookmarklet or curl.
---

Every client is a front-end over one endpoint: `POST /api/v1/capture`. Each one authenticates with its own bearer token. See [First run](/snagarr/start/first-run/#tokens) to issue one.

| Client | State | Runs on |
|--------|-------|---------|
| Web app | Works | Any browser |
| Apple Shortcut | Install Snagarr's published link | iOS, iPadOS, macOS, watchOS |
| Bookmarklet | Works from any origin | Desktop browser |
| `curl` and scripts | Works | Anywhere |
| Telegram bot | Works. The household chat client | Any device with Telegram |
| Command line | Included in the `snagarr` binary | Linux, macOS, Windows, FreeBSD |

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

1. Type into the search box. It is focused on desktop load; touch devices open without forcing the keyboard. Press `/` anywhere to return to it.
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

The API is wider than the buttons. `POST /api/v1/items/{id}/send` accepts any token and spends the caller's own service, and `DELETE /api/v1/items/{id}` accepts the capturer. A member uses `curl` for both today.

### Add it to a home screen

The client carries a web app manifest, so a phone can keep it beside the other applications.

- **iOS.** Open Snagarr in Safari. Select the share button, then **Add to Home Screen**.
- **Android.** Open Snagarr in Chrome. Select the menu, then **Install app**.

It opens without browser chrome and keeps the session it signed in with. This installs no software: it is the same web client behind an icon.

## Command line

The server binary is also a Cobra-based client. Sign in once with the same account used by the web app:

```sh
snagarr login https://snagarr.example.com
Username: amina
Password:
✓  Logged in as @amina
```

The password is used only for the login request. The returned bearer token and server address are saved in the operating system's user configuration directory with user-only permissions.

An existing capture token works too. Reading it from stdin keeps it out of shell history:

```sh
pbpaste | snagarr login https://snagarr.example.com --token-stdin
```

Capture a title, free text, or a URL:

```sh
snagarr snag "Sinners"
snagarr snag "The Bear" --note "weekend"
snagarr snag https://www.imdb.com/title/tt31193180/
pbpaste | snagarr snag -
```

The default output is designed for a terminal. List and inspect the installation with:

```sh
snagarr list
snagarr list --status needs_review
snagarr list --archived
snagarr status
```

Use `--json` on `snag`, `list`, or `status` for scripts. `SNAGARR_URL` and `SNAGARR_TOKEN` override the saved session, and the same values are available as `--server` and `--token` flags. Run `snagarr logout` to remove the saved token.

## Apple Shortcut

Snagarr publishes one Shortcut, named **Snag**. Apple signs it, so it imports with no untrusted-shortcut prompt.

```
https://www.icloud.com/shortcuts/c4b4dabe0b55481c9fe35fac0a4a266b
```

Open that link on the device, or select **Add the Apple Shortcut** in Snagarr's Settings. The Shortcut is the same for every install, and the address and token come from the import questions.

### Install it

1. Get a token. An admin issues one under **Settings → Household access → Tokens**, or with `POST /api/v1/users/{id}/tokens`.
2. Open the link on the device.
3. Tap **Get Shortcut**.
4. Answer `What is your Snagarr address?`.
5. Answer `Paste your Snagarr token`.

:::caution[Give it the address your phone can reach]
`http://localhost:8080` works only on the machine running Snagarr. Enter the address the household actually reaches, for example `https://snagarr.example.com`. Set `general.public_url` to the same value.
:::

Give each member their own token. Every capture records the member behind the token. See [First run](/snagarr/start/first-run/#tokens).

### What it sends

```
POST <your address>/api/v1/capture
Authorization: Bearer <your token>
Content-Type: application/json

{"query": "<Shortcut Input>", "source": "shortcut"}
```

It then reads `title` from the response and shows a notification, `Snagged <title>`.

Run it from the share sheet, from selected text, from the home screen or from an Apple Watch. It accepts URLs, text, rich text, articles and Safari web pages.

Send everything as `query`. Snagarr treats a value that starts with `http://` or `https://` as a link and runs it through the same resolver. The shortcut needs no branch on the input.

The response is `202 Accepted`. The item can sit in **Needs Review** for a moment while it resolves, and the notification then names the raw input rather than a title.

| Symptom | Cause |
|---------|-------|
| `401 unauthorized` | The token is wrong or revoked |
| `404`, or nothing arrives | The address answer is wrong, or it ends in `/` |
| Nothing arrives, no error | The address is not reachable from the phone |

### Publish your own

Skip this unless you want a different Shortcut. Snagarr generates no shortcut file; you build one and publish it yourself.

:::danger[Sharing publishes whatever the Text actions hold]
An iCloud link carries the current contents of the shortcut. A real token typed into a Text action goes out with the link, to everybody who has it.

Share while both Text actions hold placeholders. Put your real values back only afterwards, in your own copy. An import question does **not** clear the field for you.
:::

An iCloud link is public, so it cannot carry a token. Import questions collect the address and the token from each person at import time.

1. Open the **Shortcuts** app on a Mac. Click **+**. Name the shortcut `Snag`.
2. Add a **Text** action. Set its content to `http://localhost:8080`.
3. Add a second **Text** action. Set its content to `sngr_replace_me`.
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

5. Add **Get Dictionary Value**. Set **Key** to `title`.
6. Add **Show Notification**. Set its text to the **Dictionary Value** variable.
7. Test it: type your real address and token into the two **Text** actions, run the shortcut once, and fix any failure.
8. **Put the placeholders back into both Text actions.** Everything below publishes what they hold.
9. Click the details icon in the toolbar. Click **Setup**.

   Add the actions before this step. An import question binds to a field inside an existing action. An empty shortcut answers `This shortcut has no actions. Please add some actions to your shortcut before setting up Import Questions.`

10. Click **+**. Choose the **Text** action from step 2. Type `What is your Snagarr address?` in **Question Text**.
11. Click **+**. Choose the **Text** action from step 3. Type `Paste your Snagarr token` in **Question Text**.
12. Click **Done**.
13. Click **Details**. Select **Show in Share Sheet**. A **Receive** action appears at the top of the shortcut.
14. Click **Any** in that **Receive** action. Limit the input types to **URLs** and **Text**.
15. Click the share button. Choose **Copy iCloud Link**.
16. Send the link to the people who need it.
17. Type your real address and token back into your own copy.

:::caution[Publish a reachable address]
Whatever you put in the URL **Text** action is the default every importer sees. A shortcut published with `localhost` is useless to anybody else. Publish the address the household actually reaches.
:::

## Telegram bot

The bot is the household chat client, and the capture path on Android: type or share a title into a Telegram chat and it lands on the list with your name on it. The bot long-polls telegram.org, so it needs no public URL, no open port and no webhook. It works behind CGNAT.

### Set up the bot

1. Message [@BotFather](https://t.me/BotFather) on Telegram.
2. Send `/newbot`. Give the bot a name and a username.
3. Copy the token BotFather answers with.
4. Open **Settings** as an admin. Paste the token into the **Telegram bot** card.
5. Select **Test connection**. The card must answer with the bot's username.

`SNAGARR_TELEGRAM_BOT_TOKEN` pins the token and locks the card.

### Link the members

The bot answers only Telegram accounts on the household table. There is no separate allow-list.

1. The member messages the bot once. The bot replies with their Telegram ID.
2. An admin selects **Telegram** on the member's row in **Settings → Household** — their own row included — and enters the ID.
3. The member messages the bot again. From now on it snags.

### Use it

Send the bot a title or a link.

- The bot snags it with the member's name attached and replies with the poster and the state badge.
- A new title offers **Send to Radarr** or **Send to Sonarr**. The send uses the member's own service; for admins, another admin's is the fallback.
- An uncertain match offers up to three candidates. One tap resolves it. **None of these** parks it in Needs Review on the web.
- A title already on the list answers "already on the household list" instead of duplicating it.

A member may act on their own snags. Only an admin may act on another member's.

## Bookmarklet

The bookmarklet posts the current page URL.

1. Open the settings page as an admin.
2. Go to **Household access**.
3. Select **Browser bookmarklet**.
4. Select **Generate link**.
5. Copy the generated code. It carries a new token, readable once.
6. Make a browser bookmark. Paste the code into the address field.
7. Name the bookmark `Snag`.

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

# send to your own Radarr
curl -X POST http://localhost:8080/api/v1/items/42/send \
  -H "Authorization: Bearer $SNAG_TOKEN" -H "Content-Type: application/json" \
  -d '{"target":"radarr"}'
```

The send spends a Radarr **you** own. With none, it answers `503 not_configured`. See [Services](/snagarr/configure/services/).

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
