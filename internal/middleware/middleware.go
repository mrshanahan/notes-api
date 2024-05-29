package middleware

import (
	"database/sql"
	"fmt"
	"log/slog"
	"strconv"

	"github.com/gofiber/fiber/v2"
	notesdb "github.com/mrshanahan/notes-api/pkg/notes-db"
)

func LoadNoteFromRoute(localName string, param string, db *sql.DB) func(*fiber.Ctx) error {
	return func(c *fiber.Ctx) error {
		idStr := c.Params(param)
		id, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil {
			c.Status(fiber.StatusBadRequest)
			return c.SendString("invalid request")
		}
		found, err := notesdb.GetNote(db, id)
		if err != nil {
			slog.Error("failed to execute query to retrieve note",
				"id", id,
				"err", err)
			c.Status(fiber.StatusInternalServerError)
			// TODO: More info here?
			return c.SendString("failed to load note")
		}
		if found == nil {
			c.Status(fiber.StatusNotFound)
			return c.SendString(fmt.Sprintf("no note with id: %d", id))
		}
		c.Locals(localName, found)
		return c.Next()
	}
}

// var bearerTokenPattern *regexp.Regexp = regexp.MustCompile(`^Bearer\s+(.*)$`)

// func ValidateAccessToken(localName string, cookieName string) func(*fiber.Ctx) error {
// 	return func(c *fiber.Ctx) error {
// 		reqHeaders := c.GetReqHeaders()
// 		var tokenStr string
// 		authHeaderValue, ok := reqHeaders["Authorization"]
// 		if !ok {
// 			// If no Authorization header, try cookie auth
// 			tokenStr = c.Cookies(cookieName)
// 		} else {
// 			match := bearerTokenPattern.FindStringSubmatch(authHeaderValue[0])
// 			if match == nil {
// 				return c.SendStatus(fiber.StatusUnauthorized)
// 			}
// 			tokenStr = match[1]
// 		}

// 		token, err := auth.VerifyToken(c.Context(), tokenStr)
// 		if err != nil {
// 			return c.SendStatus(fiber.StatusUnauthorized)
// 		}
// 		c.Locals(localName, token)
// 		return c.Next()
// 	}
// }
