package service

import (
	"slices"
	"strings"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/web/entity"
)

func mkHost(t *testing.T, svc *HostService, inboundId int, remark string, order int) *entity.HostGroup {
	t.Helper()
	created, err := svc.AddHostGroup(&entity.HostGroup{
		InboundIds: []int{inboundId},
		Remark:     remark,
		SortOrder:  order,
		Hosts:      []string{remark + ".example.com"},
		Port:       8443,
	})
	if err != nil {
		t.Fatalf("AddHostGroup %s: %v", remark, err)
	}
	g, err := svc.GetHostGroup(created[0].GroupId)
	if err != nil {
		t.Fatalf("GetHostGroup %s: %v", remark, err)
	}
	return g
}

func TestAddHost_GetHostsByInbound(t *testing.T) {
	setupBulkDB(t)
	svc := &HostService{}
	ib := mkInbound(t, 443, model.VLESS, `{"clients":[]}`)
	h1 := mkHost(t, svc, ib.Id, "b", 2)
	h2 := mkHost(t, svc, ib.Id, "a", 1)

	got, err := svc.GetHostsByInbound(ib.Id)
	if err != nil {
		t.Fatalf("GetHostsByInbound: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("len = %d, want 2", len(got))
	}
	if got[0].GroupId != h2.GroupId || got[1].GroupId != h1.GroupId {
		t.Fatalf("order = [%s,%s], want [%s,%s] (sort_order asc)", got[0].GroupId, got[1].GroupId, h2.GroupId, h1.GroupId)
	}
	if got[0].Hosts[0] != "a.example.com" {
		t.Fatalf("address not persisted: %q", got[0].Hosts[0])
	}
}

func TestAddHost_RejectsUnknownInbound(t *testing.T) {
	setupBulkDB(t)
	svc := &HostService{}
	if _, err := svc.AddHostGroup(&entity.HostGroup{InboundIds: []int{99999}, Remark: "x", Hosts: []string{"test.com"}}); err == nil {
		t.Fatalf("expected error adding host to unknown inbound")
	}
}

func TestReorderHosts(t *testing.T) {
	setupBulkDB(t)
	svc := &HostService{}
	ib := mkInbound(t, 443, model.VLESS, `{"clients":[]}`)
	h1 := mkHost(t, svc, ib.Id, "h1", 0)
	h2 := mkHost(t, svc, ib.Id, "h2", 0)
	h3 := mkHost(t, svc, ib.Id, "h3", 0)

	want := []string{h3.GroupId, h1.GroupId, h2.GroupId}
	if err := svc.ReorderHostGroups(want); err != nil {
		t.Fatalf("ReorderHostGroups: %v", err)
	}
	got, _ := svc.GetHostsByInbound(ib.Id)
	for i, g := range got {
		if g.GroupId != want[i] {
			t.Fatalf("position %d = %s, want %s", i, g.GroupId, want[i])
		}
		if g.SortOrder != i {
			t.Fatalf("host %s sort_order = %d, want %d", g.GroupId, g.SortOrder, i)
		}
	}
}

func TestSetHostEnableAndBulk(t *testing.T) {
	setupBulkDB(t)
	svc := &HostService{}
	ib := mkInbound(t, 443, model.VLESS, `{"clients":[]}`)
	h1 := mkHost(t, svc, ib.Id, "h1", 0)
	h2 := mkHost(t, svc, ib.Id, "h2", 1)

	if err := svc.SetHostGroupEnable(h1.GroupId, false); err != nil {
		t.Fatalf("SetHostGroupEnable: %v", err)
	}
	if g, _ := svc.GetHostGroup(h1.GroupId); g == nil || !g.IsDisabled {
		t.Fatalf("h1 should be disabled after SetHostGroupEnable(false)")
	}

	if err := svc.SetHostsGroupEnable([]string{h1.GroupId, h2.GroupId}, true); err != nil {
		t.Fatalf("SetHostsGroupEnable(true): %v", err)
	}
	for _, gid := range []string{h1.GroupId, h2.GroupId} {
		if g, _ := svc.GetHostGroup(gid); g == nil || g.IsDisabled {
			t.Fatalf("host %s should be enabled", gid)
		}
	}
	if err := svc.SetHostsGroupEnable([]string{h1.GroupId, h2.GroupId}, false); err != nil {
		t.Fatalf("SetHostsGroupEnable(false): %v", err)
	}
	for _, gid := range []string{h1.GroupId, h2.GroupId} {
		if g, _ := svc.GetHostGroup(gid); g == nil || !g.IsDisabled {
			t.Fatalf("host %s should be disabled", gid)
		}
	}
}

func TestDisabledHostReportsEnabledClientEmails(t *testing.T) {
	setupBulkDB(t)
	db := database.GetDB()
	svc := &HostService{}
	ib := mkInbound(t, 443, model.VLESS, `{"clients":[]}`)
	created, err := svc.AddHostGroup(&entity.HostGroup{
		InboundIds: []int{ib.Id}, Remark: "private", Hosts: []string{"private.example.com"}, IsDisabled: true,
	})
	if err != nil {
		t.Fatalf("AddHostGroup: %v", err)
	}
	client := &model.ClientRecord{Email: "private-user@example.com", SubID: "private-sub", UUID: "11111111-2222-4333-8444-555555555555", Enable: true}
	if err := db.Create(client).Error; err != nil {
		t.Fatalf("create client: %v", err)
	}
	if err := db.Create(&model.ClientInbound{ClientId: client.Id, InboundId: ib.Id}).Error; err != nil {
		t.Fatalf("attach client: %v", err)
	}
	if err := db.Create(&model.ClientSubscriptionLinkInclusion{
		ClientId: client.Id,
		LinkKey:  created[0].SubscriptionLinkKey(),
	}).Error; err != nil {
		t.Fatalf("create inclusion: %v", err)
	}

	group, err := svc.GetHostGroup(created[0].GroupId)
	if err != nil {
		t.Fatalf("GetHostGroup: %v", err)
	}
	if !group.IsDisabled || len(group.EnabledClientEmails) != 1 || group.EnabledClientEmails[0] != client.Email {
		t.Fatalf("enabled client emails=%v", group.EnabledClientEmails)
	}

	if err := svc.SetHostGroupEnable(group.GroupId, true); err != nil {
		t.Fatalf("enable host: %v", err)
	}
	group, err = svc.GetHostGroup(group.GroupId)
	if err != nil {
		t.Fatalf("GetHostGroup after enable: %v", err)
	}
	if group.IsDisabled || len(group.EnabledClientEmails) != 0 {
		t.Fatalf("globally enabled host should report active-for-all without overrides: %+v", group)
	}
}

func TestDeleteHosts(t *testing.T) {
	setupBulkDB(t)
	svc := &HostService{}
	ib := mkInbound(t, 443, model.VLESS, `{"clients":[]}`)
	h1 := mkHost(t, svc, ib.Id, "h1", 0)
	h2 := mkHost(t, svc, ib.Id, "h2", 1)
	h3 := mkHost(t, svc, ib.Id, "h3", 2)

	if err := svc.DeleteHostsGroup([]string{h1.GroupId, h3.GroupId}); err != nil {
		t.Fatalf("DeleteHostsGroup: %v", err)
	}
	got, _ := svc.GetHostsByInbound(ib.Id)
	if len(got) != 1 || got[0].GroupId != h2.GroupId {
		t.Fatalf("remaining = %v, want only h2 (%s)", got, h2.GroupId)
	}
}

func TestDeleteInboundCascadesHosts(t *testing.T) {
	setupBulkDB(t)
	svc := &HostService{}
	inboundSvc := &InboundService{}
	ib := &model.Inbound{Tag: "casc", Enable: false, Port: 4443, Protocol: model.VLESS, Settings: `{"clients":[]}`}
	if err := database.GetDB().Create(ib).Error; err != nil {
		t.Fatalf("create inbound: %v", err)
	}
	h1 := mkHost(t, svc, ib.Id, "h1", 0)

	if _, err := inboundSvc.DelInbound(ib.Id); err != nil {
		t.Fatalf("DelInbound: %v", err)
	}
	got, _ := svc.GetHostsByInbound(ib.Id)
	if len(got) != 0 {
		t.Fatalf("hosts not cascaded on inbound delete, len = %d", len(got))
	}
	if _, err := svc.GetHostGroup(h1.GroupId); err == nil {
		t.Fatalf("expected group to be deleted after cascading")
	}
}

func TestGetAllTags(t *testing.T) {
	setupBulkDB(t)
	svc := &HostService{}
	ib := mkInbound(t, 443, model.VLESS, `{"clients":[]}`)
	if _, err := svc.AddHostGroup(&entity.HostGroup{InboundIds: []int{ib.Id}, Remark: "h1", Hosts: []string{"h1.com"}, Tags: []string{"EU", "CDN"}}); err != nil {
		t.Fatalf("AddHostGroup: %v", err)
	}
	if _, err := svc.AddHostGroup(&entity.HostGroup{InboundIds: []int{ib.Id}, Remark: "h2", Hosts: []string{"h2.com"}, Tags: []string{"CDN", "FAST"}}); err != nil {
		t.Fatalf("AddHostGroup: %v", err)
	}
	tags, err := svc.GetAllTags()
	if err != nil {
		t.Fatalf("GetAllTags: %v", err)
	}
	want := []string{"CDN", "EU", "FAST"}
	if len(tags) != len(want) {
		t.Fatalf("tags = %v, want %v", tags, want)
	}
	for i := range want {
		if tags[i] != want[i] {
			t.Fatalf("tags = %v, want %v", tags, want)
		}
	}
}

func TestAddHostsGroup(t *testing.T) {
	setupBulkDB(t)
	svc := &HostService{}
	ib1 := mkInbound(t, 443, model.VLESS, `{"clients":[]}`)
	ib2 := mkInbound(t, 80, model.VLESS, `{"clients":[]}`)

	req := &entity.HostGroup{
		InboundIds: []int{ib1.Id, ib2.Id},
		Hosts:      []string{"h1.com", "h2.com:443", "[2001:db8::1]:80"},
		Remark:     "BulkRemark",
		Port:       8443,
		Security:   "same",
		HostHeader: "default.example.com",
		HostHeaders: map[string]string{
			"h1.com":      "h1-header.example.com",
			"h2.com:443":  "h2-header.example.com",
			"2001:db8::1": "v6-header.example.com",
		},
	}

	created, err := svc.AddHostGroup(req)
	if err != nil {
		t.Fatalf("AddHostGroup: %v", err)
	}
	if len(created) != 6 {
		t.Fatalf("expected 6 created hosts, got %d", len(created))
	}
	for _, row := range created {
		if row.Port != 8443 {
			t.Fatalf("all rows must use group port 8443, got %d for %q", row.Port, row.Address)
		}
		if strings.Contains(row.Address, "]:") || (strings.Count(row.Address, ":") == 1 && strings.Contains(row.Address, ".com:")) {
			t.Fatalf("stored address must not contain a port: %q", row.Address)
		}
	}

	got1, _ := svc.GetHostsByInbound(ib1.Id)
	if len(got1) != 1 {
		t.Fatalf("expected 1 group for inbound 1, got %d", len(got1))
	}
	g := got1[0]
	wantHosts := []string{"h1.com", "h2.com", "2001:db8::1"}
	for _, want := range wantHosts {
		if !slices.Contains(g.Hosts, want) {
			t.Fatalf("group hosts %v missing %q", g.Hosts, want)
		}
	}
	if got := g.HostHeaders["h1.com"]; got != "h1-header.example.com" {
		t.Fatalf("h1 header=%q", got)
	}
	if got := g.HostHeaders["h2.com"]; got != "h2-header.example.com" {
		t.Fatalf("h2 header=%q", got)
	}
	if got := g.HostHeaders["2001:db8::1"]; got != "v6-header.example.com" {
		t.Fatalf("v6 header=%q", got)
	}
}

func TestNormalizeHostAddress(t *testing.T) {
	tests := map[string]string{
		"":                 "",
		" h1.com ":         "h1.com",
		"h1.com:443":       "h1.com",
		"2001:db8::1":      "2001:db8::1",
		"[2001:db8::1]:80": "2001:db8::1",
		"[2001:db8::1]":    "2001:db8::1",
	}
	for input, want := range tests {
		if got := normalizeHostAddress(input); got != want {
			t.Errorf("normalizeHostAddress(%q)=%q, want %q", input, got, want)
		}
	}
}

func TestAddHostGroup_OptionalAddress(t *testing.T) {
	setupBulkDB(t)
	svc := &HostService{}
	ib := mkInbound(t, 443, model.VLESS, `{"clients":[]}`)

	created, err := svc.AddHostGroup(&entity.HostGroup{
		InboundIds: []int{ib.Id},
		Remark:     "OptionalAddressHost",
		Hosts:      nil,
		Port:       8443,
	})
	if err != nil {
		t.Fatalf("AddHostGroup with nil Hosts failed: %v", err)
	}

	if len(created) != 1 {
		t.Fatalf("expected 1 host created, got %d", len(created))
	}

	g, err := svc.GetHostGroup(created[0].GroupId)
	if err != nil {
		t.Fatalf("GetHostGroup failed: %v", err)
	}

	if len(g.Hosts) != 1 || g.Hosts[0] != "" {
		t.Fatalf("expected blank address to stay blank, got %v", g.Hosts)
	}
}

func TestUpdateHostGroup_ValidateBeforeDelete(t *testing.T) {
	setupBulkDB(t)
	svc := &HostService{}
	ib := mkInbound(t, 443, model.VLESS, `{"clients":[]}`)
	h1 := mkHost(t, svc, ib.Id, "h1", 0)

	req := &entity.HostGroup{
		InboundIds: []int{99999},
		Remark:     "h1-updated",
		Hosts:      []string{"h1.com"},
	}
	if _, err := svc.UpdateHostGroup(h1.GroupId, req); err == nil {
		t.Fatalf("expected error updating host group with invalid inbound")
	}

	got, err := svc.GetHostGroup(h1.GroupId)
	if err != nil {
		t.Fatalf("original host group should not be deleted: %v", err)
	}
	if got.Remark != "h1" {
		t.Fatalf("original host group remark changed: %s", got.Remark)
	}

	req.InboundIds = []int{ib.Id}
	if _, err := svc.UpdateHostGroup(h1.GroupId, req); err != nil {
		t.Fatalf("valid update failed: %v", err)
	}
	got2, _ := svc.GetHostGroup(h1.GroupId)
	if got2.Remark != "h1-updated" {
		t.Fatalf("remark not updated: %s", got2.Remark)
	}
}
