package controller

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

type activeIPApiSettingForm struct {
	Enabled bool `json:"enabled" form:"enabled"`
}

func (a *SettingController) getActiveIPApiSetting(c *gin.Context) {
	enabled, err := a.settingService.GetSubActiveIpApiEnable()
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"msg":     err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"obj": gin.H{
			"enabled": enabled,
		},
	})
}

func (a *SettingController) setActiveIPApiSetting(c *gin.Context) {
	form := &activeIPApiSettingForm{}
	if err := c.ShouldBindJSON(form); err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"msg":     err.Error(),
		})
		return
	}

	if err := a.settingService.SetSubActiveIpApiEnable(form.Enabled); err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"msg":     err.Error(),
		})
		return
	}

	saved, err := a.settingService.GetSubActiveIpApiEnable()
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"msg":     err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"obj": gin.H{
			"enabled": saved,
		},
	})
}
