package runtime

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/web/entity"
)

func TestRemoteIPBlocksUseConfiguredNodeAndDoNotForwardNodeID(t *testing.T) {
	var received entity.IPBlockChange
	var requests int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		if r.Header.Get("Authorization") != "Bearer tok" {
			t.Error("missing node authentication")
		}
		if r.Method == http.MethodPost {
			if r.URL.Path != "/panel/api/server/ipBlocks/change" {
				t.Error(r.URL.Path)
			}
			if err := json.NewDecoder(r.Body).Decode(&received); err != nil {
				t.Error(err)
			}
		} else if r.URL.Path != "/panel/api/server/ipBlocks" {
			t.Error(r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"success": true, "obj": entity.IPBlockStatus{Available: true, Jail: "3x-ipl", Blocks: []entity.IPBlockEntry{{IP: "2001:db8::1", Source: "manual"}}}})
	}))
	defer srv.Close()
	remote := NewRemote(nodeForPlainServer(t, srv, "verify", "tok"), nil)
	status, err := remote.IPBlocks(context.Background())
	if err != nil || !status.Available || status.Blocks[0].IP != "2001:db8::1" {
		t.Fatalf("status: %+v %v", status, err)
	}
	_, err = remote.ChangeIPBlock(context.Background(), entity.IPBlockChange{NodeID: 9, IP: "2001:db8::1", Action: "unban", Email: "client"})
	if err != nil {
		t.Fatal(err)
	}
	if received.NodeID != 0 || received.IP != "2001:db8::1" || received.Action != "unban" || received.Email != "client" || requests != 2 {
		t.Fatalf("wrong target or payload: %+v (%d)", received, requests)
	}
}

func TestRemoteSharedIPClientsUsesAuthenticatedExactTarget(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" || r.URL.Path != "/panel/api/server/ipBlocks/clients" || r.URL.Query().Get("ip") != "2001:db8::1" || r.URL.Query().Has("nodeId") || r.Header.Get("Authorization") != "Bearer tok" {
			t.Errorf("wrong request: %s", r.URL.String())
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"success": true, "obj": entity.IPSharedClients{IP: "2001:db8::1", Clients: []entity.IPSharedClient{{Email: "remote-user", LastSeen: 123000}}, WindowSeconds: 1800, CheckedAt: 123000}})
	}))
	defer srv.Close()
	remote := NewRemote(nodeForPlainServer(t, srv, "verify", "tok"), nil)
	result, err := remote.SharedIPClients(context.Background(), "2001:db8::1")
	if err != nil || len(result.Clients) != 1 || result.Clients[0].Email != "remote-user" {
		t.Fatalf("wrong matches: %+v %v", result, err)
	}
}

func TestRemoteIPBlockSettingsExactNodeAndJSON(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.URL.Path != "/panel/api/server/ipBlocks/settings" || r.URL.RawQuery != "" || r.Header.Get("Authorization") != "Bearer tok" {
			t.Errorf("wrong target: %s", r.URL)
		}
		v := entity.IPBlockSettings{Enabled: true, BanSeconds: 600, MaxRetry: 1, FindSeconds: 32, ScanSeconds: 10, LiveSeconds: 120, HistorySeconds: 1800}
		if r.Method == "POST" {
			var body entity.IPBlockSettings
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body != v {
				t.Errorf("bad JSON: %+v %v", body, err)
			}
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"success": true, "obj": entity.IPBlockSettingsStatus{Settings: v, Available: true}})
	}))
	defer srv.Close()
	remote := NewRemote(nodeForPlainServer(t, srv, "verify", "tok"), nil)
	result, err := remote.IPBlockSettings(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if _, err = remote.SaveIPBlockSettings(context.Background(), result.Settings); err != nil {
		t.Fatal(err)
	}
	if calls != 2 {
		t.Fatal(calls)
	}
}
