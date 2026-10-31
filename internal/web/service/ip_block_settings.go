package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/util/clientip"
	"github.com/mhsanaei/3x-ui/v3/internal/web/entity"
	"net/netip"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const ipBlockSettingsKey = "ipBlockPolicy"

var ipBlockSettingsPath = "/etc/fail2ban/jail.d/zz-3x-ui-panel.local"

func DefaultIPBlockSettings() entity.IPBlockSettings {
	return entity.IPBlockSettings{Enabled: true, BanSeconds: 1800, FindSeconds: 32, MaxRetry: 1, ScanSeconds: 10, LiveSeconds: 120, HistorySeconds: 1800}
}

func validateIPBlockSettings(v entity.IPBlockSettings) error {
	if len(v.ExemptIPs) > 8192 || len(strings.Fields(v.ExemptIPs)) > 128 {
		return fmt.Errorf("too many exempt IPs")
	}
	for _, entry := range strings.Fields(v.ExemptIPs) {
		if ip, err := netip.ParseAddr(entry); err == nil && ip.Zone() == "" {
			continue
		}
		if _, err := netip.ParsePrefix(entry); err != nil {
			return fmt.Errorf("invalid exempt IP or CIDR: %s", entry)
		}
	}
	if v.BanSeconds != -1 && (v.BanSeconds < 1 || v.BanSeconds > 31536000) {
		return fmt.Errorf("banSeconds must be -1 (permanent) or 1..31536000")
	}
	if v.FindSeconds < 1 || v.FindSeconds > 86400 || v.MaxRetry < 1 || v.MaxRetry > 100 {
		return fmt.Errorf("invalid retry count or retry window")
	}
	if v.ScanSeconds < 10 || v.ScanSeconds > 300 || v.ScanSeconds%10 != 0 {
		return fmt.Errorf("scanSeconds must be a multiple of 10 between 10 and 300")
	}
	if v.LiveSeconds < v.ScanSeconds || v.LiveSeconds > 600 || v.HistorySeconds < v.LiveSeconds || v.HistorySeconds > 86400 {
		return fmt.Errorf("invalid live/history window")
	}
	return nil
}

func LoadIPBlockSettings() (entity.IPBlockSettings, error) {
	v := DefaultIPBlockSettings()
	if database.GetDB() == nil {
		return v, fmt.Errorf("database unavailable")
	}
	row, err := (&SettingService{}).getSetting(ipBlockSettingsKey)
	if database.IsNotFound(err) {
		return v, nil
	}
	if err != nil {
		return v, err
	}
	if err = json.Unmarshal([]byte(row.Value), &v); err != nil {
		return v, err
	}
	return v, validateIPBlockSettings(v)
}

func jailSettings(ctx context.Context, v *entity.IPBlockSettings) error {
	for _, field := range []struct {
		name  string
		value *int
	}{{"bantime", &v.BanSeconds}, {"findtime", &v.FindSeconds}, {"maxretry", &v.MaxRetry}} {
		out, err := runIPBlockCommand(ctx, "get", ipBlockJail, field.name)
		if err != nil {
			return err
		}
		value, err := strconv.Atoi(strings.TrimSpace(string(out)))
		if err != nil {
			return fmt.Errorf("invalid %s response", field.name)
		}
		*field.value = value
	}
	return nil
}

func (s *IPBlockService) Settings(ctx context.Context) (*entity.IPBlockSettingsStatus, error) {
	ipBlockMutationMu.Lock()
	defer ipBlockMutationMu.Unlock()
	return s.settingsUnlocked(ctx)
}
func (s *IPBlockService) settingsUnlocked(ctx context.Context) (*entity.IPBlockSettingsStatus, error) {
	v, err := LoadIPBlockSettings()
	if err != nil {
		return nil, err
	}
	status, err := s.Status(ctx)
	if err != nil {
		return nil, err
	}
	result := &entity.IPBlockSettingsStatus{Settings: v, Available: status.Available, Reason: status.Reason}
	if status.Available {
		if err = jailSettings(ctx, &result.Settings); err != nil {
			return nil, err
		}
	}
	return result, nil
}

// Atomic replacement only touches the panel-owned override, not the action/filter
// or other jails. Runtime setters preserve active bans and connections.
func writeIPBlockOverride(data []byte) error {
	dir := filepath.Dir(ipBlockSettingsPath)
	f, err := os.CreateTemp(dir, ".3x-ui-policy-*")
	if err != nil {
		return err
	}
	name := f.Name()
	defer os.Remove(name)
	if err = f.Chmod(0600); err == nil {
		_, err = f.Write(data)
	}
	if err == nil {
		err = f.Sync()
	}
	closeErr := f.Close()
	if err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	return os.Rename(name, ipBlockSettingsPath)
}
func applyJailSettings(ctx context.Context, v entity.IPBlockSettings) error {
	for _, field := range []struct {
		name  string
		value int
	}{{"bantime", v.BanSeconds}, {"findtime", v.FindSeconds}, {"maxretry", v.MaxRetry}} {
		if _, err := runIPBlockCommand(ctx, "set", ipBlockJail, field.name, strconv.Itoa(field.value)); err != nil {
			return err
		}
	}
	actual := v
	if err := jailSettings(ctx, &actual); err != nil {
		return err
	}
	if actual.BanSeconds != v.BanSeconds || actual.FindSeconds != v.FindSeconds || actual.MaxRetry != v.MaxRetry {
		return fmt.Errorf("Fail2ban did not confirm settings")
	}
	return nil
}

func (s *IPBlockService) SaveSettings(ctx context.Context, v entity.IPBlockSettings) (*entity.IPBlockSettingsStatus, error) {
	if err := validateIPBlockSettings(v); err != nil {
		return nil, err
	}
	ipBlockMutationMu.Lock()
	defer ipBlockMutationMu.Unlock()
	before, err := s.settingsUnlocked(ctx)
	if err != nil {
		return nil, err
	}
	if !before.Available {
		return nil, fmt.Errorf("IP blocking unavailable: %s", before.Reason)
	}
	old, readErr := os.ReadFile(ipBlockSettingsPath)
	if readErr != nil && !os.IsNotExist(readErr) {
		return nil, readErr
	}
	data := []byte(fmt.Sprintf("# Managed by 3x-ui IP block settings\n[3x-ipl]\nbantime = %d\nfindtime = %d\nmaxretry = %d\n", v.BanSeconds, v.FindSeconds, v.MaxRetry))
	if err = writeIPBlockOverride(data); err != nil {
		return nil, fmt.Errorf("cannot persist IP block settings: %w", err)
	}
	rollback := func(cause error) error {
		restoreCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		runtimeErr := applyJailSettings(restoreCtx, before.Settings)
		var fileErr error
		if os.IsNotExist(readErr) {
			fileErr = os.Remove(ipBlockSettingsPath)
		} else {
			fileErr = writeIPBlockOverride(old)
		}
		return errors.Join(cause, runtimeErr, fileErr)
	}
	if err = applyJailSettings(ctx, v); err != nil {
		return nil, rollback(err)
	}
	blob, _ := json.Marshal(v)
	if err = (&SettingService{}).saveSetting(ipBlockSettingsKey, string(blob)); err != nil {
		return nil, rollback(err)
	}
	return &entity.IPBlockSettingsStatus{Settings: v, Available: true}, nil
}

// Exemptions never consume client slots and cannot be newly banned via the API.
// Existing bans are deliberately left untouched; unban remains available.
func IsIPBlockExempt(value string, policy entity.IPBlockSettings) bool {
	if clientip.IsTunnelLocal(value) {
		return true
	}
	ip, err := netip.ParseAddr(strings.TrimSpace(value))
	if err != nil {
		return false
	}
	ip = ip.Unmap()
	for _, entry := range strings.Fields(policy.ExemptIPs) {
		if addr, err := netip.ParseAddr(entry); err == nil && addr.Unmap() == ip {
			return true
		}
		if prefix, err := netip.ParsePrefix(entry); err == nil {
			if prefix.Contains(ip) || (ip.Is4() && prefix.Addr().Is4In6() && prefix.Contains(netip.AddrFrom16(ip.As16()))) {
				return true
			}
		}
	}
	return false
}
