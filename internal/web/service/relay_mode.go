package service

import (
	"encoding/json"
	"maps"

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
	if process == nil || !process.IsRunning() {
		return nil
	}
	cfg := process.GetConfig()
	if cfg.RelayLightMode == relay && (!relay || len(cfg.Stats) > 0) {
		return nil
	}
	return s.RestartXray(false)
}

// This changes only the generated runtime config. Inbounds, routing rules,
// and the Handler API required to edit accounts remain available.
func applyRelayRuntimeConfig(cfg *xray.Config) {
	cfg.RelayLightMode = true
	cfg.Stats = json_util.RawMessage(`{}`)
	cfg.Metrics = nil
	cfg.LogConfig = json_util.RawMessage(`{"access":"none","loglevel":"warning"}`)
	cfg.Policy = relayStatsPolicy(cfg.Policy)
}

func relayStatsPolicy(raw json_util.RawMessage) json_util.RawMessage {
	var policy map[string]json.RawMessage
	if json.Unmarshal(raw, &policy) != nil || policy == nil {
		policy = make(map[string]json.RawMessage)
	}
	var levels map[string]map[string]json.RawMessage
	_ = json.Unmarshal(policy["levels"], &levels)
	if levels == nil {
		levels = make(map[string]map[string]json.RawMessage)
	}
	if _, exists := levels["0"]; !exists {
		levels["0"] = nil
	}
	for id, level := range levels {
		if level == nil {
			level = make(map[string]json.RawMessage)
			levels[id] = level
		}
		for _, key := range []string{"statsUserUplink", "statsUserDownlink", "statsUserOnline"} {
			level[key] = json.RawMessage(`true`)
		}
	}
	policy["levels"], _ = json.Marshal(levels)
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

// RelayClientSampler is local to the lightweight polling job. Counter
// baselines must never be carried across different Xray processes.
type RelayClientSampler struct {
	process *xray.Process
	api     xray.XrayAPI
}

func (s *RelayClientSampler) Sample(record func([]*xray.ClientTraffic) error) ([]string, bool, error) {
	process := currentXrayProcess()
	if process == nil || !process.IsRunning() || !process.GetConfig().RelayLightMode {
		return nil, false, nil
	}
	if s.process != process {
		s.process = process
		s.api.StatsLastValues = nil
	}
	if err := s.api.Init(process.GetAPIPort()); err != nil {
		return nil, false, err
	}
	defer s.api.Close()
	// Only advance the committed baseline once the database transaction has
	// succeeded, so a temporary write error is retried on the next poll.
	baseline := s.api.StatsLastValues
	s.api.StatsLastValues = maps.Clone(baseline)
	traffic, err := s.api.GetRelayClientTraffic()
	if err == nil {
		err = record(traffic)
	}
	if err != nil {
		s.api.StatsLastValues = baseline
		return nil, false, err
	}
	emails := make([]string, 0, len(traffic))
	seen := make(map[string]bool, len(traffic))
	for _, t := range traffic {
		seen[t.Email] = true
		emails = append(emails, t.Email)
	}
	users, err := s.api.GetOnlineUsers()
	if err != nil && !xray.IsUnimplementedErr(err) {
		return emails, true, err
	}
	for _, user := range users {
		if !seen[user.Email] {
			emails = append(emails, user.Email)
		}
	}
	return emails, true, nil
}
