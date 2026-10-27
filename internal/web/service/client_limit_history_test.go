package service

import (
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"
	"gorm.io/gorm"
)

func limitHistory(t *testing.T, email string) []model.ClientEvent {
	t.Helper()
	var events []model.ClientEvent
	if err := database.GetDB().Where("email = ?", email).Order("id").Find(&events).Error; err != nil {
		t.Fatal(err)
	}
	return events
}

func TestLimitHistoryManualAndLegacyAPI(t *testing.T) {
	for _, legacy := range []bool{false, true} {
		name := "manual"
		if legacy {
			name = "legacy_bot_api"
		}
		t.Run(name, func(t *testing.T) {
			setupBulkDB(t)
			svc := &ClientService{}
			client := model.Client{Email: "audit@x", ID: "aaaaaaaa-0000-4000-8000-000000000001", SubID: "audit-sub", Enable: true, TotalGB: 10 << 30, ExpiryTime: time.Now().Add(24 * time.Hour).UnixMilli()}
			first := mkInbound(t, 23471, model.VLESS, clientsSettings(t, []model.Client{client}))
			second := mkInbound(t, 23472, model.VLESS, clientsSettings(t, []model.Client{client}))
			for _, ib := range []*model.Inbound{first, second} {
				if err := svc.SyncInbound(nil, ib.Id, []model.Client{client}); err != nil {
					t.Fatal(err)
				}
			}
			updated := client
			updated.TotalGB += 5 << 30
			updated.ExpiryTime += 3 * 86400000
			before := time.Now().UnixMilli()
			if legacy {
				_, err := svc.UpdateInboundClient(&InboundService{}, &model.Inbound{Id: first.Id, Settings: clientsSettings(t, []model.Client{updated})}, client.Email)
				if err != nil {
					t.Fatal(err)
				}
			} else {
				_, err := svc.Update(&InboundService{}, lookupClientRecord(t, client.Email).Id, updated, 0)
				if err != nil {
					t.Fatal(err)
				}
			}
			for _, ib := range []*model.Inbound{first, second} {
				if err := svc.SyncInbound(nil, ib.Id, []model.Client{updated}); err != nil {
					t.Fatal(err)
				}
			}
			events := limitHistory(t, client.Email)
			if len(events) != 1 || events[0].Kind != "renewed" {
				t.Fatalf("events: %#v", events)
			}
			var detail clientLimitChange
			if err := json.Unmarshal([]byte(events[0].Details), &detail); err != nil {
				t.Fatal(err)
			}
			if detail.PreviousTotal != client.TotalGB || detail.Total != updated.TotalGB || detail.AddedBytes != 5<<30 || detail.PreviousExpiry != client.ExpiryTime || detail.Expiry != updated.ExpiryTime || detail.AddedMilliseconds == nil || *detail.AddedMilliseconds != 3*86400000 {
				t.Fatalf("incorrect before/after: %+v", detail)
			}
			if events[0].CreatedAt < before || events[0].CreatedAt > time.Now().UnixMilli() {
				t.Fatal("event timestamp outside request")
			}
		})
	}
}

func TestLimitHistoryRollbackAndUnattachedClient(t *testing.T) {
	setupBulkDB(t)
	svc := &ClientService{}
	client := model.Client{Email: "rollback@x", ID: "bbbbbbbb-0000-4000-8000-000000000001", SubID: "rollback-sub", Enable: true, TotalGB: 10 << 30}
	ib := mkInbound(t, 23473, model.VLESS, clientsSettings(t, []model.Client{client}))
	if err := svc.SyncInbound(nil, ib.Id, []model.Client{client}); err != nil {
		t.Fatal(err)
	}
	updated := client
	updated.TotalGB = 20 << 30
	err := database.GetDB().Transaction(func(tx *gorm.DB) error {
		if err := svc.SyncInbound(tx, ib.Id, []model.Client{updated}); err != nil {
			return err
		}
		return errors.New("abort")
	})
	if err == nil || len(limitHistory(t, client.Email)) != 0 || lookupClientRecord(t, client.Email).TotalGB != client.TotalGB {
		t.Fatal("rollback leaked a change or event")
	}
	rec := model.ClientRecord{Email: "unattached@x", SubID: "unattached-sub", UUID: client.ID, Enable: true, TotalGB: client.TotalGB}
	if err := database.GetDB().Create(&rec).Error; err != nil {
		t.Fatal(err)
	}
	updated.Email, updated.SubID = rec.Email, rec.SubID
	if _, err := svc.Update(&InboundService{}, rec.Id, updated, 0); err != nil {
		t.Fatal(err)
	}
	if events := limitHistory(t, rec.Email); len(events) != 1 || events[0].Kind != "renewed" {
		t.Fatalf("unattached history: %#v", events)
	}
}

func TestLimitHistoryBulkAdjustment(t *testing.T) {
	setupBulkDB(t)
	svc := &ClientService{}
	client := model.Client{Email: "bulk-audit@x", ID: "cccccccc-0000-4000-8000-000000000001", SubID: "bulk-audit-sub", Enable: true, TotalGB: 10 << 30, ExpiryTime: time.Now().Add(24 * time.Hour).UnixMilli()}
	for _, port := range []int{23474, 23475} {
		ib := mkInbound(t, port, model.VLESS, clientsSettings(t, []model.Client{client}))
		if err := svc.SyncInbound(nil, ib.Id, []model.Client{client}); err != nil {
			t.Fatal(err)
		}
	}
	if err := database.GetDB().Create(&xray.ClientTraffic{Email: client.Email, Enable: true, Total: client.TotalGB, ExpiryTime: client.ExpiryTime}).Error; err != nil {
		t.Fatal(err)
	}
	if _, _, err := svc.BulkAdjust(&InboundService{}, []string{client.Email}, 7, 5<<30, ""); err != nil {
		t.Fatal(err)
	}
	events := limitHistory(t, client.Email)
	if len(events) != 1 || events[0].Kind != "renewed" {
		t.Fatalf("bulk multi-inbound history: %#v", events)
	}
	var detail clientLimitChange
	if err := json.Unmarshal([]byte(events[0].Details), &detail); err != nil {
		t.Fatal(err)
	}
	if detail.AddedBytes != 5<<30 || detail.AddedMilliseconds == nil || *detail.AddedMilliseconds != 7*86400000 {
		t.Fatalf("bulk delta: %+v", detail)
	}
}

func TestLimitHistoryKindsAndRetention(t *testing.T) {
	setupBulkDB(t)
	db := database.GetDB()
	for _, tc := range []struct {
		oldTotal, total, oldExpiry, expiry int64
		kind                               string
	}{
		{10, 5, 0, 0, "limits_changed"},
		{10, 10, -86400000, -2 * 86400000, "renewed"},
		{10, 10, -86400000, time.Now().Add(24 * time.Hour).UnixMilli(), "activated"},
		{10, 0, 0, 0, "limits_changed"},
	} {
		previous := model.ClientRecord{Email: tc.kind, TotalGB: tc.oldTotal, ExpiryTime: tc.oldExpiry}
		current := previous
		current.TotalGB, current.ExpiryTime = tc.total, tc.expiry
		if err := recordClientLimitChange(db, previous, current); err != nil {
			t.Fatal(err)
		}
		events := limitHistory(t, tc.kind)
		if events[len(events)-1].Kind != tc.kind {
			t.Fatalf("wrong kind: %#v", events)
		}
	}
	email := "retained@x"
	if err := db.Create(&model.ClientRecord{Email: email, Enable: true}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&model.ClientEvent{Email: email, Kind: "renewed", Summary: "Old renewal", CreatedAt: time.Now().AddDate(0, 0, -90).UnixMilli()}).Error; err != nil {
		t.Fatal(err)
	}
	insights := &ClientInsightService{}
	if err := insights.Cleanup(7); err != nil {
		t.Fatal(err)
	}
	report, err := insights.GetReportForRange(email, 7, 12)
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Events) != 1 || report.Events[0].Summary != "Old renewal" {
		t.Fatalf("chart range or retention hid renewal: %#v", report.Events)
	}
}

func TestTrafficHistoryCapturesBeforeResetAndAdjustment(t *testing.T) {
	setupBulkDB(t)
	email := "traffic-audit@x"
	db := database.GetDB()
	if err := db.Create(&model.ClientRecord{Email: email, Enable: true}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&xray.ClientTraffic{Email: email, Up: 100, Down: 200, Enable: true}).Error; err != nil {
		t.Fatal(err)
	}
	svc := &InboundService{}
	if err := svc.ResetClientTrafficByEmail(email); err != nil {
		t.Fatal(err)
	}
	if err := svc.ResetClientTrafficByEmail(email); err != nil {
		t.Fatal(err)
	}
	if err := svc.UpdateClientTrafficByEmail(email, 30, 70); err != nil {
		t.Fatal(err)
	}
	events := limitHistory(t, email)
	if len(events) != 2 || events[0].Kind != "traffic_reset" || events[1].Kind != "traffic_adjusted" {
		t.Fatalf("traffic events: %#v", events)
	}
	var detail map[string]any
	if err := json.Unmarshal([]byte(events[0].Details), &detail); err != nil {
		t.Fatal(err)
	}
	if detail["previousUsedBytes"] != float64(300) || detail["usedBytes"] != float64(0) {
		t.Fatalf("wrong traffic before/after: %#v", detail)
	}
}
