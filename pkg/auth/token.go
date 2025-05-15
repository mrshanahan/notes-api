package auth

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/lestrrat-go/jwx/jwk"
	"github.com/lestrrat-go/jwx/jwt"
)

var (
	lastJwksCheckin time.Time
	cachedJwks      jwk.Set
)

const (
	JWKS_LOAD_TIMEOUT time.Duration = 6 * time.Hour
)

func VerifyToken(ctx context.Context, tokenString string) (*jwt.Token, error) {
	jwks, err := getJwks(ctx, AuthConfig.BaseUri)
	if err != nil {
		// TODO: panic here? Or just serve 401s? Not being to get JWKs is a Problem
		return nil, err
	}

	token, err := jwt.ParseString(tokenString,
		jwt.WithKeySet(jwks),
		jwt.WithIssuer(AuthConfig.BaseUri),
		jwt.WithValidate(true), // have to explicitly enable validation due to the version of jwx we're using
		//jwt.WithAudience("..."))
	)
	if err != nil {
		return nil, err
	}

	// TODO: additional validation
	return &token, nil
}

func getJwks(ctx context.Context, baseUri string) (jwk.Set, error) {
	if cachedJwks == nil || time.Since(lastJwksCheckin) > JWKS_LOAD_TIMEOUT {
		configUri, err := url.JoinPath(baseUri, "/.well-known/openid-configuration")
		if err != nil {
			return nil, err
		}
		resp, err := http.Get(configUri)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()

		config := &oidcConfig{}
		bytes, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, err
		}

		err = json.Unmarshal(bytes, config)
		if err != nil {
			return nil, err
		}

		cachedJwks, err = jwk.Fetch(ctx, config.JWKsURI)
		if err != nil {
			// TODO: panic here? Or just serve 401s? Not being to get JWKs is a Problem
			return nil, err
		}

		lastJwksCheckin = time.Now()
	}

	return cachedJwks, nil
}

type oidcConfig struct {
	JWKsURI string `json:"jwks_uri"`
}
