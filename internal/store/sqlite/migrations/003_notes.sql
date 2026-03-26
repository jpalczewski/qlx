-- +goose Up

CREATE TABLE notes (
    id           TEXT PRIMARY KEY,
    container_id TEXT REFERENCES containers(id) ON DELETE CASCADE,
    item_id      TEXT REFERENCES items(id) ON DELETE CASCADE,
    title        TEXT NOT NULL,
    content      TEXT NOT NULL DEFAULT '',
    color        TEXT NOT NULL DEFAULT '',
    icon         TEXT NOT NULL DEFAULT '',
    created_at   DATETIME NOT NULL DEFAULT (datetime('now')),
    updated_at   DATETIME NOT NULL DEFAULT (datetime('now')),
    CHECK ((container_id IS NOT NULL) != (item_id IS NOT NULL))
);

CREATE INDEX idx_notes_container ON notes(container_id);
CREATE INDEX idx_notes_item ON notes(item_id);

CREATE VIRTUAL TABLE notes_fts USING fts5(
    title, content, content=notes, content_rowid=rowid
);

-- +goose StatementBegin
CREATE TRIGGER notes_ai AFTER INSERT ON notes BEGIN
    INSERT INTO notes_fts(rowid, title, content)
    VALUES (new.rowid, new.title, new.content);
END;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TRIGGER notes_ad AFTER DELETE ON notes BEGIN
    INSERT INTO notes_fts(notes_fts, rowid, title, content)
    VALUES ('delete', old.rowid, old.title, old.content);
END;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TRIGGER notes_au AFTER UPDATE ON notes BEGIN
    INSERT INTO notes_fts(notes_fts, rowid, title, content)
    VALUES ('delete', old.rowid, old.title, old.content);
    INSERT INTO notes_fts(rowid, title, content)
    VALUES (new.rowid, new.title, new.content);
END;
-- +goose StatementEnd

-- +goose Down

DROP TRIGGER IF EXISTS notes_au;
DROP TRIGGER IF EXISTS notes_ad;
DROP TRIGGER IF EXISTS notes_ai;
DROP TABLE IF EXISTS notes_fts;
DROP INDEX IF EXISTS idx_notes_item;
DROP INDEX IF EXISTS idx_notes_container;
DROP TABLE IF EXISTS notes;
