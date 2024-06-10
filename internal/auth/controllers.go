package auth

import (
	"fmt"

	"github.com/gofiber/fiber/v2"
)

func LoginController(c *fiber.Ctx) error {
	// TODO: Actually random state
	url := AuthConfig.KeycloakLoginConfig.AuthCodeURL("randomstate")

	c.Status(fiber.StatusSeeOther)
	c.Redirect(url)
	return c.JSON(url)
}

func LogoutController(c *fiber.Ctx) error {
	c.ClearCookie(AccessTokenCookieName)
	return c.SendString("Logout successful")
}

func AuthCallbackController(c *fiber.Ctx) error {
	state := c.Query("state")
	if state != "randomstate" {
		return c.SendString("States don't Match!!")
	}

	code := c.Query("code")
	fmt.Println("Code: " + code)

	kcConfig := AuthConfig.KeycloakLoginConfig

	tokenResponse, err := kcConfig.Exchange(c.Context(), code)
	if err != nil {
		return c.SendString("Code-Token Exchange Failed")
	}

	_, err = VerifyToken(c.Context(), tokenResponse.AccessToken)
	if err != nil {
		return c.SendStatus(401)
	}

	c.Cookie(&fiber.Cookie{
		Name:  "access_token",
		Value: tokenResponse.AccessToken,
	})

	return c.Redirect("/foobar")
}
