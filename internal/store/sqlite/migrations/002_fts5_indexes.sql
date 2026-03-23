-- +goose Up

CREATE VIRTUAL TABLE items_fts USING fts5(
    name, description, content=items, content_rowid=rowid
);

CREATE VIRTUAL TABLE containers_fts USING fts5(
    name, description, content=containers, content_rowid=rowid
);

-- Items FTS sync triggers
-- +goose StatementBegin
CREATE TRIGGER items_ai AFTER INSERT ON items BEGIN
    INSERT INTO items_fts(rowid, name, description)
    VALUES (new.rowid, new.name, new.description);
END;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TRIGGER items_ad AFTER DELETE ON items BEGIN
    INSERT INTO items_fts(items_fts, rowid, name, description)
    VALUES ('delete', old.rowid, old.name, old.description);
END;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TRIGGER items_au AFTER UPDATE ON items BEGIN
    INSERT INTO items_fts(items_fts, rowid, name, description)
    VALUES ('delete', old.rowid, old.name, old.description);
    INSERT INTO items_fts(rowid, name, description)
    VALUES (new.rowid, new.name, new.description);
END;
-- +goose StatementEnd

-- Containers FTS sync triggers
-- +goose StatementBegin
CREATE TRIGGER containers_ai AFTER INSERT ON containers BEGIN
    INSERT INTO containers_fts(rowid, name, description)
    VALUES (new.rowid, new.name, new.description);
END;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TRIGGER containers_ad AFTER DELETE ON containers BEGIN
    INSERT INTO containers_fts(containers_fts, rowid, name, description)
    VALUES ('delete', old.rowid, old.name, old.description);
END;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TRIGGER containers_au AFTER UPDATE ON containers BEGIN
    INSERT INTO containers_fts(containers_fts, rowid, name, description)
    VALUES ('delete', old.rowid, old.name, old.description);
    INSERT INTO containers_fts(rowid, name, description)
    VALUES (new.rowid, new.name, new.description);
END;
-- +goose StatementEnd

-- +goose Down

DROP TRIGGER IF EXISTS containers_au;
DROP TRIGGER IF EXISTS containers_ad;
DROP TRIGGER IF EXISTS containers_ai;
DROP TRIGGER IF EXISTS items_au;
DROP TRIGGER IF EXISTS items_ad;
DROP TRIGGER IF EXISTS items_ai;
DROP TABLE IF EXISTS containers_fts;
DROP TABLE IF EXISTS items_fts;
