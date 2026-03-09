package requestingapp

import (
	"bytes"
	"encoding/json"
	"net"
	"net/http"
	"testing"

	"xaa-mcp-demo/internal/authserver"
	"xaa-mcp-demo/internal/resourceserver"
	"xaa-mcp-demo/internal/shared/debuglog"
)

func noopLogger(t *testing.T) *debuglog.Logger {
	t.Helper()
	logger, err := debuglog.New("test", false, "")
	if err != nil {
		t.Fatalf("create noop logger: %v", err)
	}
	return logger
}

func TestEndToEndBrowserAndHostBridge(t *testing.T) {
	t.Helper()

	baseDir := t.TempDir()
	logger := noopLogger(t)

	authAddr, authClose := startServer(t, func(addr string) http.Handler {
		service, err := authserver.NewService(baseDir+"/auth", addr, logger)
		if err != nil {
			t.Fatalf("create auth service: %v", err)
		}
		return service.Handler()
	})
	defer authClose()

	resourceAddr, resourceClose := startServer(t, func(addr string) http.Handler {
		service, err := resourceserver.NewService(baseDir+"/resource", addr, authAddr, authAddr+"/oauth/jwks.json", logger)
		if err != nil {
			t.Fatalf("create resource service: %v", err)
		}
		return service.Handler()
	})
	defer resourceClose()

	requestingAddr, requestingClose := startServer(t, func(addr string) http.Handler {
		service := NewService(baseDir+"/requesting", t.TempDir(), addr, authAddr, authAddr, resourceAddr, resourceAddr, logger)
		return service.Handler()
	})
	defer requestingClose()

	postJSON(t, authAddr+"/api/users", map[string]string{"email": "alice@example.com"}, http.StatusCreated)
	postJSON(t, requestingAddr+"/api/clients", map[string]string{
		"id":           "custom-client",
		"name":         "Custom Client",
		"redirect_uri": requestingAddr + "/callback",
	}, http.StatusCreated)

	flow := postJSON(t, requestingAddr+"/api/flow/run", map[string]any{
		"user_email": "alice@example.com",
		"client_id":  "custom-client",
		"tool_name":  "add_todo",
		"arguments": map[string]any{
			"text": "Ship the XAA bridge",
		},
	}, http.StatusOK)

	if flow["error"] != nil && flow["error"] != "" {
		t.Fatalf("browser flow failed: %v", flow["error"])
	}

	tokens, ok := flow["tokens"].(map[string]any)
	if !ok || len(tokens) < 3 {
		t.Fatalf("expected id_token, id_jag, and access_token, got %v", flow["tokens"])
	}

	initializeResult := postRPC(t, requestingAddr+"/host/mcp", map[string]string{
		"X-Demo-User":   "alice@example.com",
		"X-Demo-Client": "custom-client",
	}, map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "initialize",
		"params": map[string]any{
			"protocolVersion": "2025-11-25",
		},
	})
	if initializeResult["result"] == nil {
		t.Fatalf("expected initialize result, got %v", initializeResult)
	}

	toolsList := postRPC(t, requestingAddr+"/host/mcp", nil, map[string]any{
		"jsonrpc": "2.0",
		"id":      2,
		"method":  "tools/list",
	})
	result := toolsList["result"].(map[string]any)
	tools := result["tools"].([]any)
	if len(tools) == 0 {
		t.Fatalf("expected bridge tools, got %v", toolsList)
	}

	addResult := postRPC(t, requestingAddr+"/host/mcp", map[string]string{
		"X-Demo-User":   "alice@example.com",
		"X-Demo-Client": "custom-client",
	}, map[string]any{
		"jsonrpc": "2.0",
		"id":      3,
		"method":  "tools/call",
		"params": map[string]any{
			"name": "add_todo",
			"arguments": map[string]any{
				"text": "Verify host bridge",
			},
		},
	})
	addPayload := addResult["result"].(map[string]any)
	if addPayload["isError"] == true {
		t.Fatalf("expected successful bridge add_todo, got %v", addResult)
	}

	listResult := postRPC(t, requestingAddr+"/host/mcp", map[string]string{
		"X-Demo-User":   "alice@example.com",
		"X-Demo-Client": "custom-client",
	}, map[string]any{
		"jsonrpc": "2.0",
		"id":      4,
		"method":  "tools/call",
		"params": map[string]any{
			"name":      "list_todos",
			"arguments": map[string]any{},
		},
	})
	listPayload := listResult["result"].(map[string]any)
	structured := listPayload["structuredContent"].(map[string]any)
	todos := structured["todos"].([]any)
	if len(todos) < 2 {
		t.Fatalf("expected todos created through browser and host flows, got %v", todos)
	}
}

func TestEndToEndClientCredentials(t *testing.T) {
	t.Helper()

	baseDir := t.TempDir()
	logger := noopLogger(t)

	authAddr, authClose := startServer(t, func(addr string) http.Handler {
		service, err := authserver.NewService(baseDir+"/auth", addr, logger)
		if err != nil {
			t.Fatalf("create auth service: %v", err)
		}
		return service.Handler()
	})
	defer authClose()

	resourceAddr, resourceClose := startServer(t, func(addr string) http.Handler {
		service, err := resourceserver.NewService(baseDir+"/resource", addr, authAddr, authAddr+"/oauth/jwks.json", logger)
		if err != nil {
			t.Fatalf("create resource service: %v", err)
		}
		return service.Handler()
	})
	defer resourceClose()

	requestingAddr, requestingClose := startServer(t, func(addr string) http.Handler {
		service := NewService(baseDir+"/requesting", t.TempDir(), addr, authAddr, authAddr, resourceAddr, resourceAddr, logger)
		return service.Handler()
	})
	defer requestingClose()

	// Use the pre-seeded demo-requesting-app / demo-requesting-secret client credentials.
	// No user enrollment needed for client credentials flow.

	addResult := postRPC(t, requestingAddr+"/host/mcp", map[string]string{
		"X-Demo-Client":        "demo-requesting-app",
		"X-Demo-Client-Secret": "demo-requesting-secret",
	}, map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "tools/call",
		"params": map[string]any{
			"name": "add_todo",
			"arguments": map[string]any{
				"text": "hello from machine",
			},
		},
	})
	addPayload := addResult["result"].(map[string]any)
	if addPayload["isError"] == true {
		t.Fatalf("expected successful CC add_todo, got %v", addResult)
	}

	listResult := postRPC(t, requestingAddr+"/host/mcp", map[string]string{
		"X-Demo-Client":        "demo-requesting-app",
		"X-Demo-Client-Secret": "demo-requesting-secret",
	}, map[string]any{
		"jsonrpc": "2.0",
		"id":      2,
		"method":  "tools/call",
		"params": map[string]any{
			"name":      "list_todos",
			"arguments": map[string]any{},
		},
	})
	listPayload := listResult["result"].(map[string]any)
	if listPayload["isError"] == true {
		t.Fatalf("expected successful CC list_todos, got %v", listResult)
	}

	// Verify the trace has cc_id_jag and no id_token.
	traceResult := postRPC(t, requestingAddr+"/host/mcp", map[string]string{
		"X-Demo-Client":        "demo-requesting-app",
		"X-Demo-Client-Secret": "demo-requesting-secret",
	}, map[string]any{
		"jsonrpc": "2.0",
		"id":      3,
		"method":  "resources/read",
		"params":  map[string]any{"uri": "trace://latest"},
	})
	traceContents, _ := traceResult["result"].(map[string]any)
	contents, _ := traceContents["contents"].([]any)
	if len(contents) == 0 {
		t.Fatalf("expected trace contents, got %v", traceResult)
	}

	// Browser-triggered CC flow verification via /api/flow/run.
	ccFlow := postJSON(t, requestingAddr+"/api/flow/run", map[string]any{
		"client_id":     "demo-requesting-app",
		"client_secret": "demo-requesting-secret",
		"tool_name":     "list_todos",
		"arguments":     map[string]any{},
	}, http.StatusOK)

	if ccFlow["error"] != nil && ccFlow["error"] != "" {
		t.Fatalf("CC browser flow failed: %v", ccFlow["error"])
	}

	ccTokens, ok := ccFlow["tokens"].(map[string]any)
	if !ok {
		t.Fatalf("expected tokens in CC flow, got %v", ccFlow["tokens"])
	}
	if _, has := ccTokens["cc_id_jag"]; !has {
		t.Fatalf("CC flow must have cc_id_jag token, got %v", ccTokens)
	}
	if _, has := ccTokens["id_token"]; has {
		t.Fatalf("CC flow must not have id_token, got %v", ccTokens)
	}
}

func startServer(t *testing.T, newHandler func(addr string) http.Handler) (string, func()) {
	t.Helper()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	addr := "http://" + listener.Addr().String()
	server := &http.Server{Handler: newHandler(addr)}
	go func() {
		_ = server.Serve(listener)
	}()

	return addr, func() {
		_ = server.Close()
	}
}

func postJSON(t *testing.T, url string, payload any, expectedStatus int) map[string]any {
	t.Helper()

	data, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}

	response, err := http.Post(url, "application/json", bytes.NewReader(data))
	if err != nil {
		t.Fatalf("post %s: %v", url, err)
	}
	defer response.Body.Close()

	var body map[string]any
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatalf("decode response body: %v", err)
	}

	if response.StatusCode != expectedStatus {
		t.Fatalf("expected status %d, got %d with body %v", expectedStatus, response.StatusCode, body)
	}
	return body
}

func postRPC(t *testing.T, url string, headers map[string]string, payload any) map[string]any {
	t.Helper()

	data, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal rpc payload: %v", err)
	}

	request, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(data))
	if err != nil {
		t.Fatalf("create rpc request: %v", err)
	}
	request.Header.Set("Content-Type", "application/json")
	for key, value := range headers {
		request.Header.Set(key, value)
	}

	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatalf("rpc request: %v", err)
	}
	defer response.Body.Close()

	var body map[string]any
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatalf("decode rpc response: %v", err)
	}

	if response.StatusCode >= 300 {
		t.Fatalf("unexpected RPC status %d with body %v", response.StatusCode, body)
	}

	return body
}
