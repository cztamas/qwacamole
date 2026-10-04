package main

import (
	"encoding/json"
	"fmt"
	"github.com/cztamas/qwacamole/internal/store"
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
	start := time.Now()
	for time.Since(start) < 90*time.Second {
		time.Sleep(time.Second)
		after, e := read()
		if e != nil {
			return e
		}
		if after.URL == before.URL {
			continue
		}
		if !reflect.DeepEqual(after.IDs, before.IDs) {
			before = after
			continue
		}
		report := map[string]any{"same_tls_ids": after.IDs, "before_binding": before.URL, "after_binding": after.URL, "elapsed_seconds": time.Since(start).Seconds(), "independent_refresh": true}
		fmt.Println("PASS binding refreshed without TLS renewal")
		return writeJSON(filepath.Join(dir, "evidence/refresh-test.json"), report)
	}
	return fmt.Errorf("independent refresh not observed")
}
