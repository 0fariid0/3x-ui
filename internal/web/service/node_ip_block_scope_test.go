package service

import (
	"context"
	"encoding/json"
	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/web/entity"
	"github.com/mhsanaei/3x-ui/v3/internal/web/runtime"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"testing"
)

func TestOldNodeCannotTurnScopedRequestIntoHostBan(t *testing.T) {
	setupBulkDB(t)
	posts := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "POST" {
			posts++
		}
		if r.URL.Path == "/panel/api/server/ipBlocks/settings" {
			_ = json.NewEncoder(w).Encode(map[string]any{"success": true, "obj": entity.IPBlockSettingsStatus{Settings: DefaultIPBlockSettings(), Available: true}})
			return
		}
		// An old node ignores the scope query and returns its host firewall list.
		_ = json.NewEncoder(w).Encode(map[string]any{"success": true, "obj": entity.IPBlockStatus{Available: true, Jail: "3x-ipl", Blocks: []entity.IPBlockEntry{}}})
	}))
	defer srv.Close()
	u, _ := url.Parse(srv.URL)
	port, _ := strconv.Atoi(u.Port())
	node := model.Node{Name: "old-node", Scheme: "http", Address: u.Hostname(), Port: port, BasePath: "/", ApiToken: "tok", Enable: true, AllowPrivateAddress: true, TlsVerifyMode: "verify"}
	if e := database.GetDB().Create(&node).Error; e != nil {
		t.Fatal(e)
	}
	old := runtime.GetManager()
	runtime.SetManager(runtime.NewManager(runtime.LocalDeps{APIPort: func() int { return 0 }, SetNeedRestart: func() {}}))
	t.Cleanup(func() { runtime.SetManager(old) })
	svc := new(NodeService)
	if _, e := svc.ChangeIPBlock(context.Background(), entity.IPBlockChange{NodeID: node.Id, Scope: "client", Email: "account-a", IP: "192.0.2.1", Action: "ban"}); e == nil {
		t.Fatal("old node was accepted for scoped mutation")
	}
	p := DefaultIPBlockSettings()
	p.EnforcementScope = "client"
	if _, e := svc.SaveIPBlockSettings(context.Background(), node.Id, p); e == nil {
		t.Fatal("old node accepted new policy")
	}
	if posts != 0 {
		t.Fatalf("unsafe operation was sent: %d posts", posts)
	}
}
