package xray

import (
	"context"
	"encoding/json"
	"fmt"
	statsCommand "github.com/xtls/xray-core/app/stats/command"
	"golang.org/x/net/proxy"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func startGateCore(t *testing.T, bin string, cfg map[string]any, port int) {
	t.Helper()
	data, e := json.Marshal(cfg)
	if e != nil {
		t.Fatal(e)
	}
	p := filepath.Join(t.TempDir(), "config.json")
	if e = os.WriteFile(p, data, 0600); e != nil {
		t.Fatal(e)
	}
	cmd := exec.Command(bin, "run", "-c", p)
	cmd.Stderr = os.Stderr
	cmd.Stdout = os.Stderr
	if e = cmd.Start(); e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { _ = cmd.Process.Kill(); _, _ = cmd.Process.Wait() })
	waitForPort(t, port)
}
func TestFaraCoreExactAccountIPLiveDisconnect(t *testing.T) {
	for _, network := range []string{"tcp", "ws", "grpc"} {
		t.Run(network, func(t *testing.T) { runFaraGateE2E(t, network) })
	}
}
func runFaraGateE2E(t *testing.T, network string) {
	bin := os.Getenv("FARA_CORE_BINARY")
	if bin == "" {
		t.Skip("set FARA_CORE_BINARY to the bundled patched core")
	}
	apiPort, inPort := freePort(t), freePort(t)
	stream := map[string]any{"network": network}
	if network == "ws" {
		stream["wsSettings"] = map[string]any{"path": "/gate-test"}
	}
	if network == "grpc" {
		stream["grpcSettings"] = map[string]any{"serviceName": "gate-test"}
	}
	ids := []string{"11111111-1111-4111-8111-111111111111", "22222222-2222-4222-8222-222222222222"}
	startGateCore(t, bin, map[string]any{
		"log": map[string]any{"loglevel": "warning"}, "api": map[string]any{"services": []string{"StatsService"}, "tag": "api"}, "stats": map[string]any{},
		"policy":    map[string]any{"levels": map[string]any{"0": map[string]any{"statsUserOnline": true, "statsUserDownlink": true, "statsUserUplink": true}}},
		"inbounds":  []any{map[string]any{"listen": "127.0.0.1", "port": apiPort, "protocol": "tunnel", "tag": "api", "settings": map[string]any{"rewriteAddress": "127.0.0.1"}}, map[string]any{"listen": "127.0.0.1", "port": inPort, "protocol": "vless", "sniffing": map[string]any{"enabled": true, "destOverride": []string{"http", "tls"}, "routeOnly": true}, "streamSettings": stream, "settings": map[string]any{"decryption": "none", "clients": []any{map[string]any{"id": ids[0], "email": "account-a"}, map[string]any{"id": ids[1], "email": "account-b"}}}}},
		"outbounds": []any{map[string]any{"protocol": "freedom", "tag": "direct", "settings": map[string]any{"finalRules": []any{map[string]any{"action": "allow", "ip": []string{"127.0.0.0/8"}}}}}}, "routing": map[string]any{"rules": []any{map[string]any{"type": "field", "inboundTag": []string{"api"}, "outboundTag": "api"}}},
	}, apiPort)
	api := new(XrayAPI)
	if e := api.Init(apiPort); e != nil {
		t.Fatal(e)
	}
	defer api.Close()
	echo, e := net.Listen("tcp", "127.0.0.1:0")
	if e != nil {
		t.Fatal(e)
	}
	defer echo.Close()
	go func() {
		for {
			c, e := echo.Accept()
			if e != nil {
				return
			}
			go func() { defer c.Close(); _, _ = io.Copy(c, c) }()
		}
	}()
	socksPorts := []int{freePort(t), freePort(t), freePort(t)}
	inbounds, outbounds, rules := []any{}, []any{}, []any{}
	for i, p := range socksPorts {
		tag := fmt.Sprint("socks-", i)
		id := ids[0]
		if i == 1 {
			id = ids[1]
		}
		source := "127.0.0.1"
		if i == 2 {
			source = "127.0.0.2"
		}
		inbounds = append(inbounds, map[string]any{"listen": "127.0.0.1", "port": p, "protocol": "socks", "tag": tag, "settings": map[string]any{"auth": "noauth"}})
		outbounds = append(outbounds, map[string]any{"protocol": "vless", "streamSettings": stream, "tag": tag, "sendThrough": source, "settings": map[string]any{"vnext": []any{map[string]any{"address": "127.0.0.1", "port": inPort, "users": []any{map[string]any{"id": id, "encryption": "none"}}}}}})
		rules = append(rules, map[string]any{"type": "field", "inboundTag": []string{tag}, "outboundTag": tag})
	}
	startGateCore(t, bin, map[string]any{"log": map[string]any{"loglevel": "warning"}, "inbounds": inbounds, "outbounds": outbounds, "routing": map[string]any{"rules": rules}}, socksPorts[0])
	dial := func(i int) net.Conn {
		t.Helper()
		d, e := proxy.SOCKS5("tcp", fmt.Sprintf("127.0.0.1:%d", socksPorts[i]), nil, proxy.Direct)
		if e != nil {
			t.Fatal(e)
		}
		c, e := d.Dial("tcp", echo.Addr().String())
		if e != nil {
			t.Fatal(e)
		}
		t.Cleanup(func() { c.Close() })
		return c
	}
	exchange := func(c net.Conn) error {
		_ = c.SetDeadline(time.Now().Add(2 * time.Second))
		if _, e := c.Write([]byte("alive")); e != nil {
			return e
		}
		b := make([]byte, 5)
		_, e := io.ReadFull(c, b)
		if e == nil && string(b) != "alive" {
			return fmt.Errorf("bad echo")
		}
		return e
	}
	streams := []net.Conn{dial(0), dial(1), dial(2)}
	for i, c := range streams {
		if e := exchange(c); e != nil {
			t.Fatalf("initial stream %d: %v", i, e)
		}
	}
	for i := 0; i < 3; i++ {
		if e := exchange(streams[0]); e != nil {
			t.Fatal(e)
		}
	}
	reply, e := api.ClientIPControl(context.Background(), "ban", "account-a", "127.0.0.1", 60)
	if e != nil {
		t.Fatal(e)
	}
	if reply["closed"].(float64) < 1 {
		t.Fatal("no live stream was closed")
	}
	if e := exchange(streams[0]); e == nil {
		t.Fatal("victim stream remained usable")
	}
	if e := exchange(streams[1]); e != nil {
		t.Fatalf("another account on same IP was cut: %v", e)
	}
	if e := exchange(streams[2]); e != nil {
		t.Fatalf("same account on different IP was cut: %v", e)
	}
	counterDeadline := time.Now().Add(time.Second)
	for {
		counter, e := (*api.StatsServiceClient).GetStats(context.Background(), &statsCommand.GetStatsRequest{Name: "user>>>account-a>>>traffic>>>downlink"})
		if e != nil {
			t.Fatal(e)
		}
		if counter.GetStat().GetValue() >= 25 {
			break
		}
		if time.Now().After(counterDeadline) {
			t.Fatalf("user traffic accounting was lost: %d", counter.GetStat().GetValue())
		}
		time.Sleep(10 * time.Millisecond)
	}

	if e := exchange(dial(0)); e == nil {
		t.Fatal("blocked reconnection succeeded")
	}
	if _, e := api.ClientIPControl(context.Background(), "unban", "account-a", "127.0.0.1", 0); e != nil {
		t.Fatal(e)
	}
	if e := exchange(dial(0)); e != nil {
		t.Fatalf("unbanned client did not recover: %v", e)
	}
	if _, e := api.ClientIPControl(context.Background(), "ban", "account-a", "127.0.0.1", 1); e != nil {
		t.Fatal(e)
	}
	time.Sleep(1100 * time.Millisecond)
	if e := exchange(dial(0)); e != nil {
		t.Fatalf("expired block did not release: %v", e)
	}
	_ = streams[2].Close()
	deadline := time.Now().Add(3 * time.Second)
	for {
		users, e := api.GetOnlineUsers()
		if e != nil {
			t.Fatal(e)
		}
		found := false
		for _, user := range users {
			if user.Email == "account-a" {
				for _, ip := range user.IPs {
					if ip.IP == "127.0.0.2" {
						found = true
					}
				}
			}
		}
		if !found {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("closed stream remained in online IP statistics")
		}
		time.Sleep(50 * time.Millisecond)
	}

}
