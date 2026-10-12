package agent

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net"
	"net/mail"
	"net/url"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/cztamas/qwacamole/credentials"
)

// Setup is intentionally fresh deployment only. Updates remain the existing
// replacement, migration and recovery operations, never an implicit overwrite.
type SetupOptions struct {
	Hostname, Application, Chain, Key, Authority, Email, ACMEServer string
	DryRun                                                          bool
}
type SetupFile struct {
	Path string
	Mode os.FileMode
	Data []byte
}
type SetupPlan struct {
	Config      Config
	Files       []SetupFile
	Directories map[string]os.FileMode
	Existing    bool
	Commands    []string
}

func setupConfig(o SetupOptions) Config {
	return Config{Version: 1, NginxWorkerUser: "www-data", Serving: ServingConfig{RedirectHTTPS: true}, Host: o.Hostname, Application: o.Application,
		StateDir: "/var/lib/qwacamole/" + o.Hostname, RuntimeDir: "/run/qwacamole",
		ChainFiles: []string{"/etc/qwacamole/chain.pem"}, KeyFile: "/etc/qwacamole/key.pem", TrustFile: "/etc/qwacamole/authority.pem",
		CertbotLineage: "/etc/letsencrypt/live/" + o.Hostname, PublicListen: "127.0.0.1:18081", HTTPListen: "0.0.0.0:80", HTTPSListen: "0.0.0.0:443", ProbeAddress: "127.0.0.1:443",
		Webroot: "/var/lib/qwacamole-acme", NginxBinary: "/usr/sbin/nginx", NginxPrefix: "/var/lib/nginx/qwacamole", NginxConfig: "/etc/qwacamole/nginx.conf"}
}
func setupRun(ctx context.Context, name string, args ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	out, e := exec.CommandContext(ctx, name, args...).CombinedOutput()
	if e != nil {
		return nil, fmt.Errorf("%s prerequisite: %w: %s", name, e, bytes.TrimSpace(out))
	}
	return out, nil
}
func setupPlatform() error {
	data, e := os.ReadFile("/etc/os-release")
	if e != nil || runtime.GOOS != "linux" || runtime.GOARCH != "amd64" || !strings.Contains("\n"+string(data)+"\n", "\nID=ubuntu\n") || !strings.Contains(string(data), `VERSION_ID="24.04"`) {
		return fmt.Errorf("setup supports Ubuntu 24.04 Linux amd64 only; use the advanced manual guide on other targets")
	}
	if os.Geteuid() != 0 {
		return fmt.Errorf("setup requires an administrator shell (sudo)")
	}
	return nil
}
func setupPrerequisites(ctx context.Context, c Config) error {
	if _, e := user.Lookup("www-data"); e != nil {
		return fmt.Errorf("required Nginx worker identity www-data: %w", e)
	}
	out, e := setupRun(ctx, c.NginxBinary, "-V")
	if e != nil {
		return e
	}
	for _, capability := range []string{"--with-http_ssl_module", "--with-http_auth_request_module"} {
		if !strings.Contains(string(out), capability) {
			return fmt.Errorf("Nginx needs %s", capability)
		}
	}
	if _, e = setupRun(ctx, "/usr/bin/certbot", "--version"); e != nil {
		return e
	}
	out, e = setupRun(ctx, "/usr/bin/certbot", "plugins")
	if e != nil {
		return e
	}
	if !strings.Contains(string(out), "webroot") {
		return fmt.Errorf("Certbot webroot plugin is required")
	}
	if _, e = setupRun(ctx, "systemctl", "--version"); e != nil {
		return e
	}
	return nil
}
func setupConflicts(ctx context.Context, c Config) error {
	out, e := setupRun(ctx, "ps", "-eo", "args")
	if e != nil {
		return e
	}
	for _, line := range strings.Split(string(out), "\n") {
		if strings.Contains(line, "nginx: master process") {
			return fmt.Errorf("existing Nginx master detected; dedicate this host before setup; no service was stopped")
		}
	}
	for _, addr := range []string{c.HTTPListen, c.HTTPSListen, c.PublicListen} {
		listener, e := net.Listen("tcp", addr)
		if e != nil {
			return fmt.Errorf("listener %s is unavailable: %w", addr, e)
		}
		listener.Close()
	}
	if _, e = os.Lstat(c.CertbotLineage); e == nil {
		return fmt.Errorf("existing Certbot lineage detected; use the manual integration/recovery guide")
	} else if !os.IsNotExist(e) {
		return e
	}
	return nil
}
func setupInput(path string, private bool) ([]byte, error) {
	info, e := os.Lstat(path)
	if e != nil {
		return nil, e
	}
	if !filepath.IsAbs(path) || !info.Mode().IsRegular() || info.Mode().Perm()&0022 != 0 || (private && info.Mode().Perm()&0077 != 0) {
		return nil, fmt.Errorf("input %s must be an absolute regular file, not writable by others; signing key must be private", path)
	}
	return os.ReadFile(path)
}

// ancestor checks prevent writing through links or directories controlled by a
// different identity. Setup-generated files are owned by root, never workers.
func setupOwnedPath(path string) error {
	for p := path; ; p = filepath.Dir(p) {
		info, e := os.Lstat(p)
		if e == nil {
			if info.Mode()&os.ModeSymlink != 0 || info.Mode().Perm()&0022 != 0 || !setupRootOwned(info) {
				return fmt.Errorf("unsafe ownership, permissions or symlink at %s", p)
			}
		} else if !os.IsNotExist(e) {
			return e
		}
		if p == "/" || p == "." {
			break
		}
	}
	return nil
}

func validateSetupMaterial(host string, chain, key, authority []byte) error {
	dir, e := os.MkdirTemp("", "qwacamole-setup-inputs-")
	if e != nil {
		return e
	}
	defer os.RemoveAll(dir)
	for name, data := range map[string][]byte{"chain.pem": chain, "key.pem": key, "authority.pem": authority} {
		if e = os.WriteFile(filepath.Join(dir, name), data, 0600); e != nil {
			return e
		}
	}
	_, e = credentials.LoadFor([]string{filepath.Join(dir, "chain.pem")}, filepath.Join(dir, "key.pem"), []string{filepath.Join(dir, "authority.pem")}, []string{host}, time.Now())
	return e
}

func buildSetupPlan(o SetupOptions, binary string) (SetupPlan, error) {
	c := setupConfig(o)
	p := SetupPlan{Config: c, Directories: map[string]os.FileMode{}}
	if e := c.Validate(); e != nil {
		return p, e
	}
	address, e := mail.ParseAddress(o.Email)
	if e != nil || address.Address != o.Email || strings.ContainsAny(o.Email, "\r\n'\"\\ ") {
		return p, fmt.Errorf("one ordinary ACME contact email is required")
	}
	if o.ACMEServer == "" {
		o.ACMEServer = "https://acme-v02.api.letsencrypt.org/directory"
	}
	server, e := url.Parse(o.ACMEServer)
	if e != nil || server.Scheme != "https" || server.Host == "" || server.User != nil || server.Fragment != "" || strings.ContainsAny(o.ACMEServer, "\r\n'\"\\ ") {
		return p, fmt.Errorf("explicit HTTPS ACME directory URL required")
	}
	if !filepath.IsAbs(binary) || strings.ContainsAny(binary, "\r\n'\"\\ %;$") {
		return p, fmt.Errorf("install the agent at a safe absolute executable path before setup")
	}
	chain, e := setupInput(o.Chain, false)
	if e != nil {
		return p, e
	}
	key, e := setupInput(o.Key, true)
	if e != nil {
		return p, e
	}
	authority, e := setupInput(o.Authority, false)
	if e != nil {
		return p, e
	}
	if e = validateSetupMaterial(o.Hostname, chain, key, authority); e != nil {
		return p, fmt.Errorf("credential preflight: %w", e)
	}
	config, _ := json.MarshalIndent(c, "", "  ")
	add := func(path string, mode os.FileMode, data string) {
		p.Files = append(p.Files, SetupFile{path, mode, []byte(data)})
	}
	add(c.ChainFiles[0], 0600, string(chain))
	add(c.KeyFile, 0600, string(key))
	add(c.TrustFile, 0600, string(authority))
	add("/etc/qwacamole/agent.json", 0600, string(config)+"\n")
	add(c.NginxConfig, 0600, RenderNginx(c))
	add("/etc/qwacamole/deploy-hook", 0700, "#!/bin/sh\nexec "+binary+" certbot-hook -config /etc/qwacamole/agent.json\n")
	add("/etc/qwacamole/certbot.ini", 0600, "email = "+o.Email+"\nserver = "+o.ACMEServer+"\n")
	add("/etc/systemd/system/qwacamole-nginx.service", 0644, `[Unit]
Description=Dedicated Qwacamole Nginx master
After=network.target
[Service]
Type=simple
User=root
UMask=0077
ExecStartPre=/usr/sbin/nginx -t -p /var/lib/nginx/qwacamole -c /etc/qwacamole/nginx.conf
ExecStart=/usr/sbin/nginx -p /var/lib/nginx/qwacamole -c /etc/qwacamole/nginx.conf -g 'daemon off;'
ExecReload=/usr/sbin/nginx -p /var/lib/nginx/qwacamole -c /etc/qwacamole/nginx.conf -s reload
Restart=on-failure
RestartSec=5
KillMode=control-group
TimeoutStopSec=120
[Install]
WantedBy=multi-user.target
`)
	add("/etc/systemd/system/qwacamole-agent.service", 0644, `[Unit]
Description=Qwacamole binding and deployment agent
Requires=qwacamole-nginx.service
After=network.target qwacamole-nginx.service
[Service]
Type=simple
User=root
RuntimeDirectory=qwacamole
RuntimeDirectoryMode=0700
UMask=0077
ExecStart=`+binary+` serve -config /etc/qwacamole/agent.json
Restart=on-failure
RestartSec=5
KillMode=control-group
TimeoutStopSec=120
[Install]
WantedBy=multi-user.target
`)
	add("/etc/systemd/system/qwacamole-renew.service", 0644, `[Unit]
Description=Renew the dedicated Qwacamole TLS certificate
Requires=qwacamole-agent.service
After=qwacamole-agent.service
[Service]
Type=oneshot
User=root
UMask=0077
ExecStart=/usr/bin/certbot renew --cert-name `+c.Host+` --no-directory-hooks
`)
	add("/etc/systemd/system/qwacamole-renew.timer", 0644, `[Unit]
Description=Qwacamole certificate renewal
[Timer]
OnCalendar=*-*-* 00,12:00:00
RandomizedDelaySec=3600
Persistent=true
Unit=qwacamole-renew.service
[Install]
WantedBy=timers.target
`)
	for _, entry := range []struct {
		path string
		mode os.FileMode
	}{{"/etc/qwacamole", 0700}, {"/var/lib/qwacamole", 0700}, {c.StateDir, 0700}, {c.RuntimeDir, 0700}, {c.NginxPrefix, 0755}, {filepath.Join(c.NginxPrefix, "client_body_temp"), 0700}, {c.Webroot, 0755}} {
		p.Directories[entry.path] = entry.mode
	}
	manifest := map[string]string{}
	for _, file := range p.Files {
		sum := sha256.Sum256(file.Data)
		manifest[file.Path] = hex.EncodeToString(sum[:])
	}
	data, _ := json.MarshalIndent(manifest, "", "  ")
	add("/etc/qwacamole/setup-manifest.json", 0600, string(data)+"\n")
	p.Commands = []string{"systemctl daemon-reload", "systemctl enable --now qwacamole-agent.service qwacamole-renew.timer", "certbot certonly --config /etc/qwacamole/certbot.ini --agree-tos --webroot -w " + c.Webroot + " -d " + c.Host + " --cert-name " + c.Host + " --deploy-hook /etc/qwacamole/deploy-hook --no-directory-hooks", binary + " verify -config /etc/qwacamole/agent.json"}
	return p, nil
}
func checkSetupFiles(p *SetupPlan) error {
	existing := 0
	for path, mode := range p.Directories {
		workerTemp := path == filepath.Join(p.Config.NginxPrefix, "client_body_temp")
		checkedPath := path
		if workerTemp {
			checkedPath = filepath.Dir(path)
		}
		if e := setupOwnedPath(checkedPath); e != nil {
			return e
		}
		if workerTemp {
			if info, e := os.Lstat(path); e == nil {
				u, err := user.Lookup("www-data")
				if err != nil {
					return err
				}
				uid, _ := strconv.Atoi(u.Uid)
				if !setupOwnedBy(info, uint32(uid)) || info.Mode()&os.ModeSymlink != 0 {
					return fmt.Errorf("unsafe Nginx worker temp ownership: %s", path)
				}
			} else if !os.IsNotExist(e) {
				return e
			}
		}
		if info, e := os.Lstat(path); e == nil && (!info.IsDir() || info.Mode().Perm() != mode) {
			return fmt.Errorf("existing directory %s has incompatible permissions; repair explicitly", path)
		} else if e != nil && !os.IsNotExist(e) {
			return e
		}
	}
	for _, file := range p.Files {
		if e := setupOwnedPath(file.Path); e != nil {
			return e
		}
		info, e := os.Lstat(file.Path)
		if os.IsNotExist(e) {
			continue
		}
		if e != nil {
			return e
		}
		if !info.Mode().IsRegular() || info.Mode().Perm() != file.Mode {
			return fmt.Errorf("existing file %s has incompatible type or permissions", file.Path)
		}
		data, e := os.ReadFile(file.Path)
		if e != nil {
			return e
		}
		if !bytes.Equal(data, file.Data) {
			return fmt.Errorf("existing file or credentials changed: %s; use replacement, maintenance or recovery; setup never overwrites", file.Path)
		}
		existing++
	}
	if existing != 0 && existing != len(p.Files) {
		return fmt.Errorf("incomplete prior setup detected; inspect and recover explicitly before retrying")
	}
	if existing == 0 {
		entries, e := os.ReadDir(p.Config.StateDir)
		if e != nil && !os.IsNotExist(e) {
			return e
		}
		if len(entries) > 0 {
			return fmt.Errorf("existing binding state requires explicit recovery, not fresh setup")
		}
	}
	p.Existing = existing == len(p.Files)
	return nil
}
func validateSetupUnits(ctx context.Context, p SetupPlan) error {
	dir, e := os.MkdirTemp("", "qwacamole-units-")
	if e != nil {
		return e
	}
	defer os.RemoveAll(dir)
	args := []string{"verify"}
	for _, f := range p.Files {
		if strings.HasPrefix(f.Path, "/etc/systemd/system/") {
			path := filepath.Join(dir, filepath.Base(f.Path))
			if e = os.WriteFile(path, f.Data, 0644); e != nil {
				return e
			}
			args = append(args, path)
		}
	}
	_, e = setupRun(ctx, "systemd-analyze", args...)
	return e
}

// Setup preflights the whole deployment before changes. It never starts/stops
// processes, enrolls credentials or replaces existing files. A write failure is
// rolled back; binding state is never removed or altered.
func Setup(ctx context.Context, o SetupOptions) (SetupPlan, error) {
	var p SetupPlan
	if e := setupPlatform(); e != nil {
		return p, e
	}
	binary, e := os.Executable()
	if e != nil {
		return p, e
	}
	if e = setupOwnedPath(binary); e != nil {
		return p, fmt.Errorf("install the agent in a root-owned path before setup: %w", e)
	}
	p, e = buildSetupPlan(o, binary)
	if e != nil {
		return p, e
	}
	if e = setupPrerequisites(ctx, p.Config); e != nil {
		return p, e
	}
	if e = checkSetupFiles(&p); e != nil {
		return p, e
	}
	if e = validateSetupUnits(ctx, p); e != nil {
		return p, e
	}
	if p.Existing {
		return p, nil
	}
	if e = setupConflicts(ctx, p.Config); e != nil {
		return p, e
	}
	if o.DryRun {
		return p, nil
	}
	lock, e := lockState("/run/lock/qwacamole-setup.lock")
	if e != nil {
		return p, e
	}
	defer lock.Close()
	if e = checkSetupFiles(&p); e != nil {
		return p, e
	}
	if p.Existing {
		return p, nil
	}
	if e = setupConflicts(ctx, p.Config); e != nil {
		return p, e
	}
	var createdFiles, createdDirs []string
	committed := false
	defer func() {
		if !committed {
			for i := len(createdFiles) - 1; i >= 0; i-- {
				os.Remove(createdFiles[i])
			}
			for i := len(createdDirs) - 1; i >= 0; i-- {
				os.Remove(createdDirs[i])
			}
		}
	}()
	dirs := make([]string, 0, len(p.Directories))
	for path := range p.Directories {
		dirs = append(dirs, path)
	}
	sort.Slice(dirs, func(i, j int) bool { return len(dirs[i]) < len(dirs[j]) })
	for _, path := range dirs {
		if _, e = os.Lstat(path); os.IsNotExist(e) {
			if e = os.MkdirAll(path, p.Directories[path]); e != nil {
				return p, e
			}
			createdDirs = append(createdDirs, path)
		}
	}
	for _, file := range p.Files {
		f, e := os.OpenFile(file.Path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, file.Mode)
		if e != nil {
			return p, e
		}
		createdFiles = append(createdFiles, file.Path)
		_, e = f.Write(file.Data)
		if e == nil {
			e = f.Sync()
		}
		closeErr := f.Close()
		if e != nil {
			return p, e
		}
		if closeErr != nil {
			return p, closeErr
		}
	}
	if e = (Nginx{Config: p.Config}).Validate(ctx); e != nil {
		return p, e
	}
	worker, e := user.Lookup("www-data")
	if e != nil {
		return p, e
	}
	uid, _ := strconv.Atoi(worker.Uid)
	gid, _ := strconv.Atoi(worker.Gid)
	if e = os.Chown(filepath.Join(p.Config.NginxPrefix, "client_body_temp"), uid, gid); e != nil {
		return p, e
	}
	committed = true
	return p, nil
}
