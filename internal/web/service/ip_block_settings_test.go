package service

import (
	"context"
	"encoding/json"
	"github.com/mhsanaei/3x-ui/v3/internal/web/entity"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func setupPolicyClient(t *testing.T) string {
	state := setupIPBlockClient(t, nil)
	policyPath := state + ".policy"
	if err := os.WriteFile(policyPath, []byte(`{"bantime":1800,"findtime":32,"maxretry":1}`), 0600); err != nil {
		t.Fatal(err)
	}
	script := `#!/usr/bin/env python3
import json,os,sys
p=os.environ['IPBLOCK_TEST_STATE']+'.policy';a=sys.argv[1:]
with open(p) as f: v=json.load(f)
if a==['get','3x-ipl','banip']: print('203.0.113.9');sys.exit(0)
if len(a)==3 and a[:2]==['get','3x-ipl']: print(v[a[2]]);sys.exit(0)
if len(a)==4 and a[:2]==['set','3x-ipl'] and a[2] in v:
 if os.environ.get('POLICY_FAIL')==a[2]+':'+a[3]: print('denied');sys.exit(1)
 v[a[2]]=int(a[3])
 with open(p,'w') as f:json.dump(v,f)
 print('ok');sys.exit(0)
sys.exit(2)
`
	if err := os.WriteFile(filepath.Join(filepath.Dir(state), "fail2ban-client"), []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	old := ipBlockSettingsPath
	ipBlockSettingsPath = filepath.Join(filepath.Dir(state), "panel.local")
	t.Cleanup(func() { ipBlockSettingsPath = old })
	return policyPath
}

func TestIPBlockSettingsPersistAndPreserveBans(t *testing.T) {
	setupPolicyClient(t)
	svc := &IPBlockService{}
	ctx := context.Background()
	status, err := svc.Settings(ctx)
	if err != nil || status.Settings.BanSeconds != 1800 {
		t.Fatalf("%+v %v", status, err)
	}
	v := status.Settings
	v.BanSeconds = -1
	v.MaxRetry = 3
	v.FindSeconds = 90
	v.ScanSeconds = 20
	v.Enabled = false
	result, err := svc.SaveSettings(ctx, v)
	if err != nil || result.Settings != v {
		t.Fatalf("%+v %v", result, err)
	}
	stored, err := LoadIPBlockSettings()
	if err != nil || stored != v {
		t.Fatalf("not persisted: %+v %v", stored, err)
	}
	data, err := os.ReadFile(ipBlockSettingsPath)
	if err != nil || !strings.Contains(string(data), "bantime = -1") || strings.Contains(string(data), "action") {
		t.Fatalf("bad override %s %v", data, err)
	}
	after, err := svc.Status(ctx)
	if err != nil || len(after.Blocks) != 1 || after.Blocks[0].IP != "203.0.113.9" {
		t.Fatalf("active ban lost: %+v %v", after, err)
	}
}
func TestIPBlockSettingsRollBackPartialFailure(t *testing.T) {
	path := setupPolicyClient(t)
	before := []byte("# existing override\n[3x-ipl]\nbantime = 1800\n")
	if err := os.WriteFile(ipBlockSettingsPath, before, 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("POLICY_FAIL", "maxretry:4")
	v := DefaultIPBlockSettings()
	v.BanSeconds = 600
	v.MaxRetry = 4
	if _, err := (&IPBlockService{}).SaveSettings(context.Background(), v); err == nil {
		t.Fatal("expected failure")
	}
	data, _ := os.ReadFile(ipBlockSettingsPath)
	if string(data) != string(before) {
		t.Fatalf("override not restored %s", data)
	}
	raw, _ := os.ReadFile(path)
	var runtime map[string]int
	_ = json.Unmarshal(raw, &runtime)
	if runtime["bantime"] != 1800 || runtime["maxretry"] != 1 {
		t.Fatalf("runtime not restored: %s", raw)
	}
	stored, err := LoadIPBlockSettings()
	if err != nil || stored != DefaultIPBlockSettings() {
		t.Fatalf("failed settings stored: %+v %v", stored, err)
	}
}
func TestIPBlockSettingsValidationPrecedesMutation(t *testing.T) {
	setupPolicyClient(t)
	for _, change := range []func(*entity.IPBlockSettings){func(v *entity.IPBlockSettings) { v.BanSeconds = 0 }, func(v *entity.IPBlockSettings) { v.MaxRetry = 0 }, func(v *entity.IPBlockSettings) { v.ScanSeconds = 11 }, func(v *entity.IPBlockSettings) { v.LiveSeconds = 1 }, func(v *entity.IPBlockSettings) { v.HistorySeconds = 1 }} {
		v := DefaultIPBlockSettings()
		change(&v)
		if _, err := (&IPBlockService{}).SaveSettings(context.Background(), v); err == nil {
			t.Fatalf("invalid accepted: %+v", v)
		}
	}
	if _, err := os.Stat(ipBlockSettingsPath); !os.IsNotExist(err) {
		t.Fatal("invalid request wrote override")
	}
}

func TestIPBlockExemptions(t *testing.T) {
	setupPolicyClient(t)
	v := DefaultIPBlockSettings()
	v.ExemptIPs = "192.0.2.10\n203.0.113.0/24\n2001:db8::/64"
	if _, err := (&IPBlockService{}).SaveSettings(context.Background(), v); err != nil {
		t.Fatal(err)
	}
	for _, ip := range []string{"192.0.2.10", "::ffff:192.0.2.10", "203.0.113.25", "2001:db8::1", "10.30.3.1"} {
		if !IsIPBlockExempt(ip, v) {
			t.Fatalf("not exempt: %s", ip)
		}
		if _, err := (&IPBlockService{}).Change(context.Background(), entity.IPBlockChange{IP: ip, Action: "ban"}); err == nil {
			t.Fatalf("exempt IP manually banned: %s", ip)
		}
	}
	if IsIPBlockExempt("192.0.2.11", v) || IsIPBlockExempt("100.64.0.1", v) {
		t.Fatal("unrelated addresses exempted")
	}
	v.ExemptIPs = "1.2.3.4\n[bad]\n"
	if err := validateIPBlockSettings(v); err == nil {
		t.Fatal("invalid IP accepted")
	}
}
