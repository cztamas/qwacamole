package main

import (
	"encoding/json"
	"fmt"
	"github.com/cztamas/qwacamole/binding"
	"github.com/cztamas/qwacamole/store"
	"os"
	"path/filepath"
	"reflect"
	"time"
)

func refreshTest(dir string) error {
	read := func() (store.State, error) {
		var s store.State
		d, e := os.ReadFile(filepath.Join(dir, "bindings/current.json"))
		if e == nil {
			e = json.Unmarshal(d, &s)
		}
		return s, e
	}
	before, e := read()
	if e != nil {
		return e
	}
	beforeDoc, e := os.ReadFile(filepath.Join(dir, "bindings", filepath.Base(before.URL)))
	if e != nil {
		return e
	}
	start := time.Now()
	for time.Since(start) < 90*time.Second {
		time.Sleep(time.Second)
		after, e := read()
		if e != nil {
			return e
		}
		if !reflect.DeepEqual(after.IDs, before.IDs) {
			before = after
			beforeDoc, e = os.ReadFile(filepath.Join(dir, "bindings", filepath.Base(before.URL)))
			if e != nil {
				return e
			}
			continue
		}
		afterDoc, e := os.ReadFile(filepath.Join(dir, "bindings", filepath.Base(after.URL)))
		if e != nil {
			return e
		}
		if string(afterDoc) == string(beforeDoc) || after.Published <= before.Published || after.Expires <= before.Expires {
			continue
		}
		beforeHeader, _, _, _, e := binding.Decode(string(beforeDoc))
		if e != nil {
			return e
		}
		afterHeader, _, _, _, e := binding.Decode(string(afterDoc))
		if e != nil {
			return e
		}
		if before.DiscoveryURL != store.DiscoveryURL || after.DiscoveryURL != before.DiscoveryURL || afterHeader.Iat <= beforeHeader.Iat || afterHeader.Exp <= beforeHeader.Exp {
			return fmt.Errorf("refresh did not preserve stable discovery and advance signed validity")
		}
		report := map[string]any{"same_tls_ids": after.IDs, "discovery_url": after.DiscoveryURL, "before_document": before.URL, "after_document": after.URL, "before_iat": beforeHeader.Iat, "after_iat": afterHeader.Iat, "before_exp": beforeHeader.Exp, "after_exp": afterHeader.Exp, "elapsed_seconds": time.Since(start).Seconds(), "independent_refresh": true}
		fmt.Println("PASS binding refreshed without TLS renewal")
		return writeJSON(filepath.Join(dir, "evidence/refresh-test.json"), report)
	}
	return fmt.Errorf("independent refresh not observed")
}
