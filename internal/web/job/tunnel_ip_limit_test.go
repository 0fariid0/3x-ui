package job

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
)

func TestTunnelIPsRemainVisibleWithoutCountingOrBans(t *testing.T) {
	setupIntegrationDB(t)
	const email = "tunnel-limit-user"
	inbound := seedInboundOnlyWithClient(t, "tunnel-limit", email, 1)
	now := time.Now().Unix()
	row := seedClientIps(t, email, []IPWithTimestamp{{IP: "10.20.2.1", Timestamp: now - 10}})
	j := NewCheckClientIpJob()
	observed := []IPWithTimestamp{{IP: "10.30.3.1", Timestamp: now}, {IP: "::ffff:10.40.4.1", Timestamp: now}, {IP: "5.210.158.87", Timestamp: now - 5}}
	cleaned, banned := j.updateInboundClientIps(database.GetDB(), row, inbound, email, 1, observed, true, true)
	if cleaned || banned || len(j.disAllowedIps) != 0 {
		t.Fatalf("tunnel counted toward limit: %v", j.disAllowedIps)
	}
	got := ipSet(readClientIps(t, email))
	for _, ip := range []string{"10.20.2.1", "10.30.3.1", "::ffff:10.40.4.1", "5.210.158.87"} {
		if _, ok := got[ip]; !ok {
			t.Fatalf("observation removed: %s", ip)
		}
	}
	if data, _ := os.ReadFile(readIpLimitLogPath()); len(data) > 0 {
		t.Fatalf("unexpected ban log: %s", data)
	}
	observed = append(observed, IPWithTimestamp{IP: "203.0.113.25", Timestamp: now + 1})
	_, banned = j.updateInboundClientIps(database.GetDB(), row, inbound, email, 1, observed, true, true)
	if !banned || len(j.disAllowedIps) != 1 || j.disAllowedIps[0] != "5.210.158.87" {
		t.Fatalf("real IP limit stopped working: %v", j.disAllowedIps)
	}
	data, err := os.ReadFile(readIpLimitLogPath())
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "10.30.") || strings.Contains(string(data), "10.40.") || !strings.Contains(string(data), "5.210.158.87") {
		t.Fatalf("wrong ban targets: %s", data)
	}
	got = ipSet(readClientIps(t, email))
	if _, ok := got["10.30.3.1"]; !ok {
		t.Fatal("tunnel lost after public ban")
	}
}

func TestTunnelOnlyAndOrderingNeverConsumeLimit(t *testing.T) {
	for _, entries := range [][]IPWithTimestamp{
		{{IP: "10.10.1.1", Timestamp: 1}, {IP: "10.30.3.1", Timestamp: 2}},
		{{IP: "203.0.113.1", Timestamp: 1}, {IP: "10.30.3.1", Timestamp: 2}},
		{{IP: "10.30.3.1", Timestamp: 1}, {IP: "203.0.113.1", Timestamp: 2}},
	} {
		kept, banned := selectIpsToBan(entries, 1)
		if len(kept) != len(entries) || len(banned) != 0 {
			t.Fatalf("wrong selection: kept=%v banned=%v", kept, banned)
		}
	}
}
