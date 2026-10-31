package service

import (
	"context"
	"fmt"
	"github.com/mhsanaei/3x-ui/v3/internal/web/entity"
	"testing"
	"time"
)

func TestScopedServicePersistsRestoresAndUnbansExactPair(t *testing.T) {
	setupPolicyClient(t)
	old := runCoreIPControl
	t.Cleanup(func() { runCoreIPControl = old })
	active := make(map[string]map[string]any)
	runCoreIPControl = func(ctx context.Context, action, email, ip string, seconds int) (map[string]any, error) {
		key := email + "|" + ip
		if action == "ban" {
			expires := int64(0)
			if seconds > 0 {
				expires = time.Now().UnixMilli() + int64(seconds)*1000
			}
			active[key] = map[string]any{"email": email, "ip": ip, "expiresAt": float64(expires)}
		}
		if action == "unban" {
			delete(active, key)
		}
		rows := []any{}
		for _, r := range active {
			rows = append(rows, r)
		}
		return map[string]any{"blocks": rows, "version": float64(1)}, nil
	}
	ctx := context.Background()
	req := entity.IPBlockChange{Scope: "client", Email: "a", IP: "192.0.2.1", Action: "ban"}
	for _, email := range []string{"a", "b"} {
		req.Email = email
		if _, err := ChangeScopedIPBlock(ctx, req); err != nil {
			t.Fatal(err)
		}
	}
	stored, e := scopedEntries()
	if e != nil || len(stored) != 2 {
		t.Fatal(stored, e)
	}
	expiry := stored[0].ExpiresAt
	clear(active)
	if e = ReconcileScopedIPBlocks(ctx); e != nil {
		t.Fatal(e)
	}
	if len(active) != 2 {
		t.Fatal("restart lost persisted blocks")
	}
	stored, _ = scopedEntries()
	if stored[0].ExpiresAt != expiry {
		t.Fatal("restart extended ban duration")
	}
	req.Email = "a"
	req.Action = "unban"
	if _, e := ChangeScopedIPBlock(ctx, req); e != nil {
		t.Fatal(e)
	}
	if active["a|192.0.2.1"] != nil || active["b|192.0.2.1"] == nil {
		t.Fatal("wrong account unbanned")
	}
	grace, _ := LoadUnbanGrace()
	if len(grace) != 1 || grace[0].Email != "a" {
		t.Fatal("scoped grace missing", grace)
	}
	status, e := ScopedIPBlockStatus(ctx)
	if e != nil || len(status.Blocks) != 1 || status.Blocks[0].Scope != "client" {
		t.Fatal(status, e)
	}
	runCoreIPControl = func(context.Context, string, string, string, int) (map[string]any, error) {
		return nil, fmt.Errorf("unsupported core")
	}
	req.Action = "ban"
	if _, e := ChangeScopedIPBlock(ctx, req); e == nil {
		t.Fatal("unsupported core reported success")
	}
	stored, _ = scopedEntries()
	if len(stored) != 1 {
		t.Fatal("failed operation persisted")
	}
}

func TestScopedRepeatedPermanentBanKeepsOriginalDuration(t *testing.T) {
	setupPolicyClient(t)
	old := runCoreIPControl
	t.Cleanup(func() { runCoreIPControl = old })
	var received int
	runCoreIPControl = func(_ context.Context, action, email, ip string, seconds int) (map[string]any, error) {
		received = seconds
		return map[string]any{}, nil
	}
	if err := saveScopedEntries([]entity.IPBlockEntry{{Email: "a", IP: "192.0.2.1", Scope: "client", Permanent: true, BannedAt: 1}}); err != nil {
		t.Fatal(err)
	}
	// The current default policy is temporary, but a repeated ban must preserve
	// the independent permanent block that already exists for this exact pair.
	if _, err := ChangeScopedIPBlock(context.Background(), entity.IPBlockChange{Scope: "client", Email: "a", IP: "192.0.2.1", Action: "ban"}); err != nil {
		t.Fatal(err)
	}
	if received != -1 {
		t.Fatalf("permanent ban was replaced by %d seconds", received)
	}
}
