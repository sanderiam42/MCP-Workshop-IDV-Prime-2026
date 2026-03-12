package authserver

import (
	"net/http"
	"path/filepath"

	"xaa-mcp-demo/internal/shared/debuglog"
	"xaa-mcp-demo/internal/shared/jose"
)

type Service struct {
	issuer string
	store  *Store
	keys   *jose.KeySet
	logger *debuglog.Logger
}

func NewService(dataDir string, issuer string, logger *debuglog.Logger) (*Service, error) {
	keys, err := jose.LoadOrCreateRSAKey(filepath.Join(dataDir, "auth-signing-key.pem"))
	if err != nil {
		return nil, err
	}

	if logger == nil {
		logger, _ = debuglog.New("auth-server", false, "")
	}

	return &Service{
		issuer: issuer,
		store:  NewStore(dataDir),
		keys:   keys,
		logger: logger,
	}, nil
}

func (s *Service) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", s.handleHealthz)
	mux.HandleFunc("/.well-known/openid-configuration", s.handleOIDCMetadata)
	mux.HandleFunc("/oauth/jwks.json", s.handleJWKS)
	mux.HandleFunc("/authorize", s.handleAuthorize)
	mux.HandleFunc("/token", s.handleToken)
	mux.HandleFunc("/api/users", s.handleUsers)
	mux.HandleFunc("/api/clients", s.handleClients)
	mux.HandleFunc("/api/clients/", s.handleClientByID)
	mux.HandleFunc("/api/internal/clients/", s.handleInternalClientByID)
	mux.HandleFunc("/api/debug/state", s.handleDebugState)
	return debuglog.Middleware(s.logger, withJSONLoggingHeaders(mux))
}

func (s *Service) handleHealthz(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	s.writeJSON(w, http.StatusOK, map[string]string{
		"status": "ok",
	})
}

func withJSONLoggingHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		next.ServeHTTP(w, r)
	})
}
