package resourceserver

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"xaa-mcp-demo/internal/shared/demo"
	"xaa-mcp-demo/internal/shared/store"
)

type DemoClient struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Secret      string `json:"secret"`
	RedirectURI string `json:"redirect_uri"`
	CreatedAt   string `json:"created_at"`
}

type Todo struct {
	ID        string `json:"id"`
	Text      string `json:"text"`
	Done      bool   `json:"done"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

type AccessTokenEvent struct {
	UserEmail    string         `json:"user_email"`
	ClientID     string         `json:"client_id"`
	Scope        string         `json:"scope"`
	Resource     string         `json:"resource"`
	IssuedAt     string         `json:"issued_at"`
	ExpiresAt    string         `json:"expires_at"`
	TokenPreview string         `json:"token_preview"`
	Claims       map[string]any `json:"claims"`
}

type MCPEvent struct {
	UserEmail string `json:"user_email"`
	Method    string `json:"method"`
	Target    string `json:"target"`
	At        string `json:"at"`
}

type State struct {
	Clients            []DemoClient              `json:"clients"`
	Todos              map[string][]Todo         `json:"todos"`
	RecentAccessTokens []AccessTokenEvent        `json:"recent_access_tokens"`
	RecentMCPCalls     []MCPEvent                `json:"recent_mcp_calls"`
	Extra              map[string]map[string]any `json:"extra,omitempty"`
}

type Store struct {
	json *store.JSONStore[State]
}

func NewStore(dataDir string) *Store {
	return &Store{
		json: store.NewJSONStore(filepath.Join(dataDir, "resource-state.json"), seedState),
	}
}

func (s *Store) ListClients() ([]DemoClient, error) {
	state, err := s.json.Read()
	if err != nil {
		return nil, err
	}
	return append([]DemoClient(nil), state.Clients...), nil
}

func (s *Store) SaveClient(client DemoClient) (DemoClient, error) {
	client.ID = strings.TrimSpace(client.ID)
	client.Name = strings.TrimSpace(client.Name)
	client.Secret = strings.TrimSpace(client.Secret)
	client.RedirectURI = strings.TrimSpace(client.RedirectURI)
	if client.ID == "" || client.Secret == "" {
		return DemoClient{}, errors.New("client id and secret are required")
	}
	if client.Name == "" {
		client.Name = client.ID
	}
	if client.RedirectURI == "" {
		client.RedirectURI = demo.DefaultClientRedirect
	}
	if client.CreatedAt == "" {
		client.CreatedAt = nowString()
	}

	state, err := s.json.Update(func(state *State) error {
		replaced := false
		for index := range state.Clients {
			if state.Clients[index].ID == client.ID {
				client.CreatedAt = state.Clients[index].CreatedAt
				state.Clients[index] = client
				replaced = true
				break
			}
		}
		if !replaced {
			state.Clients = append(state.Clients, client)
		}
		slices.SortFunc(state.Clients, func(a, b DemoClient) int {
			return strings.Compare(a.ID, b.ID)
		})
		return nil
	})
	if err != nil {
		return DemoClient{}, err
	}

	for _, item := range state.Clients {
		if item.ID == client.ID {
			return item, nil
		}
	}
	return DemoClient{}, errors.New("client was not stored")
}

func (s *Store) GetClient(id string) (DemoClient, bool, error) {
	clients, err := s.ListClients()
	if err != nil {
		return DemoClient{}, false, err
	}
	for _, client := range clients {
		if client.ID == id {
			return client, true, nil
		}
	}
	return DemoClient{}, false, nil
}

func (s *Store) DeleteClient(id string) error {
	_, err := s.json.Update(func(state *State) error {
		filtered := state.Clients[:0]
		removed := false
		for _, client := range state.Clients {
			if client.ID == id {
				removed = true
				continue
			}
			filtered = append(filtered, client)
		}
		if !removed {
			return errors.New("client not found")
		}
		state.Clients = filtered
		return nil
	})
	return err
}

func (s *Store) ListTodos(email string) ([]Todo, error) {
	state, err := s.json.Read()
	if err != nil {
		return nil, err
	}
	items := state.Todos[strings.ToLower(strings.TrimSpace(email))]
	return append([]Todo(nil), items...), nil
}

func (s *Store) AddTodo(email, text string) (Todo, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	text = strings.TrimSpace(text)
	if email == "" || text == "" {
		return Todo{}, errors.New("email and text are required")
	}

	var created Todo
	_, err := s.json.Update(func(state *State) error {
		created = Todo{
			ID:        randomID("todo"),
			Text:      text,
			Done:      false,
			CreatedAt: nowString(),
			UpdatedAt: nowString(),
		}
		state.Todos[email] = append(state.Todos[email], created)
		return nil
	})
	return created, err
}

func (s *Store) ToggleTodo(email, todoID string) (Todo, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	var updated Todo
	_, err := s.json.Update(func(state *State) error {
		items := state.Todos[email]
		for index := range items {
			if items[index].ID != todoID {
				continue
			}
			items[index].Done = !items[index].Done
			items[index].UpdatedAt = nowString()
			state.Todos[email] = items
			updated = items[index]
			return nil
		}
		return errors.New("todo not found")
	})
	return updated, err
}

func (s *Store) DeleteTodo(email, todoID string) error {
	email = strings.ToLower(strings.TrimSpace(email))
	_, err := s.json.Update(func(state *State) error {
		items := state.Todos[email]
		filtered := items[:0]
		removed := false
		for _, item := range items {
			if item.ID == todoID {
				removed = true
				continue
			}
			filtered = append(filtered, item)
		}
		if !removed {
			return errors.New("todo not found")
		}
		state.Todos[email] = filtered
		return nil
	})
	return err
}

func (s *Store) RecordAccessToken(event AccessTokenEvent) error {
	_, err := s.json.Update(func(state *State) error {
		state.RecentAccessTokens = append([]AccessTokenEvent{event}, state.RecentAccessTokens...)
		if len(state.RecentAccessTokens) > 25 {
			state.RecentAccessTokens = state.RecentAccessTokens[:25]
		}
		return nil
	})
	return err
}

func (s *Store) RecordMCPCall(event MCPEvent) error {
	_, err := s.json.Update(func(state *State) error {
		state.RecentMCPCalls = append([]MCPEvent{event}, state.RecentMCPCalls...)
		if len(state.RecentMCPCalls) > 50 {
			state.RecentMCPCalls = state.RecentMCPCalls[:50]
		}
		return nil
	})
	return err
}

func (s *Store) DebugState() (State, error) {
	return s.json.Read()
}

func seedState(state *State) {
	state.Clients = []DemoClient{}
	state.Todos = map[string][]Todo{}
	state.RecentAccessTokens = []AccessTokenEvent{}
	state.RecentMCPCalls = []MCPEvent{}
}

func randomID(prefix string) string {
	var bytes [8]byte
	_, _ = rand.Read(bytes[:])
	return prefix + "_" + hex.EncodeToString(bytes[:])
}

func nowString() string {
	return time.Now().UTC().Format(time.RFC3339Nano)
}
