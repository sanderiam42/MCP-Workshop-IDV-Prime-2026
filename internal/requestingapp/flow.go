package requestingapp

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"xaa-mcp-demo/internal/shared/jose"
	"xaa-mcp-demo/internal/shared/mcp"
	"xaa-mcp-demo/internal/shared/trace"
)

type DemoClient struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Secret      string `json:"secret"`
	RedirectURI string `json:"redirect_uri"`
}

type FlowInput struct {
	UserEmail string         `json:"user_email"`
	ClientID  string         `json:"client_id"`
	ToolName  string         `json:"tool_name,omitempty"`
	Arguments map[string]any `json:"arguments,omitempty"`
}

type Runner struct {
	authPublicBase     string
	authInternalBase   string
	resourcePublicBase string
	resourceInternal   string
	httpClient         *http.Client
	noRedirectClient   *http.Client
}

func NewRunner(authPublicBase, authInternalBase, resourcePublicBase, resourceInternalBase string) *Runner {
	return &Runner{
		authPublicBase:     strings.TrimRight(authPublicBase, "/"),
		authInternalBase:   strings.TrimRight(authInternalBase, "/"),
		resourcePublicBase: strings.TrimRight(resourcePublicBase, "/"),
		resourceInternal:   strings.TrimRight(resourceInternalBase, "/"),
		httpClient: &http.Client{
			Timeout: 15 * time.Second,
		},
		noRedirectClient: &http.Client{
			Timeout: 15 * time.Second,
			CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
	}
}

func (r *Runner) Run(ctx context.Context, trigger string, input FlowInput) (trace.Flow, error) {
	flow := trace.NewFlow(trigger, strings.ToLower(strings.TrimSpace(input.UserEmail)), input.ClientID)
	defer func() {
		if flow.FinishedAt == "" {
			flow.Fail(nil)
		}
	}()

	if input.ToolName == "" {
		input.ToolName = "list_todos"
	}

	client, err := r.fetchClient(ctx, input.ClientID)
	if err != nil {
		flow.Fail(err)
		return *flow, err
	}

	initializePayload := map[string]any{
		"protocolVersion": mcp.ProtocolVersion,
		"capabilities": map[string]any{
			"roots": map[string]any{},
		},
		"clientInfo": map[string]any{
			"name":    "xaa-demo-requesting-app",
			"version": "1.0.0",
		},
	}

	challengeStep := flow.AddStep("Unauthenticated MCP initialize", http.MethodPost, r.resourcePublicBase+"/mcp", map[string]any{
		"method": "initialize",
		"params": initializePayload,
	})
	challengeStatus, challengeHeaders, challengeBody, err := r.postRPC(ctx, r.resourcePublicBase+"/mcp", "", map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "initialize",
		"params":  initializePayload,
	})
	if err != nil {
		flow.FinishStep(challengeStep, 0, nil, err.Error(), nil)
		flow.Fail(err)
		return *flow, err
	}
	resourceMetadataURL, challengedScope := parseResourceMetadata(challengeHeaders.Get("WWW-Authenticate"))
	flow.FinishStep(challengeStep, challengeStatus, challengeBody, "Initial protected MCP request returned a bearer challenge.", challengeHeaders)
	if challengeStatus != http.StatusUnauthorized || resourceMetadataURL == "" {
		err := errors.New("resource server did not return a valid bearer challenge")
		flow.Fail(err)
		return *flow, err
	}

	prmStep := flow.AddStep("Fetch protected resource metadata", http.MethodGet, resourceMetadataURL, nil)
	prmStatus, _, prmBody, prmRaw, err := r.getJSON(ctx, resourceMetadataURL)
	if err != nil {
		flow.FinishStep(prmStep, 0, nil, err.Error(), nil)
		flow.Fail(err)
		return *flow, err
	}
	flow.FinishStep(prmStep, prmStatus, prmBody, "", nil)

	authorizationServers, _ := prmRaw["authorization_servers"].([]any)
	if len(authorizationServers) == 0 {
		err := errors.New("protected resource metadata did not include authorization_servers")
		flow.Fail(err)
		return *flow, err
	}
	resourceAuthServer := fmt.Sprint(authorizationServers[0])

	resourceMetadataStep := flow.AddStep("Fetch resource authorization metadata", http.MethodGet, resourceAuthServer+"/.well-known/openid-configuration", nil)
	resourceMetaStatus, _, resourceMetaBody, resourceMetaRaw, err := r.getJSON(ctx, resourceAuthServer+"/.well-known/openid-configuration")
	if err != nil {
		flow.FinishStep(resourceMetadataStep, 0, nil, err.Error(), nil)
		flow.Fail(err)
		return *flow, err
	}
	flow.FinishStep(resourceMetadataStep, resourceMetaStatus, resourceMetaBody, "", nil)

	authMetadataStep := flow.AddStep("Fetch demo OIDC metadata", http.MethodGet, r.authPublicBase+"/.well-known/openid-configuration", nil)
	authMetaStatus, _, authMetaBody, authMetaRaw, err := r.getJSON(ctx, r.authPublicBase+"/.well-known/openid-configuration")
	if err != nil {
		flow.FinishStep(authMetadataStep, 0, nil, err.Error(), nil)
		flow.Fail(err)
		return *flow, err
	}
	flow.FinishStep(authMetadataStep, authMetaStatus, authMetaBody, "", nil)

	codeVerifier := randomURLToken(32)
	challenge := sha256.Sum256([]byte(codeVerifier))
	codeChallenge := base64.RawURLEncoding.EncodeToString(challenge[:])
	state := randomURLToken(12)

	authorizeURL := fmt.Sprintf("%s/authorize?response_type=code&client_id=%s&redirect_uri=%s&scope=%s&state=%s&code_challenge=%s&code_challenge_method=S256&demo_user=%s",
		r.authPublicBase,
		url.QueryEscape(client.ID),
		url.QueryEscape(client.RedirectURI),
		url.QueryEscape("openid email"),
		url.QueryEscape(state),
		url.QueryEscape(codeChallenge),
		url.QueryEscape(input.UserEmail),
	)
	authorizeStep := flow.AddStep("Authorize demo client", http.MethodGet, authorizeURL, nil)
	location, authzStatus, authzHeaders, err := r.getRedirectLocation(ctx, authorizeURL)
	if err != nil {
		flow.FinishStep(authorizeStep, 0, nil, err.Error(), nil)
		flow.Fail(err)
		return *flow, err
	}
	flow.FinishStep(authorizeStep, authzStatus, map[string]any{"location": location}, "The demo auth server auto-approves the enrolled email and returns an auth code redirect.", authzHeaders)
	if authzStatus != http.StatusFound {
		err := errors.New("authorization endpoint did not redirect with a code")
		flow.Fail(err)
		return *flow, err
	}

	redirectLocation, err := url.Parse(location)
	if err != nil {
		flow.Fail(err)
		return *flow, err
	}
	if redirectLocation.Query().Get("state") != state {
		err := errors.New("authorization state mismatch")
		flow.Fail(err)
		return *flow, err
	}
	code := redirectLocation.Query().Get("code")
	if code == "" {
		err := errors.New("authorization code missing from redirect")
		flow.Fail(err)
		return *flow, err
	}

	authTokenStep := flow.AddStep("Exchange auth code for ID token", http.MethodPost, fmt.Sprint(authMetaRaw["token_endpoint"]), map[string]any{
		"grant_type":    "authorization_code",
		"redirect_uri":  client.RedirectURI,
		"code_verifier": codeVerifier,
	})
	authTokenValues := url.Values{
		"grant_type":    []string{"authorization_code"},
		"code":          []string{code},
		"redirect_uri":  []string{client.RedirectURI},
		"code_verifier": []string{codeVerifier},
	}
	authTokenStatus, _, authTokenBody, authTokenRaw, err := r.postForm(ctx, fmt.Sprint(authMetaRaw["token_endpoint"]), client.ID, client.Secret, authTokenValues)
	if err != nil {
		flow.FinishStep(authTokenStep, 0, nil, err.Error(), nil)
		flow.Fail(err)
		return *flow, err
	}
	flow.FinishStep(authTokenStep, authTokenStatus, authTokenBody, "", nil)
	idToken := fmt.Sprint(authTokenRaw["id_token"])
	if idToken == "" {
		err := errors.New("id token missing from auth code exchange")
		flow.Fail(err)
		return *flow, err
	}
	_, idTokenClaims, _ := jose.DecodeWithoutVerify(idToken)
	flow.AddToken("id_token", jose.TokenPreview(idToken), idTokenClaims)

	exchangeStep := flow.AddStep("Exchange ID token for ID-JAG", http.MethodPost, fmt.Sprint(authMetaRaw["token_endpoint"]), map[string]any{
		"grant_type":           "urn:ietf:params:oauth:grant-type:token-exchange",
		"requested_token_type": "urn:ietf:params:oauth:token-type:id-jag",
		"audience":             r.resourcePublicBase,
		"resource":             r.resourcePublicBase + "/mcp",
		"scope":                scopeOrDefault(challengedScope),
	})
	exchangeValues := url.Values{
		"grant_type":           []string{"urn:ietf:params:oauth:grant-type:token-exchange"},
		"requested_token_type": []string{"urn:ietf:params:oauth:token-type:id-jag"},
		"audience":             []string{r.resourcePublicBase},
		"resource":             []string{r.resourcePublicBase + "/mcp"},
		"scope":                []string{scopeOrDefault(challengedScope)},
		"subject_token":        []string{idToken},
		"subject_token_type":   []string{"urn:ietf:params:oauth:token-type:id_token"},
	}
	exchangeStatus, _, exchangeBody, exchangeRaw, err := r.postForm(ctx, fmt.Sprint(authMetaRaw["token_endpoint"]), client.ID, client.Secret, exchangeValues)
	if err != nil {
		flow.FinishStep(exchangeStep, 0, nil, err.Error(), nil)
		flow.Fail(err)
		return *flow, err
	}
	flow.FinishStep(exchangeStep, exchangeStatus, exchangeBody, "", nil)
	idJag := fmt.Sprint(exchangeRaw["access_token"])
	if idJag == "" {
		err := errors.New("id-jag missing from token exchange")
		flow.Fail(err)
		return *flow, err
	}
	_, jagClaims, _ := jose.DecodeWithoutVerify(idJag)
	flow.AddToken("id_jag", jose.TokenPreview(idJag), jagClaims)

	resourceTokenStep := flow.AddStep("Exchange ID-JAG for resource access token", http.MethodPost, fmt.Sprint(resourceMetaRaw["token_endpoint"]), map[string]any{
		"grant_type": "urn:ietf:params:oauth:grant-type:jwt-bearer",
		"assertion":  jose.TokenPreview(idJag),
	})
	resourceTokenValues := url.Values{
		"grant_type": []string{"urn:ietf:params:oauth:grant-type:jwt-bearer"},
		"assertion":  []string{idJag},
	}
	resourceTokenStatus, _, resourceTokenBody, resourceTokenRaw, err := r.postForm(ctx, fmt.Sprint(resourceMetaRaw["token_endpoint"]), client.ID, client.Secret, resourceTokenValues)
	if err != nil {
		flow.FinishStep(resourceTokenStep, 0, nil, err.Error(), nil)
		flow.Fail(err)
		return *flow, err
	}
	flow.FinishStep(resourceTokenStep, resourceTokenStatus, resourceTokenBody, "", nil)
	accessToken := fmt.Sprint(resourceTokenRaw["access_token"])
	if accessToken == "" {
		err := errors.New("resource access token missing from jwt bearer exchange")
		flow.Fail(err)
		return *flow, err
	}
	_, accessClaims, _ := jose.DecodeWithoutVerify(accessToken)
	flow.AddToken("access_token", jose.TokenPreview(accessToken), accessClaims)

	authenticatedInitStep := flow.AddStep("Initialize upstream MCP session", http.MethodPost, r.resourcePublicBase+"/mcp", map[string]any{
		"method": "initialize",
		"params": initializePayload,
	})
	initStatus, _, initBody, err := r.postRPC(ctx, r.resourcePublicBase+"/mcp", accessToken, map[string]any{
		"jsonrpc": "2.0",
		"id":      2,
		"method":  "initialize",
		"params":  initializePayload,
	})
	if err != nil {
		flow.FinishStep(authenticatedInitStep, 0, nil, err.Error(), nil)
		flow.Fail(err)
		return *flow, err
	}
	flow.FinishStep(authenticatedInitStep, initStatus, initBody, "", nil)

	initializedStep := flow.AddStep("Send initialized notification", http.MethodPost, r.resourcePublicBase+"/mcp", map[string]any{
		"method": "notifications/initialized",
	})
	initializedStatus, _, initializedBody, err := r.postRPC(ctx, r.resourcePublicBase+"/mcp", accessToken, map[string]any{
		"jsonrpc": "2.0",
		"method":  "notifications/initialized",
	})
	if err != nil {
		flow.FinishStep(initializedStep, 0, nil, err.Error(), nil)
		flow.Fail(err)
		return *flow, err
	}
	flow.FinishStep(initializedStep, initializedStatus, initializedBody, "", nil)

	toolsListStep := flow.AddStep("List upstream tools", http.MethodPost, r.resourcePublicBase+"/mcp", map[string]any{"method": "tools/list"})
	toolsListStatus, _, toolsListBody, err := r.postRPC(ctx, r.resourcePublicBase+"/mcp", accessToken, map[string]any{
		"jsonrpc": "2.0",
		"id":      3,
		"method":  "tools/list",
	})
	if err != nil {
		flow.FinishStep(toolsListStep, 0, nil, err.Error(), nil)
		flow.Fail(err)
		return *flow, err
	}
	flow.FinishStep(toolsListStep, toolsListStatus, toolsListBody, "", nil)

	resourcesListStep := flow.AddStep("List upstream resources", http.MethodPost, r.resourcePublicBase+"/mcp", map[string]any{"method": "resources/list"})
	resourcesListStatus, _, resourcesListBody, err := r.postRPC(ctx, r.resourcePublicBase+"/mcp", accessToken, map[string]any{
		"jsonrpc": "2.0",
		"id":      4,
		"method":  "resources/list",
	})
	if err != nil {
		flow.FinishStep(resourcesListStep, 0, nil, err.Error(), nil)
		flow.Fail(err)
		return *flow, err
	}
	flow.FinishStep(resourcesListStep, resourcesListStatus, resourcesListBody, "", nil)

	resourceReadStep := flow.AddStep("Read todo resource", http.MethodPost, r.resourcePublicBase+"/mcp", map[string]any{
		"method": "resources/read",
		"params": map[string]any{"uri": "todo://list"},
	})
	resourceReadStatus, _, resourceReadBody, err := r.postRPC(ctx, r.resourcePublicBase+"/mcp", accessToken, map[string]any{
		"jsonrpc": "2.0",
		"id":      5,
		"method":  "resources/read",
		"params":  map[string]any{"uri": "todo://list"},
	})
	if err != nil {
		flow.FinishStep(resourceReadStep, 0, nil, err.Error(), nil)
		flow.Fail(err)
		return *flow, err
	}
	flow.FinishStep(resourceReadStep, resourceReadStatus, resourceReadBody, "", nil)

	toolCallStep := flow.AddStep("Call upstream todo tool", http.MethodPost, r.resourcePublicBase+"/mcp", map[string]any{
		"method": "tools/call",
		"params": map[string]any{
			"name":      input.ToolName,
			"arguments": input.Arguments,
		},
	})
	toolCallStatus, _, toolCallBody, err := r.postRPC(ctx, r.resourcePublicBase+"/mcp", accessToken, map[string]any{
		"jsonrpc": "2.0",
		"id":      6,
		"method":  "tools/call",
		"params": map[string]any{
			"name":      input.ToolName,
			"arguments": input.Arguments,
		},
	})
	if err != nil {
		flow.FinishStep(toolCallStep, 0, nil, err.Error(), nil)
		flow.Fail(err)
		return *flow, err
	}
	flow.FinishStep(toolCallStep, toolCallStatus, toolCallBody, "", nil)

	flow.Complete(map[string]any{
		"tool_name":         input.ToolName,
		"tool_args":         input.Arguments,
		"tool_call_result":  extractRPCResult(toolCallBody),
		"resource_snapshot": extractRPCResult(resourceReadBody),
	})
	return *flow, nil
}

func (r *Runner) fetchClient(ctx context.Context, clientID string) (DemoClient, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, r.internalURL(r.authPublicBase+"/api/internal/clients/"+url.PathEscape(clientID)), nil)
	if err != nil {
		return DemoClient{}, err
	}
	req.Header.Set("X-Demo-Internal-Request", "1")

	resp, err := r.httpClient.Do(req)
	if err != nil {
		return DemoClient{}, err
	}
	defer resp.Body.Close()

	rawBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return DemoClient{}, err
	}

	var body map[string]any
	if len(rawBody) > 0 {
		if err := json.Unmarshal(rawBody, &body); err != nil {
			return DemoClient{}, err
		}
	}

	if resp.StatusCode != http.StatusOK {
		return DemoClient{}, fmt.Errorf("fetch demo client: unexpected status %d", resp.StatusCode)
	}

	rawClient, ok := body["client"].(map[string]any)
	if !ok {
		return DemoClient{}, errors.New("client payload was not returned")
	}

	return DemoClient{
		ID:          fmt.Sprint(rawClient["id"]),
		Name:        fmt.Sprint(rawClient["name"]),
		Secret:      fmt.Sprint(rawClient["secret"]),
		RedirectURI: fmt.Sprint(rawClient["redirect_uri"]),
	}, nil
}

func (r *Runner) getJSON(ctx context.Context, publicURL string) (int, http.Header, any, map[string]any, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, r.internalURL(publicURL), nil)
	if err != nil {
		return 0, nil, nil, nil, err
	}
	resp, err := r.httpClient.Do(req)
	if err != nil {
		return 0, nil, nil, nil, err
	}
	defer resp.Body.Close()

	rawBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, nil, nil, nil, err
	}

	var parsed map[string]any
	if len(rawBody) > 0 {
		if err := json.Unmarshal(rawBody, &parsed); err != nil {
			return resp.StatusCode, resp.Header, string(rawBody), nil, nil
		}
	}
	if parsed == nil {
		parsed = map[string]any{}
	}
	return resp.StatusCode, resp.Header, parsed, parsed, nil
}

func (r *Runner) getRedirectLocation(ctx context.Context, publicURL string) (string, int, http.Header, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, r.internalURL(publicURL), nil)
	if err != nil {
		return "", 0, nil, err
	}
	resp, err := r.noRedirectClient.Do(req)
	if err != nil {
		return "", 0, nil, err
	}
	defer resp.Body.Close()
	return resp.Header.Get("Location"), resp.StatusCode, resp.Header, nil
}

func (r *Runner) postForm(ctx context.Context, publicURL, clientID, clientSecret string, values url.Values) (int, http.Header, any, map[string]any, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, r.internalURL(publicURL), strings.NewReader(values.Encode()))
	if err != nil {
		return 0, nil, nil, nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetBasicAuth(clientID, clientSecret)

	resp, err := r.httpClient.Do(req)
	if err != nil {
		return 0, nil, nil, nil, err
	}
	defer resp.Body.Close()

	rawBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, nil, nil, nil, err
	}

	var parsed map[string]any
	if len(rawBody) > 0 {
		if err := json.Unmarshal(rawBody, &parsed); err != nil {
			return resp.StatusCode, resp.Header, string(rawBody), nil, nil
		}
	}
	if parsed == nil {
		parsed = map[string]any{}
	}
	return resp.StatusCode, resp.Header, parsed, parsed, nil
}

func (r *Runner) postRPC(ctx context.Context, publicURL, bearerToken string, payload any) (int, http.Header, any, error) {
	data, err := json.Marshal(payload)
	if err != nil {
		return 0, nil, nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, r.internalURL(publicURL), bytes.NewReader(data))
	if err != nil {
		return 0, nil, nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	req.Header.Set("MCP-Protocol-Version", mcp.ProtocolVersion)
	if bearerToken != "" {
		req.Header.Set("Authorization", "Bearer "+bearerToken)
	}

	resp, err := r.httpClient.Do(req)
	if err != nil {
		return 0, nil, nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, nil, nil, err
	}
	if len(body) == 0 {
		return resp.StatusCode, resp.Header, map[string]any{}, nil
	}

	var parsed any
	if err := json.Unmarshal(body, &parsed); err != nil {
		return resp.StatusCode, resp.Header, string(body), nil
	}

	return resp.StatusCode, resp.Header, parsed, nil
}

func (r *Runner) internalURL(publicURL string) string {
	switch {
	case strings.HasPrefix(publicURL, r.authPublicBase):
		return r.authInternalBase + strings.TrimPrefix(publicURL, r.authPublicBase)
	case strings.HasPrefix(publicURL, r.resourcePublicBase):
		return r.resourceInternal + strings.TrimPrefix(publicURL, r.resourcePublicBase)
	default:
		return publicURL
	}
}

func parseResourceMetadata(header string) (string, string) {
	if header == "" {
		return "", ""
	}

	var resourceMetadata string
	var scope string
	parts := strings.Split(header, ",")
	for _, part := range parts {
		part = strings.TrimSpace(part)
		part = strings.TrimPrefix(part, "Bearer ")
		if strings.HasPrefix(part, "resource_metadata=") {
			resourceMetadata = strings.Trim(part[len("resource_metadata="):], `"`)
		}
		if strings.HasPrefix(part, "scope=") {
			scope = strings.Trim(part[len("scope="):], `"`)
		}
	}
	return resourceMetadata, scope
}

func randomURLToken(size int) string {
	buffer := make([]byte, size)
	_, _ = rand.Read(buffer)
	return base64.RawURLEncoding.EncodeToString(buffer)
}

func scopeOrDefault(scope string) string {
	if strings.TrimSpace(scope) == "" {
		return "mcp:read mcp:write"
	}
	return scope
}

func extractRPCResult(value any) any {
	payload, ok := value.(map[string]any)
	if !ok {
		return value
	}
	if result, ok := payload["result"]; ok {
		return result
	}
	return value
}
