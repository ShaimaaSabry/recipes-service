-- +goose Up

-- Units are the same for everyone, so they live here rather than per account. A unit converts to
-- any other with the same base unit: 1 cup = 240 ml and 1 tbsp = 15 ml, so 1 cup = 16 tbsp. Units
-- without a base (pack, head…) only convert through an item's own rules ("Butter: 1 pack = 200 g").
CREATE TABLE units (
    id           UUID     PRIMARY KEY,
    code         TEXT     NOT NULL UNIQUE,
    name         TEXT     NOT NULL,
    base_unit_id UUID     REFERENCES units (id),
    base_size    NUMERIC  CHECK (base_size > 0),
    -- The grocery list rounds amounts in this unit up to whole numbers (2 packs, not 1.25).
    sold_whole   BOOLEAN  NOT NULL,
    -- The order pickers list units in.
    position     SMALLINT NOT NULL UNIQUE,
    CHECK ((base_unit_id IS NULL) = (base_size IS NULL))
);

-- A base unit is its own base, with size 1. A unit is added with a new migration.
INSERT INTO units (id, code, name, base_unit_id, base_size, sold_whole, position) VALUES
    ('01a19182-b210-7b2f-bc8a-67b86218894d', 'piece',    'piece',    '01a19182-b210-7b2f-bc8a-67b86218894d', 1,    true,  1),
    ('01a19182-b210-7c67-ac5c-4a1849f479a7', 'dozen',    'dozen',    '01a19182-b210-7b2f-bc8a-67b86218894d', 12,   true,  2),
    ('01a19182-b210-7c9a-a530-ecfff12af6fc', 'kg',       'kg',       '01a19182-b210-7cf8-901a-047bd216b775', 1000, false, 3),
    ('01a19182-b210-7cf8-901a-047bd216b775', 'g',        'g',        '01a19182-b210-7cf8-901a-047bd216b775', 1,    false, 4),
    ('01a19182-b210-7d1b-8ee3-db9a0d4867ce', 'l',        'L',        '01a19182-b210-7d79-985a-0a7c2e179f60', 1000, false, 5),
    ('01a19182-b210-7d79-985a-0a7c2e179f60', 'ml',       'ml',       '01a19182-b210-7d79-985a-0a7c2e179f60', 1,    false, 6),
    ('01a19182-b210-7dee-bea8-1180a3a19102', 'pack',     'pack',     NULL,                                   NULL, true,  7),
    ('01a19182-b210-7e11-b9cf-92464dda6d94', 'box',      'box',      NULL,                                   NULL, true,  8),
    ('01a19182-b210-7e6f-a9eb-1e9255f2a540', 'bottle',   'bottle',   NULL,                                   NULL, true,  9),
    ('01a19182-b210-7e8a-875f-e3ee64a935b8', 'can',      'can',      NULL,                                   NULL, true,  10),
    ('01a19182-b210-7ef8-928f-977ccf0fb6ff', 'jar',      'jar',      NULL,                                   NULL, true,  11),
    ('01a19182-b211-700b-9cd9-5d02f5b5a919', 'bunch',    'bunch',    NULL,                                   NULL, true,  12),
    ('01a19182-b211-707d-b94d-4f94e7337f42', 'cup',      'cup',      '01a19182-b210-7d79-985a-0a7c2e179f60', 240,  false, 13),
    ('01a19182-b211-70d2-8711-252b6014aa8b', 'tbsp',     'tbsp',     '01a19182-b210-7d79-985a-0a7c2e179f60', 15,   false, 14),
    ('01a19182-b211-7130-80a9-cacc6bf8cac4', 'tsp',      'tsp',      '01a19182-b210-7d79-985a-0a7c2e179f60', 5,    false, 15),
    ('01a19182-b211-7186-87e6-c6c27bb00fb6', 'pinch',    'pinch',    NULL,                                   NULL, true,  16),
    ('01a19182-b211-71a5-bceb-3afedb0ab8dd', 'sprinkle', 'sprinkle', NULL,                                   NULL, true,  17),
    ('01a19182-b211-7207-a87a-4bec8f271f32', 'head',     'head',     NULL,                                   NULL, true,  18),
    ('01a19182-b211-7226-8fd7-e7f5ebfa367e', 'clove',    'clove',    NULL,                                   NULL, true,  19);

-- Each account's own categories and grocery items. Starter ones (source 'system') are copied in
-- from the starter data file when the account is set up, and remember which entry they came from
-- (system_key) so later changes to that file can be applied to them. They can be hidden, not
-- deleted. UNIQUE (owner_id, id) is what the composite foreign keys below point at, so one
-- account's rows can never reference another's.
CREATE TABLE grocery_categories (
    id         UUID        PRIMARY KEY,
    owner_id   TEXT        NOT NULL,
    name       TEXT        NOT NULL,
    emoji      TEXT        NOT NULL,
    source     TEXT        NOT NULL CHECK (source IN ('system', 'user')),
    system_key TEXT,
    hidden     BOOLEAN     NOT NULL DEFAULT false,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (owner_id, id),
    CHECK ((source = 'system') = (system_key IS NOT NULL))
);
CREATE UNIQUE INDEX grocery_categories_name ON grocery_categories (owner_id, lower(name));
CREATE UNIQUE INDEX grocery_categories_system_key ON grocery_categories (owner_id, system_key);

CREATE TABLE grocery_items (
    id          UUID        PRIMARY KEY,
    owner_id    TEXT        NOT NULL,
    name        TEXT        NOT NULL,
    emoji       TEXT        NOT NULL,
    category_id UUID        NOT NULL,
    source      TEXT        NOT NULL CHECK (source IN ('system', 'user')),
    system_key  TEXT,
    hidden      BOOLEAN     NOT NULL DEFAULT false,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (owner_id, id),
    FOREIGN KEY (owner_id, category_id) REFERENCES grocery_categories (owner_id, id),
    CHECK ((source = 'system') = (system_key IS NOT NULL))
);
CREATE UNIQUE INDEX grocery_items_name ON grocery_items (owner_id, lower(name));
CREATE UNIQUE INDEX grocery_items_system_key ON grocery_items (owner_id, system_key);
CREATE INDEX grocery_items_category_id ON grocery_items (category_id);

-- The units an item is bought in and cooked in, in the order the app offers them. A unit can be
-- both (tomatoes are bought and cooked by the piece): one row each.
CREATE TABLE grocery_item_units (
    item_id  UUID     NOT NULL REFERENCES grocery_items (id) ON DELETE CASCADE,
    purpose  TEXT     NOT NULL CHECK (purpose IN ('buy', 'cook')),
    unit_id  UUID     NOT NULL REFERENCES units (id),
    position SMALLINT NOT NULL,
    PRIMARY KEY (item_id, purpose, unit_id),
    UNIQUE (item_id, purpose, position)
);

-- An item's own conversions: 1 from_unit = factor to_unit, for this item only ("Garlic: 1 head =
-- 10 clove"). Standard ones (g ↔ kg) follow from the units and are never stored. A conversion may
-- name a unit the item isn't bought or cooked in, as a stepping stone (1 piece = 150 g, for
-- something bought in kg).
CREATE TABLE grocery_item_conversions (
    item_id      UUID     NOT NULL REFERENCES grocery_items (id) ON DELETE CASCADE,
    from_unit_id UUID     NOT NULL REFERENCES units (id),
    to_unit_id   UUID     NOT NULL REFERENCES units (id),
    factor       NUMERIC  NOT NULL CHECK (factor > 0),
    position     SMALLINT NOT NULL,
    PRIMARY KEY (item_id, from_unit_id, to_unit_id),
    CHECK (from_unit_id <> to_unit_id)
);
-- One conversion per pair of units, whichever way round it's written.
CREATE UNIQUE INDEX grocery_item_conversions_pair
    ON grocery_item_conversions (item_id, LEAST(from_unit_id, to_unit_id), GREATEST(from_unit_id, to_unit_id));

-- Accounts whose starter categories and items have been copied in. Copying inserts this row in
-- the same transaction, so two sign-ins at once can't copy them twice.
CREATE TABLE starter_data_copies (
    owner_id  TEXT        PRIMARY KEY,
    copied_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- +goose Down
DROP TABLE starter_data_copies;
DROP TABLE grocery_item_conversions;
DROP TABLE grocery_item_units;
DROP TABLE grocery_items;
DROP TABLE grocery_categories;
DROP TABLE units;
