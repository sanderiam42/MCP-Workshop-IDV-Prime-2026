package requestingapp

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"xaa-mcp-demo/internal/shared/debuglog"
	"xaa-mcp-demo/internal/shared/demo"
	"xaa-mcp-demo/internal/shared/jose"
)

type cachedToken struct {
	accessToken string
	expiresAt   time.Time
}

type Service struct {
	publicBase string
	webDir     string
	store      *Store
	runner     *Runner
	logger     *debuglog.Logger
	tokenCache sync.Map
}

func NewService(dataDir, webDir, publicBase, authPublicBase, authInternalBase, resourcePublicBase, resourceInternalBase string, logger *debuglog.Logger) *Service {
	if logger == nil {
		logger, _ = debuglog.New("requesting-app", false, "")
	}
	return &Service{
		publicBase: strings.TrimRight(publicBase, "/"),
		webDir:     webDir,
		store:      NewStore(dataDir),
		runner:     NewRunner(authPublicBase, authInternalBase, resourcePublicBase, resourceInternalBase, logger),
		logger:     logger,
	}
}

func (s *Service) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", s.handleHealthz)
	mux.HandleFunc("/api/dashboard", s.handleDashboard)
	mux.HandleFunc("/api/users", s.handleCreateUser)
	mux.HandleFunc("/api/clients", s.handleCreateClient)
	mux.HandleFunc("/api/clients/provision", s.handleProvisionClient)
	mux.HandleFunc("/api/flow/run", s.handleRunFlow)
	mux.HandleFunc("/host/mcp", s.handleHostMCP)
	mux.Handle("/", s.handleStatic())
	return debuglog.Middleware(s.logger, withHeaders(mux))
}

func (s *Service) handleHealthz(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	s.writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Service) handleDashboard(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	email := strings.TrimSpace(r.URL.Query().Get("email"))
	clientID := strings.TrimSpace(r.URL.Query().Get("client_id"))
	if clientID == "" {
		clientID = demo.DefaultClientID
	}

	authStatus, _, _, authBody, authErr := s.runner.getJSON(r.Context(), s.runner.authPublicBase+"/api/debug/state")
	resourceStatus, _, _, resourceBody, resourceErr := s.runner.getJSON(r.Context(), s.runner.resourcePublicBase+"/api/debug/state")
	flows, flowErr := s.store.ListFlows()

	authPayload := map[string]any{}
	if authErr == nil && authStatus == http.StatusOK {
		authPayload = sanitizeSecrets(authBody)
	}
	resourcePayload := map[string]any{}
	if resourceErr == nil && resourceStatus == http.StatusOK {
		resourcePayload = sanitizeSecrets(resourceBody)
	}
	if flowErr != nil {
		s.writeJSON(w, http.StatusInternalServerError, map[string]string{"error": flowErr.Error()})
		return
	}

	s.writeJSON(w, http.StatusOK, DashboardData{
		Auth:     authPayload,
		Resource: resourcePayload,
		Flows:    toAnySlice(flows),
		Snippets: BuildSnippets(email, clientID, s.publicBase),
		Selection: map[string]string{
			"user_email": email,
			"client_id":  clientID,
		},
	})
}

func (s *Service) handleCreateUser(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	var payload map[string]string
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		s.writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON payload"})
		return
	}

	status, body, err := s.postJSON(r.Context(), s.runner.authPublicBase+"/api/users", payload)
	if err != nil {
		s.writeJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
		return
	}
	s.writeJSON(w, status, body)
}

func (s *Service) handleCreateClient(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	var payload map[string]string
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		s.writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON payload"})
		return
	}

	clientPayload := map[string]string{
		"id":           strings.TrimSpace(payload["id"]),
		"name":         strings.TrimSpace(payload["name"]),
		"secret":       randomURLToken(16),
		"redirect_uri": strings.TrimSpace(payload["redirect_uri"]),
	}
	if clientPayload["redirect_uri"] == "" {
		clientPayload["redirect_uri"] = s.publicBase + "/callback"
	}

	authStatus, authBody, err := s.postJSON(r.Context(), s.runner.authPublicBase+"/api/clients", clientPayload)
	if err != nil {
		s.writeJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
		return
	}
	if authStatus >= 300 {
		s.writeJSON(w, authStatus, authBody)
		return
	}

	resourceStatus, resourceBody, err := s.postJSON(r.Context(), s.runner.resourcePublicBase+"/api/clients", clientPayload)
	if err != nil {
		_, _, _ = s.deleteJSON(r.Context(), s.runner.authPublicBase+"/api/clients/"+clientPayload["id"])
		s.writeJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
		return
	}
	if resourceStatus >= 300 {
		_, _, _ = s.deleteJSON(r.Context(), s.runner.authPublicBase+"/api/clients/"+clientPayload["id"])
		s.writeJSON(w, resourceStatus, resourceBody)
		return
	}

	// Return the generated secret once — it is not retrievable again after this point.
	s.writeJSON(w, http.StatusCreated, map[string]any{
		"client_id":     clientPayload["id"],
		"client_secret": clientPayload["secret"],
		"auth":          sanitizeSecrets(authBody),
		"resource":      sanitizeSecrets(resourceBody),
	})
}

func (s *Service) handleProvisionClient(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	var payload map[string]string
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		s.writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON payload"})
		return
	}

	name := strings.TrimSpace(payload["name"])
	var clientID string
	if name != "" {
		clientID = slugify(name) + "-" + randomURLToken(6)[:8]
	} else {
		clientID = "client-" + randomURLToken(6)[:8]
	}
	secret := randomURLToken(16)

	clientPayload := map[string]string{
		"id":           clientID,
		"name":         name,
		"secret":       secret,
		"redirect_uri": s.publicBase + "/callback",
	}

	authStatus, authBody, err := s.postJSON(r.Context(), s.runner.authPublicBase+"/api/clients", clientPayload)
	if err != nil {
		s.writeJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
		return
	}
	if authStatus >= 300 {
		s.writeJSON(w, authStatus, authBody)
		return
	}

	resourceStatus, resourceBody, err := s.postJSON(r.Context(), s.runner.resourcePublicBase+"/api/clients", clientPayload)
	if err != nil {
		_, _, _ = s.deleteJSON(r.Context(), s.runner.authPublicBase+"/api/clients/"+clientID)
		s.writeJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
		return
	}
	if resourceStatus >= 300 {
		_, _, _ = s.deleteJSON(r.Context(), s.runner.authPublicBase+"/api/clients/"+clientID)
		s.writeJSON(w, resourceStatus, resourceBody)
		return
	}

	s.writeJSON(w, http.StatusCreated, map[string]any{
		"client_id":     clientID,
		"client_secret": secret,
		"auth":          sanitizeSecrets(authBody),
		"resource":      sanitizeSecrets(resourceBody),
	})
}

// slugify converts a display name into a URL-safe lowercase slug.
// Non-alphanumeric characters become hyphens; consecutive hyphens are collapsed.
func slugify(name string) string {
	name = strings.ToLower(strings.TrimSpace(name))
	var b strings.Builder
	prevHyphen := true // suppress leading hyphens
	for _, r := range name {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
			prevHyphen = false
		} else if !prevHyphen {
			b.WriteRune('-')
			prevHyphen = true
		}
	}
	result := strings.TrimRight(b.String(), "-")
	if len(result) > 40 {
		result = result[:40]
	}
	if result == "" {
		return "client"
	}
	return result
}

func (s *Service) handleRunFlow(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	var input FlowInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		s.writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON payload"})
		return
	}
	if input.ClientID == "" {
		input.ClientID = demo.DefaultClientID
	}

	var (
		flow interface{}
		err  error
	)
	if input.ClientSecret != "" && input.UserEmail == "" {
		f, _, e := s.runner.RunClientCredentials(r.Context(), "browser", input)
		_ = s.store.SaveFlow(f)
		flow, err = f, e
	} else {
		f, _, e := s.runner.Run(r.Context(), "browser", input)
		_ = s.store.SaveFlow(f)
		flow, err = f, e
	}
	if err != nil {
		s.writeJSON(w, http.StatusBadRequest, flow)
		return
	}
	s.writeJSON(w, http.StatusOK, flow)
}

func (s *Service) postJSON(ctx context.Context, publicURL string, payload any) (int, map[string]any, error) {
	data, err := json.Marshal(payload)
	if err != nil {
		return 0, nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.runner.internalURL(publicURL), bytes.NewReader(data))
	if err != nil {
		return 0, nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.runner.httpClient.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer resp.Body.Close()

	rawBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, nil, err
	}

	var parsed map[string]any
	if len(rawBody) > 0 {
		if err := json.Unmarshal(rawBody, &parsed); err != nil {
			return resp.StatusCode, map[string]any{"raw": string(rawBody)}, nil
		}
	}
	if parsed == nil {
		parsed = map[string]any{}
	}
	return resp.StatusCode, parsed, nil
}

func (s *Service) deleteJSON(ctx context.Context, publicURL string) (int, map[string]any, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, s.runner.internalURL(publicURL), nil)
	if err != nil {
		return 0, nil, err
	}

	resp, err := s.runner.httpClient.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer resp.Body.Close()

	rawBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, nil, err
	}

	var parsed map[string]any
	if len(rawBody) > 0 {
		if err := json.Unmarshal(rawBody, &parsed); err != nil {
			return resp.StatusCode, map[string]any{"raw": string(rawBody)}, nil
		}
	}
	if parsed == nil {
		parsed = map[string]any{}
	}
	return resp.StatusCode, parsed, nil
}

func (s *Service) handleStatic() http.Handler {
	fileServer := http.FileServer(http.Dir(s.webDir))
	indexPath := filepath.Join(s.webDir, "index.html")

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/") || strings.HasPrefix(r.URL.Path, "/host/") {
			http.NotFound(w, r)
			return
		}

		path := filepath.Join(s.webDir, filepath.Clean(r.URL.Path))
		if info, err := os.Stat(path); err == nil && !info.IsDir() {
			fileServer.ServeHTTP(w, r)
			return
		}
		if _, err := os.Stat(indexPath); errors.Is(err, os.ErrNotExist) {
			s.writeJSON(w, http.StatusServiceUnavailable, map[string]string{
				"error": "frontend assets are not built yet",
			})
			return
		}
		http.ServeFile(w, r, indexPath)
	})
}

func (s *Service) cacheSet(key, accessToken string) {
	_, claims, _ := jose.DecodeWithoutVerify(accessToken)
	var expiresAt time.Time
	if exp, ok := claims["exp"].(float64); ok {
		expiresAt = time.Unix(int64(exp), 0).UTC()
	}
	if time.Until(expiresAt) <= 30*time.Second {
		return
	}
	s.tokenCache.Store(key, cachedToken{accessToken: accessToken, expiresAt: expiresAt})
}

func (s *Service) cacheLookup(key string) (string, bool) {
	v, ok := s.tokenCache.Load(key)
	if !ok {
		return "", false
	}
	ct := v.(cachedToken)
	if time.Until(ct.expiresAt) <= 30*time.Second {
		s.tokenCache.Delete(key)
		return "", false
	}
	return ct.accessToken, true
}

func (s *Service) writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func (s *Service) writeRPCResponse(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func sanitizeSecrets(payload map[string]any) map[string]any {
	copyPayload := cloneMap(payload)
	replaceSecrets(copyPayload)
	return copyPayload
}

func replaceSecrets(value any) {
	switch typed := value.(type) {
	case map[string]any:
		for key, item := range typed {
			if strings.EqualFold(key, "secret") {
				typed[key] = "hidden"
				continue
			}
			replaceSecrets(item)
		}
	case []any:
		for _, item := range typed {
			replaceSecrets(item)
		}
	}
}

func cloneMap(input map[string]any) map[string]any {
	data, _ := json.Marshal(input)
	var cloned map[string]any
	_ = json.Unmarshal(data, &cloned)
	if cloned == nil {
		cloned = map[string]any{}
	}
	return cloned
}

func toAnySlice[T any](items []T) []any {
	result := make([]any, 0, len(items))
	for _, item := range items {
		result = append(result, item)
	}
	return result
}

func withHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		next.ServeHTTP(w, r)
	})
}
