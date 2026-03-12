package store

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"
)

// JSONStore persists a single JSON document on disk.
type JSONStore[T any] struct {
	path string
	seed func(*T)
	mu   sync.Mutex
}

func NewJSONStore[T any](path string, seed func(*T)) *JSONStore[T] {
	return &JSONStore[T]{
		path: path,
		seed: seed,
	}
}

func (s *JSONStore[T]) Path() string {
	return s.path
}

func (s *JSONStore[T]) Read() (T, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.ensureFileLocked(); err != nil {
		var zero T
		return zero, err
	}

	return s.readLocked()
}

func (s *JSONStore[T]) Write(value T) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.writeLocked(value)
}

func (s *JSONStore[T]) Update(fn func(*T) error) (T, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.ensureFileLocked(); err != nil {
		var zero T
		return zero, err
	}

	value, err := s.readLocked()
	if err != nil {
		var zero T
		return zero, err
	}

	if err := fn(&value); err != nil {
		var zero T
		return zero, err
	}

	if err := s.writeLocked(value); err != nil {
		var zero T
		return zero, err
	}

	return value, nil
}

func (s *JSONStore[T]) ensureFileLocked() error {
	if s.path == "" {
		return errors.New("json store path is required")
	}

	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return err
	}

	if _, err := os.Stat(s.path); err == nil {
		return nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}

	var value T
	if s.seed != nil {
		s.seed(&value)
	}

	return s.writeLocked(value)
}

func (s *JSONStore[T]) readLocked() (T, error) {
	data, err := os.ReadFile(s.path)
	if err != nil {
		var zero T
		return zero, err
	}

	var value T
	if len(data) == 0 {
		if s.seed != nil {
			s.seed(&value)
		}
		return value, nil
	}

	if err := json.Unmarshal(data, &value); err != nil {
		var zero T
		return zero, err
	}

	return value, nil
}

func (s *JSONStore[T]) writeLocked(value T) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}

	tempPath := s.path + ".tmp"
	if err := os.WriteFile(tempPath, append(data, '\n'), 0o600); err != nil {
		return err
	}

	return os.Rename(tempPath, s.path)
}
