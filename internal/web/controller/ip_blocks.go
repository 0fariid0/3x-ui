package controller

import (
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
	if id > 0 {
		status, err = (&service.NodeService{}).IPBlocks(c.Request.Context(), id)
	} else {
		status, err = (&service.IPBlockService{}).Status(c.Request.Context())
	}
	jsonObj(c, status, err)
}

func (a *ServerController) changeIPBlock(c *gin.Context) {
	var req entity.IPBlockChange
	if err := c.ShouldBindJSON(&req); err != nil {
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
