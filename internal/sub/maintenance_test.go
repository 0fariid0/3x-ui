package sub

import (
	"strings"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/web/service"
)

const maintenanceFallbackLink = "vless://22222222-2222-4222-8222-222222222222@fallback.example.com:443?encryption=none&security=none&type=tcp#backup"

func setMaintenanceSettings(t *testing.T, enabled bool, mode, message, fallbacks string) {
	t.Helper()
	settingsService := &service.SettingService{}
	settings, err := settingsService.GetAllSetting()
	if err != nil {
		t.Fatalf("GetAllSetting: %v", err)
	}
	settings.SubMaintenanceEnable = enabled
	settings.SubMaintenanceMode = mode
	settings.SubMaintenanceMessage = message
	settings.SubMaintenanceFallbackLinks = fallbacks
	if err := settingsService.UpdateAllSetting(settings, service.SecretClears{}); err != nil {
		t.Fatalf("UpdateAllSetting: %v", err)
	}
}

func TestMaintenanceNoticePrecedesRegularSubscription(t *testing.T) {
	seedSubDB(t)
	seedSubInbound(t, "maintenance-notice", "primary", 4701, 1, `{"network":"tcp","security":"none"}`)
	setMaintenanceSettings(t, true, "notice", "UPDATING-{{EMAIL}}", "")

	links, _, _, _, err := NewSubService("").GetSubs("maintenance-notice", "sub.example.com")
	if err != nil {
		t.Fatalf("GetSubs: %v", err)
	}
	flat := splitLinkLines(strings.Join(links, "\n"))
	if len(flat) != 2 {
		t.Fatalf("links = %d, want maintenance + regular: %v", len(flat), flat)
	}
	if !strings.Contains(flat[0], maintenanceDisplayUUID) || !strings.Contains(flat[0], "UPDATING-") {
		t.Fatalf("first link is not the maintenance notice: %s", flat[0])
	}
	if !strings.Contains(flat[1], "203.0.113.5:4701") {
		t.Fatalf("regular link must remain after the notice: %s", flat[1])
	}
}

func TestMaintenanceFallbackWorksWithoutNoticeAndReplacesRegularLinks(t *testing.T) {
	seedSubDB(t)
	seedSubInbound(t, "maintenance-fallback", "primary", 4702, 1, `{"network":"tcp","security":"none"}`)
	setMaintenanceSettings(t, true, "fallback", "", "not-a-link\n"+maintenanceFallbackLink)

	links, _, _, _, err := NewSubService("").GetSubs("maintenance-fallback", "sub.example.com")
	if err != nil {
		t.Fatalf("GetSubs: %v", err)
	}
	flat := splitLinkLines(strings.Join(links, "\n"))
	if len(flat) != 1 || flat[0] != maintenanceFallbackLink {
		t.Fatalf("raw fallback = %v, want only validated fallback", flat)
	}
	if strings.Contains(strings.Join(flat, "\n"), "203.0.113.5:4702") {
		t.Fatalf("fallback mode leaked the regular inbound: %v", flat)
	}

	jsonOut, _, err := NewSubJsonService("", "", "", NewSubService("")).GetJson("maintenance-fallback", "sub.example.com", true)
	if err != nil {
		t.Fatalf("GetJson: %v", err)
	}
	if !strings.Contains(jsonOut, `"address": "fallback.example.com"`) || strings.Contains(jsonOut, `"address": "203.0.113.5"`) {
		t.Fatalf("JSON fallback output is wrong: %s", jsonOut)
	}

	clashOut, _, err := NewSubClashService(false, "", NewSubService("")).GetClash("maintenance-fallback", "sub.example.com")
	if err != nil {
		t.Fatalf("GetClash: %v", err)
	}
	if !strings.Contains(clashOut, "server: fallback.example.com") || strings.Contains(clashOut, "server: 203.0.113.5") {
		t.Fatalf("Clash fallback output is wrong: %s", clashOut)
	}
}
