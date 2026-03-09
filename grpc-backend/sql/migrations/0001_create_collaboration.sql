-- +goose Up
CREATE SCHEMA IF NOT EXISTS collaboration;

CREATE TABLE collaboration.space (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name        TEXT NOT NULL,
    key         TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    status      INT NOT NULL DEFAULT 1,
    visibility  INT NOT NULL DEFAULT 1,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at  TIMESTAMPTZ
);

CREATE UNIQUE INDEX space_key_unique ON collaboration.space (key) WHERE deleted_at IS NULL;

CREATE TABLE collaboration.content (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    space_id   UUID NOT NULL REFERENCES collaboration.space(id) ON DELETE CASCADE,
    title      TEXT NOT NULL,
    body       TEXT NOT NULL,
    status     INT NOT NULL DEFAULT 1,
    tags       TEXT[] NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at TIMESTAMPTZ
);

CREATE INDEX content_space_id_idx ON collaboration.content (space_id) WHERE deleted_at IS NULL;

-- +goose Down
DROP TABLE IF EXISTS collaboration.content;
DROP TABLE IF EXISTS collaboration.space;
DROP SCHEMA IF EXISTS collaboration;
