package sub

import (
	"slices"
	"strings"
	"time"

	"github.com/goccy/go-json"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/util/json_util"
)

type subscriptionDisplayCandidate struct {
	client    model.Client
	inbound   *model.Inbound
	endpoint  map[string]any
	synthetic bool
}

const (
	subscriptionDisplayHostAddress = "1.1.1.1"
	subscriptionDisplayHostPort    = 1
	subscriptionDisplayHostUUID    = "00000000-0000-4000-8000-000000000001"
)

func subscriptionDisplayInbound() *model.Inbound {
	return &model.Inbound{
		Id:             -1,
		Remark:         "Subscription Info",
		Protocol:       model.VLESS,
		Listen:         subscriptionDisplayHostAddress,
		Port:           subscriptionDisplayHostPort,
		Settings:       `{"clients":[],"decryption":"none","encryption":"none"}`,
		StreamSettings: `{"network":"tcp","security":"none","tcpSettings":{"header":{"type":"none"}}}`,
	}
}

// subscriptionDisplayContext returns one stable client context for the whole
// subscription. A client may be attached to several inbounds, but the display
// entry must be emitted only once, so callers resolve it before iterating the
// inbounds and prepend exactly one synthetic entry.
func (s *SubService) subscriptionDisplayContext(subID string, inbounds []*model.Inbound) (model.Client, *model.Inbound, bool) {
	var fallbackClient model.Client
	var fallbackInbound *model.Inbound
	for _, inbound := range inbounds {
		clients := s.matchingClients(inbound, subID)
		for _, client := range clients {
			if fallbackInbound == nil {
				fallbackClient = client
				fallbackInbound = inbound
			}
			if client.Enable {
				return client, inbound, true
			}
		}
	}
	if fallbackInbound != nil {
		return fallbackClient, fallbackInbound, true
	}

	var rec model.ClientRecord
	if err := database.GetDB().Where("sub_id = ?", subID).Order("id ASC").First(&rec).Error; err != nil {
		return model.Client{}, nil, false
	}
	return *rec.ToClient(), subscriptionDisplayInbound(), true
}

func (s *SubService) subscriptionDisplayRemark(inbound *model.Inbound, client model.Client) string {
	if !s.displayHostEnabled {
		return ""
	}
	template := strings.TrimSpace(s.displayHostRemark)
	if template == "" {
		return ""
	}
	if inbound == nil {
		inbound = subscriptionDisplayInbound()
	}
	stats := s.statsForClient(inbound, client)
	// The synthetic entry exists only to present quota/expiry information. When
	// both are unlimited it has nothing useful to show, so omit the whole entry
	// from RAW, JSON and Clash subscriptions instead of rendering infinities.
	if stats.Total <= 0 && stats.ExpiryTime == 0 {
		return ""
	}
	ctx := remarkContext{
		client:     client,
		stats:      stats,
		inbound:    inbound,
		hostRemark: "Subscription Info",
		transport:  "tcp",
		security:   "none",
	}
	remark := strings.TrimSpace(expandRemarkVars(template, ctx))
	if remark == "" {
		return client.Email
	}
	return remark
}

func (s *SubService) subscriptionDisplayCandidateFor(subID string, inbounds []*model.Inbound, format string) (subscriptionDisplayCandidate, bool) {
	if !s.displayHostEnabled {
		return subscriptionDisplayCandidate{}, false
	}
	byID := make(map[int]*model.Inbound, len(inbounds))
	for _, inbound := range inbounds {
		byID[inbound.Id] = inbound
	}
	for _, priority := range s.displayHostPriorities {
		inbound := byID[priority.InboundId]
		if inbound == nil || !inbound.Enable {
			continue
		}
		for _, client := range s.matchingClients(inbound, subID) {
			if !s.subscriptionDisplayClientActive(inbound, client) {
				continue
			}
			clone := *inbound
			s.projectThroughFallbackMaster(&clone)
			if priority.HostGroupId == "" {
				selection := s.hostEndpointsForClient(&clone, format, client.RecordID)
				if selection.managed || s.subscriptionLinkDisabled(client.RecordID, model.InboundSubscriptionLinkKey(inbound.Id)) {
					continue
				}
				return subscriptionDisplayCandidate{client: client, inbound: &clone}, true
			}
			var host model.Host
			err := database.GetDB().Where(
				"inbound_id = ? AND group_id = ? AND address = ?",
				inbound.Id, priority.HostGroupId, priority.Address,
			).Order("id ASC").First(&host).Error
			if err != nil || slices.Contains(host.ExcludeFromSubTypes, format) {
				continue
			}
			if !s.subscriptionLinkEnabled(client.RecordID, host.SubscriptionLinkKey(), !host.IsDisabled) {
				continue
			}
			endpoint := hostToExternalProxyMap(&host, s.resolveInboundAddress(&clone), clone.Port)
			return subscriptionDisplayCandidate{client: client, inbound: &clone, endpoint: endpoint}, true
		}
	}
	client, inbound, ok := s.subscriptionDisplayContext(subID, inbounds)
	if !ok {
		return subscriptionDisplayCandidate{}, false
	}
	return subscriptionDisplayCandidate{client: client, inbound: inbound, synthetic: true}, true
}

func (s *SubService) subscriptionDisplayClientActive(inbound *model.Inbound, client model.Client) bool {
	if !client.Enable {
		return false
	}
	stats := s.statsForClient(inbound, client)
	if !stats.Enable {
		return false
	}
	if stats.ExpiryTime > 0 && stats.ExpiryTime <= time.Now().UnixMilli() {
		return false
	}
	return stats.Total <= 0 || stats.Up+stats.Down < stats.Total
}

func (s *SubService) subscriptionDisplayRawLink(subID string, inbounds []*model.Inbound) string {
	candidate, ok := s.subscriptionDisplayCandidateFor(subID, inbounds, "raw")
	if !ok {
		return ""
	}
	remark := s.subscriptionDisplayRemark(candidate.inbound, candidate.client)
	if remark == "" {
		return ""
	}
	if !candidate.synthetic {
		var link string
		if candidate.endpoint != nil {
			link = s.linkFromHosts(candidate.inbound, candidate.client, []map[string]any{candidate.endpoint})
		} else {
			link = s.GetLink(candidate.inbound, candidate.client.Email)
		}
		if lines := splitLinkLines(link); len(lines) > 0 {
			return applyRemarkToLink(lines[0], remark)
		}
	}
	params := map[string]string{
		"encryption": "none",
		"security":   "none",
		"type":       "tcp",
	}
	base := "vless://" + subscriptionDisplayHostUUID + "@" + subscriptionDisplayHostAddress + ":1"
	return buildLinkWithParams(base, params, remark)
}

func (s *SubJsonService) subscriptionDisplayConfig(subReq *SubService, subID string, inbounds []*model.Inbound) json_util.RawMessage {
	candidate, ok := subReq.subscriptionDisplayCandidateFor(subID, inbounds, "json")
	if !ok {
		return nil
	}
	remark := subReq.subscriptionDisplayRemark(candidate.inbound, candidate.client)
	if remark == "" {
		return nil
	}
	if !candidate.synthetic {
		selectedInbound := candidate.inbound
		if candidate.endpoint != nil {
			clone := *candidate.inbound
			injectExternalProxy(&clone, []map[string]any{candidate.endpoint})
			selectedInbound = &clone
		}
		configs := s.getConfig(subReq, selectedInbound, candidate.client, subReq.address)
		if len(configs) > 0 {
			var config map[string]any
			if json.Unmarshal(configs[0], &config) == nil {
				config["remarks"] = remark
				result, _ := json.MarshalIndent(config, "", "  ")
				return result
			}
		}
	}
	displayInbound := subscriptionDisplayInbound()
	displayClient := candidate.client
	displayClient.ID = subscriptionDisplayHostUUID
	displayClient.Flow = ""

	stream := json_util.RawMessage(displayInbound.StreamSettings)
	outbound := s.genVless(subReq, displayInbound, stream, displayClient, "")
	outbounds := make([]json_util.RawMessage, 0, len(s.defaultOutbounds)+1)
	outbounds = append(outbounds, outbound)
	outbounds = append(outbounds, s.defaultOutbounds...)

	config := s.baseConfigForRequest(subReq)
	config["outbounds"] = outbounds
	config["remarks"] = remark
	result, _ := json.MarshalIndent(config, "", "  ")
	return result
}

func (s *SubClashService) subscriptionDisplayProxy(subReq *SubService, subID string, inbounds []*model.Inbound) map[string]any {
	candidate, ok := subReq.subscriptionDisplayCandidateFor(subID, inbounds, "clash")
	if !ok {
		return nil
	}
	remark := subReq.subscriptionDisplayRemark(candidate.inbound, candidate.client)
	if remark == "" {
		return nil
	}
	if !candidate.synthetic {
		selectedInbound := candidate.inbound
		if candidate.endpoint != nil {
			clone := *candidate.inbound
			injectExternalProxy(&clone, []map[string]any{candidate.endpoint})
			selectedInbound = &clone
		}
		proxies := s.getProxies(subReq, selectedInbound, candidate.client, subReq.address)
		if len(proxies) > 0 {
			proxies[0]["name"] = remark
			return proxies[0]
		}
	}
	return map[string]any{
		"name":    remark,
		"type":    "vless",
		"server":  subscriptionDisplayHostAddress,
		"port":    subscriptionDisplayHostPort,
		"uuid":    subscriptionDisplayHostUUID,
		"network": "tcp",
		"tls":     false,
		"udp":     true,
	}
}
