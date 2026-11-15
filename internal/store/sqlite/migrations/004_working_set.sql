-- +goose Up

CREATE TABLE container_pins (
    container_id TEXT PRIMARY KEY REFERENCES containers(id) ON DELETE CASCADE,
    position     INTEGER NOT NULL,
    created_at   INTEGER NOT NULL
);

CREATE TABLE container_recents (
    container_id   TEXT PRIMARY KEY REFERENCES containers(id) ON DELETE CASCADE,
    last_opened_at INTEGER NOT NULL,
    open_count     INTEGER NOT NULL DEFAULT 1
);

CREATE INDEX idx_container_recents_last_opened ON container_recents(last_opened_at DESC);

-- +goose Down

DROP INDEX idx_container_recents_last_opened;
DROP TABLE container_recents;
DROP TABLE container_pins;
