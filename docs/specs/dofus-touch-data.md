# Spec — Dofus Touch game data

| | |
| --- | --- |
| Status | **Draft** |
| Date | 2026-10-16 |
| Scope | kaelly-gamedata (new, including the CDN in its chart), Kaelly-encyclopedia (read side), image-renderer (set grid as a library), kaelly-amqp and Kaelly-notifier (import alerts, §9.4) |
| Out of scope | Moving Dofus data off dofusdude (the schema is ready for it, §10), `/item` and `/set` in kaellytouch-discord (delayed) |
| Schema | [`gamedata-schema.sql`](gamedata-schema.sql) |

## 1. Goal

`kaellytouch-discord` (`game = DOFUS_TOUCH = 2`) needs Dofus Touch data. dofusdude, which serves Dofus today, has no Touch data. kaelly-gamedata builds that data and keeps it up to date:

1. **Download the Touch game data from the client's own data API** on each game release, plus the almanax dates from krosmoz.
2. **Store it in MySQL**, keyed by `game`, in the shape the encyclopedia needs to answer the existing AMQP requests.
3. **Apply only what changed**: a hash per downloaded file decides whether there is anything to do, and a hash per record gives created / updated / deleted.
4. **Render set images** for created or changed sets, and push them to a CDN.
5. **Never break what is served**: a bad download is rejected before it reaches the database (§5.4).

kaelly-gamedata is the **only writer** of game data. Kaelly-encyclopedia only reads it.

### Non-goals

- Dofus data. The schema is keyed by `game` so Dofus can move later (§10), but nothing here changes the Dofus flow.
- Dofus Retro.
- Monsters (no `MONSTER` in `amqp.ItemType`).
- German Touch data: Touch has no German (§2.1). The encyclopedia falls back to English.

## 2. Background (sources checked 2026-10-16)

### 2.1 Touch client data API

The Touch client is a JavaScript app. It downloads its game tables as JSON, with no login. Reference: [Trameur/DofusFashionistaVanced `docs/touch_data_sources.md`](https://github.com/Trameur/DofusFashionistaVanced/blob/main/docs/touch_data_sources.md), which rebuilds a Touch item database from this API.

| Call | Result |
| --- | --- |
| `GET https://dt-proxy-production-login.ankama-games.com/config.json?lang=fr` | `dataUrl`, `assetsUrl` (`…/assets/3.3.9_<hash>`), `serverLanguages: [en, es, fr, pt]`, `failoverLanguage: en` |
| `GET <dataUrl>/build/script.js` | Client bundle, 5.4 MB, holds `window.buildVersion="1.74.8"`. Answers 304 to `If-None-Match` / `If-Modified-Since` (not used: at one run every 12h, every run downloads, §5.1) |
| `POST <dataUrl>/data/map` with `{"class": "<Table>", "lang": "<lg>"}` | One table, keyed by id, names already translated. GET is not routed. ETag sent, but a conditional POST still returns 200 |
| `GET <assetsUrl>/gfx/items/<iconId>.png` | Item icon (PNG; `.webp` returns 403). The path carries the assets version, so it changes on every assets release: not used |
| `GET https://static.ankama.com/dofus-touch/www/game/items/200/<iconId>.png` | **Same icon at a stable URL** (no version in the path), used by krosmoz and the official encyclopedia. Used for the bot's icons and as the renderer's input |

The data API, `script.js` and the static icons all accept a User-Agent naming KaellyBot.

Tables used, with their size on build 1.74.8:

| Table | Entries | Used for |
| --- | --- | --- |
| `Items` | 13 679 (12 939 `Item`, 740 `Weapon`) | items, almanax offerings, set members |
| `ItemTypes` | 243 | item type mapping |
| `ItemSets` | 338 | sets, set bonuses |
| `Effects` | 700 | effect templates (`#1{~1~2 à }#2 Force`), characteristics |
| `Recipes` | 4 338 | recipes (`jobId` / `jobName` ignored) |
| `AlmanaxCalendars` | 378 (ids 1–385) | almanax bonuses, **without dates** |

Facts the design relies on:

- **Deterministic**: two downloads of `Items` are byte-identical (same keys, same order, same records). A file hash is meaningful.
- **Stable ids**: every table is keyed by the Ankama `id` (key == `id` for all 13 679 items).
- **Languages**: `fr`, `en`, `es`, `pt`. `lang=de` returns English, exactly like `lang=it` (34/34 job names, 378/378 almanax names identical to `en`).
- **Production host only**: `earlyproxy.touch.dofus.com` is the test channel and serves `[WIP]` items (per the reference above). `proxyconnection.touch.dofus.com`, still used by older projects, is reported to no longer resolve. Always read `dataUrl` from `config.json`.
- **Raw d2o format** (Touch is a Dofus 2 fork): effects are `effectId` + `diceNum` / `diceSide`, conditions are strings like `CS>20&CV>6`, translated names are strings in `nameId`.
- **Items can belong to several "sets"**: 807 items have 2 to 6 `itemSetIds`. None is equipment: they are cubes, spell gems and breloques, listed in their "Ensemble de …" grouping (e.g. 150 cubes, 9 bonus tiers) and in "Succès …" groupings (no bonus). Equipment is in at most one set.

### 2.2 Almanax dates — krosmoz

The client knows the 378 bonuses but not which day gets which. The game server sends the current day after login (`AlmanachCalendarDateMessage`), and the offering is not in the table. The dates come from `https://www.krosmoz.com/{lg}/almanax/YYYY-MM-DD`:

- One page holds a **Dofus block, a Wakfu block and a DOFUS Touch block**, side by side. The Dofus and Touch blocks share `id="achievement_dofus"`, so select the block **by its `<h4>`**: "Bonus et Quêtes DOFUS Touch" (fr), "DOFUS Touch bonuses and quests" (en), "DOFUS Touch - Bonus und Quest" (de). Kaelly's old DevTouch branch uses `.last()` and only works because the Touch block happens to come last.
- **The Touch almanax differs from the Dofus one**. On 2026-10-16: Dofus "+100% XP, Caverne du Koulosse", offering 5 Corne du Boufcoul; Touch "+125% butin, Caverne du Koulosse", offering 9 Laine du Boufcoul.
- Languages: `fr`, `en`, `es`, `de` have a Touch block. `pt` returns the English page.
- Dates resolve in the past and future (2020, 2027, 2028 tested).
- **The cycle is yearly**: 7 dates (01-15, 03-03, 05-20, 07-08, 09-11, 10-16, 11-27) have the same offering and bonus in 2025, 2026 and 2027. The existing `almanaxes` table, keyed by `(month, day, game)`, fits. One crawl of the 366 month/day pairs covers the whole cycle.
- **krosmoz is behind CloudFront and filters bots**: from a non-whitelisted IP, a User-Agent naming KaellyBot gets 403 (even prefixed with `Mozilla/5.0`), a browser one gets 200. **krosmoz whitelists the KaellyBot VPS IP** (agreed with them), so from production the KaellyBot User-Agent is expected to work (checked at rollout, §11). The whitelist is tied to the IP: a VPS move needs a new agreement.
- **Join with the client data** (checked on 2026-10-16):
  - bonus text → `AlmanaxCalendars` entry 31 ("Butin", "Les chances d'obtenir du butin sont augmentées de 125 % dans le donjon de la Caverne du Koulosse.")
  - offering "Laine du Boufcoul" → item 7905 (icon 57001)
  - Normalise whitespace and strip HTML before matching: a raw comparison misses.
- **The offering image gives the icon id**: `https://static.ankama.com/dofus-touch/www/game/items/200/<iconId>.w75h75.png` (57001 on 2026-10-16). Names are not unique ("Cadeau de Nowel" is items 8337, 12132 and 12816), so the offering is matched on **name and icon id**: 2026-12-25 gives icon 89002, item 12132.
- **No kamas reward** is shown for Touch: `almanaxes.kamas_reward` stays `NULL` for Touch.

### 2.3 Not used

- **dofus-touch.com encyclopedia**: still works (old `ak-*` markup, needs a cookie jar for an SSO redirect hop), but only in fr/en/es, slower than the API and one template change away from breaking. Could serve as a cross-check.
- **dofusdude**: no Touch data and no plan for it. Hosting their pipeline for Touch was set aside in favor of our own storage.

## 3. Architecture

```
k3s CronJob (kaelly-gamedata)                         Kaelly-encyclopedia
┌───────────────────────────────────────────┐          ┌─────────────────────────┐
│ 1. config.json + buildVersion             │          │ reads game_versions     │
│ 2. download tables × fr/en/es/pt          │          │ reads items, sets,      │
│ 3. file hashes vs last APPLIED import     │  MySQL   │ almanaxes, labels...    │
│ 4. transform → record hashes → diff       │ ───────► │ cache keys carry the    │
│ 5. validation gate                        │          │ game release (§6)       │
│ 6. render + upload set images ──► CDN     │          └─────────────────────────┘
│ 7. one transaction: data + game_versions  │
│ 8. news (AMQP)                            │
└───────────────────────────────────────────┘
```

- **One CronJob, like kaelly-rss and kaelly-twitter**, **every 12 hours** (`0 */12 * * *`): each run does its work and exits. `concurrencyPolicy: Forbid`, so a slow run is never overlapped.
- **Low footprint**: one run every 12h, requests sent one after the other, raw data never republished (only the bot's answers and the set images derived from it).
  - Every source: a User-Agent naming KaellyBot with a contact URL (`KaellyBot-Gamedata/1.0 (+https://github.com/kaellybot/kaelly-gamedata)`).
  - krosmoz: the VPS IP is whitelisted (§2.2). About 366 requests a month, in one language (§5.5). `KROSMOZ_USER_AGENT` stays configurable, as a fallback if the whitelist stops covering the bot filter.
- **Every run also checks the almanax** (§5.5). That keeps a single CronJob and the same chart as kaelly-twitter.
- **One transaction** writes the data and the new `game_versions` row. The encyclopedia sees either the whole previous release or the whole new one.

## 4. Project layout

Same structure and style as kaelly-twitter: `application` wires repositories and services, `Run()` does one pass and returns, `main.go` is identical apart from the module path.

```
kaelly-gamedata/
├── .github/workflows/ci.yml        # same as kaelly-twitter (test, lint, docker, helm on tags)
├── .gitignore  .golangci.yml       # copied from kaelly-twitter
├── .template.env
├── Dockerfile                      # same as kaelly-twitter: CGO_ENABLED=0, FROM scratch (§9.3)
├── README.md
├── charts/kaelly-gamedata/         # as kaelly-twitter (cronjob, configmap, secrets) + pvc, cdn deployment/service/ingress (§8)
├── docs/specs/                     # this spec, gamedata-schema.sql
├── main.go
├── application/                    # application.go, types.go
├── models/
│   ├── constants/                  # config.go, constants.go, logs.go, games.go, mappings.go
│   ├── dtos/                       # raw Touch d2o structs, krosmoz almanax
│   ├── entities/                   # gorm entities of gamedata-schema.sql
│   └── mappers/                    # dtos → entities, effect templates, condition parsing
├── repositories/                   # one package per aggregate, types.go + <name>.go
│   ├── almanaxes/
│   ├── imports/                    # game_data_imports, game_data_import_files
│   ├── items/
│   ├── sets/
│   └── gameversions/
├── services/                       # one package per service, types.go (interface + Impl) + <name>.go
│   ├── touch/                      # client data API: config, build check, tables
│   ├── krosmoz/                    # almanax pages, Touch block
│   ├── imports/                    # orchestration: hashes, diff, validation gate, apply
│   ├── renders/                    # set images (image-renderer package)
│   ├── storages/                   # images (public/) and raw snapshots (raw/) on the volume
│   └── news/                       # AMQP news (sets, later items)
└── utils/databases/mysql.go        # copied from kaelly-twitter
```

`application.Run()` follows kaelly-twitter:

```go
func (app *Impl) Run() error {
	if err := app.db.Run(); err != nil {
		return err
	}

	if err := app.broker.Run(); err != nil {
		return err
	}

	return app.importService.Import()
}
```

Configuration (`models/constants/config.go`, same pattern and defaults map as kaelly-twitter):

| Key | Default | Note |
| --- | --- | --- |
| `MYSQL_URL`, `MYSQL_USER`, `MYSQL_PASSWORD`, `MYSQL_DATABASE` | as kaelly-twitter | |
| `RABBITMQ_ADDRESS` | `amqp://localhost:5672` | news |
| `TOUCH_CONFIG_URL` | `https://dt-proxy-production-login.ankama-games.com/config.json` | the only hardcoded host |
| `TOUCH_LANGUAGES` | `fr,en,es,pt` | |
| `KROSMOZ_ALMANAX_URL` | `https://www.krosmoz.com/%s/almanax/%s` | |
| `KROSMOZ_LANGUAGE` | `fr` | the only language crawled (§5.5) |
| `KROSMOZ_USER_AGENT` | same as `TOUCH_USER_AGENT` | VPS IP whitelisted by krosmoz; a browser UA is the fallback (§2.2) |
| `ALMANAX_REFRESH_INTERVAL` | `720h` | §5.5 |
| `MAX_DELETED_RATIO` | `0.05` | validation gate (§5.4) |
| `SNAPSHOT_RETENTION` | `10` | raw snapshots kept for the last N applied imports (§5.2) |
| `STORAGE_PATH` | `/data` | volume mount; `public/` served, `raw/` private (§8) |
| `CDN_BASE_URL` | `https://cdn.kaellybot.com/` | read by the encyclopedia too |
| `HTTP_TIMEOUT`, `TOUCH_USER_AGENT` | `60s`, `KaellyBot-Gamedata/1.0 (+https://github.com/kaellybot/kaelly-gamedata)` | Touch API and static icons (§3) |
| `DRY_RUN` | `false` | download, diff and report, write nothing (§11) |
| `METRIC_PORT`, `LOG_LEVEL`, `PRODUCTION` | as kaelly-twitter | |

## 5. Import

### 5.1 Release detection

Every run downloads everything; **the file hashes decide** whether anything changed (§5.2). At one run every 12h, a conditional request on `script.js` would save little, and downloading every time also catches data changed on Ankama's side without a new build.

1. Read `config.json`, keep `dataUrl` and `assetsUrl`.
2. `GET <dataUrl>/build/script.js`, read `buildVersion`. It labels the import and becomes `game_versions.version` if the import is applied.
3. Start an import with `source = CLIENT`, `trigger = SCHEDULED` (or `MANUAL` when run by hand).

### 5.2 Download and file hashes

For each table and language: download, `sha256` the body, write it to the volume as `raw/touch/<sha256>.json.gz` (content-addressed, so an unchanged table is stored once), record a `game_data_import_files` row.

**If every hash equals the last `APPLIED` import's, the import ends as `UNCHANGED`**: no transform, no diff, no news. Otherwise only the tables whose hash changed, and the tables that depend on them, are transformed (e.g. a new `Effects` re-renders every effect label).

Retention: after an apply, raw objects referenced by none of the last `SNAPSHOT_RETENTION` (10) applied imports are deleted.

### 5.3 Transform and diff

- Map the raw records into the entities of `gamedata-schema.sql`:
  - render effect labels per language from the `Effects` templates (`#1{~1~2 à }#2 Force` with (10, 20) gives `10 à 20 Force`)
  - parse `criteria` into the conditions tree (language-free, element ids)
  - map ids to the conventions already used by the bot (§7)
- `content_hash` = sha256 of the canonical JSON of the **stored** record, all languages included. Fields we don't store (`price`, `appearanceId`…) never produce an "updated".
- Diff by `(id, game)` against the database: created, updated, deleted, per table.

### 5.4 Validation gate

The import is **rejected** (`status = REJECTED`, nothing written, alert sent, §5.7) if any of these holds:

- a table is empty, or fails to parse
- a table loses more than `MAX_DELETED_RATIO` of its rows
- reference records are missing: item 7905 (Laine du Boufcoul), set 1 (Panoplie du Bouftou), almanax calendar 31
- a non-English language has more than 50% of a table's names identical to English. Ankama never omits a language: it silently answers in English (that is what `lang=de` does), so a broken language looks like English, not like a missing file

`game_versions` only moves after a successful apply, so a rejected release is retried on the next run.

### 5.5 Almanax

krosmoz is only used to pick, for each day, **which calendar entry and which offering**. Every text the bot shows (bonus, offering name, in fr/en/es/pt) comes from the client tables. So:

- **One language is crawled** (`KROSMOZ_LANGUAGE`, `fr`): 366 requests instead of 1 464.
- **The 366 month/day pairs are crawled**, not "the next 366 days": 29/02 is taken from the next leap year (2028-02-29), otherwise it is missing three years out of four.

On every run, after game data:

1. **Crawl** if the last applied `KROSMOZ` import is older than `ALMANAX_REFRESH_INTERVAL`. The pages are stored like any raw file (one `game_data_imports` row with `source = KROSMOZ`, one file per crawl).
2. **Join** after a crawl, and also after game data was applied, from the stored krosmoz snapshot (no new crawl). For each day, take the Touch block (§2.2), then:
   - match the bonus text against `almanax_calendar_labels.description` (fr)
   - match the offering on name (`item_labels.name`, fr) **and** icon id (from the offering image URL, §2.2)
   - read the quantity. No kamas reward is shown for Touch (`kamas_reward = NULL`)
3. Diff on `(month, day, game)` with `content_hash`, apply in one transaction.
4. A day that doesn't match its calendar entry or its offering keeps its previous row. The import ends as `PARTIAL` and lists the day in its report and its alert (§5.7), so it can be fixed by hand.

### 5.6 Apply and news

1. Render and upload the images of the sets to render (§8), before the transaction.
2. One transaction: upsert created and updated rows **in batches** (the first import is a few hundred thousand rows, labels × 4 languages), delete deleted rows, update `game_versions`, mark the import `APPLIED` with its report (counts per table).
3. Publish set news with the existing `MapSetNews(game, created, updated, deleted)` shape (`news.set` routing key), now from kaelly-gamedata for Touch. Kaelly-notifier already posts `NEWS_SET` to the shared reporting channel with the bot of the message's game.
4. `NEWS_GAME` (new version) needs nothing here: the encyclopedia publishes it when it sees the new Touch version in `game_versions` (§6).

### 5.7 Alerts

A `REJECTED`, `FAILED` or `PARTIAL` import publishes a `NEWS_IMPORT` message (§9.4): game, import id, source, build version, status, reason. Kaelly-notifier posts it to the shared reporting channel, like `NEWS_SET`. `APPLIED` and `UNCHANGED` imports send nothing.

## 6. Kaelly-encyclopedia, read side

- A Touch source next to the dofusdude one, reading the gamedata tables. `DofusDudeGame` keeps refusing Touch; the switch happens on the source, not on dofusdude.
- **Support per game and per feature**, not just per game: adding `DOFUS_TOUCH` to `GetSupportedGames()` alone would also open `/item` and `/set`. Phase 1 serves almanax requests (day, effects, resources) and the daily almanax news for Touch; item, set and list requests for Touch keep getting a `FAILED` reply until phase 2.
- **Daily almanax news** (`PublishAlmanaxNews`, behind `/config almanax`, needed on day one) reads the Touch almanax from the database.
- `checkGameVersion` reads `game_versions` for Touch instead of calling dofusdude, and publishes `NEWS_GAME` on a change as it does for Dofus.
- The Dofus-only game event handlers skip Touch: `reconcileDofusDudeIDs` (almanax) and `syncSetImages` (sets).
- **Consistent reads**: one answer takes several queries (item, labels, effects, recipe). Run them in one read-only transaction, so an import committing in between can't mix two releases in one answer.
- **Icons**: `https://static.ankama.com/dofus-touch/www/game/items/200/<icon_id>.png` (§2.1). No CDN needed for phase 1.
- **Cache keys carry the release**: `source/game/version/type/...` instead of `source/game/type/...` (`services/sources/stores.go`). Old entries are never read again and expire with the TTL. No flush, and the per-pod TinyLFU cache is covered too.
- Image URL = `CDN_BASE_URL` + `sets.image_key`, `fallback.webp` while `is_current = 0`. Only `is_wearable = 1` sets are served.
- `de` requests for Touch read `en` labels.

## 7. Mappings

| Touch | Stored as | Rule |
| --- | --- | --- |
| `effectId` | `item_effects.effect_id`, `set_bonuses.effect_id` | mapped to match dofusdude (real Dofus ids). **An id with no Dofus equivalent keeps its Touch id and its rendered label**: the bot shows it with no emoji, nothing is hidden |
| `Effects.characteristic` | `characteristic_id` | mapped to match dofusdude, used by the bot for emojis. Unmapped = no emoji |
| `typeId` | `item_types.item_type`, `item_types.equipment_type` | mapped to `amqp.ItemType` / `amqp.EquipmentType`, `NULL` = not served |
| `lang` | `locale` | `amqp.Language`: fr 1, en 2, es 3, pt 5. `de` (4) never written |
| set membership | `set_items` | from `ItemSets.items` (consistent with `Items.itemSetIds` for all items) |
| wearable set | `sets.is_wearable` | every member is equipment and the set has bonuses |

## 8. Set images and CDN

- **Render only wearable sets** (`is_wearable = 1`): the grid templates hold 1 to 25 slots, a 150-cube grouping has no image.
- Member icons are read from the stable static URL (§2.1).
- `render_hash` = sha256 of the sorted member ids, each member's `iconId`, and a renderer version constant. Bumping the constant re-renders every set after a template change. Icon bytes are not hashed: a silent redraw under the same `iconId` is rare, and bumping the constant forces a re-render.
- A set is rendered when created, or when its `render_hash` changed. The image is uploaded **before** the transaction, so the database never points to a missing image.
- A failed render or upload doesn't fail the import: the set keeps `is_current = 0` and the fallback image, and the next run retries it.
- **PNG output**, encoded with the standard library (`image/png`): no cgo, no extra dependency. The Touch icons are PNG already. The existing Dofus images stay `.webp`; the format is part of the key, so both coexist.
- **Immutable keys**: `sets/touch/<id>-<render_hash[:8]>.png`. Discord's media proxy caches embed images by URL, so a new image always gets a new URL. Old objects can be removed after a grace period.
- **The database stores the key, not the URL**. Moving from GitHub to `cdn.kaellybot.com` is a config change.
- **Storage: a volume on the VPS**, behind `services/storages` (`Put(ctx, key, body, contentType)`, `Delete(ctx, key)`) with a filesystem implementation, so another backend can be added later. Constraint: no paid service and no payment method on file (Cloudflare R2 rejected for that reason).
  - One `local-path` PersistentVolumeClaim (k3s is single-node), mounted read-write by the CronJob and read-only by a static file server.
  - `public/` holds the images and is served. `raw/` holds the snapshots and is **not served** (Ankama data, not redistributed).
  - Images are derived data: if the volume is lost, resetting `is_current = 0` re-renders them all on the next run. No backup needed for `public/`.
- **Serving: `cdn.kaellybot.com`**, a static file server (nginx) behind the k3s ingress, with **Cloudflare's free plan as a proxy** (DNS proxy and caching only: no R2, no payment method).
  - Keys are immutable, so responses carry `Cache-Control: public, max-age=31536000, immutable`: Cloudflare and Discord cache them, the VPS barely serves anything.
  - TLS between Cloudflare and the VPS with a Cloudflare Origin Certificate in a Kubernetes secret, SSL mode "Full (strict)".
  - The VPS firewall only accepts 80/443 from Cloudflare's IP ranges, so the origin isn't reachable directly.
- kaelly-cdn (GitHub) stays for hand-made assets (logos, tutorials).

## 9. Changes per project

### 9.1 kaelly-gamedata — new

Everything above. Phase 1 (§12) only needs `Items`, `ItemTypes`, `AlmanaxCalendars` and krosmoz.

### 9.2 Kaelly-encyclopedia

§6. The Dofus flow is unchanged.

### 9.3 image-renderer

- Extract the grid composition from `cmd/sets/grid/main.go` into a package that takes neutral input (`[]{slot, image}`) and **returns an `image.Image`**, with no dodugo dependency and no encoding. kaelly-gamedata imports it and encodes PNG itself; the `cmd` keeps working for Dofus.
- The package must not import `github.com/chai2010/webp`, which is cgo: kaelly-gamedata builds with `CGO_ENABLED=0` on `scratch`. The WebP encoding stays in the `cmd` only.

### 9.4 kaelly-amqp and Kaelly-notifier — import alerts

Needed from phase 1 (§5.7). In kaelly-amqp, a new news type, sketch:

```proto
// RabbitMQMessage.Type gains NEWS_IMPORT = 806, RabbitMQMessage gains newsImportMessage = 606.
// Sent by kaelly-gamedata when an import is not fully applied.
message NewsImportMessage
{
    int64  importId = 1; // game_data_imports.id, whose report holds the details
    Source source   = 2;
    string version  = 3; // build version, empty for KROSMOZ
    Status status   = 4;
    string reason   = 5; // gate rule hit, error, or unmatched almanax days

    enum Source
    {
        UNKNOWN_SOURCE = 0;
        CLIENT         = 1;
        KROSMOZ        = 2;
    }

    enum Status
    {
        UNKNOWN_STATUS = 0;
        REJECTED       = 1;
        FAILED         = 2;
        PARTIAL        = 3; // applied, but some almanax days unmatched
    }
}
```

Routing key `news.import`. The game is `RabbitMQMessage.game`, as for every message. Kaelly-notifier posts it to the shared reporting channel with the session of that game, like `NEWS_SET`.

### 9.5 kaelly-discord, kaellytouch-discord

No change for phase 1. The almanax request and answer are already game-aware.

### 9.6 Infrastructure (phase 3 only)

Nothing on the k3s is exposed today. Serving `cdn.kaellybot.com` needs:

1. **DNS**: `kaellybot.com` on Cloudflare (free plan), `cdn` record pointing to the VPS, proxied.
2. **Ingress**: k3s ships Traefik as its ingress controller, listening on 80/443, unless it was disabled at install (`kubectl -n kube-system get svc traefik`). A standard `Ingress` in the kaelly-gamedata chart routes `cdn.kaellybot.com` to the nginx service; no Traefik configuration file is needed. If Traefik was disabled, it has to be re-enabled (or another ingress controller installed).
3. **TLS**: a Cloudflare Origin Certificate stored as a Kubernetes TLS secret, referenced by the `Ingress`. Cloudflare SSL mode "Full (strict)".
4. **Firewall**: 80/443 open on the VPS for Cloudflare's IP ranges only.

## 10. SQL reference

[`gamedata-schema.sql`](gamedata-schema.sql), checked on MySQL 8.4 on top of `kaelly-amqp/docs/specs/kaellybot.sql`. Summary:

| Area | Tables |
| --- | --- |
| Release tracking | `game_data_imports` (client and krosmoz downloads), `game_data_import_files`, existing `game_versions` |
| Item types | `item_types`, `item_type_labels` |
| Items | `items`, `item_labels`, `item_effects`, `item_effect_labels`, `item_recipes` |
| Sets | existing `sets` (extended: `level`, `is_cosmetic`, `content_hash`, `render_hash`, `image_key`, `is_wearable`), `set_labels`, `set_items`, `set_bonuses`, `set_bonus_labels` |
| Almanax | `almanax_calendars`, `almanax_calendar_labels`, existing `almanaxes` (extended: `calendar_id`, `offering_item_id`, `offering_quantity`, `kamas_reward`, `content_hash`) |

The existing tables are extended, not replaced: the Dofus rows keep working with their current columns (`sets.hash`, `sets.icon`, `almanaxes.dofus_dude_effect_id`), which become legacy once Dofus moves over.

The schema is designed for Dofus too: every key includes `game`. Moving Dofus off dofusdude is a later spec.

## 11. Verification

- **Fixtures**: unit tests run on raw files saved from build 1.74.8 (mappers, effect templates, diff, gate).
- **Known values** (checked on 2026-10-16):
  - 2026-10-16 → calendar 31, offering item 7905 × 9
  - 2026-12-25 → calendar 103 ("Cadeaux surprises"), offering item 12132 × 4 ("Cadeau de Nowel", icon 89002; items 8337 and 12816 share the name)
  - item 2469 Gelano: level 60, in set "Panoplie Gelax"
  - set 1 Panoplie du Bouftou: 7 members
  - 29/02 has a row after the first crawl (taken from 2028-02-29)
  - **from the VPS**, a krosmoz request with the KaellyBot User-Agent gets 200 (the whitelist covers the bot filter). If it gets 403, set `KROSMOZ_USER_AGENT` to a browser UA and raise it with krosmoz
- **`DRY_RUN=true`** on the cluster before the first real run: full download, diff and report, nothing written.
- After the first apply: `game_versions` holds `1.74.8` for `DOFUS_TOUCH`, row counts match §2.1, `/almanax` answers on kaellytouch-discord.

## 12. Rollout

1. **Phase 1 — almanax** (kaellytouch-discord day one): `ItemTypes`, `Items` (offerings and their icons), `AlmanaxCalendars`, krosmoz. Encyclopedia: Touch source for almanax and the daily almanax news only. kaelly-amqp and Kaelly-notifier: `NEWS_IMPORT`.
2. **Phase 2 — items and sets**: `Effects`, `ItemSets`, `Recipes`, effect templates, conditions, mappings (§7). Unlocks `/item` and `/set` for Touch.
3. **Phase 3 — set images**: image-renderer as a library, storage, CDN.
4. **Later — Dofus**: same pipeline for Dofus, dofusdude removed from the encyclopedia.

## 13. Decisions (2026-10-16)

| Question | Decision |
| --- | --- |
| CDN | Volume on the VPS, served by nginx at `cdn.kaellybot.com` behind Cloudflare's free proxy. R2 rejected: it needs a payment method on file (§8) |
| Touch effect ids without a Dofus equivalent | Keep the Touch id and its label, no emoji (§7) |
| Almanax offering tie-break | Name and icon id, the icon id read from krosmoz's offering image (§2.2) |
| Almanax kamas reward | Not shown on krosmoz for Touch: `NULL` |
| Validation gate | The rules of §5.4 as proposed |
| Schedule | Every 12 hours |
| Release detection | Download everything on every run, file hashes decide. No ETag table, no daily trigger (§5.1) |
| Raw snapshots retention | Last 10 applied imports (§5.2) |
| Icon redrawn under the same `iconId` | Not detected: bump the renderer version to force a re-render (§8) |
| Terms of use | Proceed with a low footprint: 12h runs, KaellyBot User-Agent with a contact URL, raw data never republished (§3) |
| krosmoz access | The KaellyBot VPS IP is whitelisted by krosmoz (agreed with them). KaellyBot User-Agent there too; a browser UA only as a configured fallback (§2.2) |
| krosmoz crawl | One language, the 366 month/day pairs (29/02 from the next leap year), monthly; re-join from the snapshot after a game update (§5.5) |
| Alerts | `NEWS_IMPORT` to the shared reporting channel on rejected / failed imports and unmatched almanax days (§5.7, §9.4) |
| Icons | Ankama's stable static URL, no mirror (§2.1) |

No open question left. Infrastructure prerequisites for phase 3 (DNS on Cloudflare, Origin Certificate, firewall, ingress) are listed in §9.6.

## 14. Future work (not in this spec)

- Dofus import into the same tables, replacing dofusdude.
- Servers, jobs and alignment orders from the client tables (`Servers`, `Jobs`, `AlignmentOrder`). They carry curated columns today (emojis, icons, colors), so they stay manual.
- Encyclopedia loading a release in memory instead of Redis (14k items fit easily).
- Monsters.
