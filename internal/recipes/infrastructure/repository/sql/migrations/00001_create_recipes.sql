-- +goose Up
CREATE TABLE recipes (
    id         UUID        PRIMARY KEY,
    owner_id   TEXT        NOT NULL,
    name       TEXT        NOT NULL,
    -- How many people the ingredient amounts are for.
    servings   SMALLINT    NOT NULL CHECK (servings BETWEEN 1 AND 99),
    -- The photo's key in the photos bucket; NULL for a recipe without one.
    photo_key  TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    -- What the composite foreign keys point at, so one account's rows never reference another's.
    UNIQUE (owner_id, id)
);

-- +goose Down
DROP TABLE recipes;
