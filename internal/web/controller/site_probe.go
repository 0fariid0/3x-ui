package controller

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/mhsanaei/3x-ui/v3/internal/web/service"
)

func (a *ServerController) getSiteProbes(c *gin.Context) {
	view, err := a.settingService.GetSiteProbes()
	jsonObj(c, view, err)
}

func (a *ServerController) saveSiteProbes(c *gin.Context) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 48<<10)
	var req struct {
		Targets *[]service.SiteProbeTarget `json:"targets"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.Targets == nil {
		jsonObj(c, nil, errors.New("invalid probe list"))
		return
	}
	view, err := a.settingService.SaveSiteProbes(*req.Targets)
	jsonObj(c, view, err)
}

func (a *ServerController) runSiteProbes(c *gin.Context) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 4<<10)
	var req struct {
		URL string `json:"url"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		jsonObj(c, nil, errors.New("invalid probe request"))
		return
	}
	view, err := a.settingService.StartSiteProbes(req.URL, true)
	jsonObj(c, view, err)
}

func (a *ServerController) getOutboundProbeOptions(c *gin.Context) {
	options, err := a.settingService.OutboundProbeOptions()
	jsonObj(c, options, err)
}

func (a *ServerController) saveOutboundProbes(c *gin.Context) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 24<<10)
	var req struct {
		Targets *[]service.OutboundProbeTarget `json:"targets"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.Targets == nil {
		jsonObj(c, nil, errors.New("invalid outbound probe list"))
		return
	}
	view, err := a.settingService.SaveOutboundProbes(*req.Targets)
	jsonObj(c, view, err)
}
