package runtime

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"

	"github.com/mhsanaei/3x-ui/v3/internal/web/entity"
)

func (r *Remote) IPBlocks(ctx context.Context) (*entity.IPBlockStatus, error) {
	env, err := r.do(ctx, http.MethodGet, "panel/api/server/ipBlocks", nil)
	if err != nil {
		return nil, err
	}
	var status entity.IPBlockStatus
	if err := json.Unmarshal(env.Obj, &status); err != nil {
		return nil, err
	}
	return &status, nil
}

func (r *Remote) ChangeIPBlock(ctx context.Context, req entity.IPBlockChange) (*entity.IPBlockStatus, error) {
	req.NodeID = 0
	env, err := r.do(ctx, http.MethodPost, "panel/api/server/ipBlocks/change", req)
	if err != nil {
		return nil, err
	}
	var status entity.IPBlockStatus
	if err := json.Unmarshal(env.Obj, &status); err != nil {
		return nil, err
	}
	return &status, nil
}

func (r *Remote) SharedIPClients(ctx context.Context, ip string) (*entity.IPSharedClients, error) {
	env, err := r.do(ctx, http.MethodGet, "panel/api/server/ipBlocks/clients?ip="+url.QueryEscape(ip), nil)
	if err != nil {
		return nil, err
	}
	var result entity.IPSharedClients
	if err := json.Unmarshal(env.Obj, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (r *Remote) IPBlockSettings(ctx context.Context) (*entity.IPBlockSettingsStatus, error) {
	env, err := r.do(ctx, http.MethodGet, "panel/api/server/ipBlocks/settings", nil)
	if err != nil {
		return nil, err
	}
	var v entity.IPBlockSettingsStatus
	if err = json.Unmarshal(env.Obj, &v); err != nil {
		return nil, err
	}
	return &v, nil
}
func (r *Remote) SaveIPBlockSettings(ctx context.Context, v entity.IPBlockSettings) (*entity.IPBlockSettingsStatus, error) {
	env, err := r.do(ctx, http.MethodPost, "panel/api/server/ipBlocks/settings", v)
	if err != nil {
		return nil, err
	}
	var result entity.IPBlockSettingsStatus
	if err = json.Unmarshal(env.Obj, &result); err != nil {
		return nil, err
	}
	return &result, nil
}
