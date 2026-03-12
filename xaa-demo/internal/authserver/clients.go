package authserver

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
)

type createUserRequest struct {
	Email string `json:"email"`
}

type createClientRequest struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Secret      string `json:"secret,omitempty"`
	RedirectURI string `json:"redirect_uri"`
}

func (s *Service) handleUsers(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		users, err := s.store.ListUsers()
		if err != nil {
			s.writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		s.writeJSON(w, http.StatusOK, map[string]any{"users": users})
	case http.MethodPost:
		var payload createUserRequest
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			s.writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON payload"})
			return
		}
		user, err := s.store.CreateUser(payload.Email)
		if err != nil {
			s.writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		s.writeJSON(w, http.StatusCreated, map[string]any{"user": user})
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
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
		var payload createClientRequest
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			s.writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON payload"})
			return
		}

		client, err := s.store.SaveClient(DemoClient{
			ID:          strings.TrimSpace(payload.ID),
			Name:        strings.TrimSpace(payload.Name),
			Secret:      strings.TrimSpace(payload.Secret),
			RedirectURI: strings.TrimSpace(payload.RedirectURI),
		})
		if err != nil {
			s.writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		s.writeJSON(w, http.StatusCreated, map[string]any{"client": publicClient(client)})
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

func (s *Service) handleInternalClientByID(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	if r.Header.Get("X-Demo-Internal-Request") != "1" {
		s.writeJSON(w, http.StatusForbidden, map[string]string{"error": "internal access required"})
		return
	}

	clientID := strings.TrimPrefix(r.URL.Path, "/api/internal/clients/")
	if clientID == "" || clientID == "/api/internal/clients/" {
		s.writeJSON(w, http.StatusBadRequest, map[string]string{"error": "client id is required"})
		return
	}

	client, found, err := s.store.GetClient(clientID)
	if err != nil {
		s.writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if !found {
		s.writeJSON(w, http.StatusNotFound, map[string]string{"error": "client not found"})
		return
	}

	s.writeJSON(w, http.StatusOK, map[string]any{"client": client})
}

func (s *Service) handleDebugState(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	state, err := s.store.DebugState()
	if err != nil {
		if errors.Is(err, http.ErrMissingFile) {
			s.writeJSON(w, http.StatusOK, State{})
			return
		}
		s.writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	s.writeJSON(w, http.StatusOK, state)
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
