package service

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

func TestIPSharedClientsUsesExactServerRecentCanonicalObservations(t *testing.T) {
	setupBulkDB(t)
	guid, err := (&SettingService{}).GetPanelGuid()
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().Unix()
	add := func(server, email string, entries []model.ClientIpEntry) {
		t.Helper()
		raw, _ := json.Marshal(entries)
		if err := database.GetDB().Create(&model.NodeClientIp{NodeGuid: server, Email: email, Ips: string(raw)}).Error; err != nil {
			t.Fatal(err)
		}
	}
	add(guid, "a", []model.ClientIpEntry{{IP: "203.0.113.25", Timestamp: now - 20}, {IP: "::ffff:203.0.113.25", Timestamp: now - 10}})
	add(guid, "b", []model.ClientIpEntry{{IP: "::ffff:cb00:7119", Timestamp: now - 5}})
	add(guid, "stale", []model.ClientIpEntry{{IP: "203.0.113.25", Timestamp: now - 3600}})
	add("other", "remote", []model.ClientIpEntry{{IP: "203.0.113.25", Timestamp: now}})
	add(guid, "different", []model.ClientIpEntry{{IP: "203.0.113.250", Timestamp: now}})
	add(guid, "v6", []model.ClientIpEntry{{IP: "2001:0db8:0:0:0:0:0:1", Timestamp: now}})
	svc := &IPBlockService{}
	r, err := svc.SharedClients("::ffff:203.0.113.25")
	if err != nil {
		t.Fatal(err)
	}
	if r.IP != "203.0.113.25" || len(r.Clients) != 2 || r.Clients[0].Email != "b" || r.Clients[1].Email != "a" || r.Clients[1].LastSeen != (now-10)*1000 || r.WindowSeconds != 1800 {
		t.Fatalf("wrong result: %+v", r)
	}
	r, err = svc.SharedClients("2001:db8::1")
	if err != nil || len(r.Clients) != 1 || r.Clients[0].Email != "v6" {
		t.Fatalf("IPv6: %+v %v", r, err)
	}
	r, err = svc.SharedClients("192.0.2.1")
	if err != nil || r.Clients == nil || len(r.Clients) != 0 {
		t.Fatalf("empty: %+v %v", r, err)
	}
	if _, err = svc.SharedClients("203.0.113.25/24"); err == nil {
		t.Fatal("accepted CIDR")
	}
}
