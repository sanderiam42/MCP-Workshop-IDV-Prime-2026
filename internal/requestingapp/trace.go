package requestingapp

import (
	"encoding/json"
	"fmt"

	"xaa-mcp-demo/internal/shared/demo"
)

type DashboardData struct {
	Auth      map[string]any    `json:"auth"`
	Resource  map[string]any    `json:"resource"`
	Flows     []any             `json:"flows"`
	Snippets  map[string]string `json:"snippets"`
	Selection map[string]string `json:"selection"`
}

func BuildSnippets(userEmail, clientID string) map[string]string {
	if userEmail == "" {
		userEmail = "you@example.com"
	}
	if clientID == "" {
		clientID = demo.DefaultClientID
	}

	cursorConfig := map[string]any{
		"mcpServers": map[string]any{
			"xaa-demo": map[string]any{
				"url": "http://localhost:3000/host/mcp",
				"headers": map[string]string{
					"X-Demo-User":   userEmail,
					"X-Demo-Client": clientID,
				},
			},
		},
	}

	cursorJSON, _ := json.MarshalIndent(cursorConfig, "", "  ")
	codexTOML := fmt.Sprintf(
		"[mcp_servers.xaa_demo]\nurl = \"http://localhost:3000/host/mcp\"\nhttp_headers = { \"X-Demo-User\" = \"%s\", \"X-Demo-Client\" = \"%s\" }\n",
		userEmail,
		clientID,
	)

	return map[string]string{
		"cursor": string(cursorJSON),
		"codex":  codexTOML,
	}
}
