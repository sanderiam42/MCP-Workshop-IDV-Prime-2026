package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"

	"xaa-mcp-demo/internal/shared/mcp"
)

func main() {
	clientID := os.Getenv("XAA_CLIENT_ID")
	if clientID == "" {
		fmt.Fprintln(os.Stderr, "xaa-mcp-stdio: XAA_CLIENT_ID is required")
		os.Exit(1)
	}

	bridgeURL := envOrDefault("XAA_BRIDGE_URL", "http://localhost:3000/host/mcp")
	clientSecret := os.Getenv("XAA_CLIENT_SECRET")
	userEmail := os.Getenv("XAA_USER_EMAIL")

	headers := map[string]string{
		"Content-Type":  "application/json",
		"X-Demo-Client": clientID,
	}
	if clientSecret != "" && userEmail == "" {
		headers["X-Demo-Client-Secret"] = clientSecret
	}
	if userEmail != "" {
		headers["X-Demo-User"] = userEmail
	}

	httpClient := &http.Client{}
	scanner := bufio.NewScanner(os.Stdin)
	encoder := json.NewEncoder(os.Stdout)

	for scanner.Scan() {
		line := scanner.Bytes()
		var req mcp.Request
		if err := json.Unmarshal(line, &req); err != nil {
			_ = encoder.Encode(mcp.Error(nil, -32700, "parse error", err.Error()))
			continue
		}

		var resp mcp.Response
		switch req.Method {
		case "initialize":
			resp = mcp.Success(req.ID, mcp.InitializeResult(
				"xaa-mcp-stdio",
				"1.0.0",
				"stdio proxy for the XAA demo bridge",
			))
		case "notifications/initialized":
			continue
		case "tools/list":
			resp = mcp.Success(req.ID, map[string]any{"tools": bridgeTools()})
		case "resources/list":
			resp = mcp.Success(req.ID, map[string]any{"resources": bridgeResources()})
		case "tools/call", "resources/read":
			resp = callBridge(httpClient, bridgeURL, headers, req)
		default:
			resp = mcp.Error(req.ID, -32601, "method not found", nil)
		}

		if err := encoder.Encode(resp); err != nil {
			fmt.Fprintf(os.Stderr, "xaa-mcp-stdio: encode response: %v\n", err)
		}
	}

	if err := scanner.Err(); err != nil {
		fmt.Fprintf(os.Stderr, "xaa-mcp-stdio: stdin read error: %v\n", err)
		os.Exit(1)
	}
}

func callBridge(client *http.Client, url string, headers map[string]string, req mcp.Request) mcp.Response {
	body, err := json.Marshal(req)
	if err != nil {
		return mcp.Error(req.ID, -32603, err.Error(), nil)
	}

	httpReq, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return mcp.Error(req.ID, -32603, err.Error(), nil)
	}

	for k, v := range headers {
		httpReq.Header.Set(k, v)
	}

	resp, err := client.Do(httpReq)
	if err != nil {
		return mcp.Error(req.ID, -32603, err.Error(), nil)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return mcp.Error(req.ID, -32603, err.Error(), nil)
	}

	var mcpResp mcp.Response
	if err := json.Unmarshal(respBody, &mcpResp); err != nil {
		return mcp.Error(req.ID, -32603, err.Error(), nil)
	}
	return mcpResp
}

func bridgeTools() []mcp.Tool {
	return []mcp.Tool{
		{
			Name:        "list_todos",
			Description: "List todos for the selected XAA demo user.",
			InputSchema: map[string]any{
				"type":                 "object",
				"properties":           map[string]any{},
				"additionalProperties": false,
			},
		},
		{
			Name:        "add_todo",
			Description: "Add a todo for the selected XAA demo user.",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"text": map[string]any{"type": "string"},
				},
				"required":             []string{"text"},
				"additionalProperties": false,
			},
		},
		{
			Name:        "toggle_todo",
			Description: "Toggle a todo for the selected XAA demo user.",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"id": map[string]any{"type": "string"},
				},
				"required":             []string{"id"},
				"additionalProperties": false,
			},
		},
		{
			Name:        "delete_todo",
			Description: "Delete a todo for the selected XAA demo user.",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"id": map[string]any{"type": "string"},
				},
				"required":             []string{"id"},
				"additionalProperties": false,
			},
		},
	}
}

func bridgeResources() []mcp.Resource {
	return []mcp.Resource{
		{
			URI:         "trace://latest",
			Name:        "Latest XAA Trace",
			Description: "The most recent Cross App Access flow performed by the bridge.",
			MimeType:    "application/json",
		},
	}
}

func envOrDefault(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
