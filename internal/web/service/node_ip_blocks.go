package service

import (
	"context"
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
	return remote.ChangeIPBlock(ctx, req)
}
