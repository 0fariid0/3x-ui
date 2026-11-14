package service

import (
	"bytes"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"
)

func TestRelayConfigKeepsUserManagementAndRestoresMonitoring(t *testing.T) {
	seedWGInbound(t, "relay-test", 51820, []model.Client{
		{Email: "relay-on", Enable: true, PublicKey: "active-key", AllowedIPs: []string{"10.0.0.2/32"}},
		{Email: "relay-off", Enable: false, PublicKey: "disabled-key", AllowedIPs: []string{"10.0.0.3/32"}},
	})
	var inbound model.Inbound
	if err := database.GetDB().Where("tag = ?", "relay-test").First(&inbound).Error; err != nil {
		t.Fatal(err)
	}
	seedClientTraffic(t, inbound.Id, "relay-on", false)
	if err := database.GetDB().Model(&model.ClientRecord{}).Where("email = ?", "relay-off").UpdateColumn("enable", false).Error; err != nil {
		t.Fatal(err)
	}
	s := &SettingService{}
	if err := s.SetRelayLightMode(true); err != nil {
		t.Fatal(err)
	}
	cfg, err := (&XrayService{}).GetXrayConfig()
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.RelayLightMode || len(cfg.Stats) == 0 || len(cfg.Metrics) > 0 ||
		!bytes.Contains(cfg.Policy, []byte(`"statsUserOnline":true`)) ||
		!bytes.Contains(cfg.Policy, []byte(`"statsUserUplink":true`)) || !bytes.Contains(cfg.API, []byte("HandlerService")) {
		t.Fatal("relay runtime must keep user statistics and management while suspending metrics")
	}
	disabled, err := trafficDisabledEmails(database.GetDB(), []string{"relay-on"})
	if err != nil || len(disabled) != 0 {
		t.Fatalf("relay hot edits filtered quota-disabled accounts: %v %v", disabled, err)
	}
	peers := wgPeerList(t, wgInboundEmittedSettings(t, "relay-test"))
	if len(peers) != 1 || peers[0]["publicKey"] != "active-key" {
		t.Fatalf("active user or manual disable not respected: %v", peers)
	}
	if err := s.SetRelayLightMode(false); err != nil {
		t.Fatal(err)
	}
	cfg, err = (&XrayService{}).GetXrayConfig()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.RelayLightMode || !bytes.Contains(cfg.Policy, []byte("statsUserOnline")) {
		t.Fatal("normal monitoring was not restored")
	}
}

func TestRelayTrafficCountsWithoutRunningQuotaOrActivation(t *testing.T) {
	setupSettingTestDB(t)
	row := xray.ClientTraffic{Email: "relay@example", Enable: true, Total: 1, ExpiryTime: -3600000}
	if err := database.GetDB().Create(&row).Error; err != nil {
		t.Fatal(err)
	}
	if err := (&InboundService{}).RecordRelayClientTraffic([]*xray.ClientTraffic{
		{Email: row.Email, Up: 25, Down: 100},
	}); err != nil {
		t.Fatal(err)
	}
	if err := database.GetDB().First(&row, row.Id).Error; err != nil {
		t.Fatal(err)
	}
	if row.Up != 25 || row.Down != 100 || row.LastOnline == 0 || !row.Enable || row.ExpiryTime != -3600000 {
		t.Fatalf("expected usage and last seen only: %+v", row)
	}
}
