package authserver

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"xaa-mcp-demo/internal/shared/jose"
)

const (
	grantTypeAuthorizationCode = "authorization_code"
	grantTypeTokenExchange     = "urn:ietf:params:oauth:grant-type:token-exchange"
	grantTypeClientCredentials = "client_credentials"
	idTokenType                = "urn:ietf:params:oauth:token-type:id_token"
	idJagTokenType             = "urn:ietf:params:oauth:token-type:id-jag"
)

type tokenResponse struct {
	AccessToken     string `json:"access_token,omitempty"`
	TokenType       string `json:"token_type,omitempty"`
	ExpiresIn       int64  `json:"expires_in,omitempty"`
	IDToken         string `json:"id_token,omitempty"`
	Scope           string `json:"scope,omitempty"`
	IssuedTokenType string `json:"issued_token_type,omitempty"`
	Error           string `json:"error,omitempty"`
	Description     string `json:"error_description,omitempty"`
}

func (s *Service) authenticateClient(r *http.Request) (DemoClient, error) {
	clientID, clientSecret, ok := r.BasicAuth()
	if !ok {
		if err := r.ParseForm(); err != nil {
			return DemoClient{}, err
		}
		clientID = r.Form.Get("client_id")
		clientSecret = r.Form.Get("client_secret")
	}

	clientID = strings.TrimSpace(clientID)
	if clientID == "" {
		return DemoClient{}, errors.New("missing client credentials")
	}

	client, found, err := s.store.GetClient(clientID)
	if err != nil {
		return DemoClient{}, err
	}
	if !found {
		return DemoClient{}, errors.New("unknown client")
	}

	if subtle.ConstantTimeCompare([]byte(client.Secret), []byte(clientSecret)) != 1 {
		return DemoClient{}, errors.New("invalid client secret")
	}

	return client, nil
}

func (s *Service) issueIDToken(client DemoClient, userEmail, scope string) (string, map[string]any, time.Time, error) {
	expiresAt := time.Now().UTC().Add(5 * time.Minute)
	claims := map[string]any{
		"iss":   s.issuer,
		"sub":   userEmail,
		"aud":   client.ID,
		"iat":   time.Now().UTC().Unix(),
		"exp":   expiresAt.Unix(),
		"email": userEmail,
		"scope": strings.TrimSpace(scope),
	}

	token, err := s.keys.SignToken(claims, "JWT")
	if err != nil {
		return "", nil, time.Time{}, err
	}

	return token, claims, expiresAt, nil
}

func (s *Service) issueIDJAG(client DemoClient, userEmail, audience, resource, scope string) (string, map[string]any, time.Time, error) {
	expiresAt := time.Now().UTC().Add(5 * time.Minute)
	claims := map[string]any{
		"jti":       randomID("jag"),
		"iss":       s.issuer,
		"sub":       userEmail,
		"aud":       audience,
		"client_id": client.ID,
		"iat":       time.Now().UTC().Unix(),
		"exp":       expiresAt.Unix(),
		"resource":  resource,
		"scope":     strings.TrimSpace(scope),
		"email":     userEmail,
	}

	token, err := s.keys.SignToken(claims, "oauth-id-jag+jwt")
	if err != nil {
		return "", nil, time.Time{}, err
	}

	return token, claims, expiresAt, nil
}

func (s *Service) validateSubjectIDToken(token string, clientID string) (map[string]any, error) {
	header, claims, err := jose.VerifyToken(token, &s.keys.PrivateKey.PublicKey)
	if err != nil {
		return nil, err
	}
	if err := jose.ValidateTimeClaims(claims, time.Now().UTC()); err != nil {
		return nil, err
	}
	if jose.ClaimString(header, "alg") == "" {
		// no-op; keeps header used so lint does not complain when build tags differ
	}
	if jose.ClaimString(claims, "iss") != s.issuer {
		return nil, fmt.Errorf("unexpected issuer %q", jose.ClaimString(claims, "iss"))
	}
	if jose.ClaimString(claims, "aud") != clientID {
		return nil, fmt.Errorf("subject token audience mismatch")
	}
	return claims, nil
}

func (s *Service) writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func (s *Service) writeOAuthError(w http.ResponseWriter, status int, code, description string) {
	s.writeJSON(w, status, tokenResponse{
		Error:       code,
		Description: description,
	})
}

func (s *Service) recordTokenEvent(kind, userEmail, clientID, audience, resource, scope, token string, claims map[string]any, expiresAt time.Time) {
	_ = s.store.AddEvent(TokenEvent{
		Kind:         kind,
		UserEmail:    userEmail,
		ClientID:     clientID,
		Audience:     audience,
		Resource:     resource,
		Scope:        scope,
		IssuedAt:     time.Now().UTC().Format(time.RFC3339Nano),
		ExpiresAt:    expiresAt.UTC().Format(time.RFC3339Nano),
		TokenPreview: jose.TokenPreview(token),
		Claims:       claims,
	})
}

func randomID(prefix string) string {
	var bytes [8]byte
	_, _ = rand.Read(bytes[:])
	return prefix + "_" + hex.EncodeToString(bytes[:])
}
