package resourceserver

import (
	"crypto/rsa"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"xaa-mcp-demo/internal/shared/jose"
)

const (
	jwtBearerGrantType = "urn:ietf:params:oauth:grant-type:jwt-bearer"
)

type tokenResponse struct {
	AccessToken string `json:"access_token,omitempty"`
	TokenType   string `json:"token_type,omitempty"`
	ExpiresIn   int64  `json:"expires_in,omitempty"`
	Scope       string `json:"scope,omitempty"`
	Resource    string `json:"resource,omitempty"`
	Error       string `json:"error,omitempty"`
	Description string `json:"error_description,omitempty"`
}

func (s *Service) handleProtectedResourceMetadata(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	s.writeJSON(w, http.StatusOK, map[string]any{
		"resource":              s.resourceURI,
		"authorization_servers": []string{s.issuer},
		"scopes_supported":      []string{"mcp:read", "mcp:write"},
	})
}

func (s *Service) handleAuthorizationMetadata(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	payload := map[string]any{
		"issuer":                                s.issuer,
		"token_endpoint":                        s.issuer + "/oauth/token",
		"jwks_uri":                              s.issuer + "/oauth/jwks.json",
		"grant_types_supported":                 []string{jwtBearerGrantType},
		"token_endpoint_auth_methods_supported": []string{"client_secret_basic", "client_secret_post"},
		"scopes_supported":                      []string{"mcp:read", "mcp:write"},
	}
	s.writeJSON(w, http.StatusOK, payload)
}

func (s *Service) handleJWKS(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	s.writeJSON(w, http.StatusOK, s.keys.JWKS())
}

func (s *Service) handleToken(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	if err := r.ParseForm(); err != nil {
		s.writeOAuthError(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}

	if r.Form.Get("grant_type") != jwtBearerGrantType {
		s.writeOAuthError(w, http.StatusBadRequest, "unsupported_grant_type", "grant_type must be urn:ietf:params:oauth:grant-type:jwt-bearer")
		return
	}

	client, err := s.authenticateClient(r)
	if err != nil {
		s.writeOAuthError(w, http.StatusUnauthorized, "invalid_client", err.Error())
		return
	}

	assertion := strings.TrimSpace(r.Form.Get("assertion"))
	if assertion == "" {
		s.writeOAuthError(w, http.StatusBadRequest, "invalid_request", "assertion is required")
		return
	}

	claims, err := s.validateIDJAG(assertion, client.ID)
	if err != nil {
		s.writeOAuthError(w, http.StatusBadRequest, "invalid_grant", err.Error())
		return
	}

	userEmail := jose.ClaimString(claims, "email")
	scope := jose.ClaimString(claims, "scope")
	resource := jose.ClaimString(claims, "resource")
	if resource == "" {
		resource = s.resourceURI
	}
	if userEmail == "" {
		s.writeOAuthError(w, http.StatusBadRequest, "invalid_grant", "id-jag email claim is required")
		return
	}
	normalizedScopes, err := normalizeMCPScopes(scope)
	if err != nil {
		s.writeOAuthError(w, http.StatusBadRequest, "invalid_scope", err.Error())
		return
	}
	scope = strings.Join(normalizedScopes, " ")
	if resource != s.resourceURI {
		s.writeOAuthError(w, http.StatusBadRequest, "invalid_target", "requested resource does not match this resource server")
		return
	}

	accessToken, accessClaims, expiresAt, err := s.issueAccessToken(client, userEmail, resource, scope)
	if err != nil {
		s.writeOAuthError(w, http.StatusInternalServerError, "server_error", err.Error())
		return
	}

	_ = s.store.RecordAccessToken(AccessTokenEvent{
		UserEmail:    userEmail,
		ClientID:     client.ID,
		Scope:        scope,
		Resource:     resource,
		IssuedAt:     time.Now().UTC().Format(time.RFC3339Nano),
		ExpiresAt:    expiresAt.UTC().Format(time.RFC3339Nano),
		TokenPreview: jose.TokenPreview(accessToken),
		Claims:       accessClaims,
	})

	s.writeJSON(w, http.StatusOK, tokenResponse{
		AccessToken: accessToken,
		TokenType:   "Bearer",
		ExpiresIn:   int64(time.Until(expiresAt).Seconds()),
		Scope:       scope,
		Resource:    resource,
	})
}

func (s *Service) authenticateClient(r *http.Request) (DemoClient, error) {
	clientID, clientSecret, ok := r.BasicAuth()
	if !ok {
		clientID = r.Form.Get("client_id")
		clientSecret = r.Form.Get("client_secret")
	}
	clientID = strings.TrimSpace(clientID)
	clientSecret = strings.TrimSpace(clientSecret)
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

func (s *Service) validateIDJAG(token string, clientID string) (map[string]any, error) {
	publicKey, err := s.authPublicKey()
	if err != nil {
		return nil, err
	}

	header, claims, err := jose.VerifyToken(token, publicKey)
	if err != nil {
		return nil, err
	}
	if err := jose.ValidateTimeClaims(claims, time.Now().UTC()); err != nil {
		return nil, err
	}
	if jose.HeaderString(header, "typ") != "oauth-id-jag+jwt" {
		return nil, fmt.Errorf("unexpected token type %q", jose.HeaderString(header, "typ"))
	}
	if jose.ClaimString(claims, "iss") != s.authIssuer {
		return nil, fmt.Errorf("unexpected issuer %q", jose.ClaimString(claims, "iss"))
	}
	if jose.ClaimString(claims, "aud") != s.issuer {
		return nil, errors.New("id-jag audience does not match resource authorization server")
	}
	if jose.ClaimString(claims, "client_id") != clientID {
		return nil, errors.New("id-jag client_id does not match authenticated client")
	}
	return claims, nil
}

func (s *Service) issueAccessToken(client DemoClient, userEmail, resource, scope string) (string, map[string]any, time.Time, error) {
	expiresAt := time.Now().UTC().Add(10 * time.Minute)
	claims := map[string]any{
		"iss":       s.issuer,
		"sub":       userEmail,
		"aud":       s.resourceURI,
		"iat":       time.Now().UTC().Unix(),
		"exp":       expiresAt.Unix(),
		"email":     userEmail,
		"scope":     scope,
		"resource":  resource,
		"client_id": client.ID,
	}

	token, err := s.keys.SignToken(claims, "at+jwt")
	if err != nil {
		return "", nil, time.Time{}, err
	}
	return token, claims, expiresAt, nil
}

func (s *Service) validateAccessToken(token string) (map[string]any, error) {
	header, claims, err := jose.VerifyToken(token, &s.keys.PrivateKey.PublicKey)
	if err != nil {
		return nil, err
	}
	if jose.HeaderString(header, "typ") != "at+jwt" {
		return nil, errors.New("unexpected access token type")
	}
	if err := jose.ValidateTimeClaims(claims, time.Now().UTC()); err != nil {
		return nil, err
	}
	if jose.ClaimString(claims, "iss") != s.issuer {
		return nil, errors.New("unexpected access token issuer")
	}
	if jose.ClaimString(claims, "aud") != s.resourceURI {
		return nil, errors.New("unexpected access token audience")
	}
	resource := jose.ClaimString(claims, "resource")
	if resource != "" && resource != s.resourceURI {
		return nil, errors.New("unexpected access token resource")
	}
	return claims, nil
}

func (s *Service) authPublicKey() (*rsa.PublicKey, error) {
	s.authKeyMu.RLock()
	if s.authKey != nil {
		defer s.authKeyMu.RUnlock()
		return s.authKey, nil
	}
	s.authKeyMu.RUnlock()

	response, err := s.httpClient.Get(s.authJWKSURL)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetch auth jwks: unexpected status %d", response.StatusCode)
	}

	data, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, err
	}

	publicKey, _, err := jose.PublicKeyFromJWKS(data)
	if err != nil {
		return nil, err
	}

	s.authKeyMu.Lock()
	defer s.authKeyMu.Unlock()
	s.authKey = publicKey
	return publicKey, nil
}

func (s *Service) writeOAuthError(w http.ResponseWriter, status int, code, description string) {
	s.writeJSON(w, status, tokenResponse{
		Error:       code,
		Description: description,
	})
}

func (s *Service) writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}
