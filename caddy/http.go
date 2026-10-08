package qwacamole

import (
	"bufio"
	"fmt"
	"github.com/caddyserver/caddy/v2/modules/caddyhttp"
	"github.com/cztamas/qwacamole/bindinghttp"
	"github.com/cztamas/qwacamole/store"
	"io"
	"net"
	"net/http"
)

func (h *Handler) serveDocument(w http.ResponseWriter, r *http.Request) error {
	if err := bindinghttp.Serve(w, r, h.manager); err != nil {
		if e, ok := err.(*bindinghttp.Error); ok {
			return caddyhttp.Error(e.Status, e.Cause)
		}
		return err
	}
	return nil
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
	if !bindinghttp.Available(w.manager) {
		w.unavailable = true
		for key := range w.Header() {
			w.Header().Del(key)
		}
		w.Header().Set("Cache-Control", "no-store")
		w.ResponseWriter.WriteHeader(http.StatusServiceUnavailable)
		return
	}
	w.Header().Add("Link", fmt.Sprintf(`<%s>; rel="tls-certificate-binding"`, store.DiscoveryURL))
	w.ResponseWriter.WriteHeader(status)
}

// A reverse proxy can hijack before WriteHeader to write its own 101. Enforce
// availability at that boundary too; already hijacked streams remain committed.
func (w *bindingWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	if w.unavailable || !bindinghttp.Available(w.manager) {
		if !w.committed {
			w.WriteHeader(http.StatusServiceUnavailable)
		}
		return nil, nil, fmt.Errorf("binding unavailable at protocol upgrade")
	}
	c, rw, err := http.NewResponseController(w.ResponseWriter).Hijack()
	if err == nil {
		w.committed = true
	}
	return c, rw, err
}
func (w *bindingWriter) Write(d []byte) (int, error) {
	if !w.committed {
		w.WriteHeader(200)
	}
	if w.unavailable {
		// The 503 already replaced this response. A write error would make
		// Caddy's reverse proxy abort/reset HTTP/2 before delivering that status.
		return len(d), nil
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
		return io.Copy(io.Discard, r)
	}
	if rf, ok := w.ResponseWriter.(io.ReaderFrom); ok {
		return rf.ReadFrom(r)
	}
	return io.Copy(w.ResponseWriter, r)
}
