package main

import (
    "context"
    "errors"
    "fmt"
    "io"
    "log/slog"
    "net/http"
    "os"
    "strconv"
    "time"

    "github.com/go-chi/chi/v5"
    "github.com/go-chi/chi/v5/middleware"
    "github.com/go-chi/render"

    "mrshanahan.com/notes-api/internal/notes"
)

func main() {
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

    r.Route("/index", func(r chi.Router) {
        r.Use(IndexContext)
        r.Post("/validate", ValidateIndex)
    })

    r.Route("/notes", func(r chi.Router) {
        r.Use(IndexContext)
        r.Get("/", ListNotes)
        r.Post("/", CreateNote)
        r.Route("/{noteID}", func(r chi.Router) {
            r.Use(NoteContext)
            r.Get("/", GetNote)
            r.Put("/", UpdateNote)
            r.Delete("/", DeleteNote)
            r.Get("/content", GetNoteContent)
            r.Put("/content", UpdateNoteContent)
        })
    })

    slog.Info("listening for requests", "port", port);
    http.ListenAndServe(fmt.Sprintf(":%d", port), r)
}

func IndexContext(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        index, err := notes.LoadIndex()
        if err != nil {
            slog.Error("failed to load index",
                "err", err)
            render.Render(w, r, ErrInternalServerError(err))
            return
        }
        ctx := context.WithValue(r.Context(), "index", index)
        next.ServeHTTP(w, r.WithContext(ctx))
    })
}

func NoteContext(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        index := getIndexFromContext(r)
        id := chi.URLParam(r, "noteID")
        found := notes.LookupNote(id, index)
        if found == nil {
            render.Render(w, r, ErrNotFoundError(errors.New(fmt.Sprintf("no note with id: %s", id))))
            return
        }
        ctx := context.WithValue(r.Context(), "note", found)
        next.ServeHTTP(w, r.WithContext(ctx))
    })
}

func getIndexFromContext(r *http.Request) []*notes.IndexEntry {
    return r.Context().Value("index").([]*notes.IndexEntry)
}

func getNoteFromContext(r *http.Request) *notes.IndexEntry {
    return r.Context().Value("note").(*notes.IndexEntry)
}

func ListNotes(w http.ResponseWriter, r *http.Request) {
    index := getIndexFromContext(r)
    if err := render.RenderList(w, r, newNotesListResponse(index)); err != nil {
        render.Render(w, r, ErrInternalServerError(err))
    }
}

func CreateNote(w http.ResponseWriter, r *http.Request) {
    index := getIndexFromContext(r)

    data := &NoteRequest{}
    if err := render.Bind(r, data); err != nil {
        render.Render(w, r, ErrInvalidRequest(err))
    }

    entry, index := notes.NewNote(data.Note.Title, index)
    if err := notes.SaveIndex(index); err != nil {
        slog.Error("failed to save index",
            "err", err)
        render.Render(w, r, ErrInternalServerError(err))
    } else {
        render.Render(w, r, newNoteResponseWithStatus(entry, http.StatusCreated))
    }
}

func ValidateIndex(w http.ResponseWriter, r *http.Request) {
    index := getIndexFromContext(r)
    for _, entry := range index {
        if entry.CreatedOn.IsZero() {
            path := entry.Path
            info, err := os.Stat(path)
            if err != nil {
                slog.Error("failed to load entry file",
                    "id", entry.ID,
                    "path", entry.Path,
                    "err", err)
            } else {
                entry.CreatedOn = info.ModTime()
                slog.Info("updated created_on value for entry from file time",
                    "id", entry.ID,
                    "created_on", entry.CreatedOn)
            }
        }
    }

    if err := notes.SaveIndex(index); err != nil {
        slog.Error("failed to save index", err)
        render.Render(w, r, ErrInternalServerError(err))
    }

    w.WriteHeader(http.StatusNoContent)
}

func GetNote(w http.ResponseWriter, r *http.Request) {
    note := getNoteFromContext(r)
    if err := render.Render(w, r, newNoteResponse(note)); err != nil {
        render.Render(w, r, ErrInternalServerError(err))
    }
}

func UpdateNote(w http.ResponseWriter, r *http.Request) {
    index := getIndexFromContext(r)
    existingNote := getNoteFromContext(r)

    newNote := &NoteRequest{}
    if err := render.Bind(r, newNote); err != nil {
        render.Render(w, r, ErrInvalidRequest(err))
        return
    }

    if (existingNote.Title != newNote.Title) {
        existingNote.Title = newNote.Title
        notes.SaveIndex(index)
    }

    if err := notes.SaveIndex(index); err != nil {
        // In case of error, re-deleting isn't an issue - we ignore not-found errors from the FS.
        slog.Error("failed to save index; note is still deleted but entry is present in index",
            "err", err,
            "noteID", existingNote.ID)
        render.Render(w, r, ErrInternalServerError(err))
        return
    }

    w.WriteHeader(http.StatusNoContent)
}

func DeleteNote(w http.ResponseWriter, r *http.Request) {
    index := getIndexFromContext(r)
    id := chi.URLParam(r, "noteID")
    var err error
    if index, err = notes.DeleteNote(id, index); err != nil {
        slog.Error("failed to remove note",
            "err", err,
            "noteID", id)
        render.Render(w, r, ErrInternalServerError(err))
        return
    }

    if err = notes.SaveIndex(index); err != nil {
        // In case of error, re-deleting isn't an issue - we ignore not-found errors from the FS.
        slog.Error("failed to save index; note is still deleted but entry is present in index",
            "err", err,
            "noteID", id)
        render.Render(w, r, ErrInternalServerError(err))
        return
    }

    w.WriteHeader(http.StatusNoContent)
}

func GetNoteContent(w http.ResponseWriter, r *http.Request) {
    note := getNoteFromContext(r)
    content, err := notes.GetNoteContents(note)
    if err != nil {
        render.Render(w, r, ErrInternalServerError(err))
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

    f, _, err := r.FormFile("content")
    if err != nil {
        slog.Error("failed to get file reader",
            "err", err)
        return
    }

    // TODO: Don't read entire file into memory at once
    content, err := readToEnd(f)
    if err != nil {
        slog.Error("failed to read request body",
            "err", err)
        return
    }

    if err := notes.SetNoteContents(content, note); err != nil {
        slog.Error("failed to save file contents",
            "err", err,
            "noteID", note.ID)
        render.Render(w, r, ErrInternalServerError(err))
        return
    }

    w.WriteHeader(http.StatusNoContent)
}

func readToEnd(r io.Reader) ([]byte, error) {
    BUF_SIZE := 1024 * 8
    buffer := make([]byte, BUF_SIZE)
    result := []byte{}
    readMore := true
    var err error = nil
    for readMore {
        numRead, err := r.Read(buffer)
        if err != nil && !errors.Is(err, io.EOF) {
            readMore = false
        } else {
            readMore = err == nil
            result = append(result, buffer[:numRead]...)
        }
    }
    if err != nil {
        return nil, err
    }
    return result, nil
}

// API types

func newNoteResponseWithStatus(entry *notes.IndexEntry, status int) render.Renderer {
   return &Note{
       HTTPStatusCode: status,
       ID: entry.ID,
       Title: entry.Title,
       CreatedOn: entry.CreatedOn,
   }
}


func newNoteResponse(entry *notes.IndexEntry) render.Renderer {
   return &Note{
       HTTPStatusCode: http.StatusOK,
       ID: entry.ID,
       Title: entry.Title,
       CreatedOn: entry.CreatedOn,
   }
}

func newNotesListResponse(index []*notes.IndexEntry) []render.Renderer {
    response := []render.Renderer{}
    for _, n := range index {
        response = append(response, newNoteResponse(n))
    }
    return response
}

func (n *Note) Render(w http.ResponseWriter, r *http.Request) error {
    render.Status(r, n.HTTPStatusCode)
    return nil
}

type NoteRequest struct {
    *Note

    // chi does this in its examples. This allows us to
    // have a canonical API object (Note) and omit fields
    // in the request as necessary.
    ProtectedID        string `json:"id"`
    ProtectedCreatedOn time.Time `json:"created_on"`
}

func (n *NoteRequest) Bind(r *http.Request) error {
    if n.Note == nil {
        return errors.New("missing required Note fields")
    }

    n.ProtectedID = ""
    // n.ProtectedCreatedOn = fmt.Sprintf("%s", time.Now().UTC())
    n.ProtectedCreatedOn = time.Now().UTC()
    return nil
}


type Note struct {
    HTTPStatusCode int `json:"-"`
    ID          string `json:"id"`
    Title       string `json:"title"`
    CreatedOn   time.Time `json:"created_on"`
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
