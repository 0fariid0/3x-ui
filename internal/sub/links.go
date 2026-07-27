package sub

import (
	"strings"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/web/service"
)

type LinkProvider struct {
	settingService service.SettingService
}

func NewLinkProvider() *LinkProvider {
	return &LinkProvider{}
}

func (p *LinkProvider) build(host string) *SubService {
	remarkTemplate, _ := p.settingService.GetRemarkTemplate()
	svc := NewSubService(remarkTemplate)
	svc.PrepareForRequest(host)
	return svc
}

func (p *LinkProvider) SubLinksForSubId(host, subId string) ([]string, error) {
	svc := p.build(host)
	links, _, _, _, err := svc.GetSubs(subId, host)
	if err != nil {
		return nil, err
	}
	out := make([]string, 0, len(links))
	for _, l := range links {
		out = append(out, splitLinkLines(l)...)
	}
	return out, nil
}

func (p *LinkProvider) LinksForClient(host string, inbound *model.Inbound, email string) []string {
	svc := p.build(host)
	svc.projectThroughFallbackMaster(inbound)

	// The direct client-link endpoint must honor the same per-client Host
	// visibility choices as raw/JSON/Clash subscription requests.
	var rec model.ClientRecord
	if err := database.GetDB().Where("email = ?", email).First(&rec).Error; err == nil {
		client := rec.ToClient()
		svc.primeLinkClients(inbound.Id, []model.Client{*client}, false)
		selection := svc.hostEndpointsForClient(inbound, "raw", rec.Id)
		if selection.managed {
			if len(selection.endpoints) == 0 {
				return nil
			}
			return splitLinkLines(svc.linkFromHosts(inbound, *client, selection.endpoints))
		}
		if svc.subscriptionLinkDisabled(rec.Id, model.InboundSubscriptionLinkKey(inbound.Id)) {
			return nil
		}
	}
	return splitLinkLines(svc.GetLink(inbound, email))
}

func (p *LinkProvider) LinksForInbounds(host string, inbounds []*model.Inbound) []string {
	svc := p.build(host)
	var out []string
	for _, inbound := range inbounds {
		out = append(out, svc.inboundLinks(inbound)...)
	}
	return out
}

func splitLinkLines(raw string) []string {
	if raw == "" {
		return nil
	}
	parts := strings.Split(raw, "\n")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}
