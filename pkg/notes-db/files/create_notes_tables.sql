CREATE TABLE IF NOT EXISTS
    notes
    ( id INTEGER PRIMARY KEY
    , title TEXT NOT NULL
    , created_on TEXT DEFAULT CURRENT_TIMESTAMP
    , updated_on TEXT DEFAULT CURRENT_TIMESTAMP
    , content_type_id INT DEFAULT 1
    );

-- TODO: Figure out FK constraints targeting this table
CREATE TABLE IF NOT EXISTS
    content_type
    ( id INTEGER PRIMARY KEY
    , name TEXT
    );

INSERT OR REPLACE INTO content_type (id, name) VALUES (1, 'sql');

CREATE TABLE IF NOT EXISTS
    notes_content
    ( note_id INTEGER UNIQUE REFERENCES notes(id) ON DELETE CASCADE
    , content BLOB
    );
