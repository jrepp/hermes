package api

import (
	"encoding/json"
	"net/http"
	"strings"
)

// ErrorResponse is the shape every API error uses.
//
// The successful responses are JSON, and the errors were plain text, so a
// client had to branch on Content-Type before it could read a failure. Worse,
// several errors carried no body at all -- a bare status with nothing to log
// and nothing to show a user.
type ErrorResponse struct {
	// Error is a short description of what went wrong.
	Error string `json:"error"`

	// Path is the request path, so an error copied out of a log or a browser
	// console still says what it was about.
	Path string `json:"path,omitempty"`

	// Allowed lists the methods this resource accepts, on a 405.
	Allowed []string `json:"allowed,omitempty"`
}

// WriteError sends a JSON error response.
//
// The message should say what is wrong in terms the caller can act on:
// "documentID is required" rather than "bad request".
func WriteError(w http.ResponseWriter, r *http.Request, status int, message string) {
	writeErrorResponse(w, status, ErrorResponse{Error: message, Path: r.URL.Path})
}

// WriteMethodNotAllowed sends a 405 naming the methods this resource accepts.
//
// The Allow header is not optional: RFC 9110 requires a 405 to carry one, and
// nothing in this codebase sent it. Clients and proxies use it to decide
// whether a retry with another method is worth attempting, and a person
// debugging by hand gets the answer without reading the source.
func WriteMethodNotAllowed(w http.ResponseWriter, r *http.Request, allowed ...string) {
	if len(allowed) == 0 {
		// A resource that accepts nothing is a bug, but an empty Allow header
		// is a worse answer than a missing one.
		writeErrorResponse(w, http.StatusMethodNotAllowed, ErrorResponse{
			Error: r.Method + " is not allowed here",
			Path:  r.URL.Path,
		})

		return
	}

	w.Header().Set("Allow", strings.Join(allowed, ", "))
	writeErrorResponse(w, http.StatusMethodNotAllowed, ErrorResponse{
		Error:   r.Method + " is not allowed here; use " + strings.Join(allowed, " or "),
		Path:    r.URL.Path,
		Allowed: allowed,
	})
}

func writeErrorResponse(w http.ResponseWriter, status int, body ErrorResponse) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}
