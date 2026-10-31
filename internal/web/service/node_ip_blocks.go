package service

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/mhsanaei/3x-ui/v3/internal/web/entity"
	"github.com/mhsanaei/3x-ui/v3/internal/web/runtime"
)

func (s *NodeService) ipBlockRemote(id int) (*runtime.Remote, error) {
	if id <= 0 {
		return nil, fmt.Errorf("invalid node ID")
	}
	node, err := s.GetById(id)
	if err != nil {
		return nil, err
	}
	if node == nil || !node.Enable {
		return nil, fmt.Errorf("node is missing or disabled")
	}
	manager := runtime.GetManager()
	if manager == nil {
		return nil, fmt.Errorf("runtime manager unavailable")
	}
	return manager.RemoteFor(node)
}

func (s *NodeService) IPBlocks(ctx context.Context, id int) (*entity.IPBlockStatus, error) {
	remote, err := s.ipBlockRemote(id)
	if err != nil {
		return nil, err
	}
	return remote.IPBlocks(ctx)
}

func (s *NodeService) ChangeIPBlock(ctx context.Context, req entity.IPBlockChange) (*entity.IPBlockStatus, error) {
	remote, err := s.ipBlockRemote(req.NodeID)
	if err != nil {
		return nil, err
	}
	if req.Scope == "client" {
		if _, err := s.ScopedIPBlocks(ctx, req.NodeID); err != nil {
			return nil, err
		}
	}
	return remote.ChangeIPBlock(ctx, req)
}

func (s *NodeService) SharedIPClients(ctx context.Context, id int, ip string) (*entity.IPSharedClients, error) {
	canonical, err := normalizeBlockIP(ip)
	if err != nil {
		return nil, err
	}
	remote, err := s.ipBlockRemote(id)
	if err != nil {
		return nil, err
	}
	return remote.SharedIPClients(ctx, canonical)
}

func (s *NodeService) IPBlockSettings(ctx context.Context, id int) (*entity.IPBlockSettingsStatus, error) {
	remote, err := s.ipBlockRemote(id)
	if err != nil {
		return nil, err
	}
	v, err := remote.IPBlockSettings(ctx)
	if err != nil {
		return nil, err
	}
	if v.PolicyRevision < 1 {
		v.Available = false
		v.Reason = "upgrade this node to v3.9.11 before using new IP-limit controls"
		v.Settings.ConfirmScans = 1
		v.Settings.EnforcementScope = "host"
	}
	return v, nil
}
func (s *NodeService) SaveIPBlockSettings(ctx context.Context, id int, v entity.IPBlockSettings) (*entity.IPBlockSettingsStatus, error) {
	remote, err := s.ipBlockRemote(id)
	if err != nil {
		return nil, err
	}
	current, err := remote.IPBlockSettings(ctx)
	if err != nil {
		return nil, err
	}
	if current.PolicyRevision < 1 {
		return nil, fmt.Errorf("node does not support this IP policy revision; upgrade it to v3.9.11")
	}
	if v.EnforcementScope == "client" && v.Enabled {
		if _, err := s.ScopedIPBlocks(ctx, id); err != nil {
			return nil, err
		}
	}
	result, err := remote.SaveIPBlockSettings(ctx, v)
	if err != nil {
		return nil, err
	}
	if result.PolicyRevision < 1 {
		return nil, fmt.Errorf("node did not confirm the requested IP policy revision")
	}
	return result, nil
}

func (s *NodeService) ScopedIPBlocks(ctx context.Context, id int) (*entity.IPBlockStatus, error) {
	r, e := s.ipBlockRemote(id)
	if e != nil {
		return nil, e
	}
	v, err := r.ScopedIPBlocks(ctx)
	if err != nil {
		return nil, err
	}
	if v.Jail != "Fara core / account + IP" {
		return nil, fmt.Errorf("node does not support account-scoped IP blocks; upgrade the node and its bundled core to v3.9.11")
	}
	return v, nil
}

func (s *NodeService) IPLimitOverview(ctx context.Context, id int, search string, hours ...int) (*IPLimitOverview, error) {
	r, e := s.ipBlockRemote(id)
	if e != nil {
		return nil, e
	}
	raw, e := r.IPLimitOverview(ctx, search, hours...)
	if e != nil {
		return nil, e
	}
	var v IPLimitOverview
	e = json.Unmarshal(raw, &v)
	return &v, e
}
