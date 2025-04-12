package notesdb

import (
	"database/sql"
	_ "embed"
	"errors"
	"fmt"
	"time"

	_ "github.com/mattn/go-sqlite3"

	"github.com/mrshanahan/notes-api/pkg/notes"
)

var (
	//go:embed files/create_notes_tables.sql
	CREATE_NOTES_TABLES_SQL string
)

const (
	CONTENT_SQL = 1
)

type IndexEntry struct {
	*notes.Note
	ContentType int `json:"-"`
}

type IndexEntryWithPreview struct {
	*IndexEntry
	ContentPreview string `json:"content_preview"`
}

func Initialize(path string) (*sql.DB, error) {
	db, err := sql.Open("sqlite3", path)
	if err != nil {
		return nil, err
	}
	if db == nil {
		panic("WTF???")
	}

	// https://stackoverflow.com/questions/13641250/sqlite-delete-cascade-not-working
	_, err = db.Exec("PRAGMA foreign_keys=on;")
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

func NewNote(db *sql.DB, title string) (*IndexEntry, error) {
	stmt, err := db.Prepare("INSERT INTO notes (title, created_on, updated_on) VALUES (?, ?, ?)")
	if err != nil {
		return nil, err
	}

	now := time.Now().UTC()
	result, err := stmt.Exec(title, formatTime(now), formatTime(now))
	if err != nil {
		return nil, err
	}

	id, err := result.LastInsertId()
	if err != nil {
		return nil, err
	}
	entry, err := GetNote(db, id)
	if err != nil {
		return nil, err
	}

	return entry, nil
}

func GetNotesWithPreview(db *sql.DB, previewLength int) ([]*IndexEntryWithPreview, error) {
	if previewLength <= 0 || previewLength >= 100000 {
		return nil, fmt.Errorf("preview length must be greater than 0 and less than 100KB: %d", previewLength)
	}
	stmt, err := db.Prepare(`
        SELECT
            id,
            title,
            created_on,
            updated_on,
            content_type_id,
            IIF(content_type_id = 1,
                IIF(LENGTH(content) > ?,
                    SUBSTR(content, 0, ?) || '...',
                    content),
                NULL)
        FROM notes
            LEFT JOIN notes_content on notes.id = notes_content.note_id`)
	if err != nil {
		return nil, err
	}

	rows, err := stmt.Query(previewLength, previewLength)
	defer rows.Close()

	if err != nil {
		return nil, err
	}

	notes := []*IndexEntryWithPreview{}
	for rows.Next() {
		note, err := scanNoteWithPreviewRows(rows)
		if err != nil {
			return nil, err
		}
		notes = append(notes, note)
	}

	if err = rows.Err(); err != nil {
		return nil, err
	}

	return notes, nil
}

func GetNotes(db *sql.DB) ([]*IndexEntry, error) {
	stmt, err := db.Prepare("SELECT id, title, created_on, updated_on, content_type_id FROM notes")
	if err != nil {
		return nil, err
	}

	rows, err := stmt.Query()
	defer rows.Close()

	if err != nil {
		return nil, err
	}

	notes := []*IndexEntry{}
	for rows.Next() {
		note, err := scanNoteRows(rows)
		if err != nil {
			return nil, err
		}
		notes = append(notes, note)
	}

	if err = rows.Err(); err != nil {
		return nil, err
	}

	return notes, nil
}

func DeleteNote(db *sql.DB, id int64) error {
	stmt, err := db.Prepare("DELETE FROM notes WHERE id = ?")
	if err != nil {
		return err
	}

	_, err = stmt.Exec(id)
	return err
}

func GetNote(db *sql.DB, id int64) (*IndexEntry, error) {
	stmt, err := db.Prepare("SELECT id, title, created_on, updated_on, content_type_id FROM notes WHERE id = ?")
	if err != nil {
		return nil, err
	}

	row := stmt.QueryRow(id)

	note, err := scanNoteRow(row)
	if err != nil && errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	} else if err != nil {
		return nil, err
	}

	return note, nil
}

func UpdateNote(db *sql.DB, id int64, title string) error {
	stmt, err := db.Prepare("UPDATE notes SET title = ?, updated_on = ? WHERE id = ?")
	if err != nil {
		return err
	}

	_, err = stmt.Exec(title, formatTime(time.Now().UTC()), id)
	return err
}

func TouchNote(db *sql.DB, id int64) error {
	stmt, err := db.Prepare("UPDATE notes SET updated_on = ? WHERE id = ?")
	if err != nil {
		return err
	}

	_, err = stmt.Exec(formatTime(time.Now().UTC()), id)
	return err
}

func GetNoteContents(db *sql.DB, id int64) ([]byte, error) {
	stmt, err := db.Prepare("SELECT content FROM notes_content WHERE note_id = ?")
	if err != nil {
		return nil, err
	}

	row := stmt.QueryRow(id)

	var content []byte
	err = row.Scan(&content)
	if err != nil && errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	} else if err != nil {
		return nil, err
	}

	return content, nil
}

func SetNoteContents(db *sql.DB, id int64, content []byte) error {
	// TODO: Update updated_on field on main note (or have it be column in notes_content?)
	stmt, err := db.Prepare(`
        INSERT INTO notes_content (note_id, content) VALUES (?, ?)
            ON CONFLICT(note_id) DO UPDATE SET content = excluded.content`)
	if err != nil {
		return err
	}

	_, err = stmt.Exec(id, content)
	return err
}

// Private

func scanNoteRow(row *sql.Row) (*IndexEntry, error) {
	note := &IndexEntry{Note: &notes.Note{}}
	var createdOn, updatedOn string
	err := row.Scan(&note.ID, &note.Title, &createdOn, &updatedOn, &note.ContentType)
	if err != nil {
		return nil, err
	}
	note.CreatedOn, err = parseTime(createdOn)
	if err != nil {
		return nil, err
	}
	note.UpdatedOn, err = parseTime(updatedOn)
	if err != nil {
		return nil, err
	}
	return note, nil
}

func scanNoteWithPreviewRows(rows *sql.Rows) (*IndexEntryWithPreview, error) {
	note := &IndexEntryWithPreview{IndexEntry: &IndexEntry{Note: &notes.Note{}}}
	var createdOn, updatedOn string
	rows.Scan(&note.ID, &note.Title, &createdOn, &updatedOn, &note.ContentType, &note.ContentPreview)
	var err error
	note.CreatedOn, err = parseTime(createdOn)
	if err != nil {
		return nil, err
	}
	note.UpdatedOn, err = parseTime(updatedOn)
	if err != nil {
		return nil, err
	}
	return note, nil
}

func scanNoteRows(rows *sql.Rows) (*IndexEntry, error) {
	note := &IndexEntry{Note: &notes.Note{}}
	var createdOn, updatedOn string
	rows.Scan(&note.ID, &note.Title, &createdOn, &updatedOn, &note.ContentType)
	var err error
	note.CreatedOn, err = parseTime(createdOn)
	if err != nil {
		return nil, err
	}
	note.UpdatedOn, err = parseTime(updatedOn)
	if err != nil {
		return nil, err
	}
	return note, nil
}

func formatTime(t time.Time) string {
	return t.Format(time.RFC3339)
}

func parseTime(s string) (time.Time, error) {
	return time.Parse(time.RFC3339, s)
}
