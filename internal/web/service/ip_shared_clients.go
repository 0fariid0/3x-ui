package service

import (
	"encoding/json"
	"fmt"
	"net/netip"
	"sort"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/web/entity"
)

func (s *IPBlockService) SharedClients(value string) (*entity.IPSharedClients, error) {
	ip, err := normalizeBlockIP(value)
	if err != nil {
		return nil, err
	}
	guid, err := (&SettingService{}).GetPanelGuid()
	if err != nil {
		return nil, err
	}
	if guid == "" {
		return nil, fmt.Errorf("local server identity unavailable")
	}
	now := time.Now()
	result := &entity.IPSharedClients{IP: ip, Clients: []entity.IPSharedClient{}, WindowSeconds: clientIpStaleAfterSeconds, CheckedAt: now.UnixMilli()}
	rows, err := database.GetDB().Model(&model.NodeClientIp{}).Select("email", "ips").Where("node_guid = ?", guid).Rows()
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	latest := make(map[string]int64)
	for rows.Next() {
		var email, raw string
		if err := rows.Scan(&email, &raw); err != nil {
			return nil, err
		}
		if email == "" {
			continue
		}
		var entries []model.ClientIpEntry
		if json.Unmarshal([]byte(raw), &entries) != nil {
			continue
		}
		for _, entry := range entries {
			if entry.Timestamp < now.Unix()-clientIpStaleAfterSeconds {
				continue
			}
			address, err := netip.ParseAddr(entry.IP)
			if err == nil && address.Zone() == "" && address.Unmap().String() == ip && entry.Timestamp > latest[email] {
				latest[email] = entry.Timestamp
			}
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for email, seen := range latest {
		result.Clients = append(result.Clients, entity.IPSharedClient{Email: email, LastSeen: seen * 1000})
	}
	sort.Slice(result.Clients, func(i, j int) bool {
		if result.Clients[i].LastSeen == result.Clients[j].LastSeen {
			return result.Clients[i].Email < result.Clients[j].Email
		}
		return result.Clients[i].LastSeen > result.Clients[j].LastSeen
	})
	return result, nil
}
