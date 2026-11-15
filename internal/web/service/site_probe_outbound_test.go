package service

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/web/service/outbound"
)

func TestDashboardOutboundSettingsKeepSiteList(t *testing.T) {
	setupSettingTestDB(t)
	s := &SettingService{}
	sites := []SiteProbeTarget{{Name: "Example", URL: "https://example.com/", Starred: true}}
	if _, err := s.SaveSiteProbes(sites); err != nil {
		t.Fatal(err)
	}
	want := []OutboundProbeTarget{{Tag: "edge", Mode: "real"}, {Tag: "other", Mode: "tcp"}}
	view, err := s.SaveOutboundProbes(want)
	if err != nil || !reflect.DeepEqual(view.OutboundTargets, want) || !reflect.DeepEqual(view.Targets, sites) {
		t.Fatalf("saved view: %+v, %v", view, err)
	}
	if _, err := s.SaveOutboundProbes([]OutboundProbeTarget{{Tag: "edge", Mode: "unknown"}}); err == nil {
		t.Fatal("invalid mode accepted")
	}
	view, err = s.SaveSiteProbes([]SiteProbeTarget{})
	if err != nil || !reflect.DeepEqual(view.OutboundTargets, want) {
		t.Fatalf("site edit lost outbound settings: %+v, %v", view, err)
	}
}

func TestDashboardOutboundBatchesUseSavedModes(t *testing.T) {
	config := json.RawMessage(`[
		{"tag":"edge","protocol":"vless"},{"tag":"peer","protocol":"wireguard"},
		{"tag":"plain","protocol":"socks"},{"tag":"blocked","protocol":"blackhole"}
	]`)
	targets := []OutboundProbeTarget{{"edge", "real"}, {"removed", "http"}, {"peer", "tcp"}, {"plain", "http"}, {"blocked", "tcp"}}
	called := make(map[string][]string)
	got := testDashboardOutbounds(targets, config, "https://example.com/", func(batch, url, all, mode string) ([]*outbound.TestOutboundResult, error) {
		var configs []OutboundProbeOption
		if err := json.Unmarshal([]byte(batch), &configs); err != nil {
			t.Fatal(err)
		}
		if all != string(config) || url != "https://example.com/" {
			t.Fatal("missing outbound dependency context or URL")
		}
		var answers []*outbound.TestOutboundResult
		for _, c := range configs {
			called[mode] = append(called[mode], c.Tag)
			effective := mode
			if c.Tag == "peer" {
				effective = "http"
			}
			answers = append(answers, &outbound.TestOutboundResult{Tag: c.Tag, Mode: effective, Success: true, Delay: 25})
		}
		return answers, nil
	})
	if want := map[string][]string{"real": {"edge"}, "tcp": {"peer"}, "http": {"plain"}}; !reflect.DeepEqual(called, want) {
		t.Fatalf("wrong batches: %+v", called)
	}
	for i, result := range got {
		if result.OutboundProbeTarget != targets[i] {
			t.Fatalf("wrong result association: %+v", result)
		}
	}
	if got[0].Ping.Delay != 25 || got[1].Ping.Status != "unavailable" || got[2].EffectiveMode != "http" || got[4].Ping.Status != "unavailable" {
		t.Fatalf("wrong results: %+v", got)
	}
}
