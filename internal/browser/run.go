package browser

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/chromedp"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

type Options struct {
	Binary, Profile, NetLog, URL, InitURL string
	CTException                           bool
	Screenshot                            bool
}
type Result struct {
	Version         string   `json:"version"`
	Flags           []string `json:"flags"`
	NavigationError string   `json:"navigation_error,omitempty"`
	Events          []Event  `json:"events"`
}
type Event struct {
	Type   string         `json:"type"`
	Phase  int            `json:"phase"`
	Source int            `json:"source"`
	Params map[string]any `json:"params"`
}

func Run(o Options) (Result, error) {
	var result Result
	b, e := exec.Command(o.Binary, "--version").Output()
	if e != nil {
		return result, e
	}
	result.Version = string(b)
	o.Profile, e = filepath.Abs(o.Profile)
	if e != nil {
		return result, e
	}
	abs, e := filepath.Abs(o.NetLog)
	if e != nil {
		return result, e
	}
	flags := []chromedp.ExecAllocatorOption{chromedp.ExecPath(o.Binary), chromedp.UserDataDir(o.Profile), chromedp.Flag("headless", "new"), chromedp.Flag("no-first-run", true), chromedp.Flag("no-default-browser-check", true), chromedp.Flag("component-updater", "url-source=http://127.0.0.1:9"), chromedp.Flag("enable-logging", "stderr"), chromedp.Flag("vmodule", "component_installer=1,pki_metadata_component_installer=1"), chromedp.Flag("enable-features", "VerifyQWACs"), chromedp.Flag("log-net-log", abs), chromedp.Flag("net-log-capture-mode", "Everything"), chromedp.Flag("host-resolver-rules", "MAP qwac.example.com 127.0.0.1"), chromedp.Flag("no-proxy-server", true)}
	result.Flags = []string{"--enable-features=VerifyQWACs", "--component-updater=url-source=http://127.0.0.1:9", "--host-resolver-rules=MAP qwac.example.com 127.0.0.1", "--net-log-capture-mode=Everything"}
	if o.CTException {
		flags = append(flags, chromedp.Flag("disable-features", "CertificateTransparencyAskBeforeEnabling"))
		result.Flags = append(result.Flags, "--disable-features=CertificateTransparencyAskBeforeEnabling")
	}
	flags = append(flags, chromedp.CombinedOutput(os.Stderr))
	alloc, cancelAlloc := chromedp.NewExecAllocator(context.Background(), flags...)
	defer cancelAlloc()
	ctx, cancel := chromedp.NewContext(alloc)
	defer cancel()
	timeout, cancelTimeout := context.WithTimeout(ctx, 45*time.Second)
	defer cancelTimeout()
	if e = chromedp.Run(timeout, network.Enable(), network.SetCacheDisabled(true), chromedp.Navigate("about:blank")); e != nil {
		return result, e
	}
	// ComponentReady is asynchronous. Warm up a TLS-only page, then wait before
	// the fresh main-frame navigation. Root version acceptance is asserted in NetLog.
	if o.InitURL != "" {
		deadline := time.Now().Add(15 * time.Second)
		for {
			e = chromedp.Run(timeout, chromedp.Navigate(o.InitURL))
			if e == nil {
				break
			}
			if time.Now().After(deadline) {
				break
			}
			time.Sleep(time.Second)
		}
	} else {
		time.Sleep(3 * time.Second)
	}
	e = chromedp.Run(timeout, chromedp.Navigate(o.URL))
	if e != nil {
		result.NavigationError = e.Error()
	}
	time.Sleep(2 * time.Second)
	if o.Screenshot && result.NavigationError == "" {
		var pixels []byte
		if err := chromedp.Run(timeout, chromedp.FullScreenshot(&pixels, 100)); err != nil {
			return result, err
		}
		if err := os.WriteFile(strings.TrimSuffix(abs, "-netlog.json")+"-page.png", pixels, 0600); err != nil {
			return result, err
		}
	}
	if err := chromedp.Cancel(timeout); err != nil {
		return result, fmt.Errorf("graceful browser shutdown: %w", err)
	}
	result.Events, e = ReadNetLog(abs)
	return result, e
}
func ReadNetLog(path string) ([]Event, error) {
	d, e := os.ReadFile(path)
	if e != nil {
		return nil, e
	}
	var raw struct {
		Constants struct {
			Types map[string]int `json:"logEventTypes"`
		} `json:"constants"`
		Events []struct {
			Type   int `json:"type"`
			Phase  int `json:"phase"`
			Source struct {
				ID int `json:"id"`
			} `json:"source"`
			Params map[string]any `json:"params"`
		} `json:"events"`
	}
	if e = json.Unmarshal(d, &raw); e != nil {
		return nil, fmt.Errorf("NetLog decode: %w", e)
	}
	names := map[int]string{}
	for n, i := range raw.Constants.Types {
		names[i] = n
	}
	var events []Event
	for _, ev := range raw.Events {
		n := names[ev.Type]
		if n == "CERT_VERIFY_PROC_2QWAC_BINDING" || n == "CERT_VERIFY_PROC_2QWAC" || n == "CERT_VERIFY_PROC_CHROME_ROOT_STORE_VERSION" || n == "CERT_VERIFIER_JOB" {
			events = append(events, Event{n, ev.Phase, ev.Source.ID, ev.Params})
		}
	}
	return events, nil
}
