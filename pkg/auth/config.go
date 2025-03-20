package auth

import (
	"context"
	"log/slog"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"
)

var AccessTokenCookieName string = "access_token"

type Config struct {
	BaseUri     string
	LoginConfig oauth2.Config
}

var AuthConfig Config

func InitializeAuth(context context.Context, authProviderUrl string, redirectUrl string) {
	AuthConfig = *BuildAuthConfig(context, "notes-api", authProviderUrl, redirectUrl)
}

// TODO: Don't panic
func BuildAuthConfig(context context.Context, clientID string, authProviderUrl string, redirectUrl string) *Config {
	provider, err := loadOIDCConfig(context, authProviderUrl)
	if err != nil {
		panic("Could not load OIDC configuration: " + err.Error())
	}

	config := &Config{
		LoginConfig: oauth2.Config{
			ClientID:    clientID,
			Endpoint:    provider.Endpoint(),
			RedirectURL: redirectUrl,
			Scopes:      []string{"profile", "email", oidc.ScopeOpenID},
		},
		BaseUri: authProviderUrl,
	}
	return config
}

func loadOIDCConfig(context context.Context, authProviderUrl string) (*oidc.Provider, error) {
	retries := 5
	var provider *oidc.Provider
	var err error
	i := 0
	for i < retries {
		provider, err = oidc.NewProvider(context, authProviderUrl)
		if err != nil {
			// TODO: Real retries/backoff
			slog.Warn("could not load OIDC config", "attempt", i+1, "url", authProviderUrl, "err", err)
			i++
			if i < retries {
				time.Sleep(time.Second * 10)
			}
		} else {
			break
		}
	}

	if err != nil {
		return nil, err
	}
	return provider, nil
}
