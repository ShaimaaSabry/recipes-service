-- +goose Up
CREATE TABLE recipes (
    owner_id   TEXT        NOT NULL,
    id         TEXT        NOT NULL,
    name       TEXT        NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (owner_id, id)
);

-- +goose Down
DROP TABLE recipes;
