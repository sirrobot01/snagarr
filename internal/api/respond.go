package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"

	"github.com/sirrobot01/snagarr/internal/store"
)

// Error codes are part of the API contract; see docs/api.md.
const (
	codeBadRequest    = "bad_request"
	codeUnauthorized  = "unauthorized"
	codeForbidden     = "forbidden"
	codeNotFound      = "not_found"
	codeConflict      = "conflict"
	codeRateLimited   = "rate_limited"
	codeUnresolvable  = "unresolvable"
	codeUpstreamError = "upstream_error"
	codeNotConfigured = "not_configured"
	codeInternal      = "internal_error"
)

type errorBody struct {
	Error struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if v == nil {
		return
	}
	// The header is already sent, so a write failure can only be logged by the
	// caller's middleware; there is no status left to change.
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, code, format string, args ...any) {
	var body errorBody
	body.Error.Code = code
	body.Error.Message = fmt.Sprintf(format, args...)
	writeJSON(w, status, body)
}

// writeStoreError maps the one sentinel the store exposes; anything else is a
// bug rather than a client mistake.
func (s *Server) writeStoreError(w http.ResponseWriter, err error, subject string) {
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, codeNotFound, "%s does not exist", subject)
		return
	}
	s.log.Error("request failed", "subject", subject, "error", err)
	writeError(w, http.StatusInternalServerError, codeInternal, "%s could not be read", subject)
}

// decode reads a JSON body, rejecting anything oversized or malformed.
func decode(w http.ResponseWriter, r *http.Request, v any) bool {
	dec := json.NewDecoder(io.LimitReader(r.Body, 1<<20))
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		writeError(w, http.StatusBadRequest, codeBadRequest, "invalid request body: %v", err)
		return false
	}
	return true
}
