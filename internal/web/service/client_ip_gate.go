package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/web/entity"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"
	"strings"
	"time"
)

const clientIPBlocksKey = "clientScopedIPBlocks"

var runCoreIPControl = coreIPControl

func coreIPControl(ctx context.Context, action, email, ip string, seconds int) (map[string]any, error) {
	api := new(xray.XrayAPI)
	port := (&XrayService{}).GetXrayAPIPort()
	if port <= 0 {
		return nil, fmt.Errorf("running Fara Xray core required")
	}
	if err := api.Init(port); err != nil {
		return nil, err
	}
	defer api.Close()
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	return api.ClientIPControl(ctx, action, email, ip, seconds)
}
func scopedEntries() ([]entity.IPBlockEntry, error) {
	row, err := (&SettingService{}).getSetting(clientIPBlocksKey)
	if database.IsNotFound(err) {
		return []entity.IPBlockEntry{}, nil
	}
	if err != nil {
		return nil, err
	}
	var entries []entity.IPBlockEntry
	err = json.Unmarshal([]byte(row.Value), &entries)
	return entries, err
}
func saveScopedEntries(v []entity.IPBlockEntry) error {
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}
	return (&SettingService{}).saveSetting(clientIPBlocksKey, string(data))
}

// Reconcile restores nonexpired blocks after core restarts. It does not extend
// their stored expiration, and it returns an error if the core lacks support.
func ReconcileScopedIPBlocks(ctx context.Context) error {
	ipBlockMutationMu.Lock()
	defer ipBlockMutationMu.Unlock()
	entries, err := scopedEntries()
	if err != nil {
		return err
	}
	if len(entries) == 0 {
		return nil
	}
	nowForPrune := time.Now().UnixMilli()
	activeEntries := make([]entity.IPBlockEntry, 0, len(entries))
	for _, entry := range entries {
		if entry.Permanent || entry.ExpiresAt > nowForPrune {
			activeEntries = append(activeEntries, entry)
		}
	}
	if len(activeEntries) == 0 {
		return saveScopedEntries(activeEntries)
	}
	entries = activeEntries
	live, err := runCoreIPControl(ctx, "status", "", "", 0)
	if err != nil {
		return err
	}
	liveEntries := make(map[string]map[string]any)
	if values, ok := live["blocks"].([]any); ok {
		for _, raw := range values {
			if item, ok := raw.(map[string]any); ok {
				email, _ := item["email"].(string)
				ip, _ := item["ip"].(string)
				liveEntries[email+"\x00"+ip] = item
			}
		}
	}
	now := time.Now().UnixMilli()
	kept := make([]entity.IPBlockEntry, 0, len(entries))
	for _, v := range entries {
		if !v.Permanent && v.ExpiresAt <= now {
			continue
		}
		seconds := -1
		if !v.Permanent {
			seconds = int((v.ExpiresAt - now + 999) / 1000)
		}
		if item := liveEntries[v.Email+"\x00"+v.IP]; item != nil {
			expiry, _ := item["expiresAt"].(float64)
			difference := int64(expiry) - v.ExpiresAt
			if (v.Permanent && expiry == 0) || (!v.Permanent && difference >= -1500 && difference <= 1500) {
				kept = append(kept, v)
				continue
			}
		}
		if _, err = runCoreIPControl(ctx, "ban", v.Email, v.IP, seconds); err != nil {
			return err
		}
		kept = append(kept, v)
	}
	return saveScopedEntries(kept)
}
func ChangeScopedIPBlock(ctx context.Context, req entity.IPBlockChange) (*entity.IPBlockStatus, error) {
	return changeScopedIPBlock(ctx, req, "manual")
}
func AutomaticScopedIPBlock(ctx context.Context, req entity.IPBlockChange) (*entity.IPBlockStatus, error) {
	return changeScopedIPBlock(ctx, req, "automatic")
}
func changeScopedIPBlock(ctx context.Context, req entity.IPBlockChange, source string) (*entity.IPBlockStatus, error) {
	sourceIP := req.IP
	if strings.HasPrefix(sourceIP, "[") && strings.HasSuffix(sourceIP, "]") {
		sourceIP = sourceIP[1 : len(sourceIP)-1]
	}
	ip, err := normalizeBlockIP(sourceIP)
	if err != nil {
		return nil, err
	}
	if req.Email == "" || len(req.Email) > 255 || strings.ContainsAny(req.Email, "\r\n\x00") || req.NodeID != 0 || (req.Action != "ban" && req.Action != "unban") {
		return nil, fmt.Errorf("exact account and source required")
	}
	ipBlockMutationMu.Lock()
	defer ipBlockMutationMu.Unlock()
	policy, err := LoadIPBlockSettings()
	if err != nil {
		return nil, err
	}
	if req.Action == "ban" && IsIPBlockExempt(ip, policy) {
		return nil, fmt.Errorf("IP is exempt")
	}
	entries, err := scopedEntries()
	if err != nil {
		return nil, err
	}
	now := time.Now().UnixMilli()
	next := make([]entity.IPBlockEntry, 0, len(entries)+1)
	var existing *entity.IPBlockEntry
	for i, v := range entries {
		if !v.Permanent && v.ExpiresAt <= now {
			continue
		}
		if v.Email == req.Email && v.IP == ip {
			existing = &entries[i]
		} else {
			next = append(next, v)
		}
	}
	seconds := policy.BanSeconds
	if req.Action == "ban" {
		v := entity.IPBlockEntry{IP: ip, Email: req.Email, Scope: "client", Source: source, BannedAt: now, Permanent: seconds < 0}
		if seconds > 0 {
			v.ExpiresAt = now + int64(seconds)*1000
		}
		if existing != nil {
			v = *existing
			if v.Permanent {
				seconds = -1
			} else {
				seconds = int((v.ExpiresAt - now + 999) / 1000)
			}
		}
		next = append(next, v)
	}
	if _, err = runCoreIPControl(ctx, req.Action, req.Email, ip, seconds); err != nil {
		return nil, fmt.Errorf("scoped operation failed; no global ban was used: %w", err)
	}
	if err = saveScopedEntries(next); err != nil {
		restore, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		action, seconds := "unban", 0
		if existing != nil {
			action = "ban"
			seconds = -1
			if !existing.Permanent {
				seconds = int((existing.ExpiresAt - time.Now().UnixMilli() + 999) / 1000)
				if seconds < 1 {
					action = "unban"
					seconds = 0
				}
			}
		}
		_, rollbackErr := runCoreIPControl(restore, action, req.Email, ip, seconds)
		return nil, errors.Join(fmt.Errorf("persistence failed; core rollback attempted: %w", err), rollbackErr)
	}
	if req.Action == "unban" {
		if err = recordUnbanGrace(req.Email, ip, policy.UnbanGraceSeconds); err != nil {
			return nil, fmt.Errorf("unbanned but grace persistence failed: %w", err)
		}
	}
	details, _ := json.Marshal(map[string]any{"ip": ip, "scope": "client", "action": req.Action, "source": source})
	auditErr := database.GetDB().Create(&model.ClientEvent{Email: req.Email, Kind: "ip_" + req.Action, Summary: ip, Details: string(details), CreatedAt: now}).Error
	return &entity.IPBlockStatus{Available: true, Jail: "Fara core / account + IP", BanSeconds: int64(policy.BanSeconds), Blocks: next, AuditWarning: auditErr != nil}, nil
}

func ScopedIPBlockStatus(ctx context.Context) (*entity.IPBlockStatus, error) {
	live, err := runCoreIPControl(ctx, "status", "", "", 0)
	if err != nil {
		return nil, err
	}
	stored, err := scopedEntries()
	if err != nil {
		return nil, err
	}
	known := make(map[string]entity.IPBlockEntry)
	for _, v := range stored {
		known[v.Email+"\x00"+v.IP] = v
	}
	status := &entity.IPBlockStatus{Available: true, Jail: "Fara core / account + IP", Blocks: []entity.IPBlockEntry{}}
	if p, e := LoadIPBlockSettings(); e == nil {
		status.BanSeconds = int64(p.BanSeconds)
	}
	rows, _ := live["blocks"].([]any)
	for _, raw := range rows {
		row, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		email, _ := row["email"].(string)
		ip, _ := row["ip"].(string)
		expires, _ := row["expiresAt"].(float64)
		v, exists := known[email+"\x00"+ip]
		if !exists {
			v = entity.IPBlockEntry{Email: email, IP: ip, Scope: "client", Source: "unknown"}
		}
		v.ExpiresAt = int64(expires)
		v.Permanent = expires == 0
		status.Blocks = append(status.Blocks, v)
	}
	return status, nil
}
