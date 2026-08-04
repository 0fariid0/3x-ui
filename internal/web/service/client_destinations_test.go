package service

import (
	"testing"
	"time"
)

func TestParseDestinationAccessLineTrackedClient(t *testing.T) {
	loc := time.FixedZone("panel", 3*60*60+30*60)
	line := "2026/08/03 11:12:13.123456 from tcp:10.0.0.2:44000 accepted tcp:edge-chat.instagram.com:443 [inbound-1 >> direct] email: alice"
	event, ok := parseDestinationAccessLine(line, map[string]struct{}{"alice": {}}, loc)
	if !ok {
		t.Fatal("expected tracked line to parse")
	}
	if event.Email != "alice" || event.Domain != "instagram.com" || event.Service != "Instagram" || event.Owner != "Meta" {
		t.Fatalf("unexpected event: %#v", event)
	}
	if event.Port != 443 || event.Protocol != "tcp" || event.Count != 1 {
		t.Fatalf("unexpected destination metadata: %#v", event)
	}
	if event.FirstAt == 0 || event.LastAt != event.FirstAt {
		t.Fatalf("unexpected timestamps: %#v", event)
	}
}

func TestParseDestinationAccessLineIgnoresUnselectedClient(t *testing.T) {
	line := "2026/08/03 11:12:13 from tcp:10.0.0.2:44000 accepted tcp:telegram.org:443 [inbound-1 >> direct] email: bob"
	if _, ok := parseDestinationAccessLine(line, map[string]struct{}{"alice": {}}, time.UTC); ok {
		t.Fatal("unselected client must not be aggregated")
	}
}

func TestCompactDomainAndClassification(t *testing.T) {
	if got := compactDomain("r3---sn.googlevideo.com."); got != "googlevideo.com" {
		t.Fatalf("compactDomain() = %q", got)
	}
	service, owner, confidence := classifyDestination("googlevideo.com", "")
	if service != "YouTube" || owner != "Google" || confidence != "domain" {
		t.Fatalf("unexpected classification: %q %q %q", service, owner, confidence)
	}
}

func TestClassifyDestinationTelegramOfficialRange(t *testing.T) {
	service, owner, confidence := classifyDestination("", "149.154.167.91")
	if service != "Telegram" || owner != "Telegram network" || confidence != "network" {
		t.Fatalf("unexpected Telegram IP classification: %q %q %q", service, owner, confidence)
	}

	service, owner, confidence = classifyDestination("", "2001:b28:f23d::1")
	if service != "Telegram" || owner != "Telegram network" || confidence != "network" {
		t.Fatalf("unexpected Telegram IPv6 classification: %q %q %q", service, owner, confidence)
	}
}

func TestClassifyDestinationMetaRangeIsNotOverstated(t *testing.T) {
	service, owner, confidence := classifyDestination("", "157.240.10.35")
	if service != "Instagram / Meta" || owner != "Meta network" || confidence != "network" {
		t.Fatalf("unexpected Meta IP classification: %q %q %q", service, owner, confidence)
	}
}

func TestClassifyDestinationGoogleOfficialRange(t *testing.T) {
	service, owner, confidence := classifyDestination("", "142.251.20.113")
	if service != "Google" || owner != "Google network" || confidence != "network" {
		t.Fatalf("unexpected Google IP classification: %q %q %q", service, owner, confidence)
	}

	service, owner, confidence = classifyDestination("", "2607:f8b0:4005:805::200e")
	if service != "Google" || owner != "Google network" || confidence != "network" {
		t.Fatalf("unexpected Google IPv6 classification: %q %q %q", service, owner, confidence)
	}
}

func TestBundledProviderRangesSurviveEmptyRefreshSnapshot(t *testing.T) {
	destinationNetworks.mu.Lock()
	old := destinationNetworks.snapshot
	destinationNetworks.snapshot = destinationNetworkSnapshot{}
	destinationNetworks.mu.Unlock()
	defer func() {
		destinationNetworks.mu.Lock()
		destinationNetworks.snapshot = old
		destinationNetworks.mu.Unlock()
	}()

	checks := []struct {
		ip      string
		service string
	}{
		{"149.154.167.91", "Telegram"},
		{"157.240.10.35", "Instagram / Meta"},
		{"142.251.20.113", "Google"},
	}
	for _, check := range checks {
		service, _, _ := classifyDestination("", check.ip)
		if service != check.service {
			t.Fatalf("classifyDestination(%s)=%q, want %q", check.ip, service, check.service)
		}
	}
}

func TestReclassifyDestinationItemRepairsStoredOther(t *testing.T) {
	item := ClientDestinationItem{
		Key:        ":149.154.167.91:443",
		Service:    "Other",
		IP:         "149.154.167.91",
		Port:       443,
		Confidence: "ip",
	}
	reclassifyDestinationItem(&item)
	if item.Service != "Telegram" || item.Owner != "Telegram network" || item.Confidence != "network" {
		t.Fatalf("stored Other row was not repaired: %#v", item)
	}
}

func TestReclassifyDestinationItemRepairsStoredMetaOther(t *testing.T) {
	item := ClientDestinationItem{Service: "Other", IP: "157.240.10.35", Port: 443, Confidence: "ip"}
	reclassifyDestinationItem(&item)
	if item.Service != "Instagram / Meta" || item.Owner != "Meta network" || item.Confidence != "network" {
		t.Fatalf("stored Meta row was not repaired: %#v", item)
	}
}

func TestReclassifyDestinationItemRepairsStoredGoogleOther(t *testing.T) {
	item := ClientDestinationItem{Service: "Other", IP: "142.251.20.113:443", Port: 443, Confidence: "ip"}
	reclassifyDestinationItem(&item)
	if item.Service != "Google" || item.Owner != "Google network" || item.Confidence != "network" {
		t.Fatalf("stored Google row was not repaired: %#v", item)
	}
}

func TestDestinationIsActiveWindow(t *testing.T) {
	now := time.Now().UnixMilli()
	if !destinationIsActive(now-int64(2*time.Minute/time.Millisecond), now) {
		t.Fatal("destination seen two minutes ago must be active")
	}
	if destinationIsActive(now-int64(6*time.Minute/time.Millisecond), now) {
		t.Fatal("destination older than active window must not be active")
	}
	if destinationIsActive(now+1, now) {
		t.Fatal("future destination timestamp must not be active")
	}
}

func TestDestinationRollingWindowConfiguration(t *testing.T) {
	if destinationActiveWindow != 5*time.Minute {
		t.Fatalf("active destination window = %v, want 5m", destinationActiveWindow)
	}
	if destinationCleanupInterval != 5*time.Minute {
		t.Fatalf("destination cleanup interval = %v, want 5m", destinationCleanupInterval)
	}
	if destinationRollingWindow != 5*time.Minute {
		t.Fatalf("destination rolling window = %v, want 5m", destinationRollingWindow)
	}
	if destinationPruneThreshold != 10*time.Minute {
		t.Fatalf("destination prune threshold = %v, want 10m", destinationPruneThreshold)
	}
}

func TestDestinationRollingCutoffKeepsOfflineSnapshot(t *testing.T) {
	base := time.Date(2026, time.August, 4, 12, 0, 0, 0, time.UTC).UnixMilli()
	if cutoff, ok := destinationRollingCutoff(base, base+int64(9*time.Minute/time.Millisecond)); ok || cutoff != 0 {
		t.Fatalf("nine-minute span must stay intact, got cutoff=%d ok=%v", cutoff, ok)
	}
	latest := base + int64(10*time.Minute/time.Millisecond)
	cutoff, ok := destinationRollingCutoff(base, latest)
	if !ok {
		t.Fatal("ten-minute span must trigger pruning")
	}
	want := latest - int64(5*time.Minute/time.Millisecond)
	if cutoff != want {
		t.Fatalf("cutoff=%d, want %d", cutoff, want)
	}
	// No wall-clock value is involved, so the last observed window remains
	// available indefinitely after a client goes offline.
	if laterCutoff, laterOK := destinationRollingCutoff(cutoff, latest); laterOK || laterCutoff != 0 {
		t.Fatalf("retained five-minute snapshot must not prune while idle, got cutoff=%d ok=%v", laterCutoff, laterOK)
	}
}
