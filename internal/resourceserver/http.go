package resourceserver

import (
	"crypto/rsa"
	"encoding/json"
	"net/http"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"xaa-mcp-demo/internal/shared/jose"
)

type Service struct {
	issuer      string
	resourceURI string
	authIssuer  string
	authJWKSURL string
	store       *Store
	keys        *jose.KeySet
	httpClient  *http.Client
	authKeyMu   sync.RWMutex
	authKey     *rsa.PublicKey
}

func NewService(dataDir string, issuer string, authIssuer string, authJWKSURL string) (*Service, error) {
	keys, err := jose.LoadOrCreateRSAKey(filepath.Join(dataDir, "resource-signing-key.pem"))
	if err != nil {
		return nil, err
	}

	return &Service{
		issuer:      issuer,
		resourceURI: strings.TrimRight(issuer, "/") + "/mcp",
		authIssuer:  authIssuer,
		authJWKSURL: authJWKSURL,
		store:       NewStore(dataDir),
		keys:        keys,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}, nil
}

func (s *Service) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", s.handleHealthz)
	mux.HandleFunc("/.well-known/oauth-protected-resource", s.handleProtectedResourceMetadata)
	mux.HandleFunc("/.well-known/oauth-protected-resource/mcp", s.handleProtectedResourceMetadata)
	mux.HandleFunc("/.well-known/openid-configuration", s.handleAuthorizationMetadata)
	mux.HandleFunc("/.well-known/oauth-authorization-server", s.handleAuthorizationMetadata)
	mux.HandleFunc("/oauth/jwks.json", s.handleJWKS)
	mux.HandleFunc("/oauth/token", s.handleToken)
	mux.HandleFunc("/mcp", s.handleMCP)
	mux.HandleFunc("/api/clients", s.handleClients)
	mux.HandleFunc("/api/clients/", s.handleClientByID)
	mux.HandleFunc("/api/debug/state", s.handleDebugState)
	return withHeaders(mux)
}

func (s *Service) handleHealthz(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	s.writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Service) handleClients(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		clients, err := s.store.ListClients()
		if err != nil {
			s.writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		s.writeJSON(w, http.StatusOK, map[string]any{"clients": publicClients(clients)})
	case http.MethodPost:
		var client DemoClient
		if err := json.NewDecoder(r.Body).Decode(&client); err != nil {
			s.writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON payload"})
			return
		}
		stored, err := s.store.SaveClient(client)
		if err != nil {
			s.writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		s.writeJSON(w, http.StatusCreated, map[string]any{"client": publicClient(stored)})
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (s *Service) handleClientByID(w http.ResponseWriter, r *http.Request) {
	clientID := strings.TrimPrefix(r.URL.Path, "/api/clients/")
	if clientID == "" || clientID == "/api/clients/" {
		s.writeJSON(w, http.StatusBadRequest, map[string]string{"error": "client id is required"})
		return
	}

	switch r.Method {
	case http.MethodGet:
		client, found, err := s.store.GetClient(clientID)
		if err != nil {
			s.writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		if !found {
			s.writeJSON(w, http.StatusNotFound, map[string]string{"error": "client not found"})
			return
		}
		s.writeJSON(w, http.StatusOK, map[string]any{"client": publicClient(client)})
	case http.MethodDelete:
		if err := s.store.DeleteClient(clientID); err != nil {
			s.writeJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
			return
		}
		s.writeJSON(w, http.StatusOK, map[string]string{"deleted": clientID})
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (s *Service) handleDebugState(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	state, err := s.store.DebugState()
	if err != nil {
		s.writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	s.writeJSON(w, http.StatusOK, state)
}

func withHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		next.ServeHTTP(w, r)
	})
}

func publicClients(clients []DemoClient) []map[string]any {
	items := make([]map[string]any, 0, len(clients))
	for _, client := range clients {
		items = append(items, publicClient(client))
	}
	return items
}

func publicClient(client DemoClient) map[string]any {
	return map[string]any{
		"id":           client.ID,
		"name":         client.Name,
		"redirect_uri": client.RedirectURI,
		"created_at":   client.CreatedAt,
	}
}
