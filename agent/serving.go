package agent

import (
	"fmt"
	"net"
	"strconv"
	"time"
)

// ServingConfig is trusted operator input. Nil body size means 1 MiB; explicit
// zero is unlimited. Timeouts are inactivity limits, never total durations.
type ServingConfig struct {
	MaxBodyBytes        *int64      `json:"max_request_body_bytes,omitempty"`
	ClientBodyTimeout   string      `json:"client_body_timeout,omitempty"`
	UpstreamSendTimeout string      `json:"upstream_send_timeout,omitempty"`
	UpstreamReadTimeout string      `json:"upstream_read_timeout,omitempty"`
	RequestBuffering    string      `json:"request_buffering,omitempty"`
	RedirectHTTPS       bool        `json:"redirect_https,omitempty"`
	HTTPSPort           int         `json:"public_https_port,omitempty"`
	HSTS                *HSTSConfig `json:"hsts,omitempty"`
}
type HSTSConfig struct {
	MaxAgeSeconds     int64 `json:"max_age_seconds"`
	IncludeSubdomains bool  `json:"include_subdomains,omitempty"`
}
type servingValues struct {
	body               int64
	client, send, read int64
	buffering          string
}

func (s ServingConfig) values() (v servingValues, err error) {
	v.body = 1 << 20
	if s.MaxBodyBytes != nil {
		v.body = *s.MaxBodyBytes
	}
	if v.body < 0 || v.body > 1<<40 {
		return v, fmt.Errorf("max_request_body_bytes must be 0 (unlimited) through 1 TiB; omitted means 1 MiB")
	}
	for _, item := range []struct {
		text string
		dst  *int64
	}{{s.ClientBodyTimeout, &v.client}, {s.UpstreamSendTimeout, &v.send}, {s.UpstreamReadTimeout, &v.read}} {
		*item.dst = 60000
		if item.text == "" {
			continue
		}
		d, e := time.ParseDuration(item.text)
		if e != nil || d < time.Millisecond || d > 24*time.Hour || d%time.Millisecond != 0 {
			return v, fmt.Errorf("serving inactivity timeouts require whole milliseconds from 1ms through 24h; omitted means 60s; zero/unlimited unsupported")
		}
		*item.dst = d.Milliseconds()
	}
	v.buffering = s.RequestBuffering
	if v.buffering == "" {
		v.buffering = "on"
	}
	if v.buffering != "on" && v.buffering != "off" {
		return v, fmt.Errorf("request_buffering must be on or off")
	}
	if s.HTTPSPort < 0 || s.HTTPSPort > 65535 {
		return v, fmt.Errorf("public_https_port must be 1..65535 or omitted/zero to use https_listen port")
	}
	if s.HSTS != nil && (s.HSTS.MaxAgeSeconds < 0 || s.HSTS.MaxAgeSeconds > 63072000) {
		return v, fmt.Errorf("HSTS max_age_seconds must be 0..63072000; omit hsts to disable")
	}
	return v, nil
}
func (c Config) PublicAuthority() string {
	port := c.Serving.HTTPSPort
	if port == 0 {
		_, p, _ := net.SplitHostPort(c.HTTPSListen)
		port, _ = strconv.Atoi(p)
	}
	if port == 443 {
		return c.Host
	}
	return net.JoinHostPort(c.Host, strconv.Itoa(port))
}
func (c Config) hstsHeader() string {
	if c.Serving.HSTS == nil {
		return ""
	}
	value := fmt.Sprintf("max-age=%d", c.Serving.HSTS.MaxAgeSeconds)
	if c.Serving.HSTS.IncludeSubdomains {
		value += "; includeSubDomains"
	}
	return "add_header Strict-Transport-Security " + quote(value) + " always;"
}

func (h *HSTSConfig) UnmarshalJSON(data []byte) error {
	var raw struct {
		MaxAge  *int64 `json:"max_age_seconds"`
		Include bool   `json:"include_subdomains,omitempty"`
	}
	if e := decodeStrict(data, &raw); e != nil {
		return e
	}
	if raw.MaxAge == nil {
		return fmt.Errorf("hsts requires explicit max_age_seconds; 0 clears browser policy")
	}
	h.MaxAgeSeconds, h.IncludeSubdomains = *raw.MaxAge, raw.Include
	return nil
}
