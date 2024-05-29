package main

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path"
	"strconv"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"github.com/gofiber/fiber/v2/middleware/requestid"

	"github.com/mrshanahan/notes-api/internal/auth"
	"github.com/mrshanahan/notes-api/internal/middleware"
	"github.com/mrshanahan/notes-api/internal/utils"
	"github.com/mrshanahan/notes-api/pkg/notes"
	notesdb "github.com/mrshanahan/notes-api/pkg/notes-db"
)

var (
	DB              *sql.DB
	TokenCookieName string = "access_token"
	NoteLocalName   string = "note"
	TokenLocalName         = "token"
)

func main() {
	exitCode := Run()
	os.Exit(exitCode)
}

func Run() int {
	auth.InitializeAuth(context.Background())

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

	app := fiber.New()
	app.Use(requestid.New(), logger.New(), recover.New())
	app.Route("/notes", func(notes fiber.Router) {
		// notes.Use(middleware.ValidateAccessToken(TokenLocalName, TokenCookieName))
		notes.Get("/", ListNotes)
		notes.Post("/", CreateNote)
		notes.Route("/:noteID", func(note fiber.Router) {
			note.Use(middleware.LoadNoteFromRoute(NoteLocalName, "noteID", DB))
			note.Get("/", GetNote)
			note.Post("/", UpdateNote)
			note.Delete("/", DeleteNote)
			note.Get("/content", GetNoteContent)
			note.Post("/content", UpdateNoteContent)
		})
	})

	slog.Info("listening for requests", "port", port)
	err = app.Listen(fmt.Sprintf(":%d", port))
	if err != nil {
		// TODO: do we get this error if it fails to initialize or if it just fails?
		slog.Error("failed to initialize HTTP server",
			"err", err)
		return 1
	}
	return 0
}

func getNoteFromContext(c *fiber.Ctx) *notesdb.IndexEntry {
	return c.Locals("note").(*notesdb.IndexEntry)
}

func ListNotes(c *fiber.Ctx) error {
	notes, err := notesdb.GetNotes(DB)
	if err != nil {
		slog.Error("failed to execute query to retrieve notes",
			"err", err)
		return c.SendStatus(fiber.StatusInternalServerError)
	}
	return c.JSON(notes)
}

func CreateNote(c *fiber.Ctx) error {
	data := &NoteRequest{}
	err := json.Unmarshal(c.Body(), data)
	if err != nil {
		return c.SendStatus(fiber.StatusBadRequest)
	}

	entry, err := notesdb.NewNote(DB, data.Note.Title)
	if err != nil {
		slog.Error("failed to create note",
			"title", data.Note.Title,
			"err", err)
		return c.SendStatus(fiber.StatusInternalServerError)
	}
	c.Status(fiber.StatusCreated)
	return c.JSON(entry)
}

func GetNote(c *fiber.Ctx) error {
	note := getNoteFromContext(c)
	return c.JSON(note)
}

func UpdateNote(c *fiber.Ctx) error {
	existingNote := getNoteFromContext(c)

	newNote := &NoteRequest{}
	err := json.Unmarshal(c.Body(), newNote)
	if err != nil {
		return c.SendStatus(fiber.StatusBadRequest)
	}

	if existingNote.Title != newNote.Title {
		err := notesdb.UpdateNote(DB, existingNote.ID, newNote.Title)
		if err != nil {
			slog.Error("failed to update note",
				"oldTitle", existingNote.Title,
				"newTitle", newNote.Title,
				"err", err)
			return c.SendStatus(fiber.StatusInternalServerError)
		}
	}

	return c.SendStatus(fiber.StatusNoContent)
}

func DeleteNote(c *fiber.Ctx) error {
	note := getNoteFromContext(c)
	id := note.ID
	if err := notesdb.DeleteNote(DB, id); err != nil {
		slog.Error("failed to remove note",
			"err", err,
			"noteID", id)
		return c.SendStatus(fiber.StatusInternalServerError)
	}

	return c.SendStatus(fiber.StatusNoContent)
}

func GetNoteContent(c *fiber.Ctx) error {
	note := getNoteFromContext(c)
	var content []byte
	var err error
	if note.ContentType == notesdb.CONTENT_SQL {
		content, err = notesdb.GetNoteContents(DB, note.ID)
		if err != nil {
			return c.SendStatus(fiber.StatusInternalServerError)
		}
	} else {
		c.Status(fiber.StatusBadRequest)
		return c.SendString(fmt.Sprintf("note has invalid ContentType: %d", note.ContentType))
	}

	stream := bytes.NewBuffer(content)
	return c.SendStream(stream, len(content))
}

func UpdateNoteContent(c *fiber.Ctx) error {
	note := getNoteFromContext(c)

	content := []byte(c.FormValue("content"))
	if len(content) == 0 {
		fileHeader, err := c.FormFile("content")
		if err != nil && errors.Is(err, http.ErrMissingFile) {
			c.Status(fiber.StatusBadRequest)
			return c.SendString("either form value or form file required for 'content' form field")
		} else if err != nil {
			c.Status(fiber.StatusBadRequest)
			return c.SendString(fmt.Sprintf("unexpected error when reading form file: %w", err))
		}
		// TODO: Don't read entire file into memory at once
		file, err := fileHeader.Open()
		if err != nil {
			slog.Error("failed to read open file header",
				"err", err)
			c.Status(fiber.StatusInternalServerError)
			return c.SendString("failed to read form file")
		}
		content, err = utils.ReadToEnd(file)
		if err != nil {
			slog.Error("failed to read request file body",
				"err", err)
			c.Status(fiber.StatusInternalServerError)
			return c.SendString("failed to read form file")
		}
	}

	if note.ContentType == notesdb.CONTENT_SQL {
		if err := notesdb.SetNoteContents(DB, note.ID, content); err != nil {
			slog.Error("failed to save file contents",
				"err", err,
				"noteID", note.ID)
			c.Status(fiber.StatusInternalServerError)
			return c.SendString("failed to save file contents")
		}
	}

	return c.SendStatus(fiber.StatusNoContent)
}

// API types

type NoteRequest struct {
	*notes.Note

	// chi does this in its examples. This allows us to
	// have a canonical API object (Note) and omit fields
	// in the request as necessary.
	ProtectedID        int64     `json:"id"`
	ProtectedCreatedOn time.Time `json:"created_on"`
	ProtectedUpdatedOn time.Time `json:"updated_on"`
}
