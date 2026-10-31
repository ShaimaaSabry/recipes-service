-- +goose Up

-- Imports of the data the app kept on the phone before accounts, by the ID the app gave each
-- one, so an import resent after a dropped connection returns the first one's result instead of
-- importing twice. result is what the app needs afterwards (the recipes' new IDs, and notes).
CREATE TABLE local_imports (
    owner_id    TEXT        NOT NULL,
    import_id   UUID        NOT NULL,
    result      JSONB,
    imported_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (owner_id, import_id)
);

-- +goose Down
DROP TABLE local_imports;
