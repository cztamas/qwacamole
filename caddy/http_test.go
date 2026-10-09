package qwacamole

import (
	"context"
	"errors"
	"net/http/httptest"
	"testing"

	"github.com/cztamas/qwacamole/store"
)

type unreadResponse struct{ closed bool }

func (*unreadResponse) Read([]byte) (int, error) { panic("rejected body must never be read") }
func (r *unreadResponse) Close() error           { r.closed = true; return nil }

func TestRejectedResponseStopsBothCopyPaths(t *testing.T) {
	clientCtx := t.Context()
	child, cancel := context.WithCancel(clientCtx)
	defer cancel()
	res := httptest.NewRecorder()
	w := &bindingWriter{ResponseWriter: res, manager: &store.Manager{}, cancel: cancel}
	w.Header().Set("Set-Cookie", "protected=secret")
	if n, err := w.Write([]byte("protected")); n != 0 || !errors.Is(err, errBindingResponseRejected) {
		t.Fatal("rejected write continued the copy loop", n, err)
	}
	body := &unreadResponse{}
	if n, err := w.ReadFrom(body); n != 0 || !errors.Is(err, errBindingResponseRejected) || !body.closed {
		t.Fatal("rejected copy did not close without reading", n, err, body.closed)
	}
	if child.Err() != context.Canceled || clientCtx.Err() != nil {
		t.Fatal("wrong request canceled", child.Err(), clientCtx.Err())
	}
	if res.Code != 503 || res.Body.Len() != 0 || res.Header().Get("Set-Cookie") != "" || res.Header().Get("Cache-Control") != "no-store" {
		t.Fatal("protected response escaped rejection", res.Code, res.Header(), res.Body)
	}
}
