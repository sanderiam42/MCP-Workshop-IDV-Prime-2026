package debuglog

import (
	"bytes"
	"io"
	"net/http"
)

type responseCapture struct {
	http.ResponseWriter
	status int
	body   bytes.Buffer
}

func (rc *responseCapture) WriteHeader(status int) {
	rc.status = status
	rc.ResponseWriter.WriteHeader(status)
}

func (rc *responseCapture) Write(b []byte) (int, error) {
	rc.body.Write(b)
	return rc.ResponseWriter.Write(b)
}

// Middleware wraps an http.Handler, logging all inbound requests and responses.
// When logger is not verbose, next is returned unchanged.
func Middleware(logger *Logger, next http.Handler) http.Handler {
	if !logger.verbose {
		return next
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var reqBody []byte
		if r.Body != nil {
			var err error
			reqBody, err = io.ReadAll(r.Body)
			if err == nil {
				r.Body = io.NopCloser(bytes.NewReader(reqBody))
			}
		}
		logger.LogInboundRequest(r, reqBody)

		rc := &responseCapture{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rc, r)

		logger.LogInboundResponse(rc.status, w.Header(), rc.body.Bytes())
	})
}
