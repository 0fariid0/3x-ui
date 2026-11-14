package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/logger"
	"github.com/mhsanaei/3x-ui/v3/internal/util/netsafe"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"
	"golang.org/x/net/proxy"
)

const siteProbeKey = "dashboardSiteProbes"
const maxSiteProbes = 12

type SiteProbeTarget struct {
	URL     string `json:"url"`
	Name    string `json:"name"`
	Starred bool   `json:"starred"`
}

type SiteProbePath struct {
	Status string `json:"status"`
	Code   int    `json:"code,omitempty"`
	Delay  int64  `json:"delay,omitempty"`
}

type SiteProbeResult struct {
	URL    string        `json:"url"`
	At     int64         `json:"at"`
	Direct SiteProbePath `json:"direct"`
	Routed SiteProbePath `json:"routed"`
}

type SiteProbeView struct {
	Targets []SiteProbeTarget `json:"targets"`
	Results []SiteProbeResult `json:"results"`
	Running bool              `json:"running"`
}

func defaultSiteProbes() []SiteProbeTarget {
	return []SiteProbeTarget{
		{Name: "Google", URL: "https://www.google.com/generate_204", Starred: true},
		{Name: "Gemini", URL: "https://gemini.google.com/", Starred: true},
		{Name: "ChatGPT", URL: "https://chatgpt.com/", Starred: true},
		{Name: "YouTube", URL: "https://www.youtube.com/", Starred: true},
		{Name: "GitHub", URL: "https://github.com/"},
		{Name: "Instagram", URL: "https://www.instagram.com/"},
		{Name: "Telegram", URL: "https://telegram.org/"},
		{Name: "Wikipedia", URL: "https://www.wikipedia.org/"},
	}
}

var siteProbes struct {
	sync.Mutex
	results map[string]SiteProbeResult
	running bool
}

func (s *SettingService) getSiteProbeTargets() ([]SiteProbeTarget, error) {
	row, err := s.getSetting(siteProbeKey)
	if err != nil {
		if database.IsNotFound(err) {
			return defaultSiteProbes(), nil
		}
		return nil, err
	}
	targets := []SiteProbeTarget{}
	if err := json.Unmarshal([]byte(row.Value), &targets); err != nil {
		return nil, err
	}
	return normalizeSiteProbeTargets(targets)
}

func (s *SettingService) GetSiteProbes() (SiteProbeView, error) {
	siteProbes.Lock()
	defer siteProbes.Unlock()
	return s.siteProbeViewLocked()
}

func (s *SettingService) siteProbeViewLocked() (SiteProbeView, error) {
	targets, err := s.getSiteProbeTargets()
	view := SiteProbeView{Targets: targets, Results: []SiteProbeResult{}, Running: siteProbes.running}
	for _, target := range targets {
		if result, ok := siteProbes.results[target.URL]; ok {
			view.Results = append(view.Results, result)
		}
	}
	return view, err
}

func (s *SettingService) SaveSiteProbes(targets []SiteProbeTarget) (SiteProbeView, error) {
	clean, err := normalizeSiteProbeTargets(targets)
	if err != nil {
		return SiteProbeView{}, err
	}
	data, err := json.Marshal(clean)
	if err != nil {
		return SiteProbeView{}, err
	}
	siteProbes.Lock()
	defer siteProbes.Unlock()
	if err := s.saveSetting(siteProbeKey, string(data)); err != nil {
		return SiteProbeView{}, err
	}
	return s.siteProbeViewLocked()
}

func normalizeSiteProbeTargets(targets []SiteProbeTarget) ([]SiteProbeTarget, error) {
	if len(targets) > maxSiteProbes {
		return nil, fmt.Errorf("maximum %d probe addresses", maxSiteProbes)
	}
	seen := make(map[string]bool, len(targets))
	clean := make([]SiteProbeTarget, 0, len(targets))
	for _, target := range targets {
		raw := strings.TrimSpace(target.URL)
		if len(raw) == 0 || len(raw) > 2048 {
			return nil, errors.New("enter an HTTP(S) address of up to 2048 bytes")
		}
		if !strings.Contains(raw, "://") {
			raw = "https://" + raw
		}
		u, err := url.Parse(raw)
		if err != nil || u.Hostname() == "" || u.User != nil || (u.Scheme != "https" && u.Scheme != "http") {
			return nil, errors.New("enter an HTTP(S) address without login details")
		}
		host, err := netsafe.NormalizeHost(u.Hostname())
		if err != nil || (!strings.Contains(host, ".") && net.ParseIP(host) == nil) {
			return nil, errors.New("enter a public site address")
		}
		if ip := net.ParseIP(host); ip != nil && !publicProbeIP(ip) {
			return nil, errors.New("private or special-use addresses cannot be tested")
		}
		port := u.Port()
		if port != "" && !((u.Scheme == "https" && port == "443") || (u.Scheme == "http" && port == "80")) {
			return nil, errors.New("only the standard HTTP and HTTPS ports can be tested")
		}
		u.Host = strings.ToLower(host)
		if strings.Contains(host, ":") {
			u.Host = "[" + host + "]"
		}
		u.Fragment, u.RawFragment = "", ""
		if u.Path == "" {
			u.Path = "/"
		}
		target.URL = u.String()
		if seen[target.URL] {
			return nil, errors.New("this address is already in the list")
		}
		seen[target.URL] = true
		target.Name = strings.TrimSpace(target.Name)
		if target.Name == "" {
			target.Name = u.Hostname()
		}
		if len([]rune(target.Name)) > 255 {
			return nil, errors.New("site name is too long")
		}
		clean = append(clean, target)
	}
	return clean, nil
}

var blockedProbeNets = []netip.Prefix{
	netip.MustParsePrefix("0.0.0.0/8"), netip.MustParsePrefix("100.64.0.0/10"),
	netip.MustParsePrefix("192.0.0.0/24"), netip.MustParsePrefix("198.18.0.0/15"),
	netip.MustParsePrefix("240.0.0.0/4"), netip.MustParsePrefix("64:ff9b::/96"),
	netip.MustParsePrefix("64:ff9b:1::/48"), netip.MustParsePrefix("2002::/16"),
}

func publicProbeIP(ip net.IP) bool {
	addr, valid := netip.AddrFromSlice(ip)
	if !valid || !addr.IsGlobalUnicast() || netsafe.IsBlockedIP(ip) {
		return false
	}
	for _, p := range blockedProbeNets {
		if p.Contains(addr.Unmap()) {
			return false
		}
	}
	return true
}

type probeDialContext func(context.Context, string, string) (net.Conn, error)

// Use the live core and its existing rules. The temporary SOCKS inbound is
// loopback-only and authenticated with a fresh random password each run.
func openSiteProbeRoute() (probeDialContext, func(), error) {
	process := currentXrayProcess()
	if process == nil || !process.IsRunning() {
		return nil, nil, errors.New("Xray unavailable")
	}
	return openSiteProbeRouteAt(process.GetAPIPort())
}

func openSiteProbeRouteAt(apiPort int) (probeDialContext, func(), error) {
	var api xray.XrayAPI
	if err := api.Init(apiPort); err != nil {
		return nil, nil, err
	}
	var seed [24]byte
	if _, err := rand.Read(seed[:]); err != nil {
		api.Close()
		return nil, nil, err
	}
	tag := "xui-site-probe-" + hex.EncodeToString(seed[:8])
	auth := &proxy.Auth{User: "probe", Password: hex.EncodeToString(seed[8:])}
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		api.Close()
		return nil, nil, err
	}
	port := listener.Addr().(*net.TCPAddr).Port
	_ = listener.Close()
	inbound := fmt.Sprintf(`{"tag":%q,"listen":"127.0.0.1","port":%d,"protocol":"socks",
		"settings":{"auth":"password","accounts":[{"user":%q,"pass":%q}],"udp":false},
		"sniffing":{"enabled":true,"destOverride":["http","tls"],"routeOnly":true}}`, tag, port, auth.User, auth.Password)
	closeRoute := func() {
		defer api.Close()
		if err := api.DelInbound(tag); err != nil && !xray.IsMissingHandlerErr(err) {
			logger.Warning("site probe: remove temporary inbound:", err)
		}
	}
	if err := api.AddInbound([]byte(inbound)); err != nil {
		// The RPC may have reached Xray before a timeout; clean up its tag.
		closeRoute()
		return nil, nil, err
	}
	dialer, err := proxy.SOCKS5("tcp", net.JoinHostPort("127.0.0.1", fmt.Sprint(port)), auth, proxy.Direct)
	if err != nil {
		closeRoute()
		return nil, nil, err
	}
	return dialer.(proxy.ContextDialer).DialContext, closeRoute, nil
}

func probeHTTP(ctx context.Context, address, ip string, dial probeDialContext) SiteProbePath {
	start := time.Now()
	transport := &http.Transport{DisableKeepAlives: true, MaxResponseHeaderBytes: 32 << 10,
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			_, port, err := net.SplitHostPort(addr)
			if err != nil {
				return nil, err
			}
			return dial(ctx, network, net.JoinHostPort(ip, port))
		}}
	client := &http.Client{Transport: transport, CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
		return http.ErrUseLastResponse
	}}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, address, nil)
	if err != nil {
		return SiteProbePath{Status: "failed"}
	}
	req.Header.Set("User-Agent", "Fara-Site-Check/1.0")
	resp, err := client.Do(req)
	if err != nil {
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return SiteProbePath{Status: "timeout"}
		}
		return SiteProbePath{Status: "failed"}
	}
	defer resp.Body.Close()
	status := "http"
	if resp.StatusCode >= 200 && resp.StatusCode < 400 {
		status = "ok"
	}
	return SiteProbePath{Status: status, Code: resp.StatusCode, Delay: max(1, time.Since(start).Milliseconds())}
}

func checkSiteProbe(ctx context.Context, target SiteProbeTarget, route probeDialContext) SiteProbeResult {
	result := SiteProbeResult{URL: target.URL, At: time.Now().UnixMilli()}
	u, _ := url.Parse(target.URL)
	dnsCtx, cancel := context.WithTimeout(ctx, 4*time.Second)
	ips, err := net.DefaultResolver.LookupIP(dnsCtx, "ip", u.Hostname())
	cancel()
	if err != nil || len(ips) == 0 {
		result.Direct.Status, result.Routed.Status = "dns", "dns"
		return result
	}
	for _, ip := range ips {
		if !publicProbeIP(ip) {
			result.Direct.Status, result.Routed.Status = "private", "private"
			return result
		}
	}
	// Pin the same address for both paths. HTTP Host and TLS SNI are
	// preserved for Xray routing and normal certificate verification.
	chosen := ips[0]
	for _, ip := range ips {
		if ip.To4() != nil {
			chosen = ip
			break
		}
	}
	probe := func(dial probeDialContext) SiteProbePath {
		c, stop := context.WithTimeout(ctx, 6*time.Second)
		defer stop()
		return probeHTTP(c, target.URL, chosen.String(), dial)
	}
	var done sync.WaitGroup
	if route != nil {
		done.Go(func() { result.Routed = probe(route) })
	} else {
		result.Routed.Status = "unavailable"
	}
	result.Direct = probe((&net.Dialer{}).DialContext)
	done.Wait()
	return result
}

// StartSiteProbes starts one bounded batch at a time. Polling the dashboard
// only reads the cached result and never starts network requests.
func (s *SettingService) StartSiteProbes(onlyURL string) (SiteProbeView, error) {
	siteProbes.Lock()
	defer siteProbes.Unlock()
	view, err := s.siteProbeViewLocked()
	if err != nil || view.Running {
		return view, err
	}
	targets := view.Targets
	if onlyURL != "" {
		targets = nil
		for _, target := range view.Targets {
			if target.URL == onlyURL {
				targets = []SiteProbeTarget{target}
				break
			}
		}
		if len(targets) == 0 {
			return view, errors.New("save the address before testing")
		}
	}
	if len(targets) == 0 {
		return view, nil
	}
	siteProbes.running, view.Running = true, true
	go s.runSiteProbes(targets)
	return view, nil
}

func (s *SettingService) runSiteProbes(targets []SiteProbeTarget) {
	defer func() {
		siteProbes.Lock()
		siteProbes.running = false
		siteProbes.Unlock()
	}()
	ctx, stop := context.WithTimeout(context.Background(), 50*time.Second)
	defer stop()
	route, closeRoute, err := openSiteProbeRoute()
	if err == nil {
		defer closeRoute()
	} else {
		logger.Debug("site probe: Xray route unavailable:", err)
	}
	var workers sync.WaitGroup
	limit := make(chan struct{}, 3)
	for _, target := range targets {
		limit <- struct{}{}
		workers.Go(func() {
			defer func() { <-limit }()
			result := checkSiteProbe(ctx, target, route)
			siteProbes.Lock()
			defer siteProbes.Unlock()
			if siteProbes.results == nil || len(siteProbes.results) >= 4*maxSiteProbes {
				siteProbes.results = make(map[string]SiteProbeResult)
			}
			siteProbes.results[target.URL] = result
		})
	}
	workers.Wait()
}
