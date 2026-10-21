package sub

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/goccy/go-yaml"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/web/entity"
	"github.com/mhsanaei/3x-ui/v3/internal/web/service"
)

func TestSubscriptionDisplayPriorityUsesRealConfigAndPreservesOriginals(t *testing.T) {
	seedSubDB(t)
	first := seedSubInbound(t, "priority-sub", "first", 4701, 1, `{"network":"tcp","security":"none"}`)
	second := seedSubInbound(t, "priority-sub", "second", 4702, 2, `{"network":"ws","security":"tls","wsSettings":{"path":"/ws"},"tlsSettings":{"serverName":"base.example"}}`)

	if err := database.GetDB().Model(&model.ClientRecord{}).Where("sub_id = ?", "priority-sub").Updates(map[string]any{
		"total_gb": 10 * 1024 * 1024, "expiry_time": int64(4102444800000),
	}).Error; err != nil {
		t.Fatalf("limit display clients: %v", err)
	}
	host := seedHost(t, &model.Host{
		GroupId: "preferred", InboundId: second.Id, Address: "preferred.example.com", Port: 8443,
		Remark: "original-host-{{EMAIL}}|{{TRAFFIC_LEFT}}", Security: "same",
	})
	priorities := []entity.DisplayHostPriority{
		{InboundId: first.Id, HostGroupId: "missing", Address: "missing.example.com"},
		{InboundId: second.Id, HostGroupId: host.GroupId, Address: host.Address},
	}
	if err := (&service.SettingService{}).UpdateSubscriptionDisplayHost(&entity.SubscriptionDisplayHost{
		Enable: true, Remark: "DISPLAY-{{EMAIL}}", Priorities: &priorities,
	}); err != nil {
		t.Fatalf("save display priority: %v", err)
	}

	withoutDisplay := []entity.DisplayHostPriority{}
	if err := (&service.SettingService{}).UpdateSubscriptionDisplayHost(&entity.SubscriptionDisplayHost{
		Enable: false, Remark: "DISPLAY-{{EMAIL}}", Priorities: &withoutDisplay,
	}); err != nil {
		t.Fatalf("temporarily disable display: %v", err)
	}
	baseLinks, _, _, _, err := NewSubService("").GetSubs("priority-sub", "request.example.com")
	if err != nil {
		t.Fatalf("baseline raw: %v", err)
	}
	baseJSON, _, err := NewSubJsonService("", "", "", NewSubService("")).GetJson("priority-sub", "request.example.com", true)
	if err != nil {
		t.Fatal(err)
	}
	baseClash, _, err := NewSubClashService(false, "", NewSubService("")).GetClash("priority-sub", "request.example.com")
	if err != nil {
		t.Fatal(err)
	}
	if err := (&service.SettingService{}).UpdateSubscriptionDisplayHost(&entity.SubscriptionDisplayHost{
		Enable: true, Remark: "DISPLAY-{{EMAIL}}", Priorities: &priorities,
	}); err != nil {
		t.Fatalf("restore display priority: %v", err)
	}

	links, _, _, _, err := NewSubService("").GetSubs("priority-sub", "request.example.com")
	if err != nil {
		t.Fatalf("priority raw: %v", err)
	}
	flat := splitLinkLines(strings.Join(links, "\n"))
	baseFlat := splitLinkLines(strings.Join(baseLinks, "\n"))
	if len(flat) != len(baseFlat)+1 {
		t.Fatalf("raw count = %d, want %d: %v", len(flat), len(baseFlat)+1, flat)
	}
	if !strings.Contains(flat[0], "preferred.example.com:8443") || !strings.Contains(flat[0], "DISPLAY-second@e") {
		t.Fatalf("first raw link is not the selected real config with display name: %s", flat[0])
	}
	if got, want := strings.Join(flat[1:], "\n"), strings.Join(baseFlat, "\n"); got != want {
		t.Fatalf("original raw configs changed\ngot:  %s\nwant: %s", got, want)
	}

	jsonOut, _, err := NewSubJsonService("", "", "", NewSubService("")).GetJson("priority-sub", "request.example.com", true)
	if err != nil {
		t.Fatalf("priority json: %v", err)
	}
	var configs []map[string]any
	if err := json.Unmarshal([]byte(jsonOut), &configs); err != nil {
		t.Fatalf("decode priority json: %v", err)
	}
	if len(configs) < 2 || configs[0]["remarks"] != "DISPLAY-second@e" {
		t.Fatalf("json display config not first or renamed: %s", jsonOut)
	}
	if !strings.Contains(string(mustJSON(t, configs[0])), "preferred.example.com") {
		t.Fatalf("json display config does not use selected address: %s", jsonOut)
	}
	var originalConfigs []map[string]any
	if err := json.Unmarshal([]byte(baseJSON), &originalConfigs); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(configs[1:], originalConfigs) {
		t.Fatal("display priority changed original JSON configs or names")
	}

	clashOut, _, err := NewSubClashService(false, "", NewSubService("")).GetClash("priority-sub", "request.example.com")
	if err != nil {
		t.Fatalf("priority clash: %v", err)
	}
	if nameAt, serverAt := strings.Index(clashOut, "name: DISPLAY-second@e"), strings.Index(clashOut, "server: preferred.example.com"); nameAt < 0 || serverAt < 0 || nameAt > serverAt {
		t.Fatalf("clash display proxy does not use selected config and name:\n%s", clashOut)
	}
	var originalClash, selectedClash struct {
		Proxies []map[string]any `yaml:"proxies"`
	}
	if err := yaml.Unmarshal([]byte(baseClash), &originalClash); err != nil {
		t.Fatal(err)
	}
	if err := yaml.Unmarshal([]byte(clashOut), &selectedClash); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(selectedClash.Proxies[1:], originalClash.Proxies) {
		t.Fatal("display priority changed original Clash proxies or names")
	}

	stored, err := (&service.SettingService{}).GetSubscriptionDisplayHostPriorities()
	if err != nil || len(stored) != 2 {
		t.Fatalf("stored priorities = %v, err = %v", stored, err)
	}
	if err := (&service.SettingService{}).UpdateSubscriptionDisplayHost(&entity.SubscriptionDisplayHost{Enable: true, Remark: "renamed"}); err != nil {
		t.Fatalf("legacy update: %v", err)
	}
	stored, err = (&service.SettingService{}).GetSubscriptionDisplayHostPriorities()
	if err != nil || len(stored) != 2 {
		t.Fatalf("legacy update erased priorities: %v, err = %v", stored, err)
	}
	if err := (&service.SettingService{}).UpdateSubscriptionDisplayHost(&entity.SubscriptionDisplayHost{
		Enable: true, Remark: "fallback", Priorities: &withoutDisplay,
	}); err != nil {
		t.Fatalf("clear priorities: %v", err)
	}
	fallbackLinks, _, _, _, err := NewSubService("").GetSubs("priority-sub", "request.example.com")
	if err != nil {
		t.Fatalf("fallback raw: %v", err)
	}
	if fallbackFlat := splitLinkLines(strings.Join(fallbackLinks, "\n")); len(fallbackFlat) == 0 || !strings.Contains(fallbackFlat[0], "1.1.1.1:1") {
		t.Fatalf("empty priorities did not restore the previous display config: %v", fallbackLinks)
	}
}

func mustJSON(t *testing.T, value any) []byte {
	t.Helper()
	encoded, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("marshal json: %v", err)
	}
	return encoded
}
