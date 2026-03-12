package authserver

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"xaa-mcp-demo/internal/shared/store"
)

const (
	defaultClientRedirect = "http://localhost:3000/callback"
)

type User struct {
	Email     string `json:"email"`
	CreatedAt string `json:"created_at"`
}

type DemoClient struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Secret      string `json:"secret"`
	RedirectURI string `json:"redirect_uri"`
	CreatedAt   string `json:"created_at"`
}

type AuthCode struct {
	Code                string `json:"code"`
	ClientID            string `json:"client_id"`
	RedirectURI         string `json:"redirect_uri"`
	UserEmail           string `json:"user_email"`
	Scope               string `json:"scope"`
	CodeChallenge       string `json:"code_challenge"`
	CodeChallengeMethod string `json:"code_challenge_method"`
	ExpiresAt           string `json:"expires_at"`
	Used                bool   `json:"used"`
}

type TokenEvent struct {
	Kind         string         `json:"kind"`
	UserEmail    string         `json:"user_email"`
	ClientID     string         `json:"client_id"`
	Audience     string         `json:"audience,omitempty"`
	Resource     string         `json:"resource,omitempty"`
	Scope        string         `json:"scope,omitempty"`
	IssuedAt     string         `json:"issued_at"`
	ExpiresAt    string         `json:"expires_at"`
	TokenPreview string         `json:"token_preview"`
	Claims       map[string]any `json:"claims"`
}

type State struct {
	Users        []User       `json:"users"`
	Clients      []DemoClient `json:"clients"`
	AuthCodes    []AuthCode   `json:"auth_codes"`
	RecentEvents []TokenEvent `json:"recent_events"`
}

type Store struct {
	json *store.JSONStore[State]
}

func NewStore(dataDir string) *Store {
	return &Store{
		json: store.NewJSONStore(filepath.Join(dataDir, "auth-state.json"), seedState),
	}
}

func (s *Store) ListUsers() ([]User, error) {
	state, err := s.json.Read()
	if err != nil {
		return nil, err
	}
	return append([]User(nil), state.Users...), nil
}

func (s *Store) CreateUser(email string) (User, error) {
	email = strings.TrimSpace(strings.ToLower(email))
	if !strings.Contains(email, "@") {
		return User{}, fmt.Errorf("invalid email %q", email)
	}

	state, err := s.json.Update(func(state *State) error {
		for _, existing := range state.Users {
			if existing.Email == email {
				return nil
			}
		}
		state.Users = append(state.Users, User{
			Email:     email,
			CreatedAt: nowString(),
		})
		slices.SortFunc(state.Users, func(a, b User) int {
			return strings.Compare(a.Email, b.Email)
		})
		return nil
	})
	if err != nil {
		return User{}, err
	}

	for _, user := range state.Users {
		if user.Email == email {
			return user, nil
		}
	}
	return User{}, errors.New("user was not stored")
}

func (s *Store) ListClients() ([]DemoClient, error) {
	state, err := s.json.Read()
	if err != nil {
		return nil, err
	}
	return append([]DemoClient(nil), state.Clients...), nil
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

func (s *Store) SaveClient(client DemoClient) (DemoClient, error) {
	client.ID = strings.TrimSpace(client.ID)
	client.Name = strings.TrimSpace(client.Name)
	client.RedirectURI = strings.TrimSpace(client.RedirectURI)
	if client.ID == "" {
		return DemoClient{}, errors.New("client id is required")
	}
	if client.Name == "" {
		client.Name = client.ID
	}
	if client.RedirectURI == "" {
		client.RedirectURI = defaultClientRedirect
	}
	if client.Secret == "" {
		client.Secret = randomSecret()
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

func (s *Store) SaveAuthCode(code AuthCode) error {
	_, err := s.json.Update(func(state *State) error {
		state.AuthCodes = append(state.AuthCodes, code)
		cutoff := time.Now().Add(-15 * time.Minute)
		filtered := state.AuthCodes[:0]
		for _, item := range state.AuthCodes {
			expiresAt, _ := time.Parse(time.RFC3339Nano, item.ExpiresAt)
			if expiresAt.After(cutoff) {
				filtered = append(filtered, item)
			}
		}
		state.AuthCodes = filtered
		return nil
	})
	return err
}

func (s *Store) ConsumeAuthCode(code string) (AuthCode, error) {
	var consumed AuthCode
	_, err := s.json.Update(func(state *State) error {
		for index := range state.AuthCodes {
			if state.AuthCodes[index].Code != code {
				continue
			}
			if state.AuthCodes[index].Used {
				return errors.New("authorization code already used")
			}
			expiresAt, err := time.Parse(time.RFC3339Nano, state.AuthCodes[index].ExpiresAt)
			if err != nil {
				return err
			}
			if time.Now().After(expiresAt) {
				return errors.New("authorization code expired")
			}
			state.AuthCodes[index].Used = true
			consumed = state.AuthCodes[index]
			return nil
		}
		return errors.New("authorization code not found")
	})
	return consumed, err
}

func (s *Store) AddEvent(event TokenEvent) error {
	_, err := s.json.Update(func(state *State) error {
		state.RecentEvents = append([]TokenEvent{event}, state.RecentEvents...)
		if len(state.RecentEvents) > 25 {
			state.RecentEvents = state.RecentEvents[:25]
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
	state.Users = []User{}
	state.AuthCodes = []AuthCode{}
	state.RecentEvents = []TokenEvent{}
}

func nowString() string {
	return time.Now().UTC().Format(time.RFC3339Nano)
}

func randomSecret() string {
	var bytes [16]byte
	_, _ = rand.Read(bytes[:])
	return hex.EncodeToString(bytes[:])
}
