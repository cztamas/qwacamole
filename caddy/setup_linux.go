package qwacamole

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"net"
	"net/mail"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"

	caddycmd "github.com/caddyserver/caddy/v2/cmd"
)

func init() {
	f := flag.NewFlagSet("qwac-setup", flag.ContinueOnError)
	for _, name := range []string{"hostname", "upstream", "chain", "key", "authority", "email", "acme-server"} {
		f.String(name, "", "explicit operator input")
	}
	f.Bool("dry-run", false, "validate and show intended files without installation")
	caddycmd.RegisterCommand(caddycmd.Command{Name: "qwac-setup", Short: "Configure a dedicated Ubuntu Caddy service with supplied credentials", Flags: f, Func: func(flags caddycmd.Flags) (int, error) {
		e := setupCaddy(flags)
		if e != nil {
			return 1, e
		}
		return 0, nil
	}})
}
func caddyOwned(path string) error {
	for p := path; ; p = filepath.Dir(p) {
		info, e := os.Lstat(p)
		if e == nil {
			stat, ok := info.Sys().(*syscall.Stat_t)
			if !ok || stat.Uid != 0 || info.Mode()&os.ModeSymlink != 0 || info.Mode().Perm()&0022 != 0 {
				return fmt.Errorf("unsafe ownership or permissions at %s", p)
			}
		} else if !os.IsNotExist(e) {
			return e
		}
		if p == "/" {
			break
		}
	}
	return nil
}
func caddySetupCommand(name string, args []string, env []string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Env = env
	out, e := cmd.CombinedOutput()
	if e != nil {
		return fmt.Errorf("%s: %w: %s", name, e, out)
	}
	return nil
}
func setupCaddy(f caddycmd.Flags) error {
	osrelease, e := os.ReadFile("/etc/os-release")
	if e != nil || runtime.GOARCH != "amd64" || !strings.Contains("\n"+string(osrelease), "\nID=ubuntu\n") || !strings.Contains(string(osrelease), `VERSION_ID="24.04"`) || os.Geteuid() != 0 {
		return fmt.Errorf("qwac-setup requires sudo on Ubuntu 24.04 Linux amd64; other deployments use the reference")
	}
	binary, e := os.Executable()
	if e != nil {
		return e
	}
	if e = caddyOwned(binary); e != nil {
		return e
	}
	if strings.ContainsAny(binary, " \r\n\"'%;$") {
		return fmt.Errorf("install at a safe absolute executable path")
	}
	host, upstream, email := f.String("hostname"), f.String("upstream"), f.String("email")
	b := BindingConfig{ChainFiles: []string{f.String("chain")}, KeyFile: f.String("key"), TrustFiles: []string{f.String("authority")}}
	b.Host = host
	b.Dir = "/var/lib/caddy-qwac/bindings"
	if _, e = b.normalized(); e != nil {
		return e
	}
	if _, e = b.material(); e != nil {
		return fmt.Errorf("credential preflight: %w", e)
	}
	app, e := url.Parse(upstream)
	if e != nil || app.Scheme != "http" || app.Host == "" || app.User != nil || app.RawQuery != "" || app.Fragment != "" || app.Path != "" || strings.ContainsAny(upstream, "\"\\\r\n ") {
		return fmt.Errorf("one HTTP upstream origin required")
	}
	contact, e := mail.ParseAddress(email)
	if e != nil || contact.Address != email || strings.ContainsAny(email, "\"\\\r\n ") {
		return fmt.Errorf("one ACME contact email required")
	}
	acmeOptions := "email " + email
	if server := f.String("acme-server"); server != "" {
		u, err := url.Parse(server)
		if err != nil || u.Scheme != "https" || u.Host == "" || u.User != nil || u.Fragment != "" || strings.ContainsAny(server, "\"\\\r\n ") {
			return fmt.Errorf("HTTPS ACME directory URL required")
		}
		acmeOptions += "\n            dir " + server
	}
	files := map[string][]byte{}
	for source, destination := range map[string]string{b.ChainFiles[0]: "/etc/caddy-qwac/chain.pem", b.KeyFile: "/etc/caddy-qwac/key.pem", b.TrustFiles[0]: "/etc/caddy-qwac/authority.pem"} {
		info, e := os.Lstat(source)
		if e != nil {
			return e
		}
		if !filepath.IsAbs(source) || !info.Mode().IsRegular() || info.Mode().Perm()&0022 != 0 || (source == b.KeyFile && info.Mode().Perm()&0077 != 0) {
			return fmt.Errorf("supplied files must be absolute regular files and private keys must be private: %s", source)
		}
		data, e := os.ReadFile(source)
		if e != nil {
			return e
		}
		files[destination] = data
	}
	render := func(chain, key, authority string) string {
		return fmt.Sprintf(`%s {
    qwac {
        hostname %s
        chain_files %s
        key_file %s
        trust_files %s
        acme {
            %s
        }
    }
    reverse_proxy %s
}
`, host, host, strconv.Quote(chain), strconv.Quote(key), strconv.Quote(authority), acmeOptions, upstream)
	}
	files["/etc/caddy-qwac/Caddyfile"] = []byte(render("/etc/caddy-qwac/chain.pem", "/etc/caddy-qwac/key.pem", "/etc/caddy-qwac/authority.pem"))
	files["/etc/systemd/system/qwacamole-caddy.service"] = []byte(`[Unit]
Description=Dedicated Caddy with Qwacamole
After=network-online.target
Wants=network-online.target
[Service]
Type=simple
User=root
Environment=XDG_DATA_HOME=/var/lib/caddy-qwac
UMask=0077
ExecStart=` + binary + ` run --config /etc/caddy-qwac/Caddyfile --adapter caddyfile
ExecReload=` + binary + ` reload --config /etc/caddy-qwac/Caddyfile --adapter caddyfile --force
Restart=on-failure
RestartSec=5
KillMode=control-group
TimeoutStopSec=120
[Install]
WantedBy=multi-user.target
`)
	dirs := []string{"/etc/caddy-qwac", "/var/lib/caddy-qwac"}
	existing := 0
	for _, path := range dirs {
		if e = caddyOwned(path); e != nil {
			return e
		}
		if info, err := os.Lstat(path); err == nil && (!info.IsDir() || info.Mode().Perm() != 0700) {
			return fmt.Errorf("incompatible directory %s", path)
		} else if err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	for path, data := range files {
		if e = caddyOwned(path); e != nil {
			return e
		}
		info, err := os.Lstat(path)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return err
		}
		mode := os.FileMode(0600)
		if strings.HasSuffix(path, ".service") {
			mode = 0644
		}
		current, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() || info.Mode().Perm() != mode || !bytes.Equal(current, data) {
			return fmt.Errorf("existing file or credentials changed: %s; use explicit maintenance/recovery, never setup overwrite", path)
		}
		existing++
	}
	if existing != 0 && existing != len(files) {
		return fmt.Errorf("incomplete prior setup; inspect and recover explicitly")
	}
	if existing == len(files) {
		fmt.Println("Setup already matches; files, credentials and binding state preserved.")
		return nil
	}
	if entries, err := os.ReadDir("/var/lib/caddy-qwac"); err != nil && !os.IsNotExist(err) {
		return err
	} else if len(entries) > 0 {
		return fmt.Errorf("existing Caddy state requires explicit recovery")
	}
	for _, addr := range []string{"0.0.0.0:80", "0.0.0.0:443", "127.0.0.1:2019"} {
		l, err := net.Listen("tcp", addr)
		if err != nil {
			return fmt.Errorf("listener %s is unavailable; no existing service was stopped: %w", addr, err)
		}
		l.Close()
	}
	scratch, e := os.MkdirTemp("", "qwac-caddy-setup-")
	if e != nil {
		return e
	}
	defer os.RemoveAll(scratch)
	for _, name := range []string{"chain.pem", "key.pem", "authority.pem"} {
		if e = os.WriteFile(filepath.Join(scratch, name), files["/etc/caddy-qwac/"+name], 0600); e != nil {
			return e
		}
	}
	tempconfig := filepath.Join(scratch, "Caddyfile")
	if e = os.WriteFile(tempconfig, []byte(render(filepath.Join(scratch, "chain.pem"), filepath.Join(scratch, "key.pem"), filepath.Join(scratch, "authority.pem"))), 0600); e != nil {
		return e
	}
	env := append(os.Environ(), "XDG_DATA_HOME="+scratch)
	if e = caddySetupCommand(binary, []string{"validate", "--config", tempconfig, "--adapter", "caddyfile"}, env); e != nil {
		return e
	}
	unit := filepath.Join(scratch, "qwacamole-caddy.service")
	if e = os.WriteFile(unit, files["/etc/systemd/system/qwacamole-caddy.service"], 0644); e != nil {
		return e
	}
	if e = caddySetupCommand("systemd-analyze", []string{"verify", unit}, os.Environ()); e != nil {
		return e
	}
	if f.Bool("dry-run") {
		fmt.Println("Dry run: private configuration/state directories, three supplied inputs, Caddyfile and qwacamole-caddy.service; no deployment files changed.")
		return nil
	}
	lockPath := "/run/qwacamole-caddy-setup.lock"
	if e = caddyOwned(lockPath); e != nil {
		return e
	}
	lock, e := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0600)
	if e != nil {
		return e
	}
	defer lock.Close()
	if e = syscall.Flock(int(lock.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); e != nil {
		return fmt.Errorf("another setup is running: %w", e)
	}
	// Exclusive creation refuses a competing or operator-created destination.
	var createdFiles, createdDirs []string
	complete := false
	defer func() {
		if !complete {
			for _, p := range createdFiles {
				os.Remove(p)
			}
			for i := len(createdDirs) - 1; i >= 0; i-- {
				os.Remove(createdDirs[i])
			}
		}
	}()
	for _, path := range dirs {
		if _, err := os.Lstat(path); os.IsNotExist(err) {
			if e = os.Mkdir(path, 0700); e != nil {
				return e
			}
			createdDirs = append(createdDirs, path)
			if e = os.Chmod(path, 0700); e != nil {
				return e
			}
		}
	}
	for path, data := range files {
		mode := os.FileMode(0600)
		if strings.HasSuffix(path, ".service") {
			mode = 0644
		}
		file, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, mode)
		if err != nil {
			return err
		}
		createdFiles = append(createdFiles, path)
		_, e = file.Write(data)
		if e == nil {
			e = file.Chmod(mode)
		}
		if e == nil {
			e = file.Sync()
		}
		closeErr := file.Close()
		if e != nil {
			return e
		}
		if closeErr != nil {
			return closeErr
		}
	}
	for _, path := range dirs {
		info, err := os.Lstat(path)
		if err != nil {
			return err
		}
		if !info.IsDir() || info.Mode().Perm() != 0700 {
			return fmt.Errorf("setup directory %s does not have mode 0700", path)
		}
	}
	for path := range files {
		mode := os.FileMode(0600)
		if strings.HasSuffix(path, ".service") {
			mode = 0644
		}
		info, err := os.Lstat(path)
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() || info.Mode().Perm() != mode {
			return fmt.Errorf("setup file %s does not have mode %04o", path, mode)
		}
	}
	complete = true
	fmt.Println("Configured dedicated Caddy. Services have not been started.")
	fmt.Printf("sudo systemctl daemon-reload\nsudo systemctl enable --now qwacamole-caddy.service\nsudo %s qwac-check --hostname %s --authority /etc/caddy-qwac/authority.pem\n", binary, host)
	return nil
}
