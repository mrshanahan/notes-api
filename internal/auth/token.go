package auth

import (
	"context"
	"encoding/json"
	"io"
	"net/http"

	"github.com/lestrrat-go/jwx/jwk"
	"github.com/lestrrat-go/jwx/jwt"
)

func VerifyToken(ctx context.Context, tokenString string) (*jwt.Token, error) {
	jwksUri, err := getJwksUri(AuthConfig.KeycloakBaseUri)
	if err != nil {
		panic("ahhhh")
	}

	jwks, err := jwk.Fetch(ctx, jwksUri)
	if err != nil {
		// TODO: panic here? Or just serve 401s? Not being to get JWKs is a Problem
		return nil, err
	}

	token, err := jwt.ParseString(tokenString,
		jwt.WithKeySet(jwks),
		jwt.WithIssuer(AuthConfig.KeycloakBaseUri),
		//jwt.WithAudience("..."))
	)
	if err != nil {
		return nil, err
	}

	// TODO: additional validation
	return &token, nil
}

func getJwksUri(rootUri string) (string, error) {
	configUri := rootUri + "/.well-known/openid-configuration"
	resp, err := http.Get(configUri)
	if err != nil {
		panic("ahhhh")
	}
	defer resp.Body.Close()

	config := &oidcConfig{}
	bytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	err = json.Unmarshal(bytes, config)
	if err != nil {
		return "", err
	}

	return config.JWKsURI, nil
}

type oidcConfig struct {
	JWKsURI string `json:"jwks_uri"`
}
