package auth

import (
	"context"
	"fmt"

	"github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"
)

var AccessTokenCookieName string = "access_token"

type Config struct {
	BaseUri     string
	LoginConfig oauth2.Config
}

var AuthConfig Config

func InitializeAuth(context context.Context, authApiRoot string, authProviderUrl string) {
	AuthConfig = *BuildNotesApiConfig(context, authApiRoot, authProviderUrl)
}

// TODO: Don't panic
func BuildNotesApiConfig(context context.Context, authApiRoot string, authProviderUrl string) *Config {
	provider, err := oidc.NewProvider(context, authProviderUrl)
	if err != nil {
		panic("Could not load OIDC configuration: " + err.Error())
	}

	config := &Config{
		LoginConfig: oauth2.Config{
			ClientID:     "test-notes-api",
			ClientSecret: "not-a-real-secret",
			RedirectURL:  fmt.Sprintf("%s/callback", authApiRoot),
			Endpoint:     provider.Endpoint(),
			Scopes:       []string{"profile", "email", oidc.ScopeOpenID},
		},
		BaseUri: authProviderUrl,
	}
	return config
}
