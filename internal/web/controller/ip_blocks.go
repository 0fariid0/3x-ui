package controller

import (
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/mhsanaei/3x-ui/v3/internal/web/entity"
	"github.com/mhsanaei/3x-ui/v3/internal/web/service"
)

func (a *ServerController) getIPBlocks(c *gin.Context) {
	id := 0
	var err error
	if raw := c.Query("nodeId"); raw != "" {
		id, err = strconv.Atoi(raw)
		if err != nil || id < 0 {
			jsonObj(c, nil, fmt.Errorf("invalid node ID"))
			return
		}
	}
	var status *entity.IPBlockStatus
	if c.Query("scope") == "client" {
		if id > 0 {
			status, err = (&service.NodeService{}).ScopedIPBlocks(c.Request.Context(), id)
		} else {
			status, err = service.ScopedIPBlockStatus(c.Request.Context())
		}
	} else if id > 0 {
		status, err = (&service.NodeService{}).IPBlocks(c.Request.Context(), id)
	} else {
		status, err = (&service.IPBlockService{}).Status(c.Request.Context())
	}
	jsonObj(c, status, err)
}

func (a *ServerController) changeIPBlock(c *gin.Context) {
	var req entity.IPBlockChange
	if err := c.ShouldBind(&req); err != nil {
		jsonObj(c, nil, err)
		return
	}
	if req.NodeID < 0 {
		jsonObj(c, nil, fmt.Errorf("invalid node ID"))
		return
	}
	var status *entity.IPBlockStatus
	var err error
	if req.NodeID > 0 {
		status, err = (&service.NodeService{}).ChangeIPBlock(c.Request.Context(), req)
	} else {
		status, err = (&service.IPBlockService{}).Change(c.Request.Context(), req)
	}
	jsonObj(c, status, err)
}

func (a *ServerController) getIPSharedClients(c *gin.Context) {
	id := 0
	var err error
	if raw := c.Query("nodeId"); raw != "" {
		id, err = strconv.Atoi(raw)
		if err != nil || id < 0 {
			jsonObj(c, nil, fmt.Errorf("invalid node ID"))
			return
		}
	}
	var result *entity.IPSharedClients
	if id > 0 {
		result, err = (&service.NodeService{}).SharedIPClients(c.Request.Context(), id, c.Query("ip"))
	} else {
		result, err = (&service.IPBlockService{}).SharedClients(c.Query("ip"))
	}
	jsonObj(c, result, err)
}

func (a *ServerController) getIPBlockSettings(c *gin.Context) {
	id, err := ipBlockSettingsNode(c)
	if err != nil {
		jsonObj(c, nil, err)
		return
	}
	var result *entity.IPBlockSettingsStatus
	if id > 0 {
		result, err = (&service.NodeService{}).IPBlockSettings(c.Request.Context(), id)
	} else {
		result, err = (&service.IPBlockService{}).Settings(c.Request.Context())
	}
	jsonObj(c, result, err)
}
func ipBlockSettingsNode(c *gin.Context) (int, error) {
	raw := c.DefaultQuery("nodeId", "0")
	id, err := strconv.Atoi(raw)
	if err != nil || id < 0 {
		return 0, fmt.Errorf("invalid node ID")
	}
	return id, nil
}
func (a *ServerController) saveIPBlockSettings(c *gin.Context) {
	id, err := ipBlockSettingsNode(c)
	if err != nil {
		jsonObj(c, nil, err)
		return
	}
	var raw json.RawMessage
	if err = c.ShouldBindJSON(&raw); err != nil {
		jsonObj(c, nil, err)
		return
	}
	var fields map[string]json.RawMessage
	if err = json.Unmarshal(raw, &fields); err != nil || len(fields) == 0 {
		jsonObj(c, nil, fmt.Errorf("a nonempty JSON settings object is required"))
		return
	}
	for _, v := range fields {
		if string(v) == "null" {
			jsonObj(c, nil, fmt.Errorf("null settings values are not supported"))
			return
		}
	}
	var req entity.IPBlockSettings
	if id > 0 {
		current, e := (&service.NodeService{}).IPBlockSettings(c.Request.Context(), id)
		if e != nil {
			jsonObj(c, nil, e)
			return
		}
		req = current.Settings
	} else {
		current, e := service.LoadIPBlockSettings()
		if e != nil {
			jsonObj(c, nil, e)
			return
		}
		req = current
	}
	if err = json.Unmarshal(raw, &req); err != nil {
		jsonObj(c, nil, err)
		return
	}
	var result *entity.IPBlockSettingsStatus
	if id > 0 {
		result, err = (&service.NodeService{}).SaveIPBlockSettings(c.Request.Context(), id, req)
	} else {
		result, err = (&service.IPBlockService{}).SaveSettings(c.Request.Context(), req)
	}
	jsonObj(c, result, err)
}

func (a *ServerController) getIPLimitOverview(c *gin.Context) {
	hours, parseErr := strconv.Atoi(c.DefaultQuery("hours", "24"))
	if parseErr != nil || hours < 1 || hours > 744 {
		jsonObj(c, nil, fmt.Errorf("invalid time range"))
		return
	}
	id, err := ipBlockSettingsNode(c)
	if err != nil {
		jsonObj(c, nil, err)
		return
	}
	if id > 0 {
		v, e := (&service.NodeService{}).IPLimitOverview(c.Request.Context(), id, c.Query("search"), hours)
		jsonObj(c, v, e)
		return
	}
	v, e := service.ReadIPLimitOverview(c.Request.Context(), c.Query("search"), hours)
	jsonObj(c, v, e)
}
