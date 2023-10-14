package main

import (
    "errors"
    "fmt"
    "log/slog"
    "net/http"
    "os"
    "strconv"

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
            "port", port)
    } else {
        slog.Info("using custom port",
            "port", port)
    }

    r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
    r.Use(render.SetContentType(render.ContentTypeJSON))

	// r.Get("/", func(w http.ResponseWriter, r *http.Request) {
	// 	w.Write([]byte("hello world"))
	// })

    r.Route("/notes", func(r chi.Router) {
        r.Get("/", listNotes)
        r.Route("/{noteID}", func(r chi.Router) {
            r.Get("/", getNote)
            r.Get("/content", getNoteContent)
        })
    })

    http.ListenAndServe(fmt.Sprintf(":%d", port), r)
}

func listNotes(w http.ResponseWriter, r *http.Request) {
    index, err := notes.LoadIndex()
    if err != nil {
        slog.Error("failed to load index", err)
        http.Error(w, http.StatusText(500), 500)
        return
    }
    if err = render.RenderList(w, r, newNotesListResponse(index)); err != nil {
        render.Render(w, r, ErrInternalServerError(err))
    }
}

func getNote(w http.ResponseWriter, r *http.Request) {
    index, err := notes.LoadIndex()
    if err != nil {
        slog.Error("failed to load index", err)
        http.Error(w, http.StatusText(500), 500)
        return
    }

    var found *notes.IndexEntry = nil
    id := chi.URLParam(r, "noteID")
    for _, note := range index {
        if note.ID == id {
            found = note
            break
        }
    }

    if found == nil {
        render.Render(w, r, ErrNotFoundError(errors.New(fmt.Sprintf("no note with id: %s", id))))
        return
    }

    if err = render.Render(w, r, newNoteResponse(found)); err != nil {
        render.Render(w, r, ErrInternalServerError(err))
    }
}

func getNoteContent(w http.ResponseWriter, r *http.Request) {
    index, err := notes.LoadIndex()
    if err != nil {
        slog.Error("failed to load index", err)
        render.Render(w, r, ErrInternalServerError(err))
        return
    }

    var found *notes.IndexEntry = nil
    id := chi.URLParam(r, "noteID")
    for _, note := range index {
        if note.ID == id {
            found = note
            break
        }
    }

    if found == nil {
        render.Render(w, r, ErrNotFoundError(errors.New(fmt.Sprintf("no note with id: %s", id))))
        return
    }


    content, err := notes.GetNoteContents(found)
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

func newNoteResponse(index *notes.IndexEntry) render.Renderer {
   return &Note{
       ID: index.ID,
       Title: index.Title,
       CreatedOn: "",
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
    return nil
}

type Note struct {
    ID          string `json:"id"`
    Title       string `json:"title"`
    CreatedOn   string `json:"created_on"`
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
