package service

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

func setupClientSubscriptionTestDB(t *testing.T) {
	t.Helper()
	if err := database.InitDB(filepath.Join(t.TempDir(), "x-ui.db")); err != nil {
		t.Fatalf("InitDB: %v", err)
	}
	t.Cleanup(func() {
		if err := database.CloseDB(); err != nil {
			t.Fatalf("CloseDB: %v", err)
		}
	})
}

func TestDetectSubscriptionApp(t *testing.T) {
	tests := []struct {
		ua      string
		wantKey string
		wantVer string
		ok      bool
	}{
		{ua: "v2rayNG/1.9.31", wantKey: "v2rayng", wantVer: "1.9.31", ok: true},
		{ua: "HiddifyNext/2.5.7 (Android)", wantKey: "hiddify", wantVer: "2.5.7", ok: true},
		{ua: "Clash-Verge-Rev/2.2.3", wantKey: "clash-verge", wantVer: "2.2.3", ok: true},
		{ua: "Mozilla/5.0 AppleWebKit/537.36", ok: false},
		{ua: "", ok: false},
	}
	for _, tc := range tests {
		t.Run(tc.ua, func(t *testing.T) {
			got, ok := detectSubscriptionApp(tc.ua)
			if ok != tc.ok {
				t.Fatalf("ok=%v, want %v (%+v)", ok, tc.ok, got)
			}
			if !ok {
				return
			}
			if got.Key != tc.wantKey || got.Version != tc.wantVer {
				t.Fatalf("identity=%+v, want key=%q version=%q", got, tc.wantKey, tc.wantVer)
			}
		})
	}
}

func TestRecordSubscriptionAccessKeepsThreeRecentDistinctApps(t *testing.T) {
	setupClientSubscriptionTestDB(t)
	rec := &model.ClientRecord{Email: "alice@example.com", SubID: "sub-alice", UUID: "11111111-2222-4333-8444-555555555555", Enable: true}
	if err := database.GetDB().Create(rec).Error; err != nil {
		t.Fatalf("create client: %v", err)
	}

	svc := &ClientService{}
	requests := []struct {
		ua     string
		format string
	}{
		{"v2rayNG/1.9.31", "raw"},
		{"HiddifyNext/2.5.7", "json"},
		{"Streisand/1.6.0", "raw"},
		{"Shadowrocket/2.2.60", "clash"},
	}
	for _, request := range requests {
		if err := svc.RecordSubscriptionAccess(rec.SubID, request.ua, request.format); err != nil {
			t.Fatalf("RecordSubscriptionAccess(%q): %v", request.ua, err)
		}
	}

	rows, err := svc.GetSubscriptionAppsByEmail(rec.Email)
	if err != nil {
		t.Fatalf("GetSubscriptionAppsByEmail: %v", err)
	}
	if len(rows) != 3 {
		t.Fatalf("rows=%d, want 3: %#v", len(rows), rows)
	}
	seen := map[string]bool{}
	for _, row := range rows {
		seen[row.AppKey] = true
	}
	if seen["v2rayng"] {
		t.Fatalf("oldest app should be evicted: %#v", rows)
	}
	for _, key := range []string{"hiddify", "streisand", "shadowrocket"} {
		if !seen[key] {
			t.Fatalf("missing app %q: %#v", key, rows)
		}
	}

	// Refreshing an existing app moves it to the front and increments its count.
	if err := svc.RecordSubscriptionAccess(rec.SubID, "HiddifyNext/2.6.0", "raw"); err != nil {
		t.Fatal(err)
	}
	rows, err = svc.GetSubscriptionAppsByEmail(rec.Email)
	if err != nil {
		t.Fatal(err)
	}
	if rows[0].AppKey != "hiddify" || rows[0].Version != "2.6.0" || rows[0].RequestCount != 2 {
		t.Fatalf("updated row=%+v, want recent Hiddify v2.6.0 count=2", rows[0])
	}
}

func TestSubscriptionLinkOptionsAndExclusions(t *testing.T) {
	setupClientSubscriptionTestDB(t)
	db := database.GetDB()
	rec := &model.ClientRecord{Email: "bob@example.com", SubID: "sub-bob", UUID: "aaaaaaaa-bbbb-4ccc-8ddd-eeeeeeeeeeee", Enable: true}
	if err := db.Create(rec).Error; err != nil {
		t.Fatal(err)
	}
	inbound := &model.Inbound{
		UserId: 1, Tag: "vip-ws", Remark: "VIP WS", Enable: true, Listen: "0.0.0.0", Port: 443,
		Protocol: model.VLESS, Settings: `{"clients":[],"decryption":"none"}`, StreamSettings: `{"network":"ws","security":"tls"}`,
	}
	if err := db.Create(inbound).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&model.ClientInbound{ClientId: rec.Id, InboundId: inbound.Id}).Error; err != nil {
		t.Fatal(err)
	}
	hosts := []*model.Host{
		{GroupId: "vip-pool", InboundId: inbound.Id, SortOrder: 1, Remark: "Germany 1", Address: "1.1.1.1", Port: 0},
		{GroupId: "vip-pool", InboundId: inbound.Id, SortOrder: 2, Remark: "Germany 2", Address: "2.2.2.2", Port: 8443},
		{GroupId: "private-pool", InboundId: inbound.Id, SortOrder: 3, Remark: "Private", Address: "3.3.3.3", Port: 0, IsDisabled: true},
	}
	for _, host := range hosts {
		if err := db.Create(host).Error; err != nil {
			t.Fatal(err)
		}
	}

	svc := &ClientService{}
	options, err := svc.GetSubscriptionLinkOptionsByEmail(rec.Email, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(options) != 3 {
		t.Fatalf("options=%d, want 3: %#v", len(options), options)
	}
	if options[0].Name != "Germany 1" || options[0].Address != "1.1.1.1" || options[0].Port != 443 {
		t.Fatalf("first option=%+v", options[0])
	}
	if options[1].Name != "Germany 2" || options[1].Address != "2.2.2.2" || options[1].Port != 8443 {
		t.Fatalf("second option=%+v", options[1])
	}
	if options[2].GloballyEnabled || options[2].Enabled {
		t.Fatalf("globally disabled host must default off: %+v", options[2])
	}

	disabledKey := options[0].Key
	if !strings.HasPrefix(disabledKey, "host:vip-pool:") {
		t.Fatalf("unstable host key: %q", disabledKey)
	}
	if err := svc.SetSubscriptionLinkExclusionsByEmail(rec.Email, []string{disabledKey, options[2].Key}); err != nil {
		t.Fatal(err)
	}
	options, err = svc.GetSubscriptionLinkOptionsByEmail(rec.Email, nil)
	if err != nil {
		t.Fatal(err)
	}
	if options[0].Enabled || !options[1].Enabled || options[2].Enabled {
		t.Fatalf("unexpected visibility: %#v", options)
	}

	// The UI submits every switch that is off. Keep the first host on, and turn
	// the globally disabled third host on by omitting its key from disabledKeys.
	if err := svc.SetSubscriptionLinkExclusionsByEmail(rec.Email, nil); err != nil {
		t.Fatal(err)
	}
	options, err = svc.GetSubscriptionLinkOptionsByEmail(rec.Email, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !options[0].Enabled || !options[1].Enabled || !options[2].Enabled {
		t.Fatalf("all switches should now be enabled: %#v", options)
	}
	var inclusionCount int64
	if err := db.Model(&model.ClientSubscriptionLinkInclusion{}).Where("client_id = ?", rec.Id).Count(&inclusionCount).Error; err != nil {
		t.Fatal(err)
	}
	if inclusionCount != 1 {
		t.Fatalf("inclusion count=%d, want 1", inclusionCount)
	}

	// Turning the private host off again writes no inclusion and keeps globally
	// enabled hosts on by default.
	if err := svc.SetSubscriptionLinkExclusionsByEmail(rec.Email, []string{options[2].Key}); err != nil {
		t.Fatal(err)
	}
	var count int64
	if err := db.Model(&model.ClientSubscriptionLinkExclusion{}).Where("client_id = ?", rec.Id).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("exclusion count=%d, want 0", count)
	}
}
