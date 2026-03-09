package debuglog

import (
	"bytes"
	"io"
	"net/http"
)

// LoggingTransport wraps http.RoundTripper, logging all outbound HTTP calls.
type LoggingTransport struct {
	Base   http.RoundTripper // defaults to http.DefaultTransport if nil
	Logger *Logger
}

func (t *LoggingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	base := t.Base
	if base == nil {
		base = http.DefaultTransport
	}

	// Skip buffering overhead entirely when not verbose.
	if !t.Logger.verbose {
		return base.RoundTrip(req)
	}

	// Buffer request body so we can log it and restore it.
	var reqBody []byte
	if req.Body != nil {
		var err error
		reqBody, err = io.ReadAll(req.Body)
		if err != nil {
			return nil, err
		}
		req.Body = io.NopCloser(bytes.NewReader(reqBody))
	}
	t.Logger.LogRequest(req, reqBody)

	resp, err := base.RoundTrip(req)
	if err != nil {
		return nil, err
	}

	// Buffer response body so we can log it and restore it.
	var respBody []byte
	if resp.Body != nil {
		respBody, err = io.ReadAll(resp.Body)
		if err != nil {
			return nil, err
		}
		resp.Body = io.NopCloser(bytes.NewReader(respBody))
	}
	t.Logger.LogResponse(resp, respBody)

	return resp, nil
}
