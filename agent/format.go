package agent

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
)

const StateVersion = 1
const NginxFormatVersion = 1

type stateFormat struct {
	State int `json:"state_version"`
	Nginx int `json:"nginx_format_version"`
}

func decodeStrict(data []byte, dst any) error {
	d := json.NewDecoder(bytes.NewReader(data))
	d.DisallowUnknownFields()
	if e := d.Decode(dst); e != nil {
		return e
	}
	if d.More() {
		return fmt.Errorf("trailing state data")
	}
	var extra any
	if e := d.Decode(&extra); e != io.EOF {
		return fmt.Errorf("trailing state data")
	}
	return nil
}
func readFormat(dir string) (stateFormat, error) {
	var f stateFormat
	d, e := os.ReadFile(filepath.Join(dir, "format.json"))
	if os.IsNotExist(e) {
		return f, nil
	}
	if e != nil {
		return f, e
	}
	if e = decodeStrict(d, &f); e != nil {
		return f, e
	}
	if f.State != StateVersion || f.Nginx != NginxFormatVersion {
		return f, fmt.Errorf("unsupported state_version %d / nginx_format_version %d; use compatible binary; state was not migrated", f.State, f.Nginx)
	}
	return f, nil
}

// CheckStateFormat is read-only and runs before acquiring/creating state files.
func CheckStateFormat(c Config) error {
	if _, e := os.Lstat(filepath.Join(c.StateDir, "migration.json")); e == nil {
		return fmt.Errorf("offline migration interrupted; fully stop Nginx and agent, then repeat migrate -full-stop-confirmed with the same config")
	} else if !os.IsNotExist(e) {
		return e
	}
	if e := checkPersistedVersions(c.StateDir); e != nil {
		return e
	}
	f, e := readFormat(c.StateDir)
	if e != nil {
		return e
	}
	_, configErr := os.Stat(filepath.Join(c.StateDir, "deployment-config.json"))
	if os.IsNotExist(configErr) && f.State != 0 {
		return fmt.Errorf("versioned state is missing deployment-config.json; restore a consistent backup")
	}
	if os.IsNotExist(configErr) {
		if _, e := os.Lstat(filepath.Join(c.StateDir, "deployment.json")); e == nil {
			return fmt.Errorf("deployment journal exists without ownership configuration; restore a consistent backup")
		}
		entries, e := os.ReadDir(filepath.Join(c.StateDir, "generations"))
		if e != nil && !os.IsNotExist(e) {
			return e
		}
		if len(entries) > 0 {
			return fmt.Errorf("generations exist without ownership configuration; restore a consistent backup")
		}
	}
	if configErr == nil && f.State == 0 {
		return fmt.Errorf("supported legacy unversioned state requires offline migrate -full-stop-confirmed; retain all generations and binding coverage")
	} else if configErr != nil && !os.IsNotExist(configErr) {
		return configErr
	}
	if configErr == nil {
		data, e := os.ReadFile(filepath.Join(c.StateDir, "deployment-config.json"))
		if e != nil {
			return e
		}
		var prior Config
		if e = decodeStrict(data, &prior); e != nil {
			return e
		}
		if !reflect.DeepEqual(canonicalConfig(prior), canonicalConfig(c)) {
			if compatibleConfiguration(prior, c) {
				return fmt.Errorf("supported serving settings changed; fully stop Nginx and agent and run migrate -full-stop-confirmed without reconstructing state")
			}
			return fmt.Errorf("incompatible ownership, endpoint, credential or binding-policy change; retain state and plan the change separately")
		}
	}
	return nil
}
func canonicalConfig(c Config) Config { c.Version = 1; return c }

func checkPersistedVersions(dir string) error {
	data, e := os.ReadFile(filepath.Join(dir, "deployment-config.json"))
	if e == nil {
		var config Config
		if e = decodeStrict(data, &config); e != nil {
			return e
		}
		if e = config.Validate(); e != nil {
			return e
		}
	} else if !os.IsNotExist(e) {
		return e
	}
	data, e = os.ReadFile(filepath.Join(dir, "deployment.json"))
	if e == nil {
		var journal Journal
		if e = decodeStrict(data, &journal); e != nil {
			return e
		}
		if e = validateJournal(journal); e != nil {
			return e
		}
		if journal.Version != 0 && journal.Version != StateVersion {
			return fmt.Errorf("unsupported journal state_version %d; use compatible binary without mutation", journal.Version)
		}
	} else if !os.IsNotExist(e) {
		return e
	}
	return nil
}

func compatibleConfiguration(old, next Config) bool {
	a, b := canonicalConfig(old), canonicalConfig(next)
	a.Serving, b.Serving = ServingConfig{}, ServingConfig{}
	return reflect.DeepEqual(a, b)
}
