package service

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"golang.org/x/net/proxy"
)

func TestSiteProbeTargetsValidateAndPersist(t *testing.T) {
	setupSettingTestDB(t)
	service := &SettingService{}
	initial, err := service.GetSiteProbes()
	if err != nil || len(initial.Targets) < 4 || !initial.Targets[0].Starred {
		t.Fatalf("missing built-in targets: %+v %v", initial.Targets, err)
	}
	for _, address := range []string{"file:///etc/hosts", "127.0.0.1", "http://[::ffff:127.0.0.1]", "https://169.254.169.254", "https://100.100.100.200", "https://user:pass@example.com", "https://example.com:22"} {
		if _, err := service.SaveSiteProbes([]SiteProbeTarget{{URL: address}}); err == nil {
			t.Errorf("accepted %s", address)
		}
	}
	if _, err := service.SaveSiteProbes([]SiteProbeTarget{{URL: "example.com"}, {URL: "https://example.com:443/"}}); err == nil {
		t.Fatal("duplicate URL accepted")
	}
	_, err = service.SaveSiteProbes([]SiteProbeTarget{{URL: "example.org", Starred: true}, {Name: "Example", URL: "http://example.com/path"}})
	if err != nil {
		t.Fatal(err)
	}
	got, err := (&SettingService{}).GetSiteProbes()
	if err != nil || len(got.Targets) != 2 || got.Targets[0].URL != "https://example.org/" ||
		!got.Targets[0].Starred || got.Targets[1].Name != "Example" {
		t.Fatalf("saved list or order not restored: %+v %v", got, err)
	}
	if _, err := service.SaveSiteProbes([]SiteProbeTarget{}); err != nil {
		t.Fatal(err)
	}
	got, err = service.GetSiteProbes()
	if err != nil || len(got.Targets) != 0 {
		t.Fatalf("empty list should not restore defaults: %+v %v", got, err)
	}
}

func TestSiteProbeHTTPDoesNotFollowRedirect(t *testing.T) {
	var redirects atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/next" {
			redirects.Add(1)
			w.WriteHeader(http.StatusOK)
			return
		}
		http.Redirect(w, r, "/next", http.StatusFound)
	}))
	defer srv.Close()
	got := probeHTTP(context.Background(), srv.URL, "127.0.0.1", (&net.Dialer{}).DialContext)
	if got.Status != "ok" || got.Code != http.StatusFound || got.Delay <= 0 || redirects.Load() != 0 {
		t.Fatalf("expected first HTTPS/HTTP response only: %+v, redirected %d", got, redirects.Load())
	}
}

// Run against the same standalone core shipped with the panel. A Host rule
// blocks the probe, the probe's no-match path is direct, and ordinary traffic
// still uses the configured default outbound.
func TestSiteProbeUsesLiveXrayRules(t *testing.T) {
	bin := os.Getenv("FARA_CORE_BINARY")
	if bin == "" {
		t.Skip("set FARA_CORE_BINARY to exercise a real Xray process")
	}
	echo := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusNoContent) }))
	defer echo.Close()
	_, targetPort, err := net.SplitHostPort(echo.Listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	free, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := free.Addr().(*net.TCPAddr).Port
	ordinary, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	ordinaryPort := ordinary.Addr().(*net.TCPAddr).Port
	_ = free.Close()
	_ = ordinary.Close()
	config := fmt.Sprintf(`{
		"log":{"loglevel":"debug"},
        "api":{"tag":"api","services":["HandlerService","StatsService"]},
		"inbounds":[{"tag":"api","listen":"127.0.0.1","port":%d,"protocol":"tunnel","settings":{"rewriteAddress":"127.0.0.1"}},
		{"tag":"ordinary","listen":"127.0.0.1","port":%d,"protocol":"socks","settings":{"auth":"noauth"}}],
		"outbounds":[{"tag":"default-deny","protocol":"blackhole"},{"tag":"blocked","protocol":"blackhole"}],
		"routing":{"rules":[{"type":"field","inboundTag":["api"],"outboundTag":"api"},
		{"type":"field","domain":["full:blocked.example"],"outboundTag":"blocked"}]}
	}`, port, ordinaryPort)
	path := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(path, []byte(config), 0600); err != nil {
		t.Fatal(err)
	}
	process := exec.Command(bin, "run", "-c", path)
	process.Stdout, process.Stderr = os.Stdout, os.Stderr
	if err := process.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = process.Process.Kill(); _ = process.Wait() })
	deadline := time.Now().Add(4 * time.Second)
	for {
		conn, err := net.Dial("tcp", fmt.Sprintf("127.0.0.1:%d", port))
		if err == nil {
			_ = conn.Close()
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("Xray API did not start")
		}
		time.Sleep(20 * time.Millisecond)
	}
	route, cleanup, err := openSiteProbeRouteAt(port)
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	check := func(host string, dial probeDialContext) SiteProbePath {
		return probeHTTP(ctx, "http://"+host+":"+targetPort+"/", "127.0.0.1", dial)
	}
	if got := check("healthy.example", route); got.Status != "ok" || got.Code != http.StatusNoContent {
		t.Fatalf("routed probe did not reach known healthy destination: %+v", got)
	}
	ordinaryRoute, err := proxy.SOCKS5("tcp", fmt.Sprintf("127.0.0.1:%d", ordinaryPort), nil, proxy.Direct)
	if err != nil {
		t.Fatal(err)
	}
	if got := check("healthy.example", ordinaryRoute.(proxy.ContextDialer).DialContext); got.Status == "ok" {
		t.Fatalf("ordinary user traffic bypassed the configured default: %+v", got)
	}
	if got := check("blocked.example", route); got.Status == "ok" {
		t.Fatalf("probe did not honor the live domain blocking rule: %+v", got)
	}
}
