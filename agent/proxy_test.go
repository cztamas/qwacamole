package agent

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestDelayedApplicationRejectedWhenBindingDisappears(t *testing.T) {
	started, release := make(chan struct{}), make(chan struct{})
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.RequestURI() != "/private/a%2Fb?q=one" {
			t.Errorf("upstream URL changed: %s", r.URL.RequestURI())
		}
		close(started)
		<-release
		w.Header().Set("Cache-Control", "public, max-age=600")
		w.Header().Set("ETag", `"private"`)
		w.WriteHeader(201)
		w.Write([]byte("protected application body"))
	}))
	defer upstream.Close()
	var available atomic.Bool
	available.Store(true)
	h := applicationProxy(upstream.URL, available.Load)
	w := httptest.NewRecorder()
	done := make(chan struct{})
	go func() {
		h.ServeHTTP(w, httptest.NewRequest("GET", "http://agent/application/private/a%2Fb?q=one", nil))
		close(done)
	}()
	<-started
	available.Store(false)
	close(release)
	<-done
	if w.Code != 503 || w.Header().Get("Cache-Control") != "no-store" || w.Header().Get("ETag") != "" || strings.Contains(w.Body.String(), "protected") {
		t.Fatalf("protected response escaped the commit gate: %d %v %s", w.Code, w.Header(), w.Body)
	}
}

func TestDelayedStreamingResponseRejectedWithoutDraining(t *testing.T) {
	started, release, canceled := make(chan struct{}), make(chan struct{}), make(chan struct{})
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(started)
		<-release
		w.Write([]byte("protected indefinite stream"))
		w.(http.Flusher).Flush()
		<-r.Context().Done()
		close(canceled)
	}))
	defer upstream.Close()
	var available atomic.Bool
	available.Store(true)
	gateway := httptest.NewServer(applicationProxy(upstream.URL, available.Load))
	defer gateway.Close()
	done := make(chan struct{})
	go func() {
		defer close(done)
		client := &http.Client{Timeout: 2 * time.Second}
		r, e := client.Get(gateway.URL + "/application/stream")
		if e != nil {
			t.Error(e)
			return
		}
		defer r.Body.Close()
		b, e := io.ReadAll(r.Body)
		if e != nil || r.StatusCode != 503 || strings.Contains(string(b), "protected") {
			t.Error("rejected stream was drained or leaked", r.StatusCode, string(b), e)
		}
	}()
	<-started
	available.Store(false)
	close(release)
	<-done
	select {
	case <-canceled:
	case <-time.After(2 * time.Second):
		t.Fatal("rejected upstream stream was not canceled")
	}
}

func TestProtectedProxyPreservesCommittedStream(t *testing.T) {
	release := make(chan struct{})
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "public, max-age=600")
		w.Header().Add("Link", `</app.css>; rel="preload"`)
		w.WriteHeader(201)
		w.Write([]byte("first\n"))
		w.(http.Flusher).Flush()
		<-release
		w.Write([]byte("second\n"))
	}))
	defer upstream.Close()
	var available atomic.Bool
	available.Store(true)
	gateway := httptest.NewServer(applicationProxy(upstream.URL, available.Load))
	defer gateway.Close()
	r, e := http.Get(gateway.URL + "/application/stream")
	if e != nil {
		close(release)
		t.Fatal(e)
	}
	defer r.Body.Close()
	first := make([]byte, 6)
	if _, e = io.ReadFull(r.Body, first); e != nil {
		close(release)
		t.Fatal(e)
	}
	available.Store(false)
	close(release)
	rest, e := io.ReadAll(r.Body)
	if e != nil || string(first) != "first\n" || string(rest) != "second\n" || r.StatusCode != 201 || r.Header.Get("Cache-Control") != "public, max-age=600" || !strings.Contains(r.Header.Get("Link"), "preload") {
		t.Fatal("committed stream changed", r.StatusCode, r.Header, string(first), string(rest), e)
	}
	blocked, e := http.Get(gateway.URL + "/application/stream")
	if e != nil {
		t.Fatal(e)
	}
	defer blocked.Body.Close()
	if blocked.StatusCode != 503 {
		t.Fatal("new protected request bypassed gate", blocked.StatusCode)
	}
}

func TestDelayedUpgradeRejectedWhenBindingDisappears(t *testing.T) {
	started, release := make(chan struct{}), make(chan struct{})
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Upgrade") != "websocket" {
			t.Error("upgrade headers did not reach origin")
		}
		close(started)
		<-release
		conn, _, err := http.NewResponseController(w).Hijack()
		if err != nil {
			t.Error(err)
			return
		}
		defer conn.Close()
		_, _ = io.WriteString(conn, "HTTP/1.1 101 Switching Protocols\r\nConnection: Upgrade\r\nUpgrade: websocket\r\nCache-Control: public\r\n\r\n")
	}))
	defer upstream.Close()
	var available atomic.Bool
	available.Store(true)
	gateway := httptest.NewServer(applicationProxy(upstream.URL, available.Load))
	defer gateway.Close()
	done := make(chan struct{})
	go func() {
		defer close(done)
		req, _ := http.NewRequest("GET", gateway.URL+"/application/ws", nil)
		req.Header.Set("Connection", "Upgrade")
		req.Header.Set("Upgrade", "websocket")
		client := &http.Client{Timeout: 2 * time.Second}
		resp, err := client.Do(req)
		if err != nil {
			t.Error(err)
			return
		}
		defer resp.Body.Close()
		if resp.StatusCode != 503 || resp.Header.Get("Upgrade") != "" || resp.Header.Get("Cache-Control") != "no-store" {
			t.Error("upgrade escaped response-commit gate", resp.StatusCode, resp.Header)
		}
	}()
	select {
	case <-started:
	case <-done:
		t.Fatal("upgrade never reached origin")
	}
	available.Store(false)
	close(release)
	<-done
}
