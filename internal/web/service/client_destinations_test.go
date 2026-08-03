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
