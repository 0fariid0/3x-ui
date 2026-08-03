package service

import (
	"testing"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"
)

func TestDetectClientOS(t *testing.T) {
	tests := map[string]string{
		"HiddifyNext/2.6 Android 14": "Android",
		"Streisand iPhone iOS/18":    "iOS/iPadOS",
		"v2rayN Windows NT 10.0":     "Windows",
		"FoXray Macintosh Mac OS":    "macOS",
		"NekoBox Linux":              "Linux",
		"unknown-client":             "",
	}
	for ua, want := range tests {
		if got := detectClientOS(ua); got != want {
			t.Errorf("detectClientOS(%q) = %q, want %q", ua, got, want)
		}
	}
}

func TestEqualIntSetsIgnoresOrderAndDuplicates(t *testing.T) {
	if !equalIntSets([]int{3, 1, 3, 2}, []int{2, 1, 3}) {
		t.Fatal("equalIntSets should ignore order and duplicates")
	}
	if equalIntSets([]int{1, 2}, []int{1, 3}) {
		t.Fatal("equalIntSets accepted different sets")
	}
}

func TestClientInsightReportAggregatesUsageAndRecentMetadata(t *testing.T) {
	setupSettingTestDB(t)
	db := database.GetDB()
	now := time.Now()
	email := "report@example.com"
	rec := &model.ClientRecord{Email: email, SubID: "report-sub", UUID: "33333333-3333-4333-8333-333333333333", Enable: true}
	if err := db.Create(rec).Error; err != nil {
		t.Fatal(err)
	}
	inbound := &model.Inbound{UserId: 1, Tag: "report-inbound", Enable: true, Listen: "203.0.113.9", Port: 443, Protocol: model.VLESS, Remark: "Report", Settings: `{"clients":[]}`, StreamSettings: `{"network":"tcp","security":"none"}`}
	if err := db.Create(inbound).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&model.ClientInbound{ClientId: rec.Id, InboundId: inbound.Id}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&model.Host{InboundId: inbound.Id, Remark: "Edge A", Address: "edge.example.com", Port: 443}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&xray.ClientTraffic{Email: email, Enable: true, LastOnline: now.UnixMilli()}).Error; err != nil {
		t.Fatal(err)
	}
	localMidnight := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	buckets := []model.ClientTrafficBucket{
		{Email: email, BucketStart: localMidnight.AddDate(0, 0, -1).Add(12 * time.Hour).UnixMilli(), Up: 100, Down: 200},
		{Email: email, BucketStart: now.Truncate(time.Minute).UnixMilli(), Up: 300, Down: 400},
	}
	if err := db.Create(&buckets).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&model.ClientIPHistory{Email: email, IP: "198.51.100.10", FirstSeen: now.UnixMilli(), LastSeen: now.UnixMilli(), SeenCount: 2}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&model.ClientIPHistory{Email: email, IP: "198.51.100.11", FirstSeen: now.AddDate(0, 0, -10).UnixMilli(), LastSeen: now.AddDate(0, 0, -10).UnixMilli(), SeenCount: 1}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&model.ClientSubscriptionAgent{ClientId: rec.Id, AppKey: "hiddify", AppName: "Hiddify", UserAgent: "Hiddify Android", FirstSeen: now.UnixMilli(), LastSeen: now.UnixMilli()}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&model.ClientEvent{Email: email, Kind: "renewed", Summary: "Renewed"}).Error; err != nil {
		t.Fatal(err)
	}

	report, err := (&ClientInsightService{}).GetReport(email, 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(report.DailyUsage) != 2 || report.DailyUsage[0].Total+report.DailyUsage[1].Total != 1000 {
		t.Fatalf("daily usage = %#v, want two days totaling 1000", report.DailyUsage)
	}
	if report.RecentIPCount != 1 || len(report.RecentIPs) != 1 {
		t.Fatalf("recent IPs = %d/%d, want 1/1", report.RecentIPCount, len(report.RecentIPs))
	}
	if len(report.Apps) != 1 || report.Apps[0].OS != "Android" {
		t.Fatalf("apps = %#v, want Android metadata", report.Apps)
	}
	if len(report.Hosts) != 1 || report.Hosts[0].Address != "edge.example.com" {
		t.Fatalf("hosts = %#v, want attached host", report.Hosts)
	}
	if len(report.Events) != 1 || report.Events[0].Kind != "renewed" {
		t.Fatalf("events = %#v, want renewal event", report.Events)
	}
	if report.TotalUp != 400 || report.TotalDown != 600 || report.TotalUsage != 1000 {
		t.Fatalf("totals up/down/all = %d/%d/%d, want 400/600/1000", report.TotalUp, report.TotalDown, report.TotalUsage)
	}
	if report.PeakMinuteBytes != 700 || report.ActiveDays != 2 || report.ActiveMinutes != 2 {
		t.Fatalf("peak/active = %d/%d/%d, want 700/2/2", report.PeakMinuteBytes, report.ActiveDays, report.ActiveMinutes)
	}
	currentHour := now.Hour()
	if report.HourlyUsage[currentHour].Up != 300 || report.HourlyUsage[currentHour].Down != 400 || report.HourlyUsage[currentHour].Total != 700 {
		t.Fatalf("current hour = %#v, want up/down/total 300/400/700", report.HourlyUsage[currentHour])
	}
}

func TestUsageAlertsOrdersHighConsumersAndIncludesAnomalies(t *testing.T) {
	setupSettingTestDB(t)
	db := database.GetDB()
	now := time.Now().Truncate(time.Minute)
	for _, rec := range []model.ClientRecord{
		{Email: "heavy@example.com", SubID: "heavy", UUID: "44444444-4444-4444-8444-444444444444", Enable: true, TotalGB: 10_000},
		{Email: "light@example.com", SubID: "light", UUID: "55555555-5555-4555-8555-555555555555", Enable: true, TotalGB: 10_000},
	} {
		copy := rec
		if err := db.Create(&copy).Error; err != nil {
			t.Fatal(err)
		}
	}
	if err := db.Create(&[]model.ClientTrafficBucket{
		{Email: "heavy@example.com", BucketStart: now.UnixMilli(), Up: 700, Down: 800},
		{Email: "light@example.com", BucketStart: now.UnixMilli(), Up: 100, Down: 200},
	}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&model.ClientAnomaly{Email: "heavy@example.com", Kind: "spike", Status: "open", Action: "alert", CreatedAt: now.UnixMilli()}).Error; err != nil {
		t.Fatal(err)
	}
	alerts, err := (&ClientInsightService{}).GetUsageAlerts(7, 8)
	if err != nil {
		t.Fatal(err)
	}
	if len(alerts.Items) != 2 || alerts.Items[0].Email != "heavy@example.com" {
		t.Fatalf("alerts = %#v, want heavy client first", alerts.Items)
	}
	if alerts.Items[0].AnomalyCount != 1 || alerts.Items[0].Severity != "critical" {
		t.Fatalf("heavy alert anomaly/severity = %d/%s, want 1/critical", alerts.Items[0].AnomalyCount, alerts.Items[0].Severity)
	}
}

func TestInsightCleanupPreservesActiveActionAndItsManualChangeHistory(t *testing.T) {
	setupSettingTestDB(t)
	db := database.GetDB()
	now := time.Now()
	email := "active-action@example.com"
	activeAt := now.AddDate(0, 0, -10).UnixMilli()
	active := &model.ClientAnomaly{
		Email: email, Kind: "spike", Status: "acted", Action: "disable",
		CreatedAt: activeAt, ActionUntil: now.Add(24 * time.Hour).UnixMilli(),
	}
	if err := db.Create(active).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&model.ClientAnomaly{Email: email, Kind: "sharing", Status: "resolved", CreatedAt: activeAt}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&model.ClientEvent{Email: email, Kind: "updated", Summary: "Manual change", CreatedAt: now.AddDate(0, 0, -9).UnixMilli()}).Error; err != nil {
		t.Fatal(err)
	}

	if err := (&ClientInsightService{}).Cleanup(1); err != nil {
		t.Fatal(err)
	}
	var activeCount, resolvedCount, eventCount int64
	_ = db.Model(&model.ClientAnomaly{}).Where("id = ?", active.Id).Count(&activeCount).Error
	_ = db.Model(&model.ClientAnomaly{}).Where("email = ? AND status = ?", email, "resolved").Count(&resolvedCount).Error
	_ = db.Model(&model.ClientEvent{}).Where("email = ? AND kind = ?", email, "updated").Count(&eventCount).Error
	if activeCount != 1 || resolvedCount != 0 || eventCount != 1 {
		t.Fatalf("cleanup counts active/resolved/event = %d/%d/%d, want 1/0/1", activeCount, resolvedCount, eventCount)
	}
}

func TestClientInsightRollingTwelveHourTimeline(t *testing.T) {
	setupSettingTestDB(t)
	db := database.GetDB()
	email := "rolling@example.com"
	if err := db.Create(&model.ClientRecord{
		Email: email, SubID: "rolling", UUID: "66666666-6666-4666-8666-666666666666", Enable: true,
	}).Error; err != nil {
		t.Fatal(err)
	}

	now := time.Now().Truncate(time.Hour).Add(20 * time.Minute)
	service := &ClientInsightService{}
	if err := service.RecordTraffic([]*xray.ClientTraffic{{Email: email, Up: 100, Down: 200}}, now.Add(-11*time.Hour)); err != nil {
		t.Fatal(err)
	}
	if err := service.RecordTraffic([]*xray.ClientTraffic{{Email: email, Up: 300, Down: 400}}, now); err != nil {
		t.Fatal(err)
	}

	report, err := service.GetReportForRange(email, 30, 12)
	if err != nil {
		t.Fatal(err)
	}
	if report.Hours != 12 || len(report.TimelineUsage) != 12 {
		t.Fatalf("rolling range hours/points = %d/%d, want 12/12", report.Hours, len(report.TimelineUsage))
	}
	if report.TotalUp != 400 || report.TotalDown != 600 || report.TotalUsage != 1000 {
		t.Fatalf("rolling totals = %d/%d/%d, want 400/600/1000", report.TotalUp, report.TotalDown, report.TotalUsage)
	}
	if report.TimelineUsage[0].Total != 300 || report.TimelineUsage[len(report.TimelineUsage)-1].Total != 700 {
		t.Fatalf("rolling endpoints = %#v ... %#v", report.TimelineUsage[0], report.TimelineUsage[len(report.TimelineUsage)-1])
	}
}
