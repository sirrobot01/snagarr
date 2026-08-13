# Snagarr UI specification

Source: Claude Design project `Snagarr.dc.html`, system "Modernist".
Tokens and component classes live in `web/src/styles/modernist.css`. That file
is the source of truth for colour, type and spacing. Do not restate its values
in the Tailwind config; consume the CSS variables.

## 1. The system in one paragraph

Swiss/editorial. Zero border radius everywhere. One accent (`--color-accent`),
never a second hue. Hierarchy comes from **weight and rule thickness**, not
colour: 2px rules separate regions, 1px rules separate rows. Archivo at 800 for
every heading, label and control; 400 only for body copy and overviews. Micro
labels are 9-11px, uppercase, letter-spaced 0.07-0.12em. Dark is the default
ground; light is a toggle.

## 2. Layout

- Mobile-first at 390px. Desktop breakpoint at 768px.
- Desktop content column: `max-width: 900px`, centred.
- No page-level padding on mobile; regions carry their own 16px (mobile) or
  20px (desktop) inline padding.
- Region separators are `2px solid var(--color-divider)`. Row separators are
  `1px solid var(--color-line)`.

## 3. Chrome

**Header (`.nav`)** — desktop: brand `SNAGARR` (18px/800, tracking -0.02em) on
the left, then `Snag` / `List` / `Settings` links (14px), then the current user
as a micro label `MUKHTAR · ADMIN` pushed right. Active link takes
`aria-current="page"` and turns accent.

Mobile: brand on the left, a single right-aligned micro label link to the other
screen (`LIST →`). Bottom padding 12px, 2px bottom rule.

**Footer strip (`.sg-foot`)** — always present, `--color-surface` ground, two
micro labels: `POWERED BY TMDB` (required by TMDB terms, left) and a
context-dependent right label (`3 IN NEEDS REVIEW`, `SNAGGED COLLECTION SYNCED
4 MIN AGO`).

## 4. Screen — Snag (home)

Order top to bottom:

1. **Offline banner** (`.sg-banner`, conditional) — `OFFLINE — 2 CAPTURES
   QUEUED` left, `RETRYING…` right.
2. **Capture region** — 26px top / 18px bottom padding on desktop, 16px on
   mobile. A row of: the `/` hint chip (`.sg-b .sg-new`, padding 5px 8px), the
   `.sg-search` input (flex 1), and a right-aligned two-line micro label
   `LOCAL INDEX` / `+ TMDB`. Below the input on mobile, a `.sg-rule` — accent
   when focused or non-empty, `data-idle="1"` otherwise. 2px bottom rule.
   - The input is **autofocused on mount**.
3. **Result meta bar** — 9px padding, 1px bottom rule, two micro labels:
   `{n} RESULTS · LIBRARY FIRST` and `↑↓ MOVE · ⏎ SNAG` (desktop only).
4. **Results** — `.sg-row` list. Each row: poster `.sg-p` (38×57 desktop,
   44×66 mobile) · title/meta block (flex 1, `.sg-row-title` +
   `.sg-row-meta`) · one `.sg-b` badge · `.sg-row-action` (`SNAG`, or `OPEN`
   when already in library, or `✓` once snagged). Clicking the row snags
   immediately — no confirmation.
5. **When the box is idle** (empty query): Needs Review cards (§6) if any, then
   the most recent snags as the same `.sg-row` list under a micro-label header.
6. **Empty state** — a 56×84 empty poster outline (`2px solid`, 30% text), then
   `Nothing snagged yet.` (h3, 22px), then one line of body copy at 13px muted,
   max-width 280px, then a secondary button `Install the iOS Shortcut`.

**Toast** — after a snag, `.sg-toast` docked bottom-right on desktop /
bottom-centre with 16px inset on mobile: `SNAGGED — SINNERS (2025)` plus an
`UNDO` button (`.btn-primary`, 11px, tracking 0.08em). Auto-dismiss at 6s.

## 5. Screen — List

1. **Chip bar** — 14px padding, 2px bottom rule. `.sg-chip` buttons:
   `All` `Ready` `Pending` `Reviewing` `Archived`. Active chip carries
   `data-on="1"`. Item count micro label pushed right.
2. **Poster grid** — `grid-template-columns: repeat(6, 1fr)` at ≥1024px,
   `repeat(4, 1fr)` at ≥768px, `repeat(3, 1fr)` below. `gap: 1px` with the grid
   container painted `--color-divider`, so the gaps read as hairlines. Each
   cell: `--color-bg` ground, 10px padding, poster at `aspect-ratio: 2/3`
   full width, then title (12px/800), then sub (10px muted), then one badge.
3. Tapping a cell opens the detail sheet (§7).

**Alternative index view** (`1e`, build it — the chip bar gets a
`GRID ↔ INDEX` toggle at the right): a `.table` with columns
`(poster 44px) · Title · Captured · By · Source · State`. Poster thumbs are
22×33. Title cell is `<b>Title</b> <span class="text-muted">Year</span>`.
State cell is right-aligned and carries the badge. When rows are selected, a
`--color-surface` action bar appears at the bottom: `{n} SELECTED` left, then
`ARCHIVE` and `SEND TO RADARR` buttons.

## 6. Needs Review

A block with an accent header bar: `NEEDS REVIEW — 3` (15px/800) left,
`NOTHING IS EVER DROPPED` micro label right, on `--color-accent` with
`--color-bg` text.

The first (expanded) item shows:
- micro label `CAPTURED 08 JUL · TELEGRAM · AMINA`
- the raw input in 20px/800, quoted with typographic quotes
- a 3-column candidate grid (`gap: 12px`): poster at `aspect-ratio: 2/3`, then
  title 13px/800, then `2024 · 94% match` at 11px muted. The selected candidate
  gets `outline: 2px solid var(--color-accent); outline-offset: 2px`.
- action row: `CONFIRM NOSFERATU (2024)` (`.btn-primary`), `Search manually`
  (`.btn-secondary`), `Discard` (`.btn-ghost`, pushed right).

Remaining items collapse to single rows at `opacity: .6`: raw input 15px/800 on
the left, `04 JUL · SHORTCUT` micro label on the right.

## 7. Item detail — sheet (mobile) / popover (desktop)

Mobile bottom sheet: a 150px scrim above, then the panel with a 2px top rule.
Panel contents: 74×111 poster beside a block of title (h3 24px), meta
(`2025 · Movie · 137 min · Horror`, 12px muted), and one badge. Then the
overview at 13px/1.5. Then `.hr`. Then the capture-context micro label
`SNAGGED BY AMINA · FROM TELEGRAM · 12 JUL`. Then stacked full-width buttons,
all left-aligned with 12px padding: `SEND TO RADARR` (`.btn-primary`),
`Request via Overseerr`, `Archive`, `Delete` (accent text).

Desktop popover: 400px wide, `.elev-lg`. 80×120 poster beside title (19px),
meta, badge and a 12px overview. Capture-context micro label below. Actions sit
in a single row divided by 1px left borders across the bottom, above a 2px top
rule.

Use Radix primitives (`Dialog` for the sheet, `Popover` for desktop) so focus
trapping and ARIA are correct, then style them entirely with these tokens.
Do not ship shadcn's stock theme.

## 8. Settings

Header, then a grid of integration cards: `repeat(3, 1fr)` desktop /
`repeat(2, 1fr)` tablet / 1 column mobile, `gap: 2px` on a `--color-divider`
ground so the gaps read as rules. Each card, 16px padding on `--color-bg`:

- Status row: an 8×8 square (accent when connected, `2px solid` outline when
  unset, `--color-text` when errored), the service name (15px/800, muted at
  55% opacity when unset), and a right-aligned status micro label — `OK`,
  `OK · 4,182 ITEMS`, `NOT SET`, `401 — CHECK TOKEN`.
- One or two `.field` inputs. Secrets render masked as `•••••••••••••••• 4e2a`.
- A `Test connection` button (`.btn-secondary`, 12px), which becomes `RETEST`
  (`.btn-primary`) after a failure.

Below the grid, separated by a 2px rule: **Household & tokens** — an h4, then a
`.table` of users (`name` · role badge · `3 tokens · telegram 4419…` muted ·
right-aligned `Revoke` in accent). Then a button row: `Add household member`,
`Generate bookmarklet`, and `Force reconcile now` (`.btn-ghost`, pushed right).

## 9. First-run setup

620px card, four steps. Header block: micro label `SETUP · STEP 2 OF 4`, an h2
at 34px (tracking -0.03em), one line of 14px muted copy. Then a 8px-tall
progress bar built from four equal flex children — filled steps take
`--color-accent`, pending take `--color-neutral-300` (light) /
`--color-neutral-800` (dark). Then the step body at 22px/24px padding. Then a
footer row above a 2px rule: `Back` (`.btn-ghost`), `Skip for now`
(`.btn-secondary`, pushed right), `CONTINUE` (`.btn-primary`).

Step 2 body is the reference: a `.seg` control (`Plex` / `Emby` / `Jellyfin`),
two `.field` inputs, then a `.sg-note` confirmation strip on success —
`CONNECTED — 2 SECTIONS, 4,182 ITEMS. FIRST INDEX WILL TAKE ~40 S.`

## 10. Keyboard and accessibility

- `/` focuses the search input from anywhere (unless already in a field).
- `↑` `↓` move the result cursor (`data-active="1"` on `.sg-row`), `Enter`
  snags the active row, `Escape` clears the query and blurs.
- Every interactive element reaches 44px of touch target on mobile, padding
  included.
- Focus rings are `2px solid var(--color-accent)` at 2px offset — never removed.
- Rows are `<button>` elements, not clickable `<div>`s.
- Posters carry explicit width/height (or `aspect-ratio`) so nothing shifts on
  load, and use TMDB `w185` on rows / `w342` in the grid, `loading="lazy"`.
- Respect `prefers-reduced-motion`: no transitions when it is set.
