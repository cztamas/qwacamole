package browser

import (
	"context"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/cdproto/target"
	"github.com/chromedp/chromedp"
	"io"
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
	Console                               io.Writer
	ThroughTLSExpiry                      bool
}
type Result struct {
	Version         string       `json:"version"`
	Flags           []string     `json:"flags"`
	NavigationError string       `json:"navigation_error,omitempty"`
	Events          []Event      `json:"events"`
	ExpiryCheck     *ExpiryCheck `json:"expiry_check,omitempty"`
}
type ExpiryCheck struct {
	InitialTLSID    string    `json:"initial_tls_id"`
	FinalTLSID      string    `json:"final_tls_id"`
	InitialNotAfter time.Time `json:"initial_not_after"`
	CheckedAt       time.Time `json:"checked_at"`
}
type Event struct {
	Type   string         `json:"type"`
	Phase  int            `json:"phase"`
	Source int            `json:"source"`
	Params map[string]any `json:"params"`
}

func Run(o Options) (Result, error) {
	return run(context.Background(), o, false)
}

// Open keeps a visible, owned browser running until its windows are closed or
// stop is cancelled. Trust preparation is identical to the headless checks.
func Open(stop context.Context, o Options) (Result, error) {
	return run(stop, o, true)
}

func run(stop context.Context, o Options, visible bool) (Result, error) {
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
	if visible {
		flags = append(flags, chromedp.Flag("headless", false), chromedp.Flag("lang", "en-US"), chromedp.WindowSize(1100, 850))
	}
	result.Flags = []string{"--enable-features=VerifyQWACs", "--component-updater=url-source=http://127.0.0.1:9", "--host-resolver-rules=MAP qwac.example.com 127.0.0.1", "--net-log-capture-mode=Everything"}
	if o.CTException {
		flags = append(flags, chromedp.Flag("disable-features", "CertificateTransparencyAskBeforeEnabling"))
		result.Flags = append(result.Flags, "--disable-features=CertificateTransparencyAskBeforeEnabling")
	}
	console := o.Console
	if console == nil {
		console = os.Stderr
	}
	flags = append(flags, chromedp.CombinedOutput(console))
	alloc, cancelAlloc := chromedp.NewExecAllocator(context.Background(), flags...)
	defer cancelAlloc()
	ctx, cancel := chromedp.NewContext(alloc)
	defer cancel()
	// Allocate using a context without a deadline: Chrome's process inherits
	// the first Run context, so a startup deadline must not kill the visible
	// browser 45 seconds later while the user is inspecting its Security panel.
	timeout := ctx
	startupTimer := time.AfterFunc(45*time.Second, cancel)
	defer startupTimer.Stop()
	stopStartup := context.AfterFunc(stop, cancel)
	defer stopStartup()
	if e = chromedp.Run(timeout, network.Enable(), network.SetCacheDisabled(true), chromedp.Navigate("about:blank")); e != nil {
		return result, e
	}
	// ComponentReady is asynchronous. Allow its root-store update before any
	// TLS warm-up: an early verification failure can otherwise stay cached even
	// after ComponentReady. NetLog still must prove the injected root version.
	time.Sleep(3 * time.Second)
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
	startupTimer.Stop()
	stopStartup()
	var checkErr error
	if o.ThroughTLSExpiry && result.NavigationError == "" {
		result.ExpiryCheck, checkErr = checkThroughExpiry(ctx, stop, o.URL)
	}
	if visible && result.NavigationError == "" {
		fmt.Println("Chrome is open. Click the site controls icon beside the address, then Connection is secure.")
		fmt.Println("Its native Security panel should show EU Qualified Status and the signed owner [country].")
		fmt.Println("Close this demo window or press Ctrl-C here to stop the browser and local services.")
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()
	wait:
		for {
			select {
			case <-stop.Done():
				break wait
			case <-ctx.Done():
				break wait
			case <-ticker.C:
				pages, err := target.GetTargets().Do(cdp.WithExecutor(ctx, chromedp.FromContext(ctx).Browser))
				if err != nil {
					break wait
				}
				found := false
				for _, page := range pages {
					found = found || page.Type == "page"
				}
				if !found {
					break wait
				}
			}
		}
	}
	closeCtx, closeCancel := context.WithTimeout(ctx, 10*time.Second)
	defer closeCancel()
	if ctx.Err() == nil {
		if err := chromedp.Cancel(closeCtx); err != nil {
			return result, fmt.Errorf("graceful browser shutdown: %w", err)
		}
	} else {
		// A user-initiated browser quit has already disconnected CDP. Wait for
		// the owned process before reading its flushed NetLog.
		cancelAlloc()
	}
	result.Events, e = ReadNetLog(abs)
	if e == nil {
		e = checkErr
	}
	return result, e
}

// Keep navigating in the same browser so its existing HTTP/2 connection remains
// active. A fresh browser alone cannot expose lost coverage of an expired leaf.
func checkThroughExpiry(ctx, stop context.Context, url string) (*ExpiryCheck, error) {
	certificate := func() (*x509.Certificate, error) {
		chain, err := network.GetCertificate(url).Do(cdp.WithExecutor(ctx, chromedp.FromContext(ctx).Target))
		if err != nil {
			return nil, err
		}
		if len(chain) == 0 {
			return nil, fmt.Errorf("browser returned no TLS certificate")
		}
		der, err := base64.StdEncoding.DecodeString(chain[0])
		if err != nil {
			return nil, err
		}
		return x509.ParseCertificate(der)
	}
	first, err := certificate()
	if err != nil {
		return nil, err
	}
	id := func(c *x509.Certificate) string { return fmt.Sprintf("%x", sha256.Sum256(c.Raw)) }
	r := &ExpiryCheck{InitialTLSID: id(first), InitialNotAfter: first.NotAfter}
	// Wait through a complete 45-second binding lifetime after leaf expiry, so
	// independent refresh has necessarily run. Otherwise an old, cached binding
	// could hide pruning that only happens on the next publication.
	end := first.NotAfter.Add(45 * time.Second)
	if time.Until(end) <= 0 || time.Until(end) > 10*time.Minute {
		return r, fmt.Errorf("expiry check requires a live TLS certificate expiring within ten minutes")
	}
	fmt.Printf("Keeping Chrome's TLS connection active through %s\n", first.NotAfter.Format(time.RFC3339))
	for time.Now().Before(end) {
		delay := min(20*time.Second, time.Until(end))
		timer := time.NewTimer(delay)
		select {
		case <-stop.Done():
			timer.Stop()
			return r, stop.Err()
		case <-ctx.Done():
			timer.Stop()
			return r, ctx.Err()
		case <-timer.C:
		}
		navigation, cancel := context.WithTimeout(ctx, 15*time.Second)
		err := chromedp.Run(navigation, chromedp.Navigate(url), chromedp.Sleep(2*time.Second))
		cancel()
		if err != nil {
			return r, err
		}
	}
	last, err := certificate()
	if err != nil {
		return r, err
	}
	r.FinalTLSID, r.CheckedAt = id(last), time.Now()
	if r.FinalTLSID != r.InitialTLSID {
		return r, fmt.Errorf("browser reconnected; expired connection coverage was not tested")
	}
	return r, nil
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
