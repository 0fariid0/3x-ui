package service

import (
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"
)

func seedDepletedOnSibling(t *testing.T, proto model.Protocol, port int, settings string) (a, b *model.Inbound) {
	t.Helper()
	setupSettingTestDB(t)
	db := database.GetDB()
	for i, dst := range []**model.Inbound{&a, &b} {
		ib := &model.Inbound{Tag: string(proto) + "-sib-" + string(rune('a'+i)), Enable: true, Port: port + i, Protocol: proto, Settings: settings}
		if err := db.Create(ib).Error; err != nil {
			t.Fatalf("create inbound: %v", err)
		}
		clients, err := (&InboundService{}).GetClients(ib)
		if err != nil {
			t.Fatalf("GetClients: %v", err)
		}
		if err := (&ClientService{}).SyncInbound(nil, ib.Id, clients); err != nil {
			t.Fatalf("SyncInbound: %v", err)
		}
		*dst = ib
	}
	rows := []xray.ClientTraffic{
		{InboundId: b.Id, Email: "d", Enable: false, Up: 10, Total: 10},
		{InboundId: b.Id, Email: "h", Enable: true},
	}
	if err := db.Create(&rows).Error; err != nil {
		t.Fatalf("seed client_traffics: %v", err)
	}
	return a, b
}

func requireOnlyHealthy(t *testing.T, site string, emails []string) {
	t.Helper()
	if len(emails) != 1 || emails[0] != "h" {
		t.Fatalf("%s serves %v on the sibling inbound, want only [h]: depleted d is still served", site, emails)
	}
}

func TestRuntimeDropsDepletedClientWhoseStatsRowPointsAtSibling(t *testing.T) {
	const vless = `{"clients":[{"email":"d","id":"11111111-1111-1111-1111-11111111111d","enable":true},` +
		`{"email":"h","id":"11111111-1111-1111-1111-11111111111e","enable":true}],"decryption":"none"}`

	t.Run("runtime push", func(t *testing.T) {
		a, _ := seedDepletedOnSibling(t, model.VLESS, 23311, vless)
		built, err := (&InboundService{}).buildInboundForLocalRuntime(database.GetDB(), a)
		if err != nil {
			t.Fatalf("buildInboundForLocalRuntime: %v", err)
		}
		clients, err := (&InboundService{}).GetClients(built)
		if err != nil {
			t.Fatalf("GetClients: %v", err)
		}
		var emails []string
		for _, c := range clients {
			emails = append(emails, c.Email)
		}
		requireOnlyHealthy(t, "buildInboundForLocalRuntime", emails)
	})

	t.Run("mtproto sidecar", func(t *testing.T) {
		a, _ := seedDepletedOnSibling(t, model.MTProto, 23321,
			`{"clients":[{"email":"d","secret":"`+mtprotoTestSecretA+`","enable":true},`+
				`{"email":"h","secret":"`+mtprotoTestSecretB+`","enable":true}]}`)
		instances, err := (&InboundService{}).DesiredMtprotoInstances()
		if err != nil {
			t.Fatalf("DesiredMtprotoInstances: %v", err)
		}
		for _, inst := range instances {
			if inst.Id != a.Id {
				continue
			}
			var emails []string
			for _, sec := range inst.Secrets {
				emails = append(emails, sec.Name)
			}
			requireOnlyHealthy(t, "DesiredMtprotoInstances", emails)
			return
		}
		t.Fatal("sibling mtproto inbound missing from desired instances")
	})

}
