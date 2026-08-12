package service

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/logger"
)

const (
	telegramCIDRSourceURL             = "https://core.telegram.org/resources/cidr.txt"
	googleCIDRSourceURL               = "https://www.gstatic.com/ipranges/goog.json"
	ripeAnnouncedPrefixesURL          = "https://stat.ripe.net/data/announced-prefixes/data.json?sourceapp=fara-xray&resource="
	destinationNetworkRefreshInterval = 12 * time.Hour
	destinationNetworkRetryInterval   = time.Hour
	destinationNetworkHTTPTimeout     = 15 * time.Second
)

// Telegram publishes this list at telegramCIDRSourceURL. It is kept here as an
// offline fallback so classification works immediately after installation and
// when the server cannot reach the update source.
var telegramFallbackCIDRs = []string{
	"91.108.56.0/22",
	"91.108.4.0/22",
	"91.108.8.0/22",
	"91.108.16.0/22",
	"91.108.12.0/22",
	"149.154.160.0/20",
	"91.105.192.0/23",
	"91.108.20.0/22",
	"185.76.151.0/24",
	"2001:b28:f23d::/48",
	"2001:b28:f23f::/48",
	"2001:67c:4e8::/48",
	"2001:b28:f23c::/48",
	"2a0a:f280::/32",
}

// These are Meta-owned/announced aggregate blocks used as an offline fallback.
// Meta serves Instagram, Facebook and WhatsApp from shared infrastructure, so an
// IP-only match is intentionally labelled "Instagram / Meta" rather than being
// presented as a certain Instagram visit. The online updater supplements this
// list with current prefixes announced by AS32934 and AS63293.
var metaFallbackCIDRs = []string{
	"31.13.24.0/21",
	"31.13.64.0/18",
	"45.64.40.0/22",
	"57.141.0.0/24",
	"57.141.2.0/23",
	"57.141.4.0/23",
	"57.141.6.0/24",
	"57.141.8.0/24",
	"57.141.10.0/24",
	"57.141.12.0/23",
	"57.141.14.0/24",
	"57.141.16.0/22",
	"57.141.20.0/24",
	"57.141.24.0/24",
	"57.144.0.0/14",
	"66.220.144.0/20",
	"69.63.176.0/20",
	"69.171.224.0/19",
	"74.119.76.0/22",
	"102.132.96.0/20",
	"103.4.96.0/22",
	"129.134.0.0/17",
	"157.240.0.0/17",
	"157.240.192.0/18",
	"163.70.128.0/17",
	"163.77.132.0/23",
	"163.77.136.0/23",
	"173.252.64.0/18",
	"179.60.192.0/22",
	"185.60.216.0/22",
	"185.89.216.0/22",
	"204.15.20.0/22",
	"2620:0:1c00::/40",
	"2a03:2880::/32",
}

// Preparsed immutable fallbacks are checked before the refreshable snapshot.
// This makes core classifications deterministic even if an online refresh
// returns an incomplete response or a future feed is temporarily unavailable.
var (
	telegramFallbackNetworks = parseCIDRs(telegramFallbackCIDRs)
	metaFallbackNetworks     = parseCIDRs(metaFallbackCIDRs)
	googleFallbackNetworks   = parseCIDRs(googleFallbackCIDRs)
)

type destinationNetworkSnapshot struct {
	telegram   []*net.IPNet
	meta       []*net.IPNet
	google     []*net.IPNet
	lastUpdate time.Time
}

type destinationNetworkRegistry struct {
	mu          sync.RWMutex
	snapshot    destinationNetworkSnapshot
	nextRefresh time.Time
	refreshing  bool
}

var destinationNetworks = newDestinationNetworkRegistry()

func newDestinationNetworkRegistry() *destinationNetworkRegistry {
	return &destinationNetworkRegistry{
		snapshot: destinationNetworkSnapshot{
			telegram: telegramFallbackNetworks,
			meta:     metaFallbackNetworks,
			google:   googleFallbackNetworks,
		},
	}
}

func parseCIDRs(values []string) []*net.IPNet {
	seen := make(map[string]struct{}, len(values))
	out := make([]*net.IPNet, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		_, network, err := net.ParseCIDR(value)
		if err != nil {
			continue
		}
		key := network.String()
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, network)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].String() < out[j].String() })
	return out
}

func mergeCIDRs(groups ...[]string) []*net.IPNet {
	values := make([]string, 0)
	for _, group := range groups {
		values = append(values, group...)
	}
	return parseCIDRs(values)
}

func networkContains(networks []*net.IPNet, ip net.IP) bool {
	for _, network := range networks {
		if network.Contains(ip) {
			return true
		}
	}
	return false
}

// classifyDestinationIP recognizes IP-only destinations using bundled ranges
// first and refreshed provider ranges second. Domain classification still has
// priority, so googlevideo.com is shown as YouTube while an IP-only Google
// destination is conservatively shown as Google.
func classifyDestinationIP(ip net.IP) (service, owner, confidence string, ok bool) {
	if ip == nil {
		return "", "", "", false
	}

	// Never let a failed or partial refresh remove the deterministic fallbacks.
	if networkContains(telegramFallbackNetworks, ip) {
		return "Telegram", "Telegram network", "network", true
	}
	if networkContains(metaFallbackNetworks, ip) {
		return "Instagram / Meta", "Meta network", "network", true
	}
	if networkContains(googleFallbackNetworks, ip) {
		return "Google", "Google network", "network", true
	}

	destinationNetworks.mu.RLock()
	snapshot := destinationNetworks.snapshot
	telegramMatch := networkContains(snapshot.telegram, ip)
	metaMatch := networkContains(snapshot.meta, ip)
	googleMatch := networkContains(snapshot.google, ip)
	destinationNetworks.mu.RUnlock()

	if telegramMatch {
		return "Telegram", "Telegram network", "network", true
	}
	if metaMatch {
		return "Instagram / Meta", "Meta network", "network", true
	}
	if googleMatch {
		return "Google", "Google network", "network", true
	}
	return "", "", "", false
}

// RefreshDestinationNetworkRulesIfNeeded refreshes provider prefixes in the
// background. Downloads happen at most once every 12 hours, or hourly after a
// partial/complete failure. Requests run concurrently with independent timeouts
// so one slow source cannot prevent the other provider lists from updating.
func RefreshDestinationNetworkRulesIfNeeded() {
	now := time.Now()
	destinationNetworks.mu.Lock()
	if destinationNetworks.refreshing || now.Before(destinationNetworks.nextRefresh) {
		destinationNetworks.mu.Unlock()
		return
	}
	destinationNetworks.refreshing = true
	destinationNetworks.nextRefresh = now.Add(destinationNetworkRetryInterval)
	destinationNetworks.mu.Unlock()

	go refreshDestinationNetworkRules()
}

func refreshDestinationNetworkRules() {
	client := &http.Client{Timeout: destinationNetworkHTTPTimeout}

	var telegramCIDRs, googleCIDRs, meta32934, meta63293 []string
	var telegramErr, googleErr, meta32934Err, meta63293Err error

	var wg sync.WaitGroup
	wg.Add(4)
	go func() {
		defer wg.Done()
		ctx, cancel := context.WithTimeout(context.Background(), destinationNetworkHTTPTimeout)
		defer cancel()
		telegramCIDRs, telegramErr = fetchTelegramCIDRs(ctx, client)
	}()
	go func() {
		defer wg.Done()
		ctx, cancel := context.WithTimeout(context.Background(), destinationNetworkHTTPTimeout)
		defer cancel()
		googleCIDRs, googleErr = fetchGoogleCIDRs(ctx, client)
	}()
	go func() {
		defer wg.Done()
		ctx, cancel := context.WithTimeout(context.Background(), destinationNetworkHTTPTimeout)
		defer cancel()
		meta32934, meta32934Err = fetchRipeAnnouncedPrefixes(ctx, client, "AS32934")
	}()
	go func() {
		defer wg.Done()
		ctx, cancel := context.WithTimeout(context.Background(), destinationNetworkHTTPTimeout)
		defer cancel()
		meta63293, meta63293Err = fetchRipeAnnouncedPrefixes(ctx, client, "AS63293")
	}()
	wg.Wait()

	destinationNetworks.mu.Lock()
	defer destinationNetworks.mu.Unlock()
	destinationNetworks.refreshing = false

	updated := false
	allHealthy := true
	if telegramErr == nil && len(telegramCIDRs) > 0 {
		destinationNetworks.snapshot.telegram = mergeCIDRs(telegramFallbackCIDRs, telegramCIDRs)
		updated = true
	} else {
		allHealthy = false
	}
	if googleErr == nil && len(googleCIDRs) > 0 {
		destinationNetworks.snapshot.google = mergeCIDRs(googleFallbackCIDRs, googleCIDRs)
		updated = true
	} else {
		allHealthy = false
	}
	metaDynamic := append(append([]string{}, meta32934...), meta63293...)
	if len(metaDynamic) > 0 && (meta32934Err == nil || meta63293Err == nil) {
		destinationNetworks.snapshot.meta = mergeCIDRs(metaFallbackCIDRs, metaDynamic)
		updated = true
	}
	if meta32934Err != nil || meta63293Err != nil {
		allHealthy = false
	}

	if updated {
		destinationNetworks.snapshot.lastUpdate = time.Now()
	}
	if updated && allHealthy {
		destinationNetworks.nextRefresh = time.Now().Add(destinationNetworkRefreshInterval)
		logger.Debugf("[ClientDestinations] provider prefixes refreshed: telegram=%d meta=%d google=%d", len(destinationNetworks.snapshot.telegram), len(destinationNetworks.snapshot.meta), len(destinationNetworks.snapshot.google))
		return
	}

	// Keep successful partial updates but retry failed sources sooner.
	destinationNetworks.nextRefresh = time.Now().Add(destinationNetworkRetryInterval)
	logger.Debugf("[ClientDestinations] provider prefix refresh partial/failed; bundled ranges remain active (telegram=%v google=%v meta32934=%v meta63293=%v)", telegramErr, googleErr, meta32934Err, meta63293Err)
}

func newDestinationRequest(ctx context.Context, url string) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Fara-Xray/3.6.29 destination-prefix-updater")
	req.Header.Set("Accept", "text/plain, application/json")
	return req, nil
}

func fetchTelegramCIDRs(ctx context.Context, client *http.Client) ([]string, error) {
	req, err := newDestinationRequest(ctx, telegramCIDRSourceURL)
	if err != nil {
		return nil, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("telegram CIDR source returned HTTP %d", resp.StatusCode)
	}

	scanner := bufio.NewScanner(io.LimitReader(resp.Body, 256*1024))
	values := make([]string, 0, 32)
	for scanner.Scan() {
		value := strings.TrimSpace(scanner.Text())
		if value == "" || strings.HasPrefix(value, "#") {
			continue
		}
		if _, _, err := net.ParseCIDR(value); err == nil {
			values = append(values, value)
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	if len(values) == 0 {
		return nil, fmt.Errorf("telegram CIDR source returned no valid prefixes")
	}
	return values, nil
}

type googleIPRangesResponse struct {
	Prefixes []struct {
		IPv4Prefix string `json:"ipv4Prefix"`
		IPv6Prefix string `json:"ipv6Prefix"`
	} `json:"prefixes"`
}

func fetchGoogleCIDRs(ctx context.Context, client *http.Client) ([]string, error) {
	req, err := newDestinationRequest(ctx, googleCIDRSourceURL)
	if err != nil {
		return nil, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Google IP range source returned HTTP %d", resp.StatusCode)
	}

	var payload googleIPRangesResponse
	decoder := json.NewDecoder(io.LimitReader(resp.Body, 2*1024*1024))
	if err := decoder.Decode(&payload); err != nil {
		return nil, err
	}
	values := make([]string, 0, len(payload.Prefixes))
	for _, item := range payload.Prefixes {
		for _, prefix := range []string{item.IPv4Prefix, item.IPv6Prefix} {
			prefix = strings.TrimSpace(prefix)
			if prefix == "" {
				continue
			}
			if _, _, err := net.ParseCIDR(prefix); err == nil {
				values = append(values, prefix)
			}
		}
	}
	if len(values) == 0 {
		return nil, fmt.Errorf("Google IP range source returned no valid prefixes")
	}
	return values, nil
}

type ripeAnnouncedPrefixesResponse struct {
	Data struct {
		Prefixes []struct {
			Prefix string `json:"prefix"`
		} `json:"prefixes"`
	} `json:"data"`
}

func fetchRipeAnnouncedPrefixes(ctx context.Context, client *http.Client, asn string) ([]string, error) {
	asn = strings.ToUpper(strings.TrimSpace(asn))
	if !strings.HasPrefix(asn, "AS") || len(asn) <= 2 {
		return nil, fmt.Errorf("invalid ASN %q", asn)
	}
	req, err := newDestinationRequest(ctx, ripeAnnouncedPrefixesURL+asn)
	if err != nil {
		return nil, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("RIPEstat %s returned HTTP %d", asn, resp.StatusCode)
	}

	var payload ripeAnnouncedPrefixesResponse
	decoder := json.NewDecoder(io.LimitReader(resp.Body, 4*1024*1024))
	if err := decoder.Decode(&payload); err != nil {
		return nil, err
	}
	values := make([]string, 0, len(payload.Data.Prefixes))
	for _, item := range payload.Data.Prefixes {
		prefix := strings.TrimSpace(item.Prefix)
		if _, _, err := net.ParseCIDR(prefix); err == nil {
			values = append(values, prefix)
		}
	}
	if len(values) == 0 {
		return nil, fmt.Errorf("RIPEstat %s returned no valid prefixes", asn)
	}
	return values, nil
}
