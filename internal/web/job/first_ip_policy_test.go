package job

import (
	"encoding/json"
	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/web/service"
	"testing"
	"time"
)

func TestAcceptedIPSurvivesNewerActivityAndJobRestart(t *testing.T) {
	setupIntegrationDB(t)
	const email = "accepted-sticky"
	inbound := seedInboundOnlyWithClient(t, "sticky", email, 1)
	now := time.Now().Unix()
	row := seedClientIps(t, email, []IPWithTimestamp{{IP: "192.0.2.1", Timestamp: now - 10}})
	// The accepted IP has NEWER last activity than the intruder. Reversing the
	// timestamp sort alone would incorrectly evict it. A new job models restart.
	j := NewCheckClientIpJob()
	_, banned := j.updateInboundClientIps(database.GetDB(), row, inbound, email, 1, []IPWithTimestamp{{IP: "192.0.2.1", Timestamp: now}, {IP: "192.0.2.2", Timestamp: now - 5}}, true, true)
	if !banned || len(j.disAllowedIps) != 1 || j.disAllowedIps[0] != "192.0.2.2" {
		t.Fatalf("accepted IP evicted: %v", j.disAllowedIps)
	}
	// Once the incumbent has left and aged beyond the grace window, new IP wins.
	row = &model.InboundClientIps{ClientEmail: "fresh-slot", Ips: `[{"ip":"192.0.2.1","timestamp":1}]`}
	j = NewCheckClientIpJob()
	_, banned = j.updateInboundClientIps(database.GetDB(), row, inbound, "fresh-slot", 1, []IPWithTimestamp{{IP: "192.0.2.2", Timestamp: now}}, true, true)
	if banned {
		t.Fatal("disconnected old IP reserved a slot")
	}
}
func TestFirstObservationEnforcesAndPolicyCanDisable(t *testing.T) {
	setupIntegrationDB(t)
	const email = "first-scan"
	seedLinkedInboundWithClient(t, "first", email, 1)
	now := time.Now().Unix()
	j := NewCheckClientIpJob()
	j.processObserved(map[string]map[string]int64{email: {"192.0.2.1": now - 5, "192.0.2.2": now}}, true, true)
	if len(j.disAllowedIps) != 1 || j.disAllowedIps[0] != "192.0.2.2" {
		t.Fatalf("first scan skipped: %v", j.disAllowedIps)
	}
	policy := service.DefaultIPBlockSettings()
	policy.Enabled = false
	data, _ := json.Marshal(policy)
	if err := database.GetDB().Create(&model.Setting{Key: "ipBlockPolicy", Value: string(data)}).Error; err != nil {
		t.Fatal(err)
	}
	j = NewCheckClientIpJob()
	j.processObserved(map[string]map[string]int64{email: {"192.0.2.1": now, "192.0.2.3": now}}, true, true)
	if len(j.disAllowedIps) != 0 {
		t.Fatal("disabled policy banned IP")
	}
}

func TestCustomExemptionsDoNotConsumeSlots(t *testing.T) {
	p := service.DefaultIPBlockSettings()
	p.ExemptIPs = "192.0.2.0/24"
	kept, banned := selectIpsToBanWithPolicy([]IPWithTimestamp{{IP: "192.0.2.1", Timestamp: 1}, {IP: "203.0.113.1", Timestamp: 2}, {IP: "203.0.113.2", Timestamp: 3}}, 1, p)
	if len(kept) != 2 || len(banned) != 1 || banned[0].IP != "203.0.113.2" {
		t.Fatalf("wrong selection: %+v %+v", kept, banned)
	}
}

func TestNodeMergedNewIPCannotDisplaceAdmittedIP(t *testing.T) {
	setupIntegrationDB(t)
	const email = "merged-admission"
	inbound := seedInboundOnlyWithClient(t, "merged", email, 1)
	now := time.Now().Unix()
	// Node merges deliberately sort newest-first. Admission must survive that.
	row := seedClientIps(t, email, []IPWithTimestamp{{IP: "192.0.2.2", Timestamp: now}, {IP: "192.0.2.1", Timestamp: now - 5, AdmittedAt: time.Now().Add(-time.Minute).UnixMilli()}})
	j := NewCheckClientIpJob()
	_, banned := j.updateInboundClientIps(database.GetDB(), row, inbound, email, 1, []IPWithTimestamp{{IP: "192.0.2.1", Timestamp: now}, {IP: "192.0.2.2", Timestamp: now}}, true, true)
	if !banned || len(j.disAllowedIps) != 1 || j.disAllowedIps[0] != "192.0.2.2" {
		t.Fatalf("node merge evicted accepted IP: %v", j.disAllowedIps)
	}
}

func TestSharedIPProtectedAndOtherExcessSelected(t *testing.T) {
	setupIntegrationDB(t)
	inbound := seedInboundOnlyWithClient(t, "shared-safe", "shared-user", 1)
	now := time.Now().Unix()
	row := seedClientIps(t, "shared-user", []IPWithTimestamp{{IP: "192.0.2.1", Timestamp: now - 10}})
	j := NewCheckClientIpJob()
	j.sharedIPs = map[string]map[string]bool{"192.0.2.2": {"shared-user": true, "innocent": true}}
	_, banned := j.updateInboundClientIps(database.GetDB(), row, inbound, "shared-user", 1, []IPWithTimestamp{{IP: "192.0.2.1", Timestamp: now}, {IP: "192.0.2.2", Timestamp: now}}, true, true)
	if !banned || len(j.disAllowedIps) != 1 || j.disAllowedIps[0] != "192.0.2.1" {
		t.Fatalf("shared address endangered: %v", j.disAllowedIps)
	}
	j.sharedIPs["192.0.2.1"] = map[string]bool{"innocent": true}
	_, banned = j.updateInboundClientIps(database.GetDB(), row, inbound, "shared-user", 1, []IPWithTimestamp{{IP: "192.0.2.1", Timestamp: now + 1}, {IP: "192.0.2.2", Timestamp: now + 1}}, true, true)
	if banned || len(j.disAllowedIps) != 0 {
		t.Fatalf("all-shared addresses banned: %v", j.disAllowedIps)
	}
}
