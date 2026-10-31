package service

import (
	"context"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestNamedExemptionExpiryAndGrace(t *testing.T) {
	setupPolicyClient(t)
	now := time.Now().UnixMilli()
	raw := `[{"name":"office","address":"192.0.2.0/24","expiresAt":` + strconv.FormatInt(now+60000, 10) + `},{"name":"expired","address":"203.0.113.1","expiresAt":` + strconv.FormatInt(now-1000, 10) + `}]`
	p := DefaultIPBlockSettings()
	p.NamedExemptions = raw
	if e := validateIPBlockSettings(p); e != nil {
		t.Fatal(e)
	}
	if !IsIPBlockExempt("192.0.2.2", p) || IsIPBlockExempt("203.0.113.1", p) {
		t.Fatal("expiry not applied")
	}
	p.NamedExemptions = `[{"name":"bad","address":"example.com","expiresAt":0}]`
	if e := validateIPBlockSettings(p); e == nil {
		t.Fatal("hostname accepted")
	}
	if e := recordUnbanGrace("account-a", "192.0.2.1", 60); e != nil {
		t.Fatal(e)
	}
	v, e := LoadUnbanGrace()
	if e != nil || len(v) != 1 || v[0].Email != "account-a" || v[0].ExpiresAt <= now {
		t.Fatalf("%+v %v", v, e)
	}
	if e := recordUnbanGrace("account-a", "192.0.2.1", 0); e != nil {
		t.Fatal(e)
	}
	v, _ = LoadUnbanGrace()
	if len(v) != 0 {
		t.Fatal("grace did not clear")
	}
}
func TestScanHealthDoesNotClaimDisconnectedOnFailure(t *testing.T) {
	RecordIPScan(map[string]map[string]int64{"health-user": {"192.0.2.1": 1}}, true)
	state, h := IPObservationState("health-user", "192.0.2.1", time.Now().Unix(), 120)
	if state != "connected" || h.Stale {
		t.Fatal(state, h)
	}
	RecordIPScan(nil, false)
	state, h = IPObservationState("health-user", "192.0.2.1", time.Now().Unix(), 120)
	if state == "connected" || !h.Stale || h.LastSuccess == 0 {
		t.Fatal(state, h)
	}
}
func TestDiagnosticFailureIsVisible(t *testing.T) {
	setupPolicyClient(t)
	v, e := ReadIPLimitOverview(context.Background(), "")
	if e != nil {
		t.Fatal(e)
	}
	if v.Checks["fail2ban"] != "ready" || v.Checks["scopedCore"] == "ready" || strings.TrimSpace(v.Checks["scopedCore"]) == "" {
		t.Fatalf("bad diagnostics %+v", v.Checks)
	}
}
