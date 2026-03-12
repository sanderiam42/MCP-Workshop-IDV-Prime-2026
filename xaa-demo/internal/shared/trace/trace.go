package trace

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"time"
)

type TokenSummary struct {
	Kind    string         `json:"kind"`
	Preview string         `json:"preview"`
	Claims  map[string]any `json:"claims,omitempty"`
}

type Step struct {
	Name       string            `json:"name"`
	Method     string            `json:"method"`
	URL        string            `json:"url"`
	Status     int               `json:"status"`
	Request    any               `json:"request,omitempty"`
	Response   any               `json:"response,omitempty"`
	Notes      string            `json:"notes,omitempty"`
	Headers    map[string]string `json:"headers,omitempty"`
	StartedAt  string            `json:"started_at"`
	FinishedAt string            `json:"finished_at,omitempty"`
	DurationMS int64             `json:"duration_ms,omitempty"`
}

type Flow struct {
	ID         string                  `json:"id"`
	Trigger    string                  `json:"trigger"`
	UserEmail  string                  `json:"user_email"`
	ClientID   string                  `json:"client_id"`
	StartedAt  string                  `json:"started_at"`
	FinishedAt string                  `json:"finished_at,omitempty"`
	Steps      []Step                  `json:"steps"`
	Tokens     map[string]TokenSummary `json:"tokens,omitempty"`
	Result     any                     `json:"result,omitempty"`
	Error      string                  `json:"error,omitempty"`
}

func NewFlow(trigger, userEmail, clientID string) *Flow {
	return &Flow{
		ID:        randomID("flow"),
		Trigger:   trigger,
		UserEmail: userEmail,
		ClientID:  clientID,
		StartedAt: time.Now().UTC().Format(time.RFC3339Nano),
		Steps:     make([]Step, 0, 8),
		Tokens:    map[string]TokenSummary{},
	}
}

func (f *Flow) AddStep(name, method, url string, request any) int {
	f.Steps = append(f.Steps, Step{
		Name:      name,
		Method:    method,
		URL:       url,
		Request:   request,
		StartedAt: time.Now().UTC().Format(time.RFC3339Nano),
	})
	return len(f.Steps) - 1
}

func (f *Flow) FinishStep(index int, status int, response any, notes string, headers http.Header) {
	if index < 0 || index >= len(f.Steps) {
		return
	}

	startedAt, _ := time.Parse(time.RFC3339Nano, f.Steps[index].StartedAt)
	finishedAt := time.Now().UTC()
	f.Steps[index].Status = status
	f.Steps[index].Response = response
	f.Steps[index].Notes = notes
	f.Steps[index].FinishedAt = finishedAt.Format(time.RFC3339Nano)
	f.Steps[index].DurationMS = finishedAt.Sub(startedAt).Milliseconds()
	if len(headers) > 0 {
		f.Steps[index].Headers = FlattenHeaders(headers)
	}
}

func (f *Flow) AddToken(kind, preview string, claims map[string]any) {
	f.Tokens[kind] = TokenSummary{
		Kind:    kind,
		Preview: preview,
		Claims:  claims,
	}
}

func (f *Flow) Complete(result any) {
	f.Result = result
	f.FinishedAt = time.Now().UTC().Format(time.RFC3339Nano)
}

func (f *Flow) Fail(err error) {
	if err != nil {
		f.Error = err.Error()
	}
	f.FinishedAt = time.Now().UTC().Format(time.RFC3339Nano)
}

func FlattenHeaders(headers http.Header) map[string]string {
	if len(headers) == 0 {
		return nil
	}

	flat := make(map[string]string, len(headers))
	for key, values := range headers {
		if len(values) == 0 {
			continue
		}
		flat[key] = values[0]
	}
	return flat
}

func randomID(prefix string) string {
	var bytes [8]byte
	_, _ = rand.Read(bytes[:])
	return prefix + "_" + hex.EncodeToString(bytes[:])
}
