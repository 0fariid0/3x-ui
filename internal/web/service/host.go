package service

import (
	"net"
	"slices"
	"sort"
	"strconv"
	"strings"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/util/common"
	"github.com/mhsanaei/3x-ui/v3/internal/util/random"
	"github.com/mhsanaei/3x-ui/v3/internal/web/entity"

	"gorm.io/gorm"
)

type HostService struct{}

func newHostGroup(h *model.Host, groupId string) *entity.HostGroup {
	return &entity.HostGroup{
		GroupId:                groupId,
		InboundIds:             []int{},
		Hosts:                  []string{},
		SortOrder:              h.SortOrder,
		Remark:                 h.Remark,
		ServerDescription:      h.ServerDescription,
		IsDisabled:             h.IsDisabled,
		IsHidden:               h.IsHidden,
		Tags:                   h.Tags,
		Port:                   h.Port,
		Security:               h.Security,
		Sni:                    h.Sni,
		HostHeader:             h.HostHeader,
		HostHeaders:            map[string]string{},
		Path:                   h.Path,
		Alpn:                   h.Alpn,
		Fingerprint:            h.Fingerprint,
		OverrideSniFromAddress: h.OverrideSniFromAddress,
		KeepSniBlank:           h.KeepSniBlank,
		PinnedPeerCertSha256:   h.PinnedPeerCertSha256,
		VerifyPeerCertByName:   h.VerifyPeerCertByName,
		AllowInsecure:          h.AllowInsecure,
		EchConfigList:          h.EchConfigList,
		MuxParams:              h.MuxParams,
		SockoptParams:          h.SockoptParams,
		FinalMask:              h.FinalMask,
		VlessRoute:             h.VlessRoute,
		ExcludeFromSubTypes:    h.ExcludeFromSubTypes,
		NodeGuids:              h.NodeGuids,
		MihomoIpVersion:        h.MihomoIpVersion,
		MihomoX25519:           h.MihomoX25519,
		ShuffleHost:            h.ShuffleHost,
	}
}

func groupHosts(hosts []*model.Host) []*entity.HostGroup {
	groupsMap := make(map[string]*entity.HostGroup)
	var orderedGroupIds []string

	for _, h := range hosts {
		gId := h.GroupId
		if gId == "" {
			gId = "fallback_" + strconv.Itoa(h.Id)
		}

		g, exists := groupsMap[gId]
		if !exists {
			g = newHostGroup(h, gId)
			groupsMap[gId] = g
			orderedGroupIds = append(orderedGroupIds, gId)
		}

		if !slices.Contains(g.InboundIds, h.InboundId) {
			g.InboundIds = append(g.InboundIds, h.InboundId)
		}
		hostStr := h.Address
		if !slices.Contains(g.Hosts, hostStr) {
			g.Hosts = append(g.Hosts, hostStr)
		}
		if hostStr != "" {
			g.HostHeaders[hostStr] = h.HostHeader
		}
		if h.SortOrder < g.SortOrder {
			g.SortOrder = h.SortOrder
		}
	}

	res := make([]*entity.HostGroup, 0, len(orderedGroupIds))
	for _, gId := range orderedGroupIds {
		res = append(res, groupsMap[gId])
	}

	sort.SliceStable(res, func(i, j int) bool {
		if res[i].SortOrder != res[j].SortOrder {
			return res[i].SortOrder < res[j].SortOrder
		}
		return res[i].Remark < res[j].Remark
	})

	return res
}

// populateHostClientOverrides attaches the emails explicitly opted into each
// globally disabled Host group. Globally enabled groups are active for every
// client by default and intentionally return an empty list.
func populateHostClientOverrides(groups []*entity.HostGroup, hosts []*model.Host) error {
	if len(groups) == 0 || len(hosts) == 0 {
		return nil
	}
	keyToGroup := make(map[string]string)
	keyToInbound := make(map[string]int)
	keys := make([]string, 0)
	inboundIDs := make([]int, 0)
	for _, host := range hosts {
		if host == nil || !host.IsDisabled {
			continue
		}
		key := host.SubscriptionLinkKey()
		groupID := host.GroupId
		if groupID == "" {
			groupID = "fallback_" + strconv.Itoa(host.Id)
		}
		keyToGroup[key] = groupID
		keyToInbound[key] = host.InboundId
		keys = append(keys, key)
		inboundIDs = append(inboundIDs, host.InboundId)
	}
	if len(keys) == 0 {
		return nil
	}
	var inclusions []model.ClientSubscriptionLinkInclusion
	if err := database.GetDB().Where("link_key IN ?", keys).Find(&inclusions).Error; err != nil {
		return err
	}
	if len(inclusions) == 0 {
		return nil
	}
	clientIDs := make([]int, 0, len(inclusions))
	for _, row := range inclusions {
		clientIDs = append(clientIDs, row.ClientId)
	}
	var clients []model.ClientRecord
	if err := database.GetDB().Where("id IN ?", clientIDs).Find(&clients).Error; err != nil {
		return err
	}
	emailByID := make(map[int]string, len(clients))
	for _, client := range clients {
		emailByID[client.Id] = client.Email
	}

	var activeInboundIDs []int
	if err := database.GetDB().Model(&model.Inbound{}).Where("id IN ? AND enable = ?", inboundIDs, true).Pluck("id", &activeInboundIDs).Error; err != nil {
		return err
	}
	activeInbound := make(map[int]struct{}, len(activeInboundIDs))
	for _, inboundID := range activeInboundIDs {
		activeInbound[inboundID] = struct{}{}
	}
	var attachments []model.ClientInbound
	if len(activeInboundIDs) > 0 {
		if err := database.GetDB().Where("client_id IN ? AND inbound_id IN ?", clientIDs, activeInboundIDs).Find(&attachments).Error; err != nil {
			return err
		}
	}
	attached := make(map[[2]int]struct{}, len(attachments))
	for _, row := range attachments {
		attached[[2]int{row.ClientId, row.InboundId}] = struct{}{}
	}

	emailsByGroup := make(map[string]map[string]struct{})
	for _, row := range inclusions {
		inboundID := keyToInbound[row.LinkKey]
		if _, ok := activeInbound[inboundID]; !ok {
			continue
		}
		if _, ok := attached[[2]int{row.ClientId, inboundID}]; !ok {
			continue
		}
		groupID := keyToGroup[row.LinkKey]
		email := strings.TrimSpace(emailByID[row.ClientId])
		if groupID == "" || email == "" {
			continue
		}
		if emailsByGroup[groupID] == nil {
			emailsByGroup[groupID] = make(map[string]struct{})
		}
		emailsByGroup[groupID][email] = struct{}{}
	}
	for _, group := range groups {
		if group == nil || !group.IsDisabled {
			continue
		}
		set := emailsByGroup[group.GroupId]
		group.EnabledClientEmails = make([]string, 0, len(set))
		for email := range set {
			group.EnabledClientEmails = append(group.EnabledClientEmails, email)
		}
		sort.Strings(group.EnabledClientEmails)
	}
	return nil
}

func buildHostRows(groupId string, req *entity.HostGroup) []*model.Host {
	hostsToProcess := req.Hosts
	if len(hostsToProcess) == 0 {
		hostsToProcess = []string{""}
	}
	var rows []*model.Host
	for _, hostStr := range hostsToProcess {
		addr := normalizeHostAddress(hostStr)
		port := req.Port
		if port == 0 {
			if _, embeddedPort := splitHostAddressPort(hostStr); embeddedPort > 0 {
				port = embeddedPort
			}
		}
		hostHeader := req.HostHeader
		if req.HostHeaders != nil {
			mapped, ok := req.HostHeaders[addr]
			if !ok {
				mapped, ok = req.HostHeaders[strings.TrimSpace(hostStr)]
			}
			if ok {
				hostHeader = strings.TrimSpace(mapped)
			}
		}
		for _, inboundId := range req.InboundIds {
			rows = append(rows, &model.Host{
				GroupId:                groupId,
				InboundId:              inboundId,
				SortOrder:              req.SortOrder,
				Remark:                 req.Remark,
				ServerDescription:      req.ServerDescription,
				IsDisabled:             req.IsDisabled,
				IsHidden:               req.IsHidden,
				Tags:                   req.Tags,
				Address:                addr,
				Port:                   port,
				Security:               req.Security,
				Sni:                    req.Sni,
				HostHeader:             hostHeader,
				Path:                   req.Path,
				Alpn:                   req.Alpn,
				Fingerprint:            req.Fingerprint,
				OverrideSniFromAddress: req.OverrideSniFromAddress,
				KeepSniBlank:           req.KeepSniBlank,
				PinnedPeerCertSha256:   req.PinnedPeerCertSha256,
				VerifyPeerCertByName:   req.VerifyPeerCertByName,
				AllowInsecure:          req.AllowInsecure,
				EchConfigList:          req.EchConfigList,
				MuxParams:              req.MuxParams,
				SockoptParams:          req.SockoptParams,
				FinalMask:              req.FinalMask,
				VlessRoute:             req.VlessRoute,
				ExcludeFromSubTypes:    req.ExcludeFromSubTypes,
				NodeGuids:              req.NodeGuids,
				MihomoIpVersion:        req.MihomoIpVersion,
				MihomoX25519:           req.MihomoX25519,
				ShuffleHost:            req.ShuffleHost,
			})
		}
	}
	return rows
}

// adoptedHostRows projects a node's host groups onto a freshly adopted central
// inbound so TLS/SNI/fingerprint overrides survive the node-to-master import.
func adoptedHostRows(groups []*entity.HostGroup, nodeInboundId, centralInboundId int) []*model.Host {
	var rows []*model.Host
	for _, g := range groups {
		if g == nil || !slices.Contains(g.InboundIds, nodeInboundId) {
			continue
		}
		scoped := *g
		scoped.InboundIds = []int{centralInboundId}
		rows = append(rows, buildHostRows(g.GroupId, &scoped)...)
	}
	return rows
}

func validateInboundsExist(tx *gorm.DB, inboundIds []int) error {
	for _, inboundId := range inboundIds {
		var count int64
		if err := tx.Model(&model.Inbound{}).Where("id = ?", inboundId).Count(&count).Error; err != nil {
			return err
		}
		if count == 0 {
			return common.NewError("inbound not found")
		}
	}
	return nil
}

func (s *HostService) GetHosts() ([]*entity.HostGroup, error) {
	var hosts []*model.Host
	err := database.GetDB().Order("inbound_id asc, sort_order asc, id asc").Find(&hosts).Error
	if err != nil {
		return nil, err
	}
	groups := groupHosts(hosts)
	if err := populateHostClientOverrides(groups, hosts); err != nil {
		return nil, err
	}
	return groups, nil
}

func (s *HostService) GetHostsByInbound(inboundId int) ([]*entity.HostGroup, error) {
	var groupIds []string
	if err := database.GetDB().Model(&model.Host{}).Where("inbound_id = ?", inboundId).Distinct().Pluck("group_id", &groupIds).Error; err != nil {
		return nil, err
	}
	if len(groupIds) == 0 {
		return nil, nil
	}
	var hosts []*model.Host
	if err := database.GetDB().Where("group_id IN ?", groupIds).Order("sort_order asc, id asc").Find(&hosts).Error; err != nil {
		return nil, err
	}
	groups := groupHosts(hosts)
	if err := populateHostClientOverrides(groups, hosts); err != nil {
		return nil, err
	}
	return groups, nil
}

func (s *HostService) GetHostGroup(groupId string) (*entity.HostGroup, error) {
	var hosts []*model.Host
	err := database.GetDB().Where("group_id = ?", groupId).Order("sort_order asc, id asc").Find(&hosts).Error
	if err != nil {
		return nil, err
	}
	if len(hosts) == 0 {
		return nil, common.NewError("host not found")
	}
	grouped := groupHosts(hosts)
	if len(grouped) == 0 {
		return nil, common.NewError("host not found")
	}
	if err := populateHostClientOverrides(grouped, hosts); err != nil {
		return nil, err
	}
	return grouped[0], nil
}

func (s *HostService) AddHostGroup(req *entity.HostGroup) ([]*model.Host, error) {
	groupId := req.GroupId
	if groupId == "" {
		groupId = random.NumLower(16)
	}
	created := buildHostRows(groupId, req)

	err := database.GetDB().Transaction(func(tx *gorm.DB) error {
		if err := validateInboundsExist(tx, req.InboundIds); err != nil {
			return err
		}
		if len(created) > 0 {
			return tx.Create(&created).Error
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return created, nil
}

func (s *HostService) UpdateHostGroup(groupId string, req *entity.HostGroup) ([]*model.Host, error) {
	created := buildHostRows(groupId, req)

	err := database.GetDB().Transaction(func(tx *gorm.DB) error {
		var count int64
		if err := tx.Model(&model.Host{}).Where("group_id = ?", groupId).Count(&count).Error; err != nil {
			return err
		}
		if count == 0 {
			return common.NewError("host not found")
		}
		if err := validateInboundsExist(tx, req.InboundIds); err != nil {
			return err
		}
		if err := tx.Where("group_id = ?", groupId).Delete(&model.Host{}).Error; err != nil {
			return err
		}
		if len(created) > 0 {
			return tx.Create(&created).Error
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return created, nil
}

func (s *HostService) DeleteHostGroup(groupId string) error {
	return database.GetDB().Where("group_id = ?", groupId).Delete(&model.Host{}).Error
}

func (s *HostService) SetHostGroupEnable(groupId string, enable bool) error {
	return database.GetDB().Model(&model.Host{}).Where("group_id = ?", groupId).Update("is_disabled", !enable).Error
}

func (s *HostService) SetHostsGroupEnable(groupIds []string, enable bool) error {
	if len(groupIds) == 0 {
		return nil
	}
	return database.GetDB().Model(&model.Host{}).Where("group_id IN ?", groupIds).Update("is_disabled", !enable).Error
}

func (s *HostService) DeleteHostsGroup(groupIds []string) error {
	if len(groupIds) == 0 {
		return nil
	}
	return database.GetDB().Where("group_id IN ?", groupIds).Delete(&model.Host{}).Error
}

func (s *HostService) ReorderHostGroups(groupIds []string) error {
	if len(groupIds) == 0 {
		return nil
	}
	return database.GetDB().Transaction(func(tx *gorm.DB) error {
		for i, groupId := range groupIds {
			if err := tx.Model(&model.Host{}).Where("group_id = ?", groupId).Update("sort_order", i).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

func (s *HostService) GetAllTags() ([]string, error) {
	var hosts []*model.Host
	err := database.GetDB().Find(&hosts).Error
	if err != nil {
		return nil, err
	}
	set := make(map[string]struct{})
	for _, h := range hosts {
		for _, tag := range h.Tags {
			if tag != "" {
				set[tag] = struct{}{}
			}
		}
	}
	out := make([]string, 0, len(set))
	for tag := range set {
		out = append(out, tag)
	}
	sort.Strings(out)
	return out, nil
}

func normalizeHostAddress(hostStr string) string {
	hostStr = strings.TrimSpace(hostStr)
	if hostStr == "" {
		return ""
	}
	if strings.HasPrefix(hostStr, "[") {
		if end := strings.Index(hostStr, "]"); end > 0 {
			return hostStr[1:end]
		}
	}
	// A single colon is treated as host:port and the port is deliberately
	// discarded. Host rows have one explicit Port field shared by all addresses;
	// Port=0 means inherit the inbound port. Bare IPv6 contains multiple colons
	// and is preserved unchanged.
	if strings.Count(hostStr, ":") == 1 {
		return strings.TrimSpace(strings.SplitN(hostStr, ":", 2)[0])
	}
	return strings.Trim(hostStr, "[]")
}

// splitHostAddressPort extracts an optional port from host:port or
// [IPv6]:port input. Callers retain their explicit group port when set and use
// this only as a compatibility fallback for imported node snapshots.
func splitHostAddressPort(hostStr string) (string, int) {
	hostStr = strings.TrimSpace(hostStr)
	if hostStr == "" {
		return "", 0
	}
	host, portText, err := net.SplitHostPort(hostStr)
	if err != nil {
		return normalizeHostAddress(hostStr), 0
	}
	port, err := strconv.Atoi(portText)
	if err != nil || port < 1 || port > 65535 {
		return normalizeHostAddress(hostStr), 0
	}
	return strings.TrimSpace(host), port
}

func (s *HostService) GetSubscriptionDisplayHost() (*entity.SubscriptionDisplayHost, error) {
	settingService := SettingService{}
	enabled, err := settingService.GetSubscriptionDisplayHostEnable()
	if err != nil {
		return nil, err
	}
	remark, err := settingService.GetSubscriptionDisplayHostRemark()
	if err != nil {
		return nil, err
	}
	return &entity.SubscriptionDisplayHost{Enable: enabled, Remark: remark}, nil
}

func (s *HostService) UpdateSubscriptionDisplayHost(config *entity.SubscriptionDisplayHost) error {
	settingService := SettingService{}
	return settingService.UpdateSubscriptionDisplayHost(config)
}
