package qwacamole

import (
	"encoding/json"
	"fmt"
	"net"
	"path/filepath"
	"strings"
	"time"

	"github.com/caddyserver/caddy/v2"
	"github.com/cztamas/qwacamole/credentials"
	"github.com/cztamas/qwacamole/store"
)

// BindingConfig describes one literal DNS hostname and one single-host state owner.
// ChainFiles accepts leaf-first PEM bundles or separate files. TrustFiles is a
// separately configured, single QWAC authority used only for local consistency.
type BindingConfig struct {
	store.Config
	// ChainFiles supplies leaf-first PEM certificates, bundled or separate. The
	// explicit authority may be omitted here; TrustFiles supplies it separately.
	ChainFiles []string `json:"chain_files"`
	// KeyFile is an unencrypted RSA PKCS#1 or PKCS#8 PEM key matching the leaf.
	KeyFile string `json:"key_file"`
	// TrustFiles contains exactly one explicitly configured QWAC authority PEM.
	// This local consistency anchor does not add trust to visiting browsers.
	TrustFiles []string `json:"trust_files"`
}

// Retain legacy numeric nanoseconds in JSON, and accept readable durations.
func (b *BindingConfig) UnmarshalJSON(d []byte) error {
	type plain BindingConfig
	var fields map[string]json.RawMessage
	if e := json.Unmarshal(d, &fields); e != nil {
		return e
	}
	for _, name := range []string{"lifetime", "refresh_before", "document_grace"} {
		if raw := fields[name]; len(raw) != 0 && raw[0] == '"' {
			var s string
			if e := json.Unmarshal(raw, &s); e != nil {
				return e
			}
			v, e := caddy.ParseDuration(s)
			if e != nil {
				return e
			}
			fields[name], _ = json.Marshal(int64(v))
		}
	}
	d, _ = json.Marshal(fields)
	// Match Caddy's strict handling of unknown configuration fields.
	var p plain
	dec := json.NewDecoder(strings.NewReader(string(d)))
	dec.DisallowUnknownFields()
	if e := dec.Decode(&p); e != nil {
		return e
	}
	*b = BindingConfig(p)
	return nil
}

func (b BindingConfig) normalized() (BindingConfig, error) {
	if b.Dir == "" || b.Host == "" || len(b.ChainFiles) == 0 || b.KeyFile == "" || len(b.TrustFiles) == 0 {
		return b, fmt.Errorf("directory, hostname, chain_files, key_file and explicit trust_files are required")
	}
	if b.Host != strings.ToLower(b.Host) || strings.ContainsAny(b.Host, "*/:{} \\?#") || net.ParseIP(b.Host) != nil || !strings.Contains(b.Host, ".") || len(b.Host) > 253 {
		return b, fmt.Errorf("one lower-case literal DNS hostname required (no wildcard, IP, port or placeholder)")
	}
	for _, label := range strings.Split(b.Host, ".") {
		if len(label) == 0 || len(label) > 63 || label[0] == '-' || label[len(label)-1] == '-' {
			return b, fmt.Errorf("invalid DNS hostname")
		}
		for _, c := range label {
			if !(c >= 'a' && c <= 'z' || c >= '0' && c <= '9' || c == '-') {
				return b, fmt.Errorf("invalid DNS hostname")
			}
		}
	}
	var e error
	b.Config, e = b.Config.WithDefaults()
	if e != nil {
		return b, e
	}
	b.Dir, e = filepath.Abs(b.Dir)
	if e != nil {
		return b, e
	}
	b.ChainFiles = append([]string(nil), b.ChainFiles...)
	b.TrustFiles = append([]string(nil), b.TrustFiles...)
	for _, paths := range [][]string{b.ChainFiles, b.TrustFiles} {
		for n, p := range paths {
			paths[n], e = filepath.Abs(p)
			if e != nil {
				return b, e
			}
		}
	}
	b.KeyFile, e = filepath.Abs(b.KeyFile)
	return b, e
}

func (b BindingConfig) material() (*credentials.Credentials, error) {
	if len(b.TrustFiles) != 1 {
		return nil, fmt.Errorf("one explicit QWAC trust authority file required")
	}
	anchors, e := credentials.ReadCertificates(b.TrustFiles[0])
	if e != nil {
		return nil, e
	}
	if len(anchors) != 1 {
		return nil, fmt.Errorf("one QWAC authority certificate required")
	}
	// Include a supplied authority in x5c if an ordinary bundle omitted it.
	var chainPaths = append([]string(nil), b.ChainFiles...)
	cs, e := credentials.ReadCertificates(chainPaths[len(chainPaths)-1])
	if e != nil {
		return nil, e
	}
	if !cs[len(cs)-1].Equal(anchors[0]) {
		chainPaths = append(chainPaths, b.TrustFiles[0])
	}
	mat, e := credentials.Load(chainPaths, b.KeyFile)
	if e != nil {
		return nil, e
	}
	if e = mat.ValidateFor(b.Host, anchors[0], time.Now()); e != nil {
		return nil, fmt.Errorf("supplied QWAC credentials: %w", e)
	}
	return mat, nil
}
