package auth

import (
	"context"

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

func InitializeAuth(context context.Context) {
	AuthConfig = *BuildKeycloakConfig(context)
}

func BuildKeycloakConfig(context context.Context) *Config {
	baseProviderUrl := "http://localhost:8080/realms/myrealm"
	provider, err := oidc.NewProvider(context, baseProviderUrl)
	if err != nil {
		panic("Could not load OIDC configuration: " + err.Error())
	}

	config := &Config{
		KeycloakLoginConfig: oauth2.Config{
			ClientID:     "test-auth-app",
			ClientSecret: "not-the-real-secret",
			RedirectURL:  "http://localhost:9090/keycloak_callback",
			Endpoint:     provider.Endpoint(),
			Scopes:       []string{"profile", "email", oidc.ScopeOpenID},
		},
		KeycloakBaseUri: baseProviderUrl,
		// KeycloakIDTokenVerifier: provider.Verifier(&oidc.Config{ClientID: AuthConfig.KeycloakLoginConfig.ClientID}),
	}
	return config
}
