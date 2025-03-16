package auth

import (
	"context"

	"github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"
)

var AccessTokenCookieName string = "access_token"

type Config struct {
	BaseUri     string
	LoginConfig oauth2.Config
}

var AuthConfig Config

func InitializeAuth(context context.Context, authProviderUrl string) {
	AuthConfig = *BuildAuthConfig(context, "notes-api", authProviderUrl)
}

// TODO: Don't panic
func BuildAuthConfig(context context.Context, clientID string, authProviderUrl string) *Config {
	provider, err := oidc.NewProvider(context, authProviderUrl)
	if err != nil {
		panic("Could not load OIDC configuration: " + err.Error())
	}

	config := &Config{
		LoginConfig: oauth2.Config{
			ClientID: clientID,
			Endpoint: provider.Endpoint(),
			Scopes:   []string{"profile", "email", oidc.ScopeOpenID},
		},
		BaseUri: authProviderUrl,
	}
	return config
}
