package auth

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
)

type State struct {
	CameFrom string `json:"came_from"`
}

func (state State) Encode(nonce string) (string, error) {
	return encode(state, nonce, AuthConfig.LoginConfig.ClientSecret)
}

func ParseState(param string) (*State, string, error) {
	parts := strings.Split(param, ".")
	if len(parts) != 3 {
		return nil, "", fmt.Errorf("invalid state parameter - expected 3 parts, got %d", len(parts))
	}

	encodedState, nonce, signature := parts[0], parts[1], parts[2]

	rebuiltSignature := getSignature(encodedState, nonce, AuthConfig.LoginConfig.ClientSecret)
	if signature != rebuiltSignature {
		return nil, "", fmt.Errorf("invalid state parameter - signature is invalid")
	}

	var state State
	encodedStateBytes, err := base64.StdEncoding.DecodeString(encodedState)
	if err != nil {
		return nil, "", err
	}
	err = json.Unmarshal(encodedStateBytes, &state)
	if err != nil {
		return nil, "", err
	}
	return &state, nonce, nil
}

// This is based on this SO answer: https://stackoverflow.com/a/77029859
// The solution there is modified to include some useful state, in this case just
// a Base64-encoded URL to ultimately redirect to.
func encode(state State, nonce string, privateKey string) (string, error) {
	jsonStateBytes, err := json.Marshal(state)
	if err != nil {
		return "", err
	}
	jsonStateB64 := base64.StdEncoding.EncodeToString(jsonStateBytes)
	signature := getSignature(jsonStateB64, nonce, privateKey)
	return fmt.Sprintf("%s.%s.%s", jsonStateB64, nonce, signature), nil
}

func getSignature(encodedState string, nonce string, privateKey string) string {
	signaturePayload := encodedState + nonce + privateKey
	signatureHash := sha256.Sum256([]byte(signaturePayload))
	signature := base64.StdEncoding.EncodeToString(signatureHash[:])
	return signature
}
