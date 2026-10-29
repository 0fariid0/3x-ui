package service

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/web/entity"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"
)

func setupIPBlockClient(t *testing.T, ips []string) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("Fail2ban is Linux-only")
	}
	setupBulkDB(t)
	dir := t.TempDir()
	state := filepath.Join(dir, "state.json")
	data, err := json.Marshal(ips)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(state, data, 0600); err != nil {
		t.Fatal(err)
	}
	script := `#!/usr/bin/env python3
import os, sys, json
path = os.environ['IPBLOCK_TEST_STATE']
args = sys.argv[1:]
with open(path) as f: ips = json.load(f)
if args == ['get', '3x-ipl', 'banip']:
 print(' '.join(ips))
elif args == ['get', '3x-ipl', 'bantime']:
 print('1800')
elif len(args) == 4 and args[:2] == ['set', '3x-ipl'] and args[2] in ['banip', 'unbanip']:
 if os.environ.get('IPBLOCK_TEST_FAIL'):
  print('denied'); sys.exit(1)
 with open(path + '.calls', 'a') as f: f.write(json.dumps(args) + '\n')
 if not os.environ.get('IPBLOCK_TEST_NO_CHANGE'):
  if args[2] == 'banip' and args[3] not in ips: ips.append(args[3])
  if args[2] == 'unbanip': ips = [ip for ip in ips if ip != args[3]]
  with open(path, 'w') as f: json.dump(ips, f)
 print('1')
else:
 print('unexpected arguments'); sys.exit(2)
`
	if err := os.WriteFile(filepath.Join(dir, "fail2ban-client"), []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("IPBLOCK_TEST_STATE", state)
	t.Setenv("XUI_ENABLE_FAIL2BAN", "true")
	t.Setenv("XUI_LOG_FOLDER", dir)
	return state
}

func TestNormalizeBlockIP(t *testing.T) {
	for input, want := range map[string]string{"203.0.113.25": "203.0.113.25", " 2001:0db8:0000::1 ": "2001:db8::1", "::ffff:192.0.2.1": "192.0.2.1", "10.20.1.1": "10.20.1.1"} {
		got, err := normalizeBlockIP(input)
		if err != nil || got != want {
			t.Errorf("%q = %q %v, want %q", input, got, err, want)
		}
	}
	for _, input := range []string{"", "host.example", "192.0.2.1/24", "192.0.2.1; touch /tmp/bad", "--help", "127.0.0.1", "::1", "0.0.0.0", "::", "ff02::1", "fe80::1%eth0", "255.255.255.255"} {
		if got, err := normalizeBlockIP(input); err == nil {
			t.Errorf("accepted %q as %q", input, got)
		}
	}
}

func TestParseBlockedIPs(t *testing.T) {
	for _, output := range []string{"203.0.113.1 2001:db8::1", "['203.0.113.1', '2001:0db8::1']"} {
		got, err := parseBlockedIPs([]byte(output))
		if err != nil || strings.Join(got, ",") != "2001:db8::1,203.0.113.1" {
			t.Fatalf("parse %q = %v %v", output, got, err)
		}
	}
	if got, err := parseBlockedIPs([]byte("[]")); err != nil || len(got) != 0 {
		t.Fatal(got, err)
	}
	if _, err := parseBlockedIPs([]byte("Failed to read jail")); err == nil {
		t.Fatal("accepted an error as an empty list")
	}
}

func TestIPBlockStatusUsesLiveListAndAutomaticLogs(t *testing.T) {
	setupIPBlockClient(t, []string{"203.0.113.10", "2001:db8::1"})
	at := time.Now().Add(-time.Minute).Truncate(time.Second)
	log := at.Format("2006/01/02 15:04:05") + "   BAN   [Email] = customer-a [IP] = 203.0.113.10 banned for 1800 seconds.\n" + at.Format("2006/01/02 15:04:05") + "   BAN   [Email] = old-client [IP] = 203.0.113.99 banned for 1800 seconds.\n"
	if err := os.WriteFile(xray.GetIPLimitBannedLogPath(), []byte(log), 0600); err != nil {
		t.Fatal(err)
	}
	status, err := (&IPBlockService{}).Status(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !status.Available || status.BanSeconds != 1800 || len(status.Blocks) != 2 {
		t.Fatalf("status: %+v", status)
	}
	for _, entry := range status.Blocks {
		if entry.IP == "203.0.113.99" {
			t.Fatal("stale log entry became a live block")
		}
		if entry.IP == "203.0.113.10" && (entry.Source != "automatic" || entry.Email != "customer-a" || entry.BannedAt != at.UnixMilli() || entry.ExpiresAt != at.Add(1800*time.Second).UnixMilli()) {
			t.Fatalf("auto metadata: %+v", entry)
		}
		if entry.IP == "2001:db8::1" && entry.Source != "unknown" {
			t.Fatal("invented source for missing metadata")
		}
	}
}

func TestIPBlockChangeRoundTripAndIdempotency(t *testing.T) {
	state := setupIPBlockClient(t, nil)
	if err := os.WriteFile(state, []byte("[]"), 0600); err != nil {
		t.Fatal(err)
	}
	svc := &IPBlockService{}
	req := entity.IPBlockChange{IP: "2001:0db8::1", Action: "ban", Email: "customer-a"}
	status, err := svc.Change(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	if len(status.Blocks) != 1 || status.Blocks[0].IP != "2001:db8::1" || status.Blocks[0].Source != "manual" || status.AuditWarning {
		t.Fatalf("ban: %+v", status)
	}
	var wg sync.WaitGroup
	errors := make(chan error, 2)
	for range 2 {
		wg.Add(1)
		go func() { defer wg.Done(); _, err := svc.Change(context.Background(), req); errors <- err }()
	}
	wg.Wait()
	close(errors)
	for err := range errors {
		if err != nil {
			t.Fatal(err)
		}
	}
	calls, err := os.ReadFile(state + ".calls")
	if err != nil || strings.Count(string(calls), "\n") != 1 {
		t.Fatalf("duplicate command: %s %v", calls, err)
	}
	status, err = svc.Status(context.Background())
	if err != nil || status.Blocks[0].Source != "manual" {
		t.Fatalf("manual source was not retained: %+v %v", status, err)
	}
	req.Action = "unban"
	status, err = svc.Change(context.Background(), req)
	if err != nil || len(status.Blocks) != 0 {
		t.Fatalf("unban: %+v %v", status, err)
	}
	if _, err := svc.Change(context.Background(), req); err != nil {
		t.Fatal(err)
	}
	var events []model.ClientEvent
	if err := database.GetDB().Order("id").Find(&events).Error; err != nil {
		t.Fatal(err)
	}
	if len(events) != 2 || events[0].Kind != "ip_blocked" || events[1].Kind != "ip_unblocked" {
		t.Fatalf("audit: %+v", events)
	}
	var detail ipBlockAudit
	if err := json.Unmarshal([]byte(events[0].Details), &detail); err != nil {
		t.Fatal(err)
	}
	if detail.IP != "2001:db8::1" || detail.BanSeconds != 1800 {
		t.Fatalf("audit details: %+v", detail)
	}
}

func TestIPBlockChangeFailureNeverClaimsSuccess(t *testing.T) {
	for _, mode := range []string{"IPBLOCK_TEST_FAIL", "IPBLOCK_TEST_NO_CHANGE"} {
		t.Run(mode, func(t *testing.T) {
			setupIPBlockClient(t, []string{})
			t.Setenv(mode, "1")
			_, err := (&IPBlockService{}).Change(context.Background(), entity.IPBlockChange{IP: "203.0.113.25", Action: "ban", Email: "customer-a"})
			if err == nil {
				t.Fatal("unverified command reported success")
			}
			var count int64
			if err := database.GetDB().Model(&model.ClientEvent{}).Count(&count).Error; err != nil || count != 0 {
				t.Fatalf("failed operation audit count %d: %v", count, err)
			}
		})
	}
}

func TestIPBlockChangeAnonymousAuditAndFailureWarning(t *testing.T) {
	setupIPBlockClient(t, []string{})
	svc := &IPBlockService{}
	if _, err := svc.Change(context.Background(), entity.IPBlockChange{IP: "203.0.113.25", Action: "ban"}); err != nil {
		t.Fatal(err)
	}
	status, err := svc.Status(context.Background())
	if err != nil || status.Blocks[0].Source != "manual" {
		t.Fatalf("anonymous audit lost: %+v %v", status, err)
	}
	if err := database.GetDB().Migrator().DropTable(&model.ClientEvent{}); err != nil {
		t.Fatal(err)
	}
	status, err = svc.Change(context.Background(), entity.IPBlockChange{IP: "203.0.113.26", Action: "ban"})
	if err != nil || !status.AuditWarning || len(status.Blocks) != 2 {
		t.Fatalf("audit failure hid applied command: %+v %v", status, err)
	}
}

func TestIPBlockUnavailableAndInvalidRequests(t *testing.T) {
	state := setupIPBlockClient(t, []string{})
	svc := &IPBlockService{}
	for _, req := range []entity.IPBlockChange{{IP: "203.0.113.1;echo", Action: "ban"}, {IP: "203.0.113.1", Action: "restart"}, {IP: "203.0.113.1", Action: "ban", NodeID: 1}} {
		if _, err := svc.Change(context.Background(), req); err == nil {
			t.Fatal("accepted invalid operation")
		}
	}
	if _, err := os.Stat(state + ".calls"); !os.IsNotExist(err) {
		t.Fatal("invalid request invoked a mutation")
	}
	t.Setenv("XUI_ENABLE_FAIL2BAN", "false")
	status, err := svc.Status(context.Background())
	if err != nil || status.Available || status.Reason != "disabled" {
		t.Fatalf("disabled status: %+v %v", status, err)
	}
	if _, err := svc.Change(context.Background(), entity.IPBlockChange{IP: "203.0.113.1", Action: "ban"}); err == nil {
		t.Fatal("blocked while disabled")
	}
}
