package service

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"
	"net/netip"
	"os"
	"strings"
	"sync"
	"time"
)

type IPScanHealth struct {
	CheckedAt   int64  `json:"checkedAt"`
	LastSuccess int64  `json:"lastSuccess"`
	Available   bool   `json:"available"`
	Stale       bool   `json:"stale"`
	Reason      string `json:"reason"`
}

var scanState struct {
	sync.RWMutex
	health IPScanHealth
	live   map[string]map[string]int64
}

func RecordIPScan(observed map[string]map[string]int64, ok bool) {
	scanState.Lock()
	defer scanState.Unlock()
	now := time.Now().UnixMilli()
	scanState.health.CheckedAt = now
	scanState.health.Available = ok
	if ok {
		scanState.health.LastSuccess = now
		scanState.health.Reason = ""
		scanState.live = observed
	} else {
		scanState.health.Reason = "online_stats_unavailable"
	}
}
func IPObservationState(email, ip string, seen int64, window int) (string, IPScanHealth) {
	scanState.RLock()
	defer scanState.RUnlock()
	h := scanState.health
	h.Stale = !h.Available || h.LastSuccess == 0 || time.Now().UnixMilli()-h.LastSuccess > 30000
	if !h.Stale {
		if _, ok := scanState.live[email][ip]; ok {
			return "connected", h
		}
	}
	if time.Now().Unix()-seen < int64(window) {
		return "recent", h
	}
	return "history", h
}

type IPExemption struct {
	Name      string `json:"name"`
	Address   string `json:"address"`
	ExpiresAt int64  `json:"expiresAt"`
}

func activeExemptionAddresses(raw string, now int64) (string, error) {
	var rules []IPExemption
	if raw == "" {
		return "", nil
	}
	if len(raw) > 32768 {
		return "", fmt.Errorf("exemptions too large")
	}
	if err := json.Unmarshal([]byte(raw), &rules); err != nil {
		return "", err
	}
	if len(rules) > 128 {
		return "", fmt.Errorf("too many exemptions")
	}
	var addresses []string
	for _, r := range rules {
		if strings.TrimSpace(r.Name) == "" || len(r.Name) > 128 || len(strings.Fields(r.Address)) != 1 || r.ExpiresAt < 0 {
			return "", fmt.Errorf("invalid named exemption")
		}
		if r.ExpiresAt == 0 || r.ExpiresAt > now {
			addresses = append(addresses, r.Address)
		}
	}
	return strings.Join(addresses, "\n"), nil
}

const unbanGraceKey = "ipLimitUnbanGrace"

type IPUnbanGrace struct {
	Email     string `json:"email"`
	IP        string `json:"ip"`
	ExpiresAt int64  `json:"expiresAt"`
}

func LoadUnbanGrace() ([]IPUnbanGrace, error) {
	r, e := (&SettingService{}).getSetting(unbanGraceKey)
	if database.IsNotFound(e) {
		return nil, nil
	}
	if e != nil {
		return nil, e
	}
	var v []IPUnbanGrace
	e = json.Unmarshal([]byte(r.Value), &v)
	return v, e
}
func recordUnbanGrace(email, ip string, seconds int) error {
	v, e := LoadUnbanGrace()
	if e != nil {
		return e
	}
	now := time.Now().UnixMilli()
	out := make([]IPUnbanGrace, 0, len(v)+1)
	for _, r := range v {
		if r.ExpiresAt > now && !(r.Email == email && r.IP == ip) {
			out = append(out, r)
		}
	}
	if seconds > 0 {
		out = append(out, IPUnbanGrace{email, ip, now + int64(seconds)*1000})
	}
	data, _ := json.Marshal(out)
	return (&SettingService{}).saveSetting(unbanGraceKey, string(data))
}

type IPLimitOverview struct {
	History   []model.ClientIPHistory `json:"history"`
	Health    IPScanHealth            `json:"health"`
	Events    []model.ClientEvent     `json:"events"`
	Checks    map[string]string       `json:"checks"`
	CheckedAt int64                   `json:"checkedAt"`
}

func ReadIPLimitOverview(ctx context.Context, search string, hours ...int) (*IPLimitOverview, error) {
	_, h := IPObservationState("", "", 0, 0)
	result := &IPLimitOverview{Health: h, History: []model.ClientIPHistory{}, Events: []model.ClientEvent{}, Checks: make(map[string]string), CheckedAt: time.Now().UnixMilli()}
	if len(search) > 255 {
		return nil, fmt.Errorf("search too long")
	}
	window := 24
	if len(hours) > 0 {
		window = hours[0]
	}
	if window < 1 || window > 744 {
		return nil, fmt.Errorf("invalid time range")
	}
	since := time.Now().Add(-time.Duration(window) * time.Hour).UnixMilli()
	q := database.GetDB().Where("created_at >= ?", since).Where("kind IN ?", []string{"ip_limit_decision", "ip_blocked", "ip_unblocked", "ip_ban", "ip_unban"})
	if search != "" {
		if ip, e := netip.ParseAddr(search); e == nil {
			q = q.Where("summary = ? OR details LIKE ?", ip.Unmap().String(), "%\"ip\":\""+ip.Unmap().String()+"\"%")
		} else {
			q = q.Where("email = ? OR summary = ?", search, search)
		}
	}
	if e := q.Order("created_at DESC, id DESC").Limit(200).Find(&result.Events).Error; e != nil {
		return nil, e
	}
	hq := database.GetDB().Model(&model.ClientIPHistory{}).Where("last_seen >= ?", since)
	if search != "" {
		hq = hq.Where("email = ? OR ip = ?", search, search)
	}
	if e := hq.Order("last_seen DESC, id DESC").Limit(200).Find(&result.History).Error; e != nil {
		return nil, e
	}
	if _, e := runCoreIPControl(ctx, "status", "", "", 0); e == nil {
		result.Checks["scopedCore"] = "ready"
	} else {
		result.Checks["scopedCore"] = e.Error()
	}
	if s, e := (&IPBlockService{}).Status(ctx); e != nil {
		result.Checks["fail2ban"] = e.Error()
	} else if s.Available {
		result.Checks["fail2ban"] = "ready"
	} else {
		result.Checks["fail2ban"] = s.Reason
	}
	if f, e := os.Open(xray.GetIPLimitLogPath()); e == nil {
		f.Close()
		result.Checks["logReadable"] = "ready"
	} else {
		result.Checks["logReadable"] = e.Error()
	}
	return result, nil
}

func IsUnbanGraceActive(v []IPUnbanGrace, email, source string, now int64) bool {
	source = strings.TrimSpace(source)
	if strings.HasPrefix(source, "[") && strings.HasSuffix(source, "]") {
		source = source[1 : len(source)-1]
	}
	ip, err := netip.ParseAddr(source)
	if err != nil {
		return false
	}
	target := ip.Unmap().String()
	for _, rule := range v {
		if (rule.Email == "" || rule.Email == email) && rule.IP == target && rule.ExpiresAt > now {
			return true
		}
	}
	return false
}
