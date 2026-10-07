// Package cacheproxy supplies a deliberately small real HTTP caching intermediary
// for development acceptance checks. It never belongs to the installed plugin.
package cacheproxy

import (
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Proxy caches one configured public origin response, preserving its body, Link
// fields and cache policy. Requests cannot select a different upstream URL.
type Proxy struct {
	Origin          string
	Client          *http.Client
	mu              sync.Mutex
	header          http.Header
	body            []byte
	status          int
	stored, expires time.Time
	requests, hits  int
}

// Counts proves that responses came from the intermediary's stored entry.
func (p *Proxy) Counts() (requests, hits int) {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.requests, p.hits
}

func (p *Proxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.WriteHeader(405)
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	p.requests++
	hit := time.Now().Before(p.expires)
	if !hit {
		res, e := p.Client.Get(p.Origin)
		if e != nil {
			http.Error(w, e.Error(), 502)
			return
		}
		d, e := io.ReadAll(res.Body)
		res.Body.Close()
		if e != nil {
			http.Error(w, e.Error(), 502)
			return
		}
		p.header, p.body, p.status = res.Header.Clone(), d, res.StatusCode
		p.stored, p.expires = time.Now(), time.Time{}
		cc := strings.ToLower(res.Header.Get("Cache-Control"))
		if res.StatusCode == 200 && strings.Contains(cc, "public") && !strings.Contains(cc, "no-store") && !strings.Contains(cc, "no-cache") {
			for _, option := range strings.Split(cc, ",") {
				if value, ok := strings.CutPrefix(strings.TrimSpace(option), "max-age="); ok {
					if seconds, e := strconv.Atoi(value); e == nil && seconds > 0 {
						p.expires = p.stored.Add(time.Duration(seconds) * time.Second)
					}
				}
			}
		}
	} else {
		p.hits++
	}
	for name, values := range p.header {
		for _, value := range values {
			w.Header().Add(name, value)
		}
	}
	w.Header().Set("Age", strconv.Itoa(int(time.Since(p.stored)/time.Second)))
	if hit {
		w.Header().Set("X-Qwac-Cache", "hit")
	} else {
		w.Header().Set("X-Qwac-Cache", "miss")
	}
	w.WriteHeader(p.status)
	if r.Method != http.MethodHead {
		_, _ = w.Write(p.body)
	}
}
