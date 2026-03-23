-- +goose Up

CREATE TABLE tags (
    id          TEXT PRIMARY KEY,
    parent_id   TEXT REFERENCES tags(id),
    name        TEXT NOT NULL,
    color       TEXT NOT NULL DEFAULT '',
    icon        TEXT NOT NULL DEFAULT '',
    created_at  DATETIME NOT NULL DEFAULT (datetime('now')),
    updated_at  DATETIME NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE containers (
    id          TEXT PRIMARY KEY,
    parent_id   TEXT REFERENCES containers(id),
    name        TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    color       TEXT NOT NULL DEFAULT '',
    icon        TEXT NOT NULL DEFAULT '',
    created_at  DATETIME NOT NULL DEFAULT (datetime('now')),
    updated_at  DATETIME NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE items (
    id           TEXT PRIMARY KEY,
    container_id TEXT NOT NULL REFERENCES containers(id),
    name         TEXT NOT NULL,
    description  TEXT NOT NULL DEFAULT '',
    quantity     INTEGER NOT NULL DEFAULT 1,
    color        TEXT NOT NULL DEFAULT '',
    icon         TEXT NOT NULL DEFAULT '',
    created_at   DATETIME NOT NULL DEFAULT (datetime('now')),
    updated_at   DATETIME NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE item_tags (
    item_id TEXT NOT NULL REFERENCES items(id) ON DELETE CASCADE,
    tag_id  TEXT NOT NULL REFERENCES tags(id) ON DELETE CASCADE,
    PRIMARY KEY (item_id, tag_id)
);

CREATE TABLE container_tags (
    container_id TEXT NOT NULL REFERENCES containers(id) ON DELETE CASCADE,
    tag_id       TEXT NOT NULL REFERENCES tags(id) ON DELETE CASCADE,
    PRIMARY KEY (container_id, tag_id)
);

CREATE TABLE printer_configs (
    id        TEXT PRIMARY KEY,
    name      TEXT NOT NULL,
    encoder   TEXT NOT NULL,
    model     TEXT NOT NULL,
    transport TEXT NOT NULL,
    address   TEXT NOT NULL DEFAULT '',
    offset_x  INTEGER NOT NULL DEFAULT 0,
    offset_y  INTEGER NOT NULL DEFAULT 0
);

CREATE TABLE templates (
    id         TEXT PRIMARY KEY,
    name       TEXT NOT NULL,
    tags       TEXT NOT NULL DEFAULT '[]',
    target     TEXT NOT NULL DEFAULT 'universal',
    width_mm   REAL NOT NULL DEFAULT 0,
    height_mm  REAL NOT NULL DEFAULT 0,
    width_px   INTEGER NOT NULL DEFAULT 0,
    height_px  INTEGER NOT NULL DEFAULT 0,
    elements   TEXT NOT NULL DEFAULT '[]',
    created_at DATETIME NOT NULL DEFAULT (datetime('now')),
    updated_at DATETIME NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX idx_containers_parent  ON containers(parent_id);
CREATE INDEX idx_items_container    ON items(container_id);
CREATE INDEX idx_tags_parent        ON tags(parent_id);
CREATE INDEX idx_item_tags_tag      ON item_tags(tag_id);
CREATE INDEX idx_container_tags_tag ON container_tags(tag_id);

-- +goose Down

DROP INDEX IF EXISTS idx_container_tags_tag;
DROP INDEX IF EXISTS idx_item_tags_tag;
DROP INDEX IF EXISTS idx_tags_parent;
DROP INDEX IF EXISTS idx_items_container;
DROP INDEX IF EXISTS idx_containers_parent;
DROP TABLE IF EXISTS container_tags;
DROP TABLE IF EXISTS item_tags;
DROP TABLE IF EXISTS templates;
DROP TABLE IF EXISTS printer_configs;
DROP TABLE IF EXISTS items;
DROP TABLE IF EXISTS containers;
DROP TABLE IF EXISTS tags;
