package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/web/service/outbound"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"
)

const outboundProbeKey = "dashboardOutboundProbes"

type OutboundProbeTarget struct {
	Tag  string `json:"tag"`
	Mode string `json:"mode"`
}

type OutboundProbeResult struct {
	OutboundProbeTarget
	At            int64         `json:"at"`
	Ping          SiteProbePath `json:"ping"`
	EffectiveMode string        `json:"effectiveMode,omitempty"`
}

type OutboundProbeOption struct {
	Tag      string `json:"tag"`
	Protocol string `json:"protocol"`
}

func normalizeOutboundProbeTargets(targets []OutboundProbeTarget) ([]OutboundProbeTarget, error) {
	if len(targets) > maxSiteProbes {
		return nil, fmt.Errorf("maximum %d outbound probes", maxSiteProbes)
	}
	seen := make(map[string]bool, len(targets))
	clean := make([]OutboundProbeTarget, 0, len(targets))
	for _, target := range targets {
		if strings.TrimSpace(target.Tag) == "" || utf8.RuneCountInString(target.Tag) > 255 || seen[target.Tag] {
			return nil, errors.New("invalid or duplicate outbound tag")
		}
		switch target.Mode {
		case "tcp", "http", "real":
		default:
			return nil, errors.New("invalid outbound probe mode")
		}
		seen[target.Tag] = true
		clean = append(clean, target)
	}
	return clean, nil
}

func (s *SettingService) getOutboundProbeTargets() ([]OutboundProbeTarget, error) {
	row, err := s.getSetting(outboundProbeKey)
	if err != nil {
		if database.IsNotFound(err) {
			return []OutboundProbeTarget{}, nil
		}
		return nil, err
	}
	var targets []OutboundProbeTarget
	if err := json.Unmarshal([]byte(row.Value), &targets); err != nil {
		return nil, err
	}
	return normalizeOutboundProbeTargets(targets)
}

func (s *SettingService) SaveOutboundProbes(targets []OutboundProbeTarget) (SiteProbeView, error) {
	clean, err := normalizeOutboundProbeTargets(targets)
	if err != nil {
		return SiteProbeView{}, err
	}
	data, err := json.Marshal(clean)
	if err != nil {
		return SiteProbeView{}, err
	}
	siteProbes.Lock()
	defer siteProbes.Unlock()
	if err := s.saveSetting(outboundProbeKey, string(data)); err != nil {
		return SiteProbeView{}, err
	}
	for tag, row := range siteProbes.outboundResults {
		keep := false
		for _, target := range clean {
			keep = keep || row.OutboundProbeTarget == target
		}
		if !keep {
			delete(siteProbes.outboundResults, tag)
		}
	}
	return s.siteProbeViewLocked()
}

// Read the active config instead of rebuilding it (which also manages clients).
// When Xray is stopped, use the saved outbounds and their subscription entries
// just as the standalone outbound tester does.
func (s *SettingService) outboundProbeConfigs() (json.RawMessage, error) {
	if process := currentXrayProcess(); process != nil && process.IsRunning() {
		return json.RawMessage(process.GetConfig().OutboundConfigs), nil
	}
	raw, err := s.GetXrayConfigTemplate()
	if err != nil {
		return nil, err
	}
	var cfg xray.Config
	if err := json.Unmarshal([]byte(UnwrapXrayTemplateConfig(raw)), &cfg); err != nil {
		return nil, err
	}
	prepend, appendList, err := (&OutboundSubscriptionService{}).activeOutboundsSplit()
	if err != nil {
		return nil, err
	}
	mergeSubscriptionOutbounds(&cfg, prepend, appendList)
	return json.RawMessage(cfg.OutboundConfigs), nil
}

func parseOutboundProbeConfigs(raw json.RawMessage) ([]OutboundProbeOption, map[string]json.RawMessage, error) {
	options := []OutboundProbeOption{}
	byTag := make(map[string]json.RawMessage)
	var items []json.RawMessage
	if len(raw) != 0 {
		if err := json.Unmarshal(raw, &items); err != nil {
			return nil, nil, err
		}
	}
	for _, item := range items {
		var option OutboundProbeOption
		if json.Unmarshal(item, &option) != nil || option.Tag == "" || option.Tag == "blocked" {
			continue
		}
		switch option.Protocol {
		case "", "blackhole", "loopback", "dns", "freedom":
			continue
		}
		if _, seen := byTag[option.Tag]; !seen {
			options = append(options, option)
			byTag[option.Tag] = item
		}
	}
	return options, byTag, nil
}

func (s *SettingService) OutboundProbeOptions() ([]OutboundProbeOption, error) {
	raw, err := s.outboundProbeConfigs()
	if err != nil {
		return nil, err
	}
	options, _, err := parseOutboundProbeConfigs(raw)
	return options, err
}

type outboundProbeBatch func(string, string, string, string) ([]*outbound.TestOutboundResult, error)

func testDashboardOutbounds(targets []OutboundProbeTarget, config json.RawMessage, testURL string, probe outboundProbeBatch) []OutboundProbeResult {
	results := make([]OutboundProbeResult, len(targets))
	groups := make(map[string][]int)
	_, byTag, _ := parseOutboundProbeConfigs(config)
	for i, target := range targets {
		results[i] = OutboundProbeResult{OutboundProbeTarget: target, At: time.Now().UnixMilli(),
			Ping: SiteProbePath{Status: "unavailable"}}
		if _, ok := byTag[target.Tag]; ok {
			groups[target.Mode] = append(groups[target.Mode], i)
		}
	}
	// One group at a time so HTTP and real-delay batches do not compete for
	// the outbound tester's shared Xray process lock.
	for _, mode := range []string{"tcp", "http", "real"} {
		indices := groups[mode]
		if len(indices) == 0 {
			continue
		}
		items := make([]json.RawMessage, len(indices))
		for j, i := range indices {
			items[j] = byTag[targets[i].Tag]
		}
		raw, err := json.Marshal(items)
		if err != nil {
			continue
		}
		answers, err := probe(string(raw), testURL, string(config), mode)
		if err != nil || len(answers) != len(indices) {
			continue
		}
		for j, i := range indices {
			answer := answers[j]
			if answer == nil || answer.Tag != targets[i].Tag {
				continue
			}
			path := SiteProbePath{Status: "failed"}
			if answer.Success {
				path = SiteProbePath{Status: "ok", Code: answer.HTTPStatus, Delay: max(1, answer.Delay)}
			}
			results[i].At = time.Now().UnixMilli()
			results[i].Ping = path
			results[i].EffectiveMode = answer.Mode
		}
	}
	return results
}

func (s *SettingService) runOutboundProbes(targets []OutboundProbeTarget) {
	raw, rawErr := s.outboundProbeConfigs()
	testURL, urlErr := s.GetXrayOutboundTestUrl()
	if urlErr == nil {
		// Match the outbound tester endpoints. URLs come from server settings,
		// not from dashboard input.
		testURL, urlErr = SanitizePublicHTTPURL(testURL, false)
	}
	if rawErr != nil || urlErr != nil {
		raw = nil // publish unavailable rather than silently using another URL
	}
	results := testDashboardOutbounds(targets, raw, testURL, (&outbound.OutboundService{}).TestOutbounds)
	siteProbes.Lock()
	defer siteProbes.Unlock()
	current, err := s.getOutboundProbeTargets()
	if err != nil {
		return
	}
	if siteProbes.outboundResults == nil {
		siteProbes.outboundResults = make(map[string]OutboundProbeResult)
	}
	for _, result := range results {
		for _, target := range current {
			if result.OutboundProbeTarget == target {
				siteProbes.outboundResults[target.Tag] = result
			}
		}
	}
}
