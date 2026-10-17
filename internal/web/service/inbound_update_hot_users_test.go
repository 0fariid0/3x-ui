package service

import (
	"context"
	"encoding/json"
	"errors"
	"slices"
	"sync"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

type inboundOpsProbe struct {
	fakeNodeRuntime
	mu        sync.Mutex
	ops       []string
	removeErr error
	addErr    error
}

func (p *inboundOpsProbe) record(op string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.ops = append(p.ops, op)
}

func (p *inboundOpsProbe) AddInbound(context.Context, *model.Inbound) error {
	p.record("add-inbound")
	return nil
}

func (p *inboundOpsProbe) DelInbound(context.Context, *model.Inbound) error {
	p.record("del-inbound")
	return nil
}

func (p *inboundOpsProbe) UpdateInbound(context.Context, *model.Inbound, *model.Inbound) error {
	p.record("update-inbound")
	return nil
}

func (p *inboundOpsProbe) AddUser(_ context.Context, _ *model.Inbound, user map[string]any) error {
	email, _ := user["email"].(string)
	p.record("add-user " + email)
	p.mu.Lock()
	defer p.mu.Unlock()
	err := p.addErr
	p.addErr = nil
	return err
}

func (p *inboundOpsProbe) RemoveUser(_ context.Context, _ *model.Inbound, email string) error {
	p.record("remove-user " + email)
	return p.removeErr
}

func (p *inboundOpsProbe) recorded() []string {
	p.mu.Lock()
	defer p.mu.Unlock()
	return append([]string(nil), p.ops...)
}

func ssHotSettings(t *testing.T, method, serverKey string, clients []model.Client) string {
	t.Helper()
	var settings map[string]any
	if err := json.Unmarshal([]byte(clientsSettings(t, clients)), &settings); err != nil {
		t.Fatalf("unmarshal settings: %v", err)
	}
	settings["method"] = method
	settings["network"] = "tcp,udp"
	if serverKey != "" {
		settings["password"] = serverKey
	} else {
		for _, c := range settings["clients"].([]any) {
			c.(map[string]any)["method"] = method
		}
	}
	out, err := json.Marshal(settings)
	if err != nil {
		t.Fatalf("marshal settings: %v", err)
	}
	return string(out)
}

func TestNodeShadowsocksPushChangingOnlyClientsKeepsTheHandler(t *testing.T) {
	ciphers := []struct{ name, method, serverKey, keyA, keyB string }{
		{"legacy aead", "aes-256-gcm", "", "a-pass", "b-pass"},
		{"2022", "2022-blake3-aes-128-gcm", "AAECAwQFBgcICQoLDA0ODw==", "EBESExQVFhcYGRobHB0eHw==", "ICEiIyQlJicoKSorLC0uLw=="},
	}
	for _, cipher := range ciphers {
		a := model.Client{Email: "ss-a", Password: cipher.keyA, Enable: true, CreatedAt: 1, UpdatedAt: 1}
		b := model.Client{Email: "ss-b", Password: cipher.keyB, Enable: true, CreatedAt: 1, UpdatedAt: 1}
		bOff := b
		bOff.Enable = false
		cases := []struct {
			name           string
			stored, pushed []model.Client
			wantOps        []string
			portShift      int
			removeErr      error
		}{
			{"disabling one client removes only that user", []model.Client{a, b}, []model.Client{a, bOff}, []string{"remove-user ss-b"}, 0, nil},
			{"renewing a disabled client adds only that user", []model.Client{a, bOff}, []model.Client{a, b}, []string{"add-user ss-b"}, 0, nil},
			{"port change rebuilds", []model.Client{a, b}, []model.Client{a, b}, []string{"del-inbound", "add-inbound"}, 1, nil},
			{"failed user op falls back", []model.Client{a, b}, []model.Client{a, bOff}, []string{"remove-user ss-b", "del-inbound", "add-inbound"}, 0, errors.New("handler unavailable")},
		}
		for _, tc := range cases {
			t.Run(cipher.name+"/"+tc.name, func(t *testing.T) {
				setupConflictDB(t)
				setRestartOnClientDisable(t, true)
				probe := &inboundOpsProbe{removeErr: tc.removeErr}
				useTestRuntimeManager(t).SetLocalRuntimeOverride(probe)

				seedInboundConflict(t, "hot-ss", "0.0.0.0", 50083, model.Shadowsocks, `{"network":"tcp"}`,
					ssHotSettings(t, cipher.method, cipher.serverKey, tc.stored))
				stored := loadInboundByTag(t, "hot-ss")
				if err := (&ClientService{}).SyncInbound(nil, stored.Id, tc.stored); err != nil {
					t.Fatalf("SyncInbound: %v", err)
				}

				pushed := *stored
				pushed.Port += tc.portShift
				pushed.Settings = ssHotSettings(t, cipher.method, cipher.serverKey, tc.pushed)
				_, needRestart, err := (&InboundService{}).UpdateInbound(&pushed)
				if err != nil {
					t.Fatalf("UpdateInbound: %v", err)
				}
				if got := probe.recorded(); !slices.Equal(got, tc.wantOps) {
					t.Fatalf("core operations = %q, want %q", got, tc.wantOps)
				}
				if needRestart {
					t.Fatal("needRestart = true, want the push applied without restarting the core")
				}
			})
		}
	}
}
