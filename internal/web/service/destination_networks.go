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
	ripeAnnouncedPrefixesURL          = "https://stat.ripe.net/data/announced-prefixes/data.json?resource="
	destinationNetworkRefreshInterval = 12 * time.Hour
	destinationNetworkRetryInterval   = time.Hour
	destinationNetworkHTTPTimeout     = 12 * time.Second
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

type destinationNetworkSnapshot struct {
	telegram   []*net.IPNet
	meta       []*net.IPNet
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
			telegram: parseCIDRs(telegramFallbackCIDRs),
			meta:     parseCIDRs(metaFallbackCIDRs),
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

// classifyDestinationIP recognizes IP-only destinations using official Telegram
// ranges and Meta BGP prefixes. Domain-based classification still has priority.
func classifyDestinationIP(ip net.IP) (service, owner, confidence string, ok bool) {
	if ip == nil {
		return "", "", "", false
	}
	destinationNetworks.mu.RLock()
	snapshot := destinationNetworks.snapshot
	telegramMatch := networkContains(snapshot.telegram, ip)
	metaMatch := networkContains(snapshot.meta, ip)
	destinationNetworks.mu.RUnlock()

	if telegramMatch {
		return "Telegram", "Telegram network", "network", true
	}
	if metaMatch {
		return "Instagram / Meta", "Meta network", "network", true
	}
	return "", "", "", false
}

// RefreshDestinationNetworkRulesIfNeeded refreshes network prefixes in the
// background. It is safe to call from the 10-second destination ingestion job;
// actual downloads happen at most once every 12 hours, or hourly after failure.
func RefreshDestinationNetworkRulesIfNeeded() {
	now := time.Now()
	destinationNetworks.mu.Lock()
	if destinationNetworks.refreshing || now.Before(destinationNetworks.nextRefresh) {
		destinationNetworks.mu.Unlock()
		return
	}
	destinationNetworks.refreshing = true
	// Reserve the retry window immediately so repeated cron runs cannot spawn
	// duplicate refresh goroutines while a slow request is in progress.
	destinationNetworks.nextRefresh = now.Add(destinationNetworkRetryInterval)
	destinationNetworks.mu.Unlock()

	go refreshDestinationNetworkRules()
}

func refreshDestinationNetworkRules() {
	ctx, cancel := context.WithTimeout(context.Background(), destinationNetworkHTTPTimeout)
	defer cancel()

	client := &http.Client{Timeout: destinationNetworkHTTPTimeout}
	telegramCIDRs, telegramErr := fetchTelegramCIDRs(ctx, client)
	meta32934, meta32934Err := fetchRipeAnnouncedPrefixes(ctx, client, "AS32934")
	meta63293, meta63293Err := fetchRipeAnnouncedPrefixes(ctx, client, "AS63293")

	destinationNetworks.mu.Lock()
	defer destinationNetworks.mu.Unlock()
	destinationNetworks.refreshing = false

	updated := false
	if telegramErr == nil && len(telegramCIDRs) >= len(telegramFallbackCIDRs) {
		destinationNetworks.snapshot.telegram = mergeCIDRs(telegramFallbackCIDRs, telegramCIDRs)
		updated = true
	}
	metaDynamic := append(meta32934, meta63293...)
	if len(metaDynamic) > 0 && (meta32934Err == nil || meta63293Err == nil) {
		destinationNetworks.snapshot.meta = mergeCIDRs(metaFallbackCIDRs, metaDynamic)
		updated = true
	}

	if updated {
		destinationNetworks.snapshot.lastUpdate = time.Now()
		destinationNetworks.nextRefresh = time.Now().Add(destinationNetworkRefreshInterval)
		logger.Debugf("[ClientDestinations] destination prefix rules refreshed: telegram=%d meta=%d", len(destinationNetworks.snapshot.telegram), len(destinationNetworks.snapshot.meta))
		return
	}

	destinationNetworks.nextRefresh = time.Now().Add(destinationNetworkRetryInterval)
	logger.Debugf("[ClientDestinations] prefix refresh failed; using bundled ranges (telegram=%v, meta32934=%v, meta63293=%v)", telegramErr, meta32934Err, meta63293Err)
}

func newDestinationRequest(ctx context.Context, url string) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Fara-Xray/3.6.10 destination-prefix-updater")
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
