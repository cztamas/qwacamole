// Package browser prepares only isolated profiles and inspects native NetLog.
package browser

import (
	"crypto/x509"
	"encoding/json"
	"fmt"
	"google.golang.org/protobuf/encoding/protowire"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
)

const RootVersion = 1000000

func fieldBytes(n protowire.Number, v []byte) []byte {
	return protowire.AppendBytes(protowire.AppendTag(nil, n, protowire.BytesType), v)
}
func fieldInt(n protowire.Number, v uint64) []byte {
	return protowire.AppendVarint(protowire.AppendTag(nil, n, protowire.VarintType), v)
}

// Replace only requested fields, copying all other encoded bytes unchanged.
func Replace(src []byte, replacements map[protowire.Number][]byte) ([]byte, error) {
	var dst []byte
	for len(src) > 0 {
		n, t, k := protowire.ConsumeTag(src)
		if k < 0 {
			return nil, protowire.ParseError(k)
		}
		v := protowire.ConsumeFieldValue(n, t, src[k:])
		if v < 0 {
			return nil, protowire.ParseError(v)
		}
		if _, ok := replacements[n]; !ok {
			dst = append(dst, src[:k+v]...)
		}
		src = src[k+v:]
	}
	keys := make([]int, 0, len(replacements))
	for n := range replacements {
		keys = append(keys, int(n))
	}
	sort.Ints(keys)
	for _, n := range keys {
		dst = append(dst, replacements[protowire.Number(n)]...)
	}
	return dst, nil
}
func Authority(cert *x509.Certificate, tls, qwac bool) []byte {
	a := fieldBytes(1, cert.Raw)
	a = append(a, fieldInt(6, boolInt(qwac))...)
	a = append(a, fieldInt(10, boolInt(tls))...)
	return fieldBytes(3, a)
}
func boolInt(v bool) uint64 {
	if v {
		return 1
	}
	return 0
}
func Detect() (string, string, error) {
	home, e := os.UserHomeDir()
	if e != nil {
		return "", "", e
	}
	for _, name := range []string{"Google Chrome", "Google Chrome Canary", "Chromium"} {
		bin := "/Applications/" + name + ".app/Contents/MacOS/" + name
		if _, e = os.Stat(bin); e == nil {
			base := filepath.Join(home, "Library/Application Support/Google", name[7:], "PKIMetadata")
			if name == "Chromium" {
				base = filepath.Join(home, "Library/Application Support/Chromium/PKIMetadata")
			}
			paths, _ := filepath.Glob(filepath.Join(base, "*", "manifest.json"))
			sort.Strings(paths)
			if len(paths) > 0 {
				return bin, filepath.Dir(paths[len(paths)-1]), nil
			}
		}
	}
	return "", "", fmt.Errorf("supply --browser and --metadata paths to a stock compatible binary and genuine component")
}
func Prepare(dir, component string, tlsRoot, qwacRoot *x509.Certificate, tlsTrust, qwacTrust bool) error {
	if _, e := os.Stat(dir); e == nil {
		return fmt.Errorf("profile already exists: %s", dir)
	}
	if e := os.MkdirAll(dir, 0700); e != nil {
		return e
	}
	manifest, e := os.ReadFile(filepath.Join(component, "manifest.json"))
	if e != nil {
		return e
	}
	var m struct {
		Version string `json:"version"`
	}
	if e = json.Unmarshal(manifest, &m); e != nil {
		return e
	}
	if m.Version == "" {
		return fmt.Errorf("genuine component version missing")
	}
	dst := filepath.Join(dir, "PKIMetadata", m.Version)
	e = filepath.WalkDir(component, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(component, path)
		if err != nil {
			return err
		}
		out := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(out, 0700)
		}
		if d.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("component symlink rejected")
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(out, b, 0600)
	})
	if e != nil {
		return e
	}
	path := filepath.Join(dst, "crs.pb")
	src, e := os.ReadFile(path)
	if e != nil {
		return e
	}
	src, e = Replace(src, map[protowire.Number][]byte{2: fieldInt(2, RootVersion)})
	if e != nil {
		return e
	}
	if tlsTrust {
		src = append(src, Authority(tlsRoot, true, false)...)
	}
	if qwacTrust {
		src = append(src, Authority(qwacRoot, false, true)...)
	}
	if e = os.WriteFile(path, src, 0600); e != nil {
		return e
	}
	return os.WriteFile(filepath.Join(dir, "First Run"), nil, 0600)
}
