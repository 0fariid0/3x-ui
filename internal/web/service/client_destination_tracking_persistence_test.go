package service

import (
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

func TestUpdate_PersistsDestinationTrackingAcrossStaleInboundSync(t *testing.T) {
	setupBulkDB(t)
	svc := &ClientService{}
	inboundSvc := &InboundService{}

	email := "destination-persist@x"
	stale := model.Client{
		Email:               email,
		ID:                  "11111111-1111-1111-1111-111111111111",
		SubID:               email,
		Enable:              true,
		DestinationTracking: false,
	}
	first := mkInbound(t, 53101, model.VLESS, clientsSettings(t, []model.Client{stale}))
	second := mkInbound(t, 53102, model.VLESS, clientsSettings(t, []model.Client{stale}))
	if err := svc.SyncInbound(nil, first.Id, []model.Client{stale}); err != nil {
		t.Fatalf("seed first linkage: %v", err)
	}
	if err := svc.SyncInbound(nil, second.Id, []model.Client{stale}); err != nil {
		t.Fatalf("seed second linkage: %v", err)
	}

	rec, err := svc.GetRecordByEmail(nil, email)
	if err != nil {
		t.Fatalf("GetRecordByEmail: %v", err)
	}
	enabled := rec.ToClient()
	enabled.DestinationTracking = true
	if _, err := svc.Update(inboundSvc, rec.Id, *enabled, 0); err != nil {
		t.Fatalf("enable destination tracking: %v", err)
	}

	got, err := svc.GetRecordByEmail(nil, email)
	if err != nil {
		t.Fatalf("read enabled client: %v", err)
	}
	if !got.DestinationTracking {
		t.Fatal("destination tracking turned off immediately after enabling")
	}

	// A sibling inbound or node can report an older client snapshot after the
	// edit. It must not be allowed to overwrite the canonical opt-in.
	if err := svc.SyncInbound(nil, second.Id, []model.Client{stale}); err != nil {
		t.Fatalf("sync stale sibling snapshot: %v", err)
	}
	got, err = svc.GetRecordByEmail(nil, email)
	if err != nil {
		t.Fatalf("read after stale sync: %v", err)
	}
	if !got.DestinationTracking {
		t.Fatal("stale inbound snapshot disabled destination tracking")
	}

	disabled := got.ToClient()
	disabled.DestinationTracking = false
	if _, err := svc.Update(inboundSvc, got.Id, *disabled, 0); err != nil {
		t.Fatalf("disable destination tracking: %v", err)
	}
	got, err = svc.GetRecordByEmail(nil, email)
	if err != nil {
		t.Fatalf("read disabled client: %v", err)
	}
	if got.DestinationTracking {
		t.Fatal("explicitly disabling destination tracking was not persisted")
	}
}
