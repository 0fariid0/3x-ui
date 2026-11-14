package service

import (
	"encoding/json"

	"github.com/mhsanaei/3x-ui/v3/internal/util/json_util"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"
)

// RelayLightMode is separate from bulk settings so older clients cannot
// accidentally overwrite the operator's choice.
func (s *SettingService) GetRelayLightMode() (bool, error) {
	return s.getBool("relayLightMode")
}

func (s *SettingService) SetRelayLightMode(enabled bool) error {
	return s.setBool("relayLightMode", enabled)
}

func (s *XrayService) ApplyRelayMode(relay bool) error {
	process := currentXrayProcess()
	if process == nil || !process.IsRunning() || process.GetConfig().RelayLightMode == relay {
		return nil
	}
	return s.RestartXray(false)
}

// This changes only the generated runtime config. Inbounds, routing rules,
// and the Handler API required to edit accounts remain available.
func applyRelayRuntimeConfig(cfg *xray.Config) {
	cfg.RelayLightMode = true
	cfg.Stats = nil
	cfg.Metrics = nil
	cfg.LogConfig = json_util.RawMessage(`{"access":"none","loglevel":"warning"}`)
	cfg.Policy = stripRelayStatsPolicy(cfg.Policy)
}

func stripRelayStatsPolicy(raw json_util.RawMessage) json_util.RawMessage {
	var policy map[string]json.RawMessage
	if json.Unmarshal(raw, &policy) != nil || policy == nil {
		return raw
	}
	var levels map[string]map[string]json.RawMessage
	if json.Unmarshal(policy["levels"], &levels) == nil && levels != nil {
		for _, level := range levels {
			for _, key := range []string{"statsUserUplink", "statsUserDownlink", "statsUserOnline"} {
				delete(level, key)
			}
		}
		policy["levels"], _ = json.Marshal(levels)
	}
	var system map[string]json.RawMessage
	if json.Unmarshal(policy["system"], &system) == nil && system != nil {
		for _, key := range []string{"statsInboundUplink", "statsInboundDownlink", "statsOutboundUplink", "statsOutboundDownlink"} {
			delete(system, key)
		}
		policy["system"], _ = json.Marshal(system)
	}
	result, err := json.Marshal(policy)
	if err != nil {
		return raw
	}
	return result
}
