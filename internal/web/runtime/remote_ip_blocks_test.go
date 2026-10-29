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
