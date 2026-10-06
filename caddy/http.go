package qwacamole

import (
	"fmt"
	"github.com/caddyserver/caddy/v2/modules/caddyhttp"
	"github.com/cztamas/qwacamole/store"
	"io"
	"net/http"
	"time"
)

func (h *Handler) serveDocument(w http.ResponseWriter, r *http.Request) error {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		return caddyhttp.Error(405, nil)
	}
	d, e := h.manager.Document(r.URL.Path)
	if e != nil {
		return caddyhttp.Error(404, e)
	}
	w.Header().Set("Content-Type", "application/jose")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Length", fmt.Sprint(len(d)))
	if r.Method == http.MethodHead {
		w.WriteHeader(200)
		return nil
	}
	_, e = w.Write(d)
	return e
}

// Commit-time advertisement survives downstream Header.Set and streams without
// buffering. ResponseController reaches optional interfaces through Unwrap.
type bindingWriter struct {
	http.ResponseWriter
	manager                *store.Manager
	committed, unavailable bool
}

func (w *bindingWriter) Unwrap() http.ResponseWriter { return w.ResponseWriter }
func (w *bindingWriter) WriteHeader(status int) {
	if w.committed {
		return
	}
	if status >= 100 && status < 200 && status != 101 {
		w.ResponseWriter.WriteHeader(status)
		return
	}
	w.committed = true
	s := w.manager.Snapshot()
	if s.URL == "" || s.Expires <= time.Now().Unix() {
		w.unavailable = true
		w.Header().Del("Content-Length")
		w.ResponseWriter.WriteHeader(503)
		return
	}
	w.Header().Add("Link", fmt.Sprintf(`<%s>; rel="tls-certificate-binding"`, s.URL))
	w.ResponseWriter.WriteHeader(status)
}
func (w *bindingWriter) Write(d []byte) (int, error) {
	if !w.committed {
		w.WriteHeader(200)
	}
	if w.unavailable {
		return 0, fmt.Errorf("binding expired before application committed response")
	}
	return w.ResponseWriter.Write(d)
}
func (w *bindingWriter) Flush() {
	if !w.committed {
		w.WriteHeader(200)
	}
	_ = http.NewResponseController(w.ResponseWriter).Flush()
}
func (w *bindingWriter) ReadFrom(r io.Reader) (int64, error) {
	if !w.committed {
		w.WriteHeader(200)
	}
	if w.unavailable {
		return 0, fmt.Errorf("binding unavailable")
	}
	if rf, ok := w.ResponseWriter.(io.ReaderFrom); ok {
		return rf.ReadFrom(r)
	}
	return io.Copy(w.ResponseWriter, r)
}
