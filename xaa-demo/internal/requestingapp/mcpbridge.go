package requestingapp

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"xaa-mcp-demo/internal/shared/mcp"
	"xaa-mcp-demo/internal/shared/trace"
)

func (s *Service) handleHostMCP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	var request mcp.Request
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		s.writeRPCResponse(w, http.StatusBadRequest, mcp.Error(nil, -32700, "invalid JSON-RPC payload", nil))
		return
	}

	switch request.Method {
	case "initialize":
		s.writeRPCResponse(w, http.StatusOK, mcp.Success(request.ID, mcp.InitializeResult(
			"xaa-demo-bridge",
			"1.0.0",
			"Host-facing MCP bridge that performs the full Cross App Access flow before calling the protected todo MCP server.",
		)))
	case "notifications/initialized":
		w.WriteHeader(http.StatusAccepted)
	case "tools/list":
		s.writeRPCResponse(w, http.StatusOK, mcp.Success(request.ID, map[string]any{
			"tools": bridgeTools(),
		}))
	case "resources/list":
		s.writeRPCResponse(w, http.StatusOK, mcp.Success(request.ID, map[string]any{
			"resources": bridgeResources(),
		}))
	case "resources/read":
		var params mcp.ResourceReadParams
		if err := json.Unmarshal(request.Params, &params); err != nil {
			s.writeRPCResponse(w, http.StatusOK, mcp.Error(request.ID, -32602, "invalid resources/read params", nil))
			return
		}
		if params.URI != "trace://latest" {
			s.writeRPCResponse(w, http.StatusOK, mcp.Error(request.ID, -32602, "unknown resource", nil))
			return
		}
		latest, err := s.store.LatestFlow()
		if err != nil {
			s.writeRPCResponse(w, http.StatusOK, mcp.Error(request.ID, -32603, err.Error(), nil))
			return
		}
		payload := map[string]any{"message": "No flow has been run yet."}
		if latest != nil {
			payload = map[string]any{"flow": latest}
		}
		s.writeRPCResponse(w, http.StatusOK, mcp.Success(request.ID, map[string]any{
			"contents": []mcp.ResourceContents{
				{
					URI:      "trace://latest",
					MimeType: "application/json",
					Text:     mustJSON(payload),
				},
			},
		}))
	case "tools/call":
		var params mcp.ToolCallParams
		if err := json.Unmarshal(request.Params, &params); err != nil {
			s.writeRPCResponse(w, http.StatusOK, mcp.Error(request.ID, -32602, "invalid tools/call params", nil))
			return
		}
		userEmail, clientID, clientSecret, useClientCredentials, err := bridgeContext(r)
		if err != nil {
			s.writeRPCResponse(w, http.StatusOK, mcp.Success(request.ID, errorToolResult(err)))
			return
		}
		cacheKey := bridgeCacheKey(useClientCredentials, userEmail, clientID)
		var flow trace.Flow
		if cached, ok := s.cacheLookup(cacheKey); ok {
			flow, err = s.runner.RunWithCachedToken(r.Context(), "host", FlowInput{
				UserEmail:    userEmail,
				ClientID:     clientID,
				ClientSecret: clientSecret,
				ToolName:     params.Name,
				Arguments:    params.Arguments,
			}, cached)
		} else if useClientCredentials {
			var tok string
			flow, tok, err = s.runner.RunClientCredentials(r.Context(), "host", FlowInput{
				ClientID:     clientID,
				ClientSecret: clientSecret,
				ToolName:     params.Name,
				Arguments:    params.Arguments,
			})
			if err == nil {
				s.cacheSet(cacheKey, tok)
			}
		} else {
			var tok string
			flow, tok, err = s.runner.Run(r.Context(), "host", FlowInput{
				UserEmail: userEmail,
				ClientID:  clientID,
				ToolName:  params.Name,
				Arguments: params.Arguments,
			})
			if err == nil {
				s.cacheSet(cacheKey, tok)
			}
		}
		_ = s.store.SaveFlow(flow)
		if err != nil {
			s.writeRPCResponse(w, http.StatusOK, mcp.Success(request.ID, errorToolResult(err)))
			return
		}

		resultPayload, _ := flow.Result.(map[string]any)
		toolResult, _ := resultPayload["tool_call_result"].(map[string]any)
		if toolResult == nil {
			toolResult = map[string]any{
				"isError": true,
				"content": []mcp.TextContent{
					{Type: "text", Text: "Tool call completed but no structured result was returned."},
				},
			}
		}
		s.writeRPCResponse(w, http.StatusOK, mcp.Success(request.ID, toolResult))
	default:
		s.writeRPCResponse(w, http.StatusOK, mcp.Error(request.ID, -32601, "method not found", nil))
	}
}

func bridgeTools() []mcp.Tool {
	return []mcp.Tool{
		{
			Name:        "list_todos",
			Description: "List all todos.",
			InputSchema: map[string]any{
				"type":                 "object",
				"properties":           map[string]any{},
				"additionalProperties": false,
			},
		},
		{
			Name:        "add_todo",
			Description: "Add a new todo.",
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
			Description: "Mark a todo complete or incomplete by its ID.",
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
			Description: "Delete a todo by its ID.",
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

func bridgeContext(r *http.Request) (userEmail, clientID, clientSecret string, useClientCredentials bool, err error) {
	userEmail = strings.TrimSpace(r.Header.Get("X-Demo-User"))
	clientID = strings.TrimSpace(r.Header.Get("X-Demo-Client"))
	clientSecret = strings.TrimSpace(r.Header.Get("X-Demo-Client-Secret"))
	if clientID == "" {
		err = errors.New("X-Demo-Client header is required")
		return
	}
	if clientSecret != "" && userEmail == "" {
		useClientCredentials = true
		return
	}
	if userEmail != "" {
		return
	}
	err = errors.New("X-Demo-User or X-Demo-Client-Secret header is required for host-triggered tool calls")
	return
}

func errorToolResult(err error) map[string]any {
	return map[string]any{
		"isError": true,
		"content": []mcp.TextContent{
			{
				Type: "text",
				Text: err.Error(),
			},
		},
		"structuredContent": map[string]any{
			"error": err.Error(),
		},
	}
}

func mustJSON(value any) string {
	data, _ := json.MarshalIndent(value, "", "  ")
	return string(data)
}

func bridgeCacheKey(useCC bool, userEmail, clientID string) string {
	if useCC {
		return "cc:" + clientID
	}
	return "ac:" + userEmail + ":" + clientID
}
