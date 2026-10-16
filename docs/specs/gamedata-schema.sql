-- Game data schema, DRAFT (2026-10-16). See dofus-touch-data.md.
-- Written by kaelly-gamedata (CronJob), read by kaelly-encyclopedia.
-- Conventions follow kaelly-amqp/docs/specs/kaellybot.sql:
--   * every row carries `game` (amqp.Game: 1 DOFUS, 2 DOFUS_TOUCH), since Ankama ids collide across games
--   * translations live in *_labels tables keyed by `locale` (amqp.Language: 1 FR, 2 EN, 3 ES, 4 DE, 5 PT)
--   * Touch only has FR/EN/ES/PT; DE is never written, the encyclopedia falls back to EN
--   * `content_hash` = sha256 of the record as stored (all locales included), used for the diff
-- Only the current release lives here; history lives in the raw snapshots (raw/ on the volume, not served).

-- ---------------------------------------------------------------------------
-- Release tracking
-- ---------------------------------------------------------------------------

-- One row per download: the client tables on every run, the krosmoz almanax
-- when its refresh is due. The last KROSMOZ row tells when the almanax was crawled.
CREATE TABLE `game_data_imports` (
  `id`             bigint NOT NULL AUTO_INCREMENT,
  `game`           int NOT NULL,
  `source`         enum('CLIENT','KROSMOZ') NOT NULL,
  `build_version`  varchar(50) DEFAULT NULL,      -- window.buildVersion, e.g. 1.74.8 (CLIENT only)
  `assets_version` varchar(100) DEFAULT NULL,     -- assetsUrl suffix, e.g. 3.3.9_znDI... (CLIENT only)
  `trigger`        enum('SCHEDULED','MANUAL') NOT NULL,
  `status`         enum('RUNNING','APPLIED','PARTIAL','UNCHANGED','REJECTED','FAILED') NOT NULL,
  `started_at`     datetime NOT NULL,
  `finished_at`    datetime DEFAULT NULL,
  `report`         json DEFAULT NULL,             -- created/updated/deleted per table, rejection reason
  PRIMARY KEY (`id`),
  KEY `idx_imports_game_status` (`game`, `source`, `status`, `started_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

-- One row per downloaded file. "Has the data changed?" = do these hashes differ
-- from those of the last APPLIED import.
CREATE TABLE `game_data_import_files` (
  `import_id`  bigint NOT NULL,
  `class`      varchar(100) NOT NULL,             -- Items, ItemSets, Effects, ..., or krosmoz/almanax
  `locale`     int NOT NULL,                      -- 0 (ANY) for language-free files
  `sha256`     char(64) NOT NULL,
  `size_bytes` int NOT NULL,
  `object_key` varchar(250) NOT NULL,             -- raw/touch/<sha256>.json.gz (content-addressed)
  PRIMARY KEY (`import_id`, `class`, `locale`),
  CONSTRAINT `fk_import_files_import` FOREIGN KEY (`import_id`)
    REFERENCES `game_data_imports` (`id`) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

-- EXISTING: `game_versions` (id = game, version) stays the "current release"
-- pointer the encyclopedia reads. Updated in the same transaction as the data.

-- ---------------------------------------------------------------------------
-- Item types
-- ---------------------------------------------------------------------------

CREATE TABLE `item_types` (
  `id`             int NOT NULL,                  -- Ankama typeId (243 in Touch)
  `game`           int NOT NULL,
  `super_type_id`  int NOT NULL,
  `item_type`      int DEFAULT NULL,              -- mapped to amqp.ItemType, NULL = type not served
  `equipment_type` int DEFAULT NULL,              -- mapped to amqp.EquipmentType, NULL if not an equipment
  `content_hash`   char(64) NOT NULL,
  PRIMARY KEY (`id`, `game`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE `item_type_labels` (
  `item_type_id` int NOT NULL,
  `game`         int NOT NULL,
  `locale`       int NOT NULL,
  `label`        varchar(250) NOT NULL,
  PRIMARY KEY (`item_type_id`, `game`, `locale`),
  CONSTRAINT `fk_item_type_labels` FOREIGN KEY (`item_type_id`, `game`)
    REFERENCES `item_types` (`id`, `game`) ON DELETE CASCADE ON UPDATE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

-- ---------------------------------------------------------------------------
-- Items (equipment, weapons, resources, consumables...)
-- ---------------------------------------------------------------------------

CREATE TABLE `items` (
  `id`                int NOT NULL,               -- Ankama item id (13 679 in Touch)
  `game`              int NOT NULL,
  `type_id`           int NOT NULL,
  `level`             int NOT NULL,
  `icon_id`           int NOT NULL,               -- <assetsUrl>/gfx/items/<icon_id>.png
  `pods`              int NOT NULL,               -- realWeight
  `criteria`          varchar(500) DEFAULT NULL,  -- raw, e.g. CS>20&CV>6
  `conditions`        json DEFAULT NULL,          -- parsed tree, language-free (element ids)
  -- weapon-only, NULL otherwise
  `ap_cost`           int DEFAULT NULL,
  `min_range`         int DEFAULT NULL,
  `max_range`         int DEFAULT NULL,
  `max_cast_per_turn` int DEFAULT NULL,
  `critical_rate`     int DEFAULT NULL,           -- criticalHitProbability
  `critical_bonus`    int DEFAULT NULL,
  `two_handed`        tinyint(1) DEFAULT NULL,
  `content_hash`      char(64) NOT NULL,
  PRIMARY KEY (`id`, `game`),
  KEY `idx_items_type` (`game`, `type_id`),
  CONSTRAINT `fk_items_type` FOREIGN KEY (`type_id`, `game`)
    REFERENCES `item_types` (`id`, `game`) ON DELETE RESTRICT
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

-- _ai_ci collation: LIKE is case- and accent-insensitive ("gelano" finds "Gélano").
-- About 55k rows for Touch (13.7k x 4 locales): a %LIKE% scan stays cheap.
CREATE TABLE `item_labels` (
  `item_id`     int NOT NULL,
  `game`        int NOT NULL,
  `locale`      int NOT NULL,
  `name`        varchar(250) NOT NULL,
  `description` text,
  PRIMARY KEY (`item_id`, `game`, `locale`),
  KEY `idx_item_labels_name` (`game`, `locale`, `name`),
  CONSTRAINT `fk_item_labels` FOREIGN KEY (`item_id`, `game`)
    REFERENCES `items` (`id`, `game`) ON DELETE CASCADE ON UPDATE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE `item_effects` (
  `item_id`           int NOT NULL,
  `game`              int NOT NULL,
  `position`          int NOT NULL,               -- order in possibleEffects
  `effect_id`         int NOT NULL,               -- Ankama effectId, mapped to match dofusdude (real Dofus ids)
  `characteristic_id` int DEFAULT NULL,           -- Effects.characteristic, mapped to match dofusdude, for emoji mapping
  `min_value`         int NOT NULL,               -- diceNum
  `max_value`         int NOT NULL,               -- diceSide (or = min)
  `is_weapon_effect`  tinyint(1) NOT NULL,        -- damage lines vs bonus lines
  PRIMARY KEY (`item_id`, `game`, `position`),
  CONSTRAINT `fk_item_effects` FOREIGN KEY (`item_id`, `game`)
    REFERENCES `items` (`id`, `game`) ON DELETE CASCADE ON UPDATE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

-- Rendered by the importer from the Effects templates:
-- "#1{~1~2 à }#2 Force" + (10, 20) -> "10 à 20 Force"
CREATE TABLE `item_effect_labels` (
  `item_id`  int NOT NULL,
  `game`     int NOT NULL,
  `position` int NOT NULL,
  `locale`   int NOT NULL,
  `label`    varchar(250) NOT NULL,
  PRIMARY KEY (`item_id`, `game`, `position`, `locale`),
  CONSTRAINT `fk_item_effect_labels` FOREIGN KEY (`item_id`, `game`, `position`)
    REFERENCES `item_effects` (`item_id`, `game`, `position`) ON DELETE CASCADE ON UPDATE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

-- From Recipes (4 338 in Touch): resultId -> ingredientIds / quantities (jobId/jobName ignored)
CREATE TABLE `item_recipes` (
  `item_id`       int NOT NULL,
  `game`          int NOT NULL,
  `position`      int NOT NULL,
  `ingredient_id` int NOT NULL,
  `quantity`      int NOT NULL,
  PRIMARY KEY (`item_id`, `game`, `position`),
  CONSTRAINT `fk_item_recipes_item` FOREIGN KEY (`item_id`, `game`)
    REFERENCES `items` (`id`, `game`) ON DELETE CASCADE ON UPDATE CASCADE,
  CONSTRAINT `fk_item_recipes_ingredient` FOREIGN KEY (`ingredient_id`, `game`)
    REFERENCES `items` (`id`, `game`) ON DELETE RESTRICT
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

-- ---------------------------------------------------------------------------
-- Sets
-- ---------------------------------------------------------------------------

-- EXISTING table, extended. Today's Dofus rows (id, game, hash, icon, is_current)
-- keep working; the new columns are NULL for Dofus until the Dofus import moves over.
ALTER TABLE `sets`
  ADD COLUMN `level`        int DEFAULT NULL,
  ADD COLUMN `is_cosmetic`  tinyint(1) DEFAULT NULL,
  ADD COLUMN `content_hash` char(64) DEFAULT NULL,      -- data diff
  ADD COLUMN `render_hash`  char(64) DEFAULT NULL,      -- item ids + icon ids + renderer version
  ADD COLUMN `image_key`    varchar(250) DEFAULT NULL,  -- sets/touch/<id>-<hash8>.png, URL = CDN base + key
  ADD COLUMN `is_wearable`  tinyint(1) NOT NULL DEFAULT 1;
-- Semantics: is_current = 1 <=> image_key points to an image rendered from render_hash.
-- is_wearable = 1 <=> every member is an equipment and the set has bonuses. Only those
-- are served by /set and rendered. Touch also stores groupings like "Ensemble de Cubes Air"
-- (150 cubes) or "Succès 9 cubes communs" (no bonus) in ItemSets: is_wearable = 0.
-- DEFAULT 1 keeps today's Dofus rows served; the importer sets it explicitly per game.
-- `hash` and `icon` become legacy (Dofus flow), dropped once the Dofus import moves over.

CREATE TABLE `set_labels` (
  `set_id` int NOT NULL,
  `game`   int NOT NULL,
  `locale` int NOT NULL,
  `name`   varchar(250) NOT NULL,
  PRIMARY KEY (`set_id`, `game`, `locale`),
  KEY `idx_set_labels_name` (`game`, `locale`, `name`),
  CONSTRAINT `fk_set_labels` FOREIGN KEY (`set_id`, `game`)
    REFERENCES `sets` (`id`, `game`) ON DELETE CASCADE ON UPDATE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

-- Many-to-many: 807 Touch items belong to 2..6 sets (itemSetIds). None is an equipment:
-- all are cubes, spell gems or breloques, listed both in their "Ensemble" and in
-- "Succès" groupings. Equipment is always in at most one set.
CREATE TABLE `set_items` (
  `set_id`   int NOT NULL,
  `game`     int NOT NULL,
  `item_id`  int NOT NULL,
  `position` int NOT NULL,                        -- order in ItemSets.items
  PRIMARY KEY (`set_id`, `game`, `item_id`),
  KEY `idx_set_items_item` (`game`, `item_id`),
  CONSTRAINT `fk_set_items_set` FOREIGN KEY (`set_id`, `game`)
    REFERENCES `sets` (`id`, `game`) ON DELETE CASCADE ON UPDATE CASCADE,
  CONSTRAINT `fk_set_items_item` FOREIGN KEY (`item_id`, `game`)
    REFERENCES `items` (`id`, `game`) ON DELETE CASCADE ON UPDATE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

-- ItemSets.effects[n] = bonus when n items are worn
CREATE TABLE `set_bonuses` (
  `set_id`            int NOT NULL,
  `game`              int NOT NULL,
  `item_count`        int NOT NULL,
  `position`          int NOT NULL,
  `effect_id`         int NOT NULL,
  `characteristic_id` int DEFAULT NULL,
  `value`             int NOT NULL,
  PRIMARY KEY (`set_id`, `game`, `item_count`, `position`),
  CONSTRAINT `fk_set_bonuses` FOREIGN KEY (`set_id`, `game`)
    REFERENCES `sets` (`id`, `game`) ON DELETE CASCADE ON UPDATE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE `set_bonus_labels` (
  `set_id`     int NOT NULL,
  `game`       int NOT NULL,
  `item_count` int NOT NULL,
  `position`   int NOT NULL,
  `locale`     int NOT NULL,
  `label`      varchar(250) NOT NULL,
  PRIMARY KEY (`set_id`, `game`, `item_count`, `position`, `locale`),
  CONSTRAINT `fk_set_bonus_labels` FOREIGN KEY (`set_id`, `game`, `item_count`, `position`)
    REFERENCES `set_bonuses` (`set_id`, `game`, `item_count`, `position`) ON DELETE CASCADE ON UPDATE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

-- ---------------------------------------------------------------------------
-- Almanax
-- ---------------------------------------------------------------------------

-- From the client's AlmanaxCalendars (378 in Touch): the bonuses, without dates.
CREATE TABLE `almanax_calendars` (
  `id`           int NOT NULL,
  `game`         int NOT NULL,
  `npc_id`       int NOT NULL,
  `content_hash` char(64) NOT NULL,
  PRIMARY KEY (`id`, `game`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

-- nameId = effect name ("Butin"), searched by /almanax effects; descId = bonus text
CREATE TABLE `almanax_calendar_labels` (
  `calendar_id` int NOT NULL,
  `game`        int NOT NULL,
  `locale`      int NOT NULL,
  `name`        varchar(250) NOT NULL,
  `description` text NOT NULL,
  PRIMARY KEY (`calendar_id`, `game`, `locale`),
  KEY `idx_almanax_calendar_labels_name` (`game`, `locale`, `name`),
  CONSTRAINT `fk_almanax_calendar_labels` FOREIGN KEY (`calendar_id`, `game`)
    REFERENCES `almanax_calendars` (`id`, `game`) ON DELETE CASCADE ON UPDATE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

-- EXISTING table (month, day, game, dofus_dude_effect_id), extended for Touch.
-- Filled by the krosmoz crawl, joined to the client data:
--   bonus text      -> almanax_calendars via almanax_calendar_labels.description
--   offering name   -> items via item_labels.name
ALTER TABLE `almanaxes`
  MODIFY COLUMN `dofus_dude_effect_id` varchar(100) DEFAULT NULL,  -- Dofus only
  ADD COLUMN `calendar_id`       int DEFAULT NULL,                 -- Touch
  ADD COLUMN `offering_item_id`  int DEFAULT NULL,
  ADD COLUMN `offering_quantity` int DEFAULT NULL,
  ADD COLUMN `kamas_reward`      int DEFAULT NULL,                 -- best effort, NULL if krosmoz doesn't expose it
  ADD COLUMN `content_hash`      char(64) DEFAULT NULL,
  ADD CONSTRAINT `fk_almanaxes_calendar` FOREIGN KEY (`calendar_id`, `game`)
    REFERENCES `almanax_calendars` (`id`, `game`) ON DELETE RESTRICT,
  ADD CONSTRAINT `fk_almanaxes_offering` FOREIGN KEY (`offering_item_id`, `game`)
    REFERENCES `items` (`id`, `game`) ON DELETE RESTRICT;
