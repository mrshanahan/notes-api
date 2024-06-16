package auth

import (
	"context"
	"fmt"

	"github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"
)

var AccessTokenCookieName string = "access_token"

type Config struct {
	KeycloakBaseUri         string
	KeycloakLoginConfig     oauth2.Config
	KeycloakIDTokenVerifier *oidc.IDTokenVerifier
}

var AuthConfig Config

func InitializeAuth(context context.Context, authApiRoot string) {
	AuthConfig = *BuildKeycloakConfig(context, authApiRoot)
}

func BuildKeycloakConfig(context context.Context, authApiRoot string) *Config {
	baseProviderUrl := "http://localhost:8080/realms/myrealm"
	provider, err := oidc.NewProvider(context, baseProviderUrl)
	if err != nil {
		panic("Could not load OIDC configuration: " + err.Error())
	}

	config := &Config{
		KeycloakLoginConfig: oauth2.Config{
			ClientID:     "test-notes-api",
			ClientSecret: "not-the-real-secret",
			RedirectURL:  fmt.Sprintf("%s/callback", authApiRoot),
			Endpoint:     provider.Endpoint(),
			Scopes:       []string{"profile", "email", oidc.ScopeOpenID},
		},
		KeycloakBaseUri: baseProviderUrl,
		// KeycloakIDTokenVerifier: provider.Verifier(&oidc.Config{ClientID: AuthConfig.KeycloakLoginConfig.ClientID}),
	}
	return config
}
