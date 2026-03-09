package resourceserver

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"xaa-mcp-demo/internal/shared/mcp"
)

func TestHandleMCPRejectsUnknownResourceURI(t *testing.T) {
	t.Parallel()

	service, err := NewService(t.TempDir(), "http://resource.example", "http://auth.example", "http://auth.example/oauth/jwks.json", nil)
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	token, _, _, err := service.issueAccessToken(DemoClient{ID: "demo-requesting-app"}, "alice@example.com", service.resourceURI, "mcp:read")
	if err != nil {
		t.Fatalf("issue access token: %v", err)
	}

	params, err := json.Marshal(mcp.ResourceReadParams{URI: "todo://unknown"})
	if err != nil {
		t.Fatalf("marshal params: %v", err)
	}
	body, err := json.Marshal(mcp.Request{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "resources/read",
		Params:  params,
	})
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/mcp", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+token)
	recorder := httptest.NewRecorder()

	service.handleMCP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200 response, got %d", recorder.Code)
	}

	var response mcp.Response
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.Error == nil || response.Error.Message != "unknown resource URI" {
		t.Fatalf("expected unknown resource URI error, got %#v", response.Error)
	}
}

func TestHandleMCPRequiresWriteScopeForMutations(t *testing.T) {
	t.Parallel()

	service, err := NewService(t.TempDir(), "http://resource.example", "http://auth.example", "http://auth.example/oauth/jwks.json", nil)
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	token, _, _, err := service.issueAccessToken(DemoClient{ID: "demo-requesting-app"}, "alice@example.com", service.resourceURI, "mcp:read")
	if err != nil {
		t.Fatalf("issue access token: %v", err)
	}

	params, err := json.Marshal(mcp.ToolCallParams{
		Name:      "add_todo",
		Arguments: map[string]any{"text": "Write tests"},
	})
	if err != nil {
		t.Fatalf("marshal params: %v", err)
	}
	body, err := json.Marshal(mcp.Request{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "tools/call",
		Params:  params,
	})
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/mcp", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+token)
	recorder := httptest.NewRecorder()

	service.handleMCP(recorder, req)

	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 response, got %d", recorder.Code)
	}
}
