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

func BuildSnippets(userEmail, clientID, publicBase string) map[string]string {
	if userEmail == "" {
		userEmail = "you@example.com"
	}
	if clientID == "" {
		clientID = demo.DefaultClientID
	}
	if publicBase == "" {
		publicBase = "http://localhost:3000"
	}
	mcpURL := publicBase + "/host/mcp"

	cursorConfig := map[string]any{
		"mcpServers": map[string]any{
			"xaa-demo-user": map[string]any{
				"url": mcpURL,
				"headers": map[string]string{
					"X-Demo-User":   userEmail,
					"X-Demo-Client": clientID,
				},
			},
		},
	}

	cursorJSON, _ := json.MarshalIndent(cursorConfig, "", "  ")
	codexTOML := fmt.Sprintf(
		"[mcp_servers.xaa_demo_user]\nurl = \"%s\"\nhttp_headers = { \"X-Demo-User\" = \"%s\", \"X-Demo-Client\" = \"%s\" }\n",
		mcpURL,
		userEmail,
		clientID,
	)

	cursorCCConfig := map[string]any{
		"mcpServers": map[string]any{
			"xaa-demo-machine": map[string]any{
				"url": mcpURL,
				"headers": map[string]string{
					"X-Demo-Client":        "<your-client-id>",
					"X-Demo-Client-Secret": "<your-client-secret>",
				},
			},
		},
	}
	cursorCCJSON, _ := json.MarshalIndent(cursorCCConfig, "", "  ")
	codexCCTOML := fmt.Sprintf("[mcp_servers.xaa_demo_machine]\nurl = \"%s\"\nhttp_headers = { \"X-Demo-Client\" = \"<your-client-id>\", \"X-Demo-Client-Secret\" = \"<your-client-secret>\" }\n", mcpURL)

	return map[string]string{
		"cursor":    string(cursorJSON),
		"codex":     codexTOML,
		"cursor_cc": string(cursorCCJSON),
		"codex_cc":  codexCCTOML,
	}
}
