package service

import (
	"fmt"
	"strings"
	"time"
	"unicode"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const maxRecentSubscriptionApps = 3

// ClientSubscriptionLinkOption describes one managed subscription output that
// can be enabled or hidden for a specific client.
type ClientSubscriptionLinkOption struct {
	Key         string   `json:"key"`
	Name        string   `json:"name"`
	Address     string   `json:"address"`
	Port        int      `json:"port"`
	InboundID   int      `json:"inboundId"`
	InboundName string   `json:"inboundName"`
	Protocol    string   `json:"protocol"`
	Source      string   `json:"source"` // host|inbound
	Formats     []string `json:"formats"`
	Enabled     bool     `json:"enabled"`
}

type subscriptionAppIdentity struct {
	Key     string
	Name    string
	Version string
}

type subscriptionAppSignature struct {
	key     string
	name    string
	markers []string
}

var subscriptionAppSignatures = []subscriptionAppSignature{
	{key: "clash-verge", name: "Clash Verge", markers: []string{"clash-verge-rev", "clash-verge"}},
	{key: "clash-meta-android", name: "Clash Meta for Android", markers: []string{"clashmetaforandroid", "clash meta for android"}},
	{key: "clash-android", name: "Clash for Android", markers: []string{"clashforandroid", "clash for android"}},
	{key: "flclash", name: "FlClash", markers: []string{"flclash"}},
	{key: "mihomo", name: "Mihomo", markers: []string{"mihomo", "clash-meta"}},
	{key: "hiddify", name: "Hiddify", markers: []string{"hiddify-next", "hiddifynext", "hiddify"}},
	{key: "v2rayng", name: "v2rayNG", markers: []string{"v2rayng"}},
	{key: "v2rayn", name: "v2rayN", markers: []string{"v2rayn"}},
	{key: "nekobox", name: "NekoBox", markers: []string{"nekobox"}},
	{key: "nekoray", name: "NekoRay", markers: []string{"nekoray"}},
	{key: "streisand", name: "Streisand", markers: []string{"streisand"}},
	{key: "shadowrocket", name: "Shadowrocket", markers: []string{"shadowrocket"}},
	{key: "sing-box", name: "sing-box", markers: []string{"sing-box", "singbox"}},
	{key: "karing", name: "Karing", markers: []string{"karing"}},
	{key: "happ", name: "Happ", markers: []string{"happ"}},
	{key: "v2box", name: "V2Box", markers: []string{"v2box"}},
	{key: "oneclick", name: "OneClick", markers: []string{"oneclick"}},
	{key: "foxray", name: "FoXray", markers: []string{"foxray"}},
	{key: "fair", name: "Fair", markers: []string{"fair-vpn", "fairvpn"}},
	{key: "sagernet", name: "SagerNet", markers: []string{"sagernet"}},
	{key: "matsuri", name: "Matsuri", markers: []string{"matsuri"}},
	{key: "kitsunebi", name: "Kitsunebi", markers: []string{"kitsunebi"}},
	{key: "napsternetv", name: "NapsternetV", markers: []string{"napsternetv"}},
	{key: "surfboard", name: "Surfboard", markers: []string{"surfboard"}},
}

func detectSubscriptionApp(userAgent string) (subscriptionAppIdentity, bool) {
	userAgent = strings.TrimSpace(userAgent)
	if userAgent == "" {
		return subscriptionAppIdentity{}, false
	}
	if len(userAgent) > 512 {
		userAgent = userAgent[:512]
	}
	lower := strings.ToLower(userAgent)
	for _, sig := range subscriptionAppSignatures {
		for _, marker := range sig.markers {
			if idx := strings.Index(lower, marker); idx >= 0 {
				return subscriptionAppIdentity{Key: sig.key, Name: sig.name, Version: versionAfterMarker(userAgent, idx+len(marker))}, true
			}
		}
	}

	// Browser visits to the human subscription page must not consume one of the
	// three app slots. Known VPN apps above are detected before this guard.
	if strings.Contains(lower, "mozilla/") || strings.Contains(lower, "applewebkit/") {
		return subscriptionAppIdentity{}, false
	}

	first := strings.Fields(userAgent)
	if len(first) == 0 {
		return subscriptionAppIdentity{}, false
	}
	token := strings.Trim(first[0], "()[]{}")
	name, version := token, ""
	if slash := strings.Index(token, "/"); slash > 0 {
		name, version = token[:slash], token[slash+1:]
	}
	name = cleanAppToken(name, 64)
	version = cleanAppToken(version, 64)
	if len(name) < 2 {
		return subscriptionAppIdentity{}, false
	}
	key := strings.ToLower(name)
	return subscriptionAppIdentity{Key: key, Name: name, Version: version}, true
}

func versionAfterMarker(userAgent string, start int) string {
	if start >= len(userAgent) {
		return ""
	}
	rest := strings.TrimLeftFunc(userAgent[start:], func(r rune) bool {
		return r == '/' || r == '-' || r == '_' || unicode.IsSpace(r)
	})
	for i, r := range rest {
		if unicode.IsSpace(r) || r == ';' || r == ')' || r == '(' || r == ',' {
			rest = rest[:i]
			break
		}
	}
	return cleanAppToken(rest, 64)
}

func cleanAppToken(value string, max int) string {
	value = strings.TrimSpace(value)
	value = strings.Trim(value, "()[]{};,\"")
	if len(value) > max {
		value = value[:max]
	}
	return value
}

// RecordSubscriptionAccess stores the last use of a subscription by app. It is
// deliberately best-effort: callers should log errors but never fail a
// subscription response because telemetry could not be written.
func (s *ClientService) RecordSubscriptionAccess(subID, userAgent, format string) error {
	identity, ok := detectSubscriptionApp(userAgent)
	if !ok || strings.TrimSpace(subID) == "" {
		return nil
	}
	format = strings.ToLower(strings.TrimSpace(format))
	if format != "raw" && format != "json" && format != "clash" {
		format = "raw"
	}
	ua := strings.TrimSpace(userAgent)
	if len(ua) > 512 {
		ua = ua[:512]
	}

	var clients []model.ClientRecord
	if err := database.GetDB().Select("id").Where("sub_id = ?", subID).Find(&clients).Error; err != nil {
		return err
	}
	if len(clients) == 0 {
		return nil
	}
	now := time.Now().UnixMilli()
	return database.GetDB().Transaction(func(tx *gorm.DB) error {
		for _, client := range clients {
			row := model.ClientSubscriptionAgent{
				ClientId:     client.Id,
				AppKey:       identity.Key,
				AppName:      identity.Name,
				Version:      identity.Version,
				UserAgent:    ua,
				Format:       format,
				RequestCount: 1,
				FirstSeen:    now,
				LastSeen:     now,
			}
			// One atomic upsert works on both SQLite and PostgreSQL and avoids a
			// unique-key race when an app refreshes the same URL concurrently.
			if err := tx.Clauses(clause.OnConflict{
				Columns: []clause.Column{{Name: "client_id"}, {Name: "app_key"}},
				DoUpdates: clause.Assignments(map[string]any{
					"app_name":   identity.Name,
					"version":    identity.Version,
					"user_agent": ua,
					"format":     format,
					// Keep the recent-app order deterministic even when two refreshes
					// land inside the same millisecond.
					"last_seen":     gorm.Expr("CASE WHEN last_seen >= ? THEN last_seen + 1 ELSE ? END", now, now),
					"request_count": gorm.Expr("request_count + 1"),
				}),
			}).Create(&row).Error; err != nil {
				return err
			}

			var staleIDs []int
			if err := tx.Model(&model.ClientSubscriptionAgent{}).
				Where("client_id = ?", client.Id).
				Order("last_seen DESC, id DESC").
				Offset(maxRecentSubscriptionApps).
				Pluck("id", &staleIDs).Error; err != nil {
				return err
			}
			if len(staleIDs) > 0 {
				if err := tx.Where("id IN ?", staleIDs).Delete(&model.ClientSubscriptionAgent{}).Error; err != nil {
					return err
				}
			}
		}
		return nil
	})
}

func (s *ClientService) GetSubscriptionAppsByEmail(email string) ([]model.ClientSubscriptionAgent, error) {
	rec, err := s.GetRecordByEmail(nil, email)
	if err != nil {
		return nil, err
	}
	var rows []model.ClientSubscriptionAgent
	err = database.GetDB().Where("client_id = ?", rec.Id).
		Order("last_seen DESC, id DESC").Limit(maxRecentSubscriptionApps).Find(&rows).Error
	return rows, err
}

func subscriptionFormats(excluded []string) []string {
	blocked := make(map[string]struct{}, len(excluded))
	for _, format := range excluded {
		blocked[strings.ToLower(strings.TrimSpace(format))] = struct{}{}
	}
	out := make([]string, 0, 3)
	for _, format := range []string{"raw", "json", "clash"} {
		if _, skip := blocked[format]; !skip {
			out = append(out, format)
		}
	}
	return out
}

func inboundDisplayName(inbound *model.Inbound) string {
	if name := strings.TrimSpace(inbound.Remark); name != "" {
		return name
	}
	if name := strings.TrimSpace(inbound.Tag); name != "" {
		return name
	}
	return fmt.Sprintf("Inbound %d", inbound.Id)
}

func inboundDisplayAddress(inbound *model.Inbound) string {
	if value := strings.TrimSpace(inbound.ShareAddr); value != "" {
		return value
	}
	value := strings.TrimSpace(inbound.Listen)
	if value == "" || value == "0.0.0.0" || value == "::" {
		return "default"
	}
	return value
}

// GetSubscriptionLinkOptionsByEmail returns the links generated from managed
// Hosts (or the inbound default when no Host exists) together with the current
// per-client visibility state. inboundIDs optionally previews a changed set of
// attachments before the client form is saved.
func (s *ClientService) GetSubscriptionLinkOptionsByEmail(email string, inboundIDs []int) ([]ClientSubscriptionLinkOption, error) {
	rec, err := s.GetRecordByEmail(nil, email)
	if err != nil {
		return nil, err
	}
	if len(inboundIDs) == 0 {
		inboundIDs, err = s.GetInboundIdsForRecord(rec.Id)
		if err != nil {
			return nil, err
		}
	}
	if len(inboundIDs) == 0 {
		return []ClientSubscriptionLinkOption{}, nil
	}

	var inbounds []*model.Inbound
	if err := database.GetDB().Where("id IN ? AND enable = ?", inboundIDs, true).
		Order("sub_sort_index ASC, id ASC").Find(&inbounds).Error; err != nil {
		return nil, err
	}
	var hosts []*model.Host
	if err := database.GetDB().Where("inbound_id IN ? AND is_disabled = ?", inboundIDs, false).
		Order("sort_order ASC, id ASC").Find(&hosts).Error; err != nil {
		return nil, err
	}
	hostsByInbound := make(map[int][]*model.Host)
	for _, host := range hosts {
		hostsByInbound[host.InboundId] = append(hostsByInbound[host.InboundId], host)
	}
	var exclusions []model.ClientSubscriptionLinkExclusion
	if err := database.GetDB().Where("client_id = ?", rec.Id).Find(&exclusions).Error; err != nil {
		return nil, err
	}
	disabled := make(map[string]struct{}, len(exclusions))
	for _, exclusion := range exclusions {
		disabled[exclusion.LinkKey] = struct{}{}
	}

	options := make([]ClientSubscriptionLinkOption, 0)
	for _, inbound := range inbounds {
		inboundName := inboundDisplayName(inbound)
		rows := hostsByInbound[inbound.Id]
		if len(rows) == 0 {
			key := model.InboundSubscriptionLinkKey(inbound.Id)
			_, isDisabled := disabled[key]
			options = append(options, ClientSubscriptionLinkOption{
				Key: key, Name: inboundName, Address: inboundDisplayAddress(inbound), Port: inbound.Port,
				InboundID: inbound.Id, InboundName: inboundName, Protocol: string(inbound.Protocol),
				Source: "inbound", Formats: []string{"raw", "json", "clash"}, Enabled: !isDisabled,
			})
			continue
		}
		for _, host := range rows {
			formats := subscriptionFormats(host.ExcludeFromSubTypes)
			if len(formats) == 0 {
				continue
			}
			key := host.SubscriptionLinkKey()
			_, isDisabled := disabled[key]
			name := strings.TrimSpace(host.Remark)
			if name == "" {
				name = inboundName
			}
			address := strings.TrimSpace(host.Address)
			if address == "" {
				address = inboundDisplayAddress(inbound)
			}
			port := host.Port
			if port == 0 {
				port = inbound.Port
			}
			options = append(options, ClientSubscriptionLinkOption{
				Key: key, Name: name, Address: address, Port: port,
				InboundID: inbound.Id, InboundName: inboundName, Protocol: string(inbound.Protocol),
				Source: "host", Formats: formats, Enabled: !isDisabled,
			})
		}
	}
	return options, nil
}

func (s *ClientService) SetSubscriptionLinkExclusionsByEmail(email string, disabledKeys []string) error {
	rec, err := s.GetRecordByEmail(nil, email)
	if err != nil {
		return err
	}
	options, err := s.GetSubscriptionLinkOptionsByEmail(email, nil)
	if err != nil {
		return err
	}
	allowed := make(map[string]struct{}, len(options))
	for _, option := range options {
		allowed[option.Key] = struct{}{}
	}
	unique := make(map[string]struct{}, len(disabledKeys))
	for _, key := range disabledKeys {
		key = strings.TrimSpace(key)
		if key == "" {
			continue
		}
		if _, ok := allowed[key]; !ok {
			return fmt.Errorf("subscription link is not available for this client: %s", key)
		}
		unique[key] = struct{}{}
	}
	return database.GetDB().Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("client_id = ?", rec.Id).Delete(&model.ClientSubscriptionLinkExclusion{}).Error; err != nil {
			return err
		}
		if len(unique) == 0 {
			return nil
		}
		rows := make([]model.ClientSubscriptionLinkExclusion, 0, len(unique))
		for key := range unique {
			rows = append(rows, model.ClientSubscriptionLinkExclusion{ClientId: rec.Id, LinkKey: key})
		}
		return tx.Create(&rows).Error
	})
}
