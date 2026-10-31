package service

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/netip"
	"os"
	"os/exec"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/util/clientip"
	"github.com/mhsanaei/3x-ui/v3/internal/web/entity"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"
)

const ipBlockJail = "3x-ipl"

var ipBlockMutationMu sync.Mutex

var ipBlockLogPattern = regexp.MustCompile(`^(\d{4}/\d{2}/\d{2} \d{2}:\d{2}:\d{2})\s+(BAN|UNBAN)\s+\[Email\]\s*=\s*(.*?)\s*\[IP\]\s*=\s*(\S+)\s+(?:banned for (-?\d+) seconds\.|unbanned\.)`)

type IPBlockService struct{}

type ipBlockAudit struct {
	IP         string `json:"ip"`
	Action     string `json:"action"`
	BanSeconds int64  `json:"banSeconds"`
}

func normalizeBlockIP(value string) (string, error) {
	if len(value) > 128 {
		return "", fmt.Errorf("invalid IP address")
	}
	ip, err := netip.ParseAddr(strings.TrimSpace(value))
	if err != nil || ip.Zone() != "" {
		return "", fmt.Errorf("invalid IPv4 or IPv6 address")
	}
	ip = ip.Unmap()
	if ip.IsUnspecified() || ip.IsLoopback() || ip.IsMulticast() || ip.IsLinkLocalUnicast() || ip == netip.AddrFrom4([4]byte{255, 255, 255, 255}) {
		return "", fmt.Errorf("this IP address cannot be blocked")
	}
	return ip.String(), nil
}

func runIPBlockCommand(ctx context.Context, args ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	output, err := exec.CommandContext(ctx, "fail2ban-client", args...).CombinedOutput()
	if err != nil {
		if len(output) > 2048 {
			output = output[:2048]
		}
		return nil, fmt.Errorf("fail2ban: %w: %s", err, strings.TrimSpace(string(output)))
	}
	return output, nil
}

func parseBlockedIPs(output []byte) ([]string, error) {
	clean := strings.Map(func(r rune) rune {
		if strings.ContainsRune("[],'\"", r) {
			return ' '
		}
		return r
	}, string(output))
	seen := make(map[string]bool)
	ips := make([]string, 0)
	for _, token := range strings.Fields(clean) {
		ip, err := netip.ParseAddr(token)
		if err != nil || ip.Zone() != "" {
			return nil, fmt.Errorf("invalid fail2ban ban list response")
		}
		canonical := ip.Unmap().String()
		if !seen[canonical] {
			seen[canonical] = true
			ips = append(ips, canonical)
		}
	}
	sort.Strings(ips)
	return ips, nil
}

func readIPBlockLog(path string, entries map[string]entity.IPBlockEntry) {
	file, err := os.Open(path)
	if err != nil {
		return
	}
	defer file.Close()
	if stat, err := file.Stat(); err == nil && stat.Size() > 4<<20 {
		if _, err := file.Seek(-(4 << 20), io.SeekEnd); err != nil {
			return
		}
	}
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 4096), 1<<20)
	for scanner.Scan() {
		match := ipBlockLogPattern.FindStringSubmatch(scanner.Text())
		if match == nil {
			continue
		}
		ip, err := netip.ParseAddr(match[4])
		if err != nil {
			continue
		}
		key := ip.Unmap().String()
		if match[2] == "UNBAN" {
			delete(entries, key)
			continue
		}
		at, err := time.ParseInLocation("2006/01/02 15:04:05", match[1], time.Local)
		if err != nil {
			continue
		}
		seconds, err := strconv.ParseInt(match[5], 10, 64)
		if err != nil {
			continue
		}
		entry := entity.IPBlockEntry{IP: key, Source: "unknown", BannedAt: at.UnixMilli(), Permanent: seconds < 0}
		if match[3] != "" && !strings.ContainsAny(match[3], "<>") {
			entry.Source, entry.Email = "automatic", match[3]
		}
		if seconds > 0 && seconds < 10*365*86400 {
			entry.ExpiresAt = at.Add(time.Duration(seconds) * time.Second).UnixMilli()
		}
		entries[key] = entry
	}
}

func enrichIPBlocks(entries map[string]entity.IPBlockEntry) {
	if database.GetDB() == nil {
		return
	}
	var events []model.ClientEvent
	if database.GetDB().Where("kind IN ?", []string{"ip_blocked", "ip_unblocked"}).Order("created_at DESC, id DESC").Limit(5000).Find(&events).Error != nil {
		return
	}
	seen := make(map[string]bool)
	for _, event := range events {
		var audit ipBlockAudit
		if json.Unmarshal([]byte(event.Details), &audit) != nil || seen[audit.IP] {
			continue
		}
		seen[audit.IP] = true
		if event.Kind != "ip_blocked" {
			continue
		}
		logged, exists := entries[audit.IP]
		if exists && logged.BannedAt > event.CreatedAt {
			continue
		}
		entry := entity.IPBlockEntry{IP: audit.IP, Source: "manual", Email: event.Email, BannedAt: event.CreatedAt, Permanent: audit.BanSeconds < 0}
		if audit.BanSeconds > 0 && audit.BanSeconds < 10*365*86400 {
			entry.ExpiresAt = event.CreatedAt + audit.BanSeconds*1000
		}
		entries[audit.IP] = entry
	}
}

func (s *IPBlockService) Status(ctx context.Context) (*entity.IPBlockStatus, error) {
	status := &entity.IPBlockStatus{Jail: ipBlockJail, Blocks: []entity.IPBlockEntry{}}
	if runtime.GOOS == "windows" {
		status.Reason = "windows"
		return status, nil
	}
	if !isFail2banEnabled() {
		status.Reason = "disabled"
		return status, nil
	}
	if _, err := exec.LookPath("fail2ban-client"); err != nil {
		status.Reason = "not_installed"
		return status, nil
	}
	output, err := runIPBlockCommand(ctx, "get", ipBlockJail, "banip")
	if err != nil {
		return nil, fmt.Errorf("IP block status unavailable; check the running 3x-ipl jail and permissions: %w", err)
	}
	ips, err := parseBlockedIPs(output)
	if err != nil {
		return nil, err
	}
	status.Available = true
	if output, err := runIPBlockCommand(ctx, "get", ipBlockJail, "bantime"); err == nil {
		status.BanSeconds, _ = strconv.ParseInt(strings.TrimSpace(string(output)), 10, 64)
	}
	entries := make(map[string]entity.IPBlockEntry)
	readIPBlockLog(xray.GetIPLimitBannedPrevLogPath(), entries)
	readIPBlockLog(xray.GetIPLimitBannedLogPath(), entries)
	enrichIPBlocks(entries)
	for _, ip := range ips {
		entry, exists := entries[ip]
		if !exists {
			entry = entity.IPBlockEntry{IP: ip, Source: "unknown"}
		}
		status.Blocks = append(status.Blocks, entry)
	}
	return status, nil
}

func (s *IPBlockService) Change(ctx context.Context, req entity.IPBlockChange) (*entity.IPBlockStatus, error) {
	ip, err := normalizeBlockIP(req.IP)
	if err != nil {
		return nil, err
	}
	if req.Action != "ban" && req.Action != "unban" {
		return nil, fmt.Errorf("action must be ban or unban")
	}
	if req.NodeID != 0 || len(req.Email) > 255 || strings.ContainsAny(req.Email, "\r\n\x00") {
		return nil, fmt.Errorf("invalid IP block request")
	}
	if req.Action == "ban" && clientip.IsTunnelLocal(ip) {
		return nil, fmt.Errorf("GRE-TUN local addresses are excluded from IP limits and cannot be blocked")
	}
	ipBlockMutationMu.Lock()
	defer ipBlockMutationMu.Unlock()
	before, err := s.Status(ctx)
	if err != nil {
		return nil, err
	}
	if !before.Available {
		return nil, fmt.Errorf("IP blocking unavailable: %s", before.Reason)
	}
	blocked := false
	for _, entry := range before.Blocks {
		if entry.IP == ip {
			blocked = true
			break
		}
	}
	if blocked == (req.Action == "ban") {
		return before, nil
	}
	command := "banip"
	if req.Action == "unban" {
		command = "unbanip"
	}
	if _, err := runIPBlockCommand(ctx, "set", ipBlockJail, command, ip); err != nil {
		return nil, err
	}
	after, err := s.Status(ctx)
	if err != nil {
		return nil, fmt.Errorf("command sent but verification failed; refresh before retrying: %w", err)
	}
	blocked = false
	for _, entry := range after.Blocks {
		if entry.IP == ip {
			blocked = true
			break
		}
	}
	if blocked != (req.Action == "ban") {
		return nil, fmt.Errorf("fail2ban did not confirm the requested IP state; refresh before retrying")
	}
	kind := "ip_blocked"
	if req.Action == "unban" {
		kind = "ip_unblocked"
	}
	details, _ := json.Marshal(ipBlockAudit{IP: ip, Action: req.Action, BanSeconds: before.BanSeconds})
	var auditErr error
	if database.GetDB() == nil {
		auditErr = fmt.Errorf("database unavailable")
	} else {
		auditErr = database.GetDB().Create(&model.ClientEvent{Email: strings.TrimSpace(req.Email), Kind: kind, Summary: ip, Details: string(details), CreatedAt: time.Now().UnixMilli()}).Error
	}
	if auditErr != nil {
		after.AuditWarning = true
	} else if req.Action == "ban" {
		for i := range after.Blocks {
			if after.Blocks[i].IP == ip {
				after.Blocks[i].Source, after.Blocks[i].Email = "manual", req.Email
				after.Blocks[i].BannedAt = time.Now().UnixMilli()
				if before.BanSeconds > 0 && before.BanSeconds < 10*365*86400 {
					after.Blocks[i].ExpiresAt = after.Blocks[i].BannedAt + before.BanSeconds*1000
				}
				after.Blocks[i].Permanent = before.BanSeconds < 0
			}
		}
	}
	return after, nil
}
