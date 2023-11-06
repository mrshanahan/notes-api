package notesdb

import (
    "database/sql"
    _ "embed"
    // "errors"

    _ "github.com/mattn/go-sqlite3"
)

var (
    //go:embed files/create_notes_tables.sql
    CREATE_NOTES_TABLES_SQL string
)


func Initialize(path string) (*sql.DB, error) {
    db, err := sql.Open("sqlite3", path)
    if err != nil {
        return nil, err
    }

    tx, err := db.Begin()
    if err != nil {
        return nil, err
    }

    _, err = db.Exec(CREATE_NOTES_TABLES_SQL)
    if err != nil {
        tx.Rollback()
        return nil, err
    }

    err = tx.Commit()
    if err != nil {
        tx.Rollback()
        return nil, err
    }

    return db, nil
}
