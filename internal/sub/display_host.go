package sub

import (
	"maps"
	"strings"

	"github.com/goccy/go-json"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/util/json_util"
)

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
	ctx := remarkContext{
		client:     client,
		stats:      s.statsForClient(inbound, client),
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

func (s *SubService) subscriptionDisplayRawLink(inbound *model.Inbound, client model.Client) string {
	remark := s.subscriptionDisplayRemark(inbound, client)
	if remark == "" {
		return ""
	}
	params := map[string]string{
		"encryption": "none",
		"security":   "none",
		"type":       "tcp",
	}
	base := "vless://" + subscriptionDisplayHostUUID + "@" + subscriptionDisplayHostAddress + ":1"
	return buildLinkWithParams(base, params, remark)
}

func (s *SubJsonService) subscriptionDisplayConfig(subReq *SubService, inbound *model.Inbound, client model.Client) json_util.RawMessage {
	remark := subReq.subscriptionDisplayRemark(inbound, client)
	if remark == "" {
		return nil
	}
	displayInbound := subscriptionDisplayInbound()
	displayClient := client
	displayClient.ID = subscriptionDisplayHostUUID
	displayClient.Flow = ""

	stream := json_util.RawMessage(displayInbound.StreamSettings)
	outbound := s.genVless(subReq, displayInbound, stream, displayClient, "")
	outbounds := make([]json_util.RawMessage, 0, len(s.defaultOutbounds)+1)
	outbounds = append(outbounds, outbound)
	outbounds = append(outbounds, s.defaultOutbounds...)

	config := make(map[string]any)
	maps.Copy(config, s.configJson)
	config["outbounds"] = outbounds
	config["remarks"] = remark
	result, _ := json.MarshalIndent(config, "", "  ")
	return result
}

func (s *SubClashService) subscriptionDisplayProxy(subReq *SubService, inbound *model.Inbound, client model.Client) map[string]any {
	remark := subReq.subscriptionDisplayRemark(inbound, client)
	if remark == "" {
		return nil
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
