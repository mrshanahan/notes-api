package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"github.com/gofiber/fiber/v2/middleware/requestid"

	"github.com/mrshanahan/notes-api/internal/cache"
	"github.com/mrshanahan/notes-api/internal/utils"
	"github.com/mrshanahan/notes-api/pkg/auth"
	"github.com/mrshanahan/notes-api/pkg/middleware"
	"github.com/mrshanahan/notes-api/pkg/notes"
	notesdb "github.com/mrshanahan/notes-api/pkg/notes-db"
)

var (
	DB                       *sql.DB
	TokenCookieName          string = "access_token"
	NoteLocalName            string = "note"
	TokenLocalName           string = "token"
	NotesConfigDirectory     string = path.Join(os.Getenv("HOME"), ".notes")
	DefaultPort              int    = 3333
	DefaultNotesDatabaseName string = "notes.sqlite"
)

func main() {
	exitCode := Run()
	os.Exit(exitCode)
}

func Run() int {
	if len(os.Args) > 1 && utils.Any(os.Args[1:], func(x string) bool { return x == "-h" || x == "--help" || x == "-?" }) {
		printHelp()
		return 0
	}

	var dbPath string
	dbPathDir := os.Getenv("NOTES_API_DB_DIR")
	if dbPathDir == "" {
		if err := os.MkdirAll(NotesConfigDirectory, 0777); err != nil {
			slog.Error("failed to create notes directory",
				"path", NotesConfigDirectory,
				"err", err)
			return 1
		}
		dbPath = path.Join(NotesConfigDirectory, DefaultNotesDatabaseName)
		slog.Info("no path provided for DB; using default",
			"path", dbPath)
	} else {
		slog.Info("given DB directory", "dir", dbPathDir)
		if err := os.MkdirAll(dbPathDir, 0777); err != nil {
			slog.Error("failed to create custom notes DB path parent",
				"path", dbPathDir,
				"err", err)
			return 1
		}
		dbPath = path.Join(dbPathDir, DefaultNotesDatabaseName)
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
	port, err := strconv.Atoi(portStr)
	if err != nil {
		port = DefaultPort
		slog.Info("no valid port provided via NOTES_API_PORT, using default",
			"portStr", portStr,
			"defaultPort", port)
	} else {
		slog.Info("using custom port",
			"port", port)
	}

	disableAuth := false
	disableAuthOption := strings.TrimSpace(os.Getenv("NOTES_API_DISABLE_AUTH"))
	if disableAuthOption != "" {
		slog.Warn("disabling authentication framework - THIS SHOULD ONLY BE RUN FOR TESTING!")
		disableAuth = true
	}

	if !disableAuth {
		authProviderUrl := os.Getenv("NOTES_API_AUTH_PROVIDER_URL")
		if authProviderUrl == "" {
			panic("Required value for NOTES_API_AUTH_PROVIDER_URL but none provided")
		}
		redirectUrl := os.Getenv("NOTES_API_REDIRECT_URL")
		if redirectUrl == "" {
			panic("Required value for NOTES_API_REDIRECT_URL but none provided")
		}
		auth.InitializeAuth(context.Background(), authProviderUrl, redirectUrl)
	} else {
		slog.Warn("skipping initialization of authentication framework", "disableAuth", disableAuth)
	}

	app := fiber.New()
	app.Use(requestid.New(), logger.New(), recover.New())
	app.Use(cors.New(cors.Config{
		AllowOrigins: "http://localhost:4444",
	}))
	app.Route("/notes", func(notes fiber.Router) {
		if !disableAuth {
			notes.Use(middleware.ValidateAccessToken(TokenLocalName, TokenCookieName))
		} else {
			slog.Warn("skipping registration of token validation middleware", "disableAuth", disableAuth)
		}
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
	if !disableAuth {
		app.Route("/auth", func(auth fiber.Router) {
			auth.Get("/login", Login)
			auth.Get("/logout", Logout)
			auth.Get("/callback", AuthCallback)
		})
	} else {
		slog.Warn("skipping registration of authentication-related endpoints", "disableAuth", disableAuth)
	}

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

func printHelp() {
	fmt.Fprintf(os.Stderr, `
notes-api [-h|--help|-?]

OPTIONS:
	-h|--help|-?	Display this help message and exit

ENVIRONMENT VARIABLES:
	NOTES_API_AUTH_PROVIDER_URL: (required) Base URL of the authorization server
	NOTES_API_DB_DIR:            (optional) Path to directory where notes.sqlite is located (default: %s)
	NOTES_API_PORT:              (optional) Port on which API should be hosted (default: %d)
`,
		NotesConfigDirectory,
		DefaultPort)
}

func getNoteFromContext(c *fiber.Ctx) *notesdb.IndexEntry {
	return c.Locals("note").(*notesdb.IndexEntry)
}

func ListNotes(c *fiber.Ctx) error {
	includePreview := strings.ToLower(c.Query("includePreview", "false"))
	if includePreview == "true" {
		notes, err := notesdb.GetNotesWithPreview(DB, 200)
		if err != nil {
			slog.Error("failed to execute query to retrieve notes",
				"err", err)
			return c.SendStatus(fiber.StatusInternalServerError)
		}
		return c.JSON(notes)
	} else {
		notes, err := notesdb.GetNotes(DB)
		if err != nil {
			slog.Error("failed to execute query to retrieve notes",
				"err", err)
			return c.SendStatus(fiber.StatusInternalServerError)
		}
		return c.JSON(notes)
	}
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

// Auth-related controllers

var nonceCache *cache.TimedCache[string] = cache.NewTimedCache[string](5*time.Minute, 100)

func createNonce() (string, error) {
	randomBytes := make([]byte, 32)
	_, err := rand.Read(randomBytes)
	if err != nil {
		return "", err
	}
	nonce := base64.StdEncoding.EncodeToString(randomBytes)
	nonceCache.Insert(nonce)
	return nonce, nil
}

func Login(c *fiber.Ctx) error {
	cameFromParam := c.Query("came_from")
	var cameFrom string
	if cameFromParam != "" {
		cameFromBytes, err := base64.URLEncoding.DecodeString(cameFromParam)
		if err == nil {
			cameFrom = string(cameFromBytes)
		}
	}

	state := &auth.State{CameFrom: cameFrom}
	nonce, err := createNonce()
	if err != nil {
		return err // TODO: Do something else here?
	}

	stateParam, err := state.Encode(nonce)
	if err != nil {
		return err // TODO: Do something else here?
	}
	url := auth.AuthConfig.LoginConfig.AuthCodeURL(stateParam)

	c.Status(fiber.StatusSeeOther)
	c.Redirect(url)
	return c.JSON(url)
}

func Logout(c *fiber.Ctx) error {
	// TODO: Invalidate token(s)
	c.ClearCookie(TokenCookieName)
	return c.SendString("Logout successful")
}

func AuthCallback(c *fiber.Ctx) error {
	stateParam := c.Query("state")
	state, nonce, err := auth.ParseState(stateParam)
	if err != nil {
		c.Status(fiber.StatusUnauthorized)
		return c.SendString(fmt.Sprintf("state is invalid: %s", err))
	}
	if _, ok := nonceCache.GetAndRemove(nonce); !ok {
		c.Status(fiber.StatusUnauthorized)
		return c.SendString("state is invalid: nonce not found in cache")
	}

	code := c.Query("code")
	fmt.Println("Code: " + code)

	kcConfig := auth.AuthConfig.LoginConfig

	token, err := kcConfig.Exchange(context.Background(), code)
	if err != nil {
		return c.SendString("Code-Token Exchange Failed")
	}

	_, err = auth.VerifyToken(c.Context(), token.AccessToken)
	if err != nil {
		return c.SendStatus(401)
	}

	c.Cookie(&fiber.Cookie{
		Name:  "access_token",
		Value: token.AccessToken,
	})

	if state.CameFrom != "" {
		c.Redirect(state.CameFrom)
	}
	return c.SendString("Login successful")
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
