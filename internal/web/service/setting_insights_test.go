package service

import "testing"

func TestMaintenanceAndAnomalySettingsDefaultsAndPersistence(t *testing.T) {
	setupSettingTestDB(t)
	s := &SettingService{}
	settings, err := s.GetAllSetting()
	if err != nil {
		t.Fatal(err)
	}
	if settings.SubMaintenanceEnable || settings.SubMaintenanceMode != "notice" {
		t.Fatalf("maintenance defaults = %#v", settings)
	}
	if settings.AnomalyEnable || settings.AnomalyAction != "alert" || settings.AnomalySpikeMBPerMinute != 1024 || settings.AnomalyHistoryDays != 30 {
		t.Fatalf("anomaly defaults = %#v", settings)
	}

	settings.SubMaintenanceEnable = true
	settings.SubMaintenanceMode = "fallback"
	settings.SubMaintenanceMessage = "Maintenance"
	settings.SubMaintenanceFallbackLinks = "vless://example"
	settings.AnomalyEnable = true
	settings.AnomalyAction = "disable"
	settings.AnomalySpikeMBPerMinute = 256
	settings.AnomalySustainedMBPerMinute = 128
	settings.AnomalySustainedMinutes = 5
	settings.AnomalySharedIPThreshold = 4
	settings.AnomalyActionMinutes = 15
	settings.AnomalyCooldownMinutes = 30
	settings.AnomalyHistoryDays = 120
	if err := s.UpdateAllSetting(settings, SecretClears{}); err != nil {
		t.Fatal(err)
	}

	got, err := s.GetAllSetting()
	if err != nil {
		t.Fatal(err)
	}
	if !got.SubMaintenanceEnable || got.SubMaintenanceMode != "fallback" || got.AnomalyAction != "disable" || got.AnomalySpikeMBPerMinute != 256 || got.AnomalyHistoryDays != 120 {
		t.Fatalf("persisted settings = %#v", got)
	}
}

func TestAnomalyThrottleRequiresDedicatedInbound(t *testing.T) {
	setupSettingTestDB(t)
	s := &SettingService{}
	settings, err := s.GetAllSetting()
	if err != nil {
		t.Fatal(err)
	}
	settings.AnomalyEnable = true
	settings.AnomalyAction = "throttle"
	settings.AnomalyThrottleInboundId = 0
	if err := s.UpdateAllSetting(settings, SecretClears{}); err == nil {
		t.Fatal("throttle action without an inbound ID was accepted")
	}
}
