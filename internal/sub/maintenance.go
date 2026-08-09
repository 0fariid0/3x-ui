package sub

import (
	"fmt"
	"strings"

	"github.com/goccy/go-json"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/util/json_util"
)

const maintenanceDisplayUUID = "00000000-0000-4000-8000-000000000002"

func (s *SubService) maintenanceActive() bool {
	return s.maintenanceEnabled && strings.TrimSpace(s.maintenanceMessage) != ""
}

func (s *SubService) maintenanceFallbackRawLinks() []string {
	if !s.maintenanceEnabled {
		return nil
	}
	lines := splitLinkLines(s.maintenanceFallbackLinks)
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		if parseExternalLink(line) == nil {
			continue
		}
		out = append(out, line)
	}
	return out
}

func (s *SubService) maintenanceFallbackOnly() bool {
	return s.maintenanceEnabled && s.maintenanceMode == "fallback" && len(s.maintenanceFallbackRawLinks()) > 0
}

func (s *SubService) maintenanceRemark(inbound *model.Inbound, client model.Client) string {
	if !s.maintenanceActive() {
		return ""
	}
	if inbound == nil {
		inbound = subscriptionDisplayInbound()
	}
	ctx := remarkContext{
		client:     client,
		stats:      s.statsForClient(inbound, client),
		inbound:    inbound,
		hostRemark: "Maintenance",
		transport:  "tcp",
		security:   "none",
	}
	remark := strings.TrimSpace(expandRemarkVars(s.maintenanceMessage, ctx))
	if remark == "" {
		return "Maintenance"
	}
	return remark
}

func (s *SubService) maintenanceRawLink(inbound *model.Inbound, client model.Client) string {
	remark := s.maintenanceRemark(inbound, client)
	if remark == "" {
		return ""
	}
	params := map[string]string{
		"encryption": "none",
		"security":   "none",
		"type":       "tcp",
	}
	base := "vless://" + maintenanceDisplayUUID + "@" + subscriptionDisplayHostAddress + ":1"
	return buildLinkWithParams(base, params, remark)
}

func (s *SubJsonService) maintenanceConfig(subReq *SubService, inbound *model.Inbound, client model.Client) json_util.RawMessage {
	remark := subReq.maintenanceRemark(inbound, client)
	if remark == "" {
		return nil
	}
	displayInbound := subscriptionDisplayInbound()
	displayClient := client
	displayClient.ID = maintenanceDisplayUUID
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

func (s *SubJsonService) maintenanceFallbackConfigs(subReq *SubService) []json_util.RawMessage {
	links := subReq.maintenanceFallbackRawLinks()
	out := make([]json_util.RawMessage, 0, len(links))
	for i, raw := range links {
		proxy := parsedExternalOutbound(raw)
		if proxy == nil {
			continue
		}
		outbounds := make([]json_util.RawMessage, 0, len(s.defaultOutbounds)+1)
		outbounds = append(outbounds, proxy)
		outbounds = append(outbounds, s.defaultOutbounds...)
		config := s.baseConfigForRequest(subReq)
		config["outbounds"] = outbounds
		config["remarks"] = fmt.Sprintf("Fallback %d", i+1)
		b, _ := json.MarshalIndent(config, "", "  ")
		out = append(out, b)
	}
	return out
}

func (s *SubClashService) maintenanceProxy(subReq *SubService, inbound *model.Inbound, client model.Client) map[string]any {
	remark := subReq.maintenanceRemark(inbound, client)
	if remark == "" {
		return nil
	}
	return map[string]any{
		"name":    remark,
		"type":    "vless",
		"server":  subscriptionDisplayHostAddress,
		"port":    subscriptionDisplayHostPort,
		"uuid":    maintenanceDisplayUUID,
		"network": "tcp",
		"tls":     false,
		"udp":     true,
	}
}

func (s *SubClashService) maintenanceFallbackProxies(subReq *SubService) []map[string]any {
	links := subReq.maintenanceFallbackRawLinks()
	out := make([]map[string]any, 0, len(links))
	for i, raw := range links {
		if proxy := s.clashProxyFromExternal(raw, fmt.Sprintf("Fallback %d", i+1)); proxy != nil {
			out = append(out, proxy)
		}
	}
	return out
}
