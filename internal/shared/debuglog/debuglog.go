package debuglog

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"
)

// Logger is a structured debug logger for HTTP calls, tokens, and flow steps.
// All methods are no-ops when verbose == false.
type Logger struct {
	component string
	verbose   bool
	logger    *log.Logger
}

// New creates a Logger. If verbose is false, all methods are no-ops.
// If logFile is non-empty, output goes to that file (appended) as well as stderr.
func New(component string, verbose bool, logFile string) (*Logger, error) {
	if !verbose {
		return &Logger{component: component, verbose: false}, nil
	}

	var out io.Writer = os.Stderr
	if logFile != "" {
		f, err := os.OpenFile(logFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			return nil, fmt.Errorf("open log file %q: %w", logFile, err)
		}
		out = io.MultiWriter(os.Stderr, f)
	}

	return &Logger{
		component: component,
		verbose:   true,
		logger:    log.New(out, "", 0),
	}, nil
}

func (l *Logger) Verbose() bool { return l.verbose }

func (l *Logger) printf(format string, args ...any) {
	if !l.verbose || l.logger == nil {
		return
	}
	ts := time.Now().UTC().Format(time.RFC3339)
	l.logger.Printf("[%s] [%s] "+format, append([]any{ts, l.component}, args...)...)
}

// LogRequest logs an outbound HTTP request.
func (l *Logger) LogRequest(req *http.Request, body []byte) {
	if !l.verbose {
		return
	}
	var sb strings.Builder
	fmt.Fprintf(&sb, "── OUTBOUND REQUEST ─────────────────\n")
	fmt.Fprintf(&sb, "  %s %s\n", req.Method, req.URL.String())
	fmt.Fprintf(&sb, "  Headers:\n")
	for k, vs := range req.Header {
		fmt.Fprintf(&sb, "    %s: %s\n", k, strings.Join(vs, ", "))
	}
	if len(body) > 0 {
		fmt.Fprintf(&sb, "  Body:\n    %s\n", string(body))
	}
	l.printf("%s", sb.String())
}

// LogResponse logs an outbound HTTP response.
func (l *Logger) LogResponse(resp *http.Response, body []byte) {
	if !l.verbose {
		return
	}
	var sb strings.Builder
	fmt.Fprintf(&sb, "── OUTBOUND RESPONSE ────────────────\n")
	fmt.Fprintf(&sb, "  %s\n", resp.Status)
	fmt.Fprintf(&sb, "  Headers:\n")
	for k, vs := range resp.Header {
		fmt.Fprintf(&sb, "    %s: %s\n", k, strings.Join(vs, ", "))
	}
	if len(body) > 0 {
		fmt.Fprintf(&sb, "  Body:\n    %s\n", string(body))
	}
	l.printf("%s", sb.String())
}

// LogInboundRequest logs an inbound HTTP request to this service.
func (l *Logger) LogInboundRequest(req *http.Request, body []byte) {
	if !l.verbose {
		return
	}
	var sb strings.Builder
	fmt.Fprintf(&sb, "── INBOUND REQUEST ──────────────────\n")
	fmt.Fprintf(&sb, "  %s %s\n", req.Method, req.URL.String())
	fmt.Fprintf(&sb, "  Headers:\n")
	for k, vs := range req.Header {
		fmt.Fprintf(&sb, "    %s: %s\n", k, strings.Join(vs, ", "))
	}
	if len(body) > 0 {
		fmt.Fprintf(&sb, "  Body:\n    %s\n", string(body))
	}
	l.printf("%s", sb.String())
}

// LogInboundResponse logs what this service returned.
func (l *Logger) LogInboundResponse(status int, headers http.Header, body []byte) {
	if !l.verbose {
		return
	}
	var sb strings.Builder
	fmt.Fprintf(&sb, "── INBOUND RESPONSE ─────────────────\n")
	fmt.Fprintf(&sb, "  %d\n", status)
	fmt.Fprintf(&sb, "  Headers:\n")
	for k, vs := range headers {
		fmt.Fprintf(&sb, "    %s: %s\n", k, strings.Join(vs, ", "))
	}
	if len(body) > 0 {
		fmt.Fprintf(&sb, "  Body:\n    %s\n", string(body))
	}
	l.printf("%s", sb.String())
}

// LogToken logs a token being issued or received.
func (l *Logger) LogToken(event, kind, subject, clientID string, claims map[string]any, expiresAt time.Time) {
	if !l.verbose {
		return
	}
	l.printf("── TOKEN %s ──────────────────────────\n  Kind: %s  Client: %s  Sub: %s\n  Claims: %v\n  Expires: %s\n",
		strings.ToUpper(event), kind, clientID, subject, claims, expiresAt.UTC().Format(time.RFC3339))
}

// LogStep logs a named flow step.
func (l *Logger) LogStep(name, method, url string) {
	if !l.verbose {
		return
	}
	l.printf("── STEP: %s\n  %s %s\n", name, method, url)
}
