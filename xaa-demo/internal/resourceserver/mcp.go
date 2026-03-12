package resourceserver

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"strings"
	"time"

	"xaa-mcp-demo/internal/shared/mcp"
)

func (s *Service) handleMCP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	requestBody := mcp.Request{}
	if err := json.NewDecoder(r.Body).Decode(&requestBody); err != nil {
		s.writeRPCResponse(w, http.StatusBadRequest, mcp.Error(nil, -32700, "invalid JSON-RPC body", nil))
		return
	}

	token := strings.TrimSpace(strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer "))
	if token == "" {
		s.writeChallenge(w)
		return
	}

	claims, err := s.validateAccessToken(token)
	if err != nil {
		s.writeChallenge(w)
		return
	}

	userEmail := strings.ToLower(strings.TrimSpace(fmt.Sprint(claims["email"])))

	switch requestBody.Method {
	case "notifications/initialized":
		_ = s.store.RecordMCPCall(MCPEvent{
			UserEmail: userEmail,
			Method:    requestBody.Method,
			Target:    "",
			At:        time.Now().UTC().Format(time.RFC3339Nano),
		})
		w.WriteHeader(http.StatusAccepted)
		return
	case "initialize":
		if !hasScope(claims, "mcp:read") {
			s.writeChallenge(w)
			return
		}
		_ = s.store.RecordMCPCall(MCPEvent{
			UserEmail: userEmail,
			Method:    requestBody.Method,
			Target:    "",
			At:        time.Now().UTC().Format(time.RFC3339Nano),
		})
		s.writeRPCResponse(w, http.StatusOK, mcp.Success(requestBody.ID, mcp.InitializeResult(
			"todo-resource-app",
			"1.0.0",
			"Protected MCP todo server backed by Cross App Access demo tokens.",
		)))
	case "tools/list":
		if !hasScope(claims, "mcp:read") {
			s.writeChallenge(w)
			return
		}
		_ = s.store.RecordMCPCall(MCPEvent{
			UserEmail: userEmail,
			Method:    requestBody.Method,
			Target:    "",
			At:        time.Now().UTC().Format(time.RFC3339Nano),
		})
		s.writeRPCResponse(w, http.StatusOK, mcp.Success(requestBody.ID, map[string]any{
			"tools": resourceTools(),
		}))
	case "resources/list":
		if !hasScope(claims, "mcp:read") {
			s.writeChallenge(w)
			return
		}
		_ = s.store.RecordMCPCall(MCPEvent{
			UserEmail: userEmail,
			Method:    requestBody.Method,
			Target:    "",
			At:        time.Now().UTC().Format(time.RFC3339Nano),
		})
		s.writeRPCResponse(w, http.StatusOK, mcp.Success(requestBody.ID, map[string]any{
			"resources": resourceResources(),
		}))
	case "resources/read":
		if !hasScope(claims, "mcp:read") {
			s.writeChallenge(w)
			return
		}
		var params mcp.ResourceReadParams
		if err := json.Unmarshal(requestBody.Params, &params); err != nil {
			s.writeRPCResponse(w, http.StatusOK, mcp.Error(requestBody.ID, -32602, "invalid resources/read params", nil))
			return
		}
		if params.URI != "todo://list" {
			s.writeRPCResponse(w, http.StatusOK, mcp.Error(requestBody.ID, -32602, "unknown resource URI", nil))
			return
		}
		items, err := s.store.ListTodos(userEmail)
		if err != nil {
			s.writeRPCResponse(w, http.StatusOK, mcp.Error(requestBody.ID, -32603, err.Error(), nil))
			return
		}
		_ = s.store.RecordMCPCall(MCPEvent{
			UserEmail: userEmail,
			Method:    requestBody.Method,
			Target:    params.URI,
			At:        time.Now().UTC().Format(time.RFC3339Nano),
		})
		s.writeRPCResponse(w, http.StatusOK, mcp.Success(requestBody.ID, map[string]any{
			"contents": []mcp.ResourceContents{
				{
					URI:      params.URI,
					MimeType: "application/json",
					Text:     mustJSON(map[string]any{"email": userEmail, "todos": items}),
				},
			},
		}))
	case "tools/call":
		var params mcp.ToolCallParams
		if err := json.Unmarshal(requestBody.Params, &params); err != nil {
			s.writeRPCResponse(w, http.StatusOK, mcp.Error(requestBody.ID, -32602, "invalid tools/call params", nil))
			return
		}
		if !hasScope(claims, requiredScopeForTool(params.Name)) {
			s.writeChallenge(w)
			return
		}
		result, err := s.runTool(userEmail, params)
		if err != nil {
			s.writeRPCResponse(w, http.StatusOK, mcp.Success(requestBody.ID, map[string]any{
				"isError": true,
				"content": []mcp.TextContent{
					{Type: "text", Text: err.Error()},
				},
			}))
			return
		}
		_ = s.store.RecordMCPCall(MCPEvent{
			UserEmail: userEmail,
			Method:    requestBody.Method,
			Target:    params.Name,
			At:        time.Now().UTC().Format(time.RFC3339Nano),
		})
		s.writeRPCResponse(w, http.StatusOK, mcp.Success(requestBody.ID, result))
	default:
		s.writeRPCResponse(w, http.StatusOK, mcp.Error(requestBody.ID, -32601, "method not found", nil))
	}
}

func (s *Service) runTool(userEmail string, params mcp.ToolCallParams) (map[string]any, error) {
	switch params.Name {
	case "list_todos":
		items, err := s.store.ListTodos(userEmail)
		if err != nil {
			return nil, err
		}
		return toolResult("Listed todos.", map[string]any{"email": userEmail, "todos": items}), nil
	case "add_todo":
		text, _ := params.Arguments["text"].(string)
		item, err := s.store.AddTodo(userEmail, text)
		if err != nil {
			return nil, err
		}
		return toolResult("Added todo.", map[string]any{"email": userEmail, "todo": item}), nil
	case "toggle_todo":
		id, _ := params.Arguments["id"].(string)
		item, err := s.store.ToggleTodo(userEmail, id)
		if err != nil {
			return nil, err
		}
		return toolResult("Toggled todo.", map[string]any{"email": userEmail, "todo": item}), nil
	case "delete_todo":
		id, _ := params.Arguments["id"].(string)
		if err := s.store.DeleteTodo(userEmail, id); err != nil {
			return nil, err
		}
		return toolResult("Deleted todo.", map[string]any{"email": userEmail, "deleted_id": id}), nil
	default:
		return nil, errors.New("unknown tool")
	}
}

func resourceTools() []mcp.Tool {
	return []mcp.Tool{
		{
			Name:        "list_todos",
			Description: "List todos for the authenticated email address.",
			InputSchema: map[string]any{
				"type":                 "object",
				"properties":           map[string]any{},
				"additionalProperties": false,
			},
		},
		{
			Name:        "add_todo",
			Description: "Add a todo for the authenticated email address.",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"text": map[string]any{
						"type": "string",
					},
				},
				"required":             []string{"text"},
				"additionalProperties": false,
			},
		},
		{
			Name:        "toggle_todo",
			Description: "Toggle the completion state of a todo by id.",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"id": map[string]any{
						"type": "string",
					},
				},
				"required":             []string{"id"},
				"additionalProperties": false,
			},
		},
		{
			Name:        "delete_todo",
			Description: "Delete a todo by id.",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"id": map[string]any{
						"type": "string",
					},
				},
				"required":             []string{"id"},
				"additionalProperties": false,
			},
		},
	}
}

func resourceResources() []mcp.Resource {
	return []mcp.Resource{
		{
			URI:         "todo://list",
			Name:        "Current Todos",
			Description: "The current todo list for the authenticated user.",
			MimeType:    "application/json",
		},
	}
}

func toolResult(message string, payload any) map[string]any {
	return map[string]any{
		"isError": false,
		"content": []mcp.TextContent{
			{
				Type: "text",
				Text: message,
			},
		},
		"structuredContent": payload,
	}
}

func mustJSON(value any) string {
	data, _ := json.MarshalIndent(value, "", "  ")
	return string(data)
}

func (s *Service) writeChallenge(w http.ResponseWriter) {
	w.Header().Set("WWW-Authenticate", fmt.Sprintf(
		`Bearer realm="mcp", resource_metadata="%s/.well-known/oauth-protected-resource/mcp", scope="mcp:read mcp:write"`,
		strings.TrimRight(s.issuer, "/"),
	))
	w.WriteHeader(http.StatusUnauthorized)
}

func (s *Service) writeRPCResponse(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func requiredScopeForTool(name string) string {
	switch name {
	case "list_todos":
		return "mcp:read"
	case "add_todo", "toggle_todo", "delete_todo":
		return "mcp:write"
	default:
		return "mcp:read"
	}
}

func normalizeMCPScopes(raw string) ([]string, error) {
	if strings.TrimSpace(raw) == "" {
		return []string{"mcp:read"}, nil
	}

	allowed := []string{"mcp:read", "mcp:write"}
	fields := strings.Fields(raw)
	normalized := make([]string, 0, len(fields))
	for _, field := range fields {
		if !slices.Contains(allowed, field) {
			return nil, fmt.Errorf("unsupported scope %q", field)
		}
		if !slices.Contains(normalized, field) {
			normalized = append(normalized, field)
		}
	}
	if len(normalized) == 0 {
		return []string{"mcp:read"}, nil
	}
	return normalized, nil
}

func hasScope(claims map[string]any, scope string) bool {
	scopes, err := normalizeMCPScopes(fmt.Sprint(claims["scope"]))
	if err != nil {
		return false
	}
	return slices.Contains(scopes, scope)
}
