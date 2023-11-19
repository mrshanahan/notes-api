package main

import (
    "context"
    "database/sql"
    "errors"
    "fmt"
    "log/slog"
    "net/http"
    "os"
    "path"
    "strconv"
    "time"

    "github.com/go-chi/chi/v5"
    "github.com/go-chi/chi/v5/middleware"
    "github.com/go-chi/render"

    "mrshanahan.com/notes-api/pkg/notes-db"
    "mrshanahan.com/notes-api/pkg/notes"
    "mrshanahan.com/notes-api/internal/utils"
)

var DB *sql.DB

func main() {
    exitCode := Run()
    os.Exit(exitCode)
}

func Run() int {
    dbPath := os.Getenv("NOTES_API_DB")
    if dbPath == "" {
        home := os.Getenv("HOME")
        notesConfigDirectory := path.Join(home, ".notes")
        if err := os.MkdirAll(notesConfigDirectory, 0777); err != nil {
            slog.Error("failed to create notes directory",
                "path", notesConfigDirectory,
                "err", err)
            return 1
        }
        dbPath = path.Join(notesConfigDirectory, "notes.sqlite")
        slog.Info("no path provided for DB; using default",
            "path", dbPath)
    } else {
        slog.Info("given DB path", "path", dbPath)
        dbPathDir := path.Dir(dbPath)
        if err := os.MkdirAll(dbPathDir, 0777); err != nil {
            slog.Error("failed to create custom notes DB path parent",
                "path", dbPathDir,
                "err", err)
            return 1
        }
    }

    if _, err := os.Open(dbPath); err != nil && errors.Is(err, os.ErrNotExist) {
        slog.Info("DB does not exist; it will be created during initialization",
            "path", dbPath)
    }

    db, err := notesdb.Initialize(dbPath)
    if err != nil {
        fmt.Printf("failed to initialize: %s\n", err)
        return 1
    }
    DB = db
    defer DB.Close()

    portStr := os.Getenv("NOTES_API_PORT")
    defaultPort := 3333
    port, err := strconv.Atoi(portStr)
    if err != nil {
        port = defaultPort
        slog.Info("no valid port provided via NOTES_API_PORT, using default",
            "portStr", portStr,
            "defaultPort", port)
    } else {
        slog.Info("using custom port",
            "port", port)
    }

    r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
    r.Use(render.SetContentType(render.ContentTypeJSON))

    r.Route("/notes", func(r chi.Router) {
        r.Get("/", ListNotes)
        r.Post("/", CreateNote)
        r.Route("/{noteID}", func(r chi.Router) {
            r.Use(NoteContext)
            r.Get("/", GetNote)
            r.Post("/", UpdateNote)
            r.Delete("/", DeleteNote)
            r.Get("/content", GetNoteContent)
            r.Post("/content", UpdateNoteContent)
        })
    })

    slog.Info("listening for requests", "port", port);
    err = http.ListenAndServe(fmt.Sprintf(":%d", port), r)
    if err != nil {
        slog.Error("failed to initialize HTTP server",
            "err", err)
        return 1
    }
    return 0
}

func NoteContext(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        idStr := chi.URLParam(r, "noteID")
        id, err := strconv.ParseInt(idStr, 10, 64)
        if err != nil {
            render.Render(w, r, ErrInvalidRequest(err))
            return
        }
        found, err := notesdb.GetNote(DB, id)
        if err != nil {
            slog.Error("failed to execute query to retrieve note",
                "id", id,
                "err", err)
            render.Render(w, r, ErrInternalServerError(err))
            return
        }
        if found == nil {
            render.Render(w, r, ErrNotFoundError(errors.New(fmt.Sprintf("no note with id: %d", id))))
            return
        }
        ctx := context.WithValue(r.Context(), "note", found)
        next.ServeHTTP(w, r.WithContext(ctx))
    })
}

func getNoteFromContext(r *http.Request) *notesdb.IndexEntry {
    return r.Context().Value("note").(*notesdb.IndexEntry)
}

func ListNotes(w http.ResponseWriter, r *http.Request) {
    notes, err := notesdb.GetNotes(DB)
    if err != nil {
        slog.Error("failed to execute query to retrieve notes",
            "err", err)
        render.Render(w, r, ErrInternalServerError(err))
        return
    }
    if err := render.RenderList(w, r, newNotesListResponse(notes)); err != nil {
        render.Render(w, r, ErrInternalServerError(err))
    }
}

func CreateNote(w http.ResponseWriter, r *http.Request) {
    data := &NoteRequest{}
    if err := render.Bind(r, data); err != nil {
        render.Render(w, r, ErrInvalidRequest(err))
    }

    entry, err := notesdb.NewNote(DB, data.Note.Title)
    if err != nil {
        slog.Error("failed to create note",
            "title", data.Note.Title,
            "err", err)
        render.Render(w, r, ErrInternalServerError(err))
    } else {
        render.Render(w, r, newNoteResponseWithStatus(entry, http.StatusCreated))
    }
}

func GetNote(w http.ResponseWriter, r *http.Request) {
    note := getNoteFromContext(r)
    if err := render.Render(w, r, newNoteResponse(note)); err != nil {
        render.Render(w, r, ErrInternalServerError(err))
    }
}

func UpdateNote(w http.ResponseWriter, r *http.Request) {
    existingNote := getNoteFromContext(r)

    newNote := &NoteRequest{}
    if err := render.Bind(r, newNote); err != nil {
        render.Render(w, r, ErrInvalidRequest(err))
        return
    }

    if (existingNote.Title != newNote.Title) {
        err := notesdb.UpdateNote(DB, existingNote.ID, newNote.Title)
        if err != nil {
            slog.Error("failed to update note",
                "oldTitle", existingNote.Title,
                "newTitle", newNote.Title,
                "err", err)
            render.Render(w, r, ErrInternalServerError(err))
            return
        }
    }

    w.WriteHeader(http.StatusNoContent)
}

func DeleteNote(w http.ResponseWriter, r *http.Request) {
    idStr := chi.URLParam(r, "noteID")
    id, err := strconv.ParseInt(idStr, 10, 64)
    if err != nil {
        render.Render(w, r, ErrInvalidRequest(err))
        return
    }
    if err = notesdb.DeleteNote(DB, id); err != nil {
        slog.Error("failed to remove note",
            "err", err,
            "noteID", id)
        render.Render(w, r, ErrInternalServerError(err))
        return
    }

    w.WriteHeader(http.StatusNoContent)
}

func GetNoteContent(w http.ResponseWriter, r *http.Request) {
    note := getNoteFromContext(r)
    var content []byte
    var err error
    if note.ContentType == notesdb.CONTENT_SQL {
        content, err = notesdb.GetNoteContents(DB, note.ID)
        if err != nil {
            render.Render(w, r, ErrInternalServerError(err))
            return
        }
    } else {
        render.Render(w, r, ErrInvalidRequest(errors.New(fmt.Sprintf("note has invalid ContentType: %d", note.ContentType))))
        return
    }

    // This call as-is appears to write the following headers:
    //
    // < HTTP/1.1 200 OK
    // < Date: <date>
    // < Content-Type: text/plain; charset=utf-8
    // < Transfer-Encoding: chunked
    //
    // The status code is expected, as that's the default status
    // code set if Write is called without WriteHeader
    // (see: https://pkg.go.dev/net/http#ResponseWriter). Go
    // will attempt to detect the correct Content-Type based on
    // the first 512 bytes.
    w.Write(content)
}

func UpdateNoteContent(w http.ResponseWriter, r *http.Request) {
    note := getNoteFromContext(r)

    content := []byte(r.FormValue("content"))
    if len(content) == 0 {
        f, _, err := r.FormFile("content")
        if err != nil && errors.Is(err, http.ErrMissingFile) {
            render.Render(w, r, ErrInvalidRequest(errors.New("either form value or form file required for 'content' form field")))
            return
        }
        // TODO: Don't read entire file into memory at once
        content, err = utils.ReadToEnd(f)
        if err != nil {
            slog.Error("failed to read request body",
                "err", err)
            render.Render(w, r, ErrInternalServerError(errors.New("failed to read form file")))
            return
        }
    }

    if note.ContentType == notesdb.CONTENT_SQL {
        if err := notesdb.SetNoteContents(DB, note.ID, content); err != nil {
            slog.Error("failed to save file contents",
                "err", err,
                "noteID", note.ID)
            render.Render(w, r, ErrInternalServerError(err))
            return
        }
    }

    w.WriteHeader(http.StatusNoContent)
}


// API types

func newNoteResponseWithStatus(entry *notesdb.IndexEntry, status int) render.Renderer {
   return &NoteResponse{
       HTTPStatusCode: status,
       Note: &notes.Note{
           entry.ID,
           entry.Title,
           entry.CreatedOn,
           entry.UpdatedOn,
       },
   }
}


func newNoteResponse(entry *notesdb.IndexEntry) render.Renderer {
   return &NoteResponse{
       HTTPStatusCode: http.StatusOK,
       Note: &notes.Note{
           entry.ID,
           entry.Title,
           entry.CreatedOn,
           entry.UpdatedOn,
       },
   }
}

func newNotesListResponse(index []*notesdb.IndexEntry) []render.Renderer {
    response := []render.Renderer{}
    for _, n := range index {
        response = append(response, newNoteResponse(n))
    }
    return response
}

func (n *NoteResponse) Render(w http.ResponseWriter, r *http.Request) error {
    render.Status(r, n.HTTPStatusCode)
    return nil
}

type NoteRequest struct {
    *NoteResponse

    // chi does this in its examples. This allows us to
    // have a canonical API object (Note) and omit fields
    // in the request as necessary.
    ProtectedID        int64 `json:"id"`
    ProtectedCreatedOn time.Time `json:"created_on"`
    ProtectedUpdatedOn time.Time `json:"updated_on"`
}

func (n *NoteRequest) Bind(r *http.Request) error {
    if n.Note == nil {
        return errors.New("missing required Note fields")
    }

    n.ProtectedID = 0
    // n.ProtectedCreatedOn = fmt.Sprintf("%s", time.Now().UTC())
    n.ProtectedCreatedOn = time.Now().UTC()
    n.ProtectedUpdatedOn = time.Now().UTC()
    return nil
}


type NoteResponse struct {
    *notes.Note
    HTTPStatusCode int `json:"-"`
}

type ErrResponse struct {
	Err            error `json:"-"` // low-level runtime error
	HTTPStatusCode int   `json:"-"` // http response status code

	StatusText string `json:"status"`          // user-level status message
	AppCode    int64  `json:"code,omitempty"`  // application-specific error code
	ErrorText  string `json:"error,omitempty"` // application-level error message, for debugging
}

func (e *ErrResponse) Render(w http.ResponseWriter, r *http.Request) error {
	render.Status(r, e.HTTPStatusCode)
	return nil
}

func ErrInternalServerError(err error) render.Renderer {
    return &ErrResponse{
        Err:            err,
        HTTPStatusCode: 500,
        StatusText:     "An error occurred",
        ErrorText:      err.Error(),
    }
}

func ErrNotFoundError(err error) render.Renderer {
    return &ErrResponse{
        Err:            err,
        HTTPStatusCode: 404,
        StatusText:     "Resource not found",
        ErrorText:      err.Error(),
    }
}

func ErrInvalidRequest(err error) render.Renderer {
    return &ErrResponse{
        Err:            err,
        HTTPStatusCode: 400,
        StatusText:     "Invalid request",
        ErrorText:      err.Error(),
    }
}
