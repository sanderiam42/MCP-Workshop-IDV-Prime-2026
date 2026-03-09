package authserver

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"time"

	"xaa-mcp-demo/internal/shared/jose"
)

func (s *Service) handleOIDCMetadata(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	s.writeJSON(w, http.StatusOK, map[string]any{
		"issuer":                                s.issuer,
		"authorization_endpoint":                s.issuer + "/authorize",
		"token_endpoint":                        s.issuer + "/token",
		"jwks_uri":                              s.issuer + "/oauth/jwks.json",
		"response_types_supported":              []string{"code"},
		"subject_types_supported":               []string{"public"},
		"id_token_signing_alg_values_supported": []string{"RS256"},
		"grant_types_supported": []string{
			grantTypeAuthorizationCode,
			grantTypeTokenExchange,
			grantTypeClientCredentials,
		},
		"token_endpoint_auth_methods_supported": []string{
			"client_secret_basic",
			"client_secret_post",
		},
		"scopes_supported": []string{"openid", "email", "mcp:read", "mcp:write"},
	})
}

func (s *Service) handleJWKS(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	s.writeJSON(w, http.StatusOK, s.keys.JWKS())
}

func (s *Service) handleAuthorize(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	query := r.URL.Query()
	if query.Get("response_type") != "code" {
		s.writeJSON(w, http.StatusBadRequest, map[string]string{"error": "response_type=code is required"})
		return
	}

	clientID := strings.TrimSpace(query.Get("client_id"))
	redirectURI := strings.TrimSpace(query.Get("redirect_uri"))
	state := strings.TrimSpace(query.Get("state"))
	scope := strings.TrimSpace(query.Get("scope"))
	codeChallenge := strings.TrimSpace(query.Get("code_challenge"))
	codeChallengeMethod := strings.TrimSpace(query.Get("code_challenge_method"))
	demoUser := strings.TrimSpace(strings.ToLower(query.Get("demo_user")))

	if codeChallenge == "" || codeChallengeMethod != "S256" {
		s.writeJSON(w, http.StatusBadRequest, map[string]string{"error": "PKCE with S256 is required"})
		return
	}
	if demoUser == "" {
		s.writeJSON(w, http.StatusBadRequest, map[string]string{"error": "demo_user query parameter is required"})
		return
	}

	client, found, err := s.store.GetClient(clientID)
	if err != nil {
		s.writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if !found {
		s.writeJSON(w, http.StatusBadRequest, map[string]string{"error": "unknown client"})
		return
	}
	if client.RedirectURI != redirectURI {
		s.writeJSON(w, http.StatusBadRequest, map[string]string{"error": "redirect URI does not match registered client"})
		return
	}

	users, err := s.store.ListUsers()
	if err != nil {
		s.writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	userFound := false
	for _, user := range users {
		if user.Email == demoUser {
			userFound = true
			break
		}
	}
	if !userFound {
		s.writeJSON(w, http.StatusBadRequest, map[string]string{"error": "demo user is not enrolled"})
		return
	}

	authCode := AuthCode{
		Code:                randomHex(16),
		ClientID:            clientID,
		RedirectURI:         redirectURI,
		UserEmail:           demoUser,
		Scope:               scope,
		CodeChallenge:       codeChallenge,
		CodeChallengeMethod: codeChallengeMethod,
		ExpiresAt:           time.Now().UTC().Add(2 * time.Minute).Format(time.RFC3339Nano),
	}
	if err := s.store.SaveAuthCode(authCode); err != nil {
		s.writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	location, err := url.Parse(redirectURI)
	if err != nil {
		s.writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid redirect URI"})
		return
	}
	values := location.Query()
	values.Set("code", authCode.Code)
	if state != "" {
		values.Set("state", state)
	}
	location.RawQuery = values.Encode()
	http.Redirect(w, r, location.String(), http.StatusFound)
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

	client, err := s.authenticateClient(r)
	if err != nil {
		s.writeOAuthError(w, http.StatusUnauthorized, "invalid_client", err.Error())
		return
	}

	switch r.Form.Get("grant_type") {
	case grantTypeAuthorizationCode:
		s.handleAuthorizationCodeGrant(w, r, client)
	case grantTypeTokenExchange:
		s.handleTokenExchangeGrant(w, r, client)
	case grantTypeClientCredentials:
		s.handleClientCredentialsGrant(w, r, client)
	default:
		s.writeOAuthError(w, http.StatusBadRequest, "unsupported_grant_type", "grant type is not supported")
	}
}

func (s *Service) handleAuthorizationCodeGrant(w http.ResponseWriter, r *http.Request, client DemoClient) {
	code := strings.TrimSpace(r.Form.Get("code"))
	redirectURI := strings.TrimSpace(r.Form.Get("redirect_uri"))
	codeVerifier := strings.TrimSpace(r.Form.Get("code_verifier"))

	if code == "" || redirectURI == "" || codeVerifier == "" {
		s.writeOAuthError(w, http.StatusBadRequest, "invalid_request", "code, redirect_uri, and code_verifier are required")
		return
	}

	authCode, err := s.store.ConsumeAuthCode(code)
	if err != nil {
		s.writeOAuthError(w, http.StatusBadRequest, "invalid_grant", err.Error())
		return
	}

	if authCode.ClientID != client.ID || authCode.RedirectURI != redirectURI {
		s.writeOAuthError(w, http.StatusBadRequest, "invalid_grant", "authorization code does not match client")
		return
	}

	expectedChallenge := sha256.Sum256([]byte(codeVerifier))
	encodedChallenge := base64.RawURLEncoding.EncodeToString(expectedChallenge[:])
	if encodedChallenge != authCode.CodeChallenge {
		s.writeOAuthError(w, http.StatusBadRequest, "invalid_grant", "PKCE verification failed")
		return
	}

	idToken, claims, expiresAt, err := s.issueIDToken(client, authCode.UserEmail, authCode.Scope)
	if err != nil {
		s.writeOAuthError(w, http.StatusInternalServerError, "server_error", err.Error())
		return
	}

	s.recordTokenEvent("id_token", authCode.UserEmail, client.ID, client.ID, "", authCode.Scope, idToken, claims, expiresAt)
	s.writeJSON(w, http.StatusOK, tokenResponse{
		AccessToken: "demo-sso-access-" + randomHex(10),
		TokenType:   "Bearer",
		ExpiresIn:   int64(time.Until(expiresAt).Seconds()),
		IDToken:     idToken,
		Scope:       authCode.Scope,
	})
}

func (s *Service) handleTokenExchangeGrant(w http.ResponseWriter, r *http.Request, client DemoClient) {
	if r.Form.Get("requested_token_type") != idJagTokenType {
		s.writeOAuthError(w, http.StatusBadRequest, "invalid_request", "requested_token_type must be urn:ietf:params:oauth:token-type:id-jag")
		return
	}
	if r.Form.Get("subject_token_type") != idTokenType {
		s.writeOAuthError(w, http.StatusBadRequest, "invalid_request", "subject_token_type must be urn:ietf:params:oauth:token-type:id_token")
		return
	}

	subjectToken := strings.TrimSpace(r.Form.Get("subject_token"))
	audience := strings.TrimSpace(r.Form.Get("audience"))
	resource := strings.TrimSpace(r.Form.Get("resource"))
	scope, err := normalizeMCPScopes(r.Form.Get("scope"))
	if err != nil {
		s.writeOAuthError(w, http.StatusBadRequest, "invalid_scope", err.Error())
		return
	}
	if subjectToken == "" || audience == "" || resource == "" {
		s.writeOAuthError(w, http.StatusBadRequest, "invalid_request", "subject_token, audience, and resource are required")
		return
	}

	subjectClaims, err := s.validateSubjectIDToken(subjectToken, client.ID)
	if err != nil {
		s.writeOAuthError(w, http.StatusBadRequest, "invalid_grant", err.Error())
		return
	}

	userEmail := strings.ToLower(strings.TrimSpace(jose.ClaimString(subjectClaims, "email")))
	if userEmail == "" {
		s.writeOAuthError(w, http.StatusBadRequest, "invalid_grant", "subject token email claim is required")
		return
	}
	idJag, claims, expiresAt, err := s.issueIDJAG(client, userEmail, audience, resource, scope)
	if err != nil {
		s.writeOAuthError(w, http.StatusInternalServerError, "server_error", err.Error())
		return
	}

	s.recordTokenEvent("id_jag", userEmail, client.ID, audience, resource, scope, idJag, claims, expiresAt)
	s.writeJSON(w, http.StatusOK, tokenResponse{
		AccessToken:     idJag,
		TokenType:       "N_A",
		ExpiresIn:       int64(time.Until(expiresAt).Seconds()),
		Scope:           scope,
		IssuedTokenType: idJagTokenType,
	})
}

func (s *Service) handleClientCredentialsGrant(w http.ResponseWriter, r *http.Request, client DemoClient) {
	audience := strings.TrimSpace(r.Form.Get("audience"))
	resource := strings.TrimSpace(r.Form.Get("resource"))
	scope, err := normalizeMCPScopes(r.Form.Get("scope"))
	if err != nil {
		s.writeOAuthError(w, http.StatusBadRequest, "invalid_scope", err.Error())
		return
	}
	if audience == "" || resource == "" {
		s.writeOAuthError(w, http.StatusBadRequest, "invalid_request", "audience and resource are required")
		return
	}

	idJag, claims, expiresAt, err := s.issueIDJAG(client, client.ID, audience, resource, scope)
	if err != nil {
		s.writeOAuthError(w, http.StatusInternalServerError, "server_error", err.Error())
		return
	}

	s.recordTokenEvent("cc_id_jag", client.ID, client.ID, audience, resource, scope, idJag, claims, expiresAt)
	s.writeJSON(w, http.StatusOK, tokenResponse{
		AccessToken:     idJag,
		TokenType:       "N_A",
		ExpiresIn:       int64(time.Until(expiresAt).Seconds()),
		Scope:           scope,
		IssuedTokenType: idJagTokenType,
	})
}

func randomHex(length int) string {
	bytes := make([]byte, length)
	_, _ = rand.Read(bytes)
	return hex.EncodeToString(bytes)
}

func normalizeMCPScopes(raw string) (string, error) {
	if strings.TrimSpace(raw) == "" {
		return "mcp:read", nil
	}

	allowed := []string{"mcp:read", "mcp:write"}
	fields := strings.Fields(raw)
	normalized := make([]string, 0, len(fields))
	for _, field := range fields {
		if !slices.Contains(allowed, field) {
			return "", fmt.Errorf("unsupported scope %q", field)
		}
		if !slices.Contains(normalized, field) {
			normalized = append(normalized, field)
		}
	}
	if len(normalized) == 0 {
		return "mcp:read", nil
	}
	return strings.Join(normalized, " "), nil
}
