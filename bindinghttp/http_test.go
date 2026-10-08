package bindinghttp

import (
	"github.com/cztamas/qwacamole/store"
	"net/http/httptest"
	"testing"
)

func TestUnavailableAndMethodResponsesAreUncachedAndNonrecursive(t *testing.T) {
	for _, test := range []struct {
		method, path string
		status       int
	}{{"GET", store.DiscoveryURL, 503}, {"HEAD", store.DiscoveryURL, 503}, {"POST", store.DiscoveryURL, 405}, {"GET", "/bindings/../../secret", 404}} {
		w := httptest.NewRecorder()
		m := &store.Manager{}
		Handler(m).ServeHTTP(w, httptest.NewRequest(test.method, "http://example.test"+test.path, nil))
		if w.Code != test.status || w.Header().Get("Cache-Control") != "no-store" || w.Header().Get("Link") != "" {
			t.Fatal(test, w.Code, w.Header())
		}
		if Available(m) {
			t.Fatal("empty manager available")
		}
	}
}
