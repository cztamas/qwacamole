// Package bindinghttp delivers verified committed binding snapshots. It contains
// no deployment control, private keys, integration leases or application proxy.
package bindinghttp

import (
	"fmt"
	"net/http"

	"github.com/cztamas/qwacamole/store"
)

// Error distinguishes missing immutable artifacts from unavailable current state.
type Error struct {
	Status int
	Cause  error
}

func (e *Error) Error() string { return fmt.Sprintf("binding HTTP %d: %v", e.Status, e.Cause) }
func (e *Error) Unwrap() error { return e.Cause }

// Available checks the committed document, including its signature, coverage and
// complete QWAC chain validity. A timestamp alone is not availability evidence.
func Available(m *store.Manager) bool {
	_, err := m.CurrentDocument()
	return err == nil
}

// Serve writes GET/HEAD binding bytes from one committed manager snapshot.
// On error, adapters write their own error response using Error.Status; headers
// already prohibit caching. Binding responses never add discovery Links.
func Serve(w http.ResponseWriter, r *http.Request, m *store.Manager) error {
	w.Header().Set("Cache-Control", "no-store")
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		return &Error{http.StatusMethodNotAllowed, fmt.Errorf("GET or HEAD required")}
	}
	var d []byte
	var err error
	status := http.StatusNotFound
	if r.URL.Path == store.DiscoveryURL {
		d, err = m.CurrentDocument()
		status = http.StatusServiceUnavailable
	} else {
		d, err = m.Document(r.URL.Path)
	}
	if err != nil {
		return &Error{status, err}
	}
	w.Header().Set("Content-Type", "application/jose")
	w.Header().Set("Content-Length", fmt.Sprint(len(d)))
	if r.Method == http.MethodHead {
		w.WriteHeader(http.StatusOK)
		return nil
	}
	_, err = w.Write(d)
	return err
}

// Handler adapts Serve to ordinary net/http, without revealing storage errors.
func Handler(m *store.Manager) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := Serve(w, r, m); err != nil {
			status := http.StatusInternalServerError
			if e, ok := err.(*Error); ok {
				status = e.Status
			}
			http.Error(w, http.StatusText(status), status)
		}
	})
}
