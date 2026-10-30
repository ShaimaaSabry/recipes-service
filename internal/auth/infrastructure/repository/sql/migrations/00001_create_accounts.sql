-- +goose Up
CREATE TABLE users (
    id         UUID        PRIMARY KEY,
    email      TEXT        NOT NULL DEFAULT '',
    name       TEXT        NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- How a user signs in: the provider's permanent ID for them. Apple and Google sign-ins with the
-- same email are separate accounts; emails aren't trusted to prove they're the same person.
CREATE TABLE user_identities (
    provider   TEXT        NOT NULL CHECK (provider IN ('apple', 'google', 'dev')),
    subject    TEXT        NOT NULL,
    user_id    UUID        NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    email      TEXT        NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (provider, subject)
);
CREATE INDEX user_identities_user_id ON user_identities (user_id);

-- Only the SHA-256 of each refresh token. A family is one sign-in's chain of tokens.
CREATE TABLE refresh_tokens (
    id         UUID        PRIMARY KEY,
    user_id    UUID        NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    family_id  UUID        NOT NULL,
    token_hash BYTEA       NOT NULL UNIQUE,
    expires_at TIMESTAMPTZ NOT NULL,
    used_at    TIMESTAMPTZ,
    revoked_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX refresh_tokens_family_id ON refresh_tokens (family_id);
CREATE INDEX refresh_tokens_user_id ON refresh_tokens (user_id);

-- +goose Down
DROP TABLE refresh_tokens;
DROP TABLE user_identities;
DROP TABLE users;
