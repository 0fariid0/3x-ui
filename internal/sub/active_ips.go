package sub

import (
    "encoding/json"
    "net/http"
    "strings"
    "time"
    "github.com/gin-gonic/gin"
    "github.com/mhsanaei/3x-ui/v3/internal/database"
    "github.com/mhsanaei/3x-ui/v3/internal/database/model"
    "github.com/mhsanaei/3x-ui/v3/internal/web/service"
)

type activeIPEntry struct { IP string `json:"ip"`; Timestamp int64 `json:"timestamp"` }

func activeIPCountHandler(c *gin.Context) {
    var settings service.SettingService
    enabled, err := settings.GetSubActiveIpApiEnable()
    if err != nil || !enabled { c.JSON(http.StatusNotFound, gin.H{"success":false,"msg":"active ip api disabled"}); return }
    subID:=strings.TrimSpace(c.Param("subid"))
    if subID=="" { c.JSON(http.StatusBadRequest,gin.H{"success":false,"msg":"invalid sub id"}); return }
    db:=database.GetDB()
    var clients []model.Client
    if err:=db.Where("sub_id = ?",subID).Find(&clients).Error; err!=nil { c.JSON(http.StatusInternalServerError,gin.H{"success":false}); return }
    if len(clients)==0 { c.JSON(http.StatusNotFound,gin.H{"success":false,"msg":"subscription not found"}); return }
    emails:=make([]string,0,len(clients))
    for _,cl:=range clients { if e:=strings.ToLower(strings.TrimSpace(cl.Email)); e!="" { emails=append(emails,e) } }
    var rows []model.InboundClientIps
    if err:=db.Where("LOWER(client_email) IN ?",emails).Find(&rows).Error; err!=nil { c.JSON(http.StatusInternalServerError,gin.H{"success":false}); return }
    cutoff:=time.Now().Add(-20*time.Second).Unix()
    unique:=map[string]struct{}{}
    for _,row:=range rows {
        var entries []activeIPEntry
        if json.Unmarshal([]byte(row.Ips),&entries)!=nil { continue }
        for _,x:=range entries { if x.Timestamp>=cutoff && strings.TrimSpace(x.IP)!="" { unique[x.IP]=struct{}{} } }
    }
    c.Header("Cache-Control","no-store, no-cache, must-revalidate, max-age=0")
    c.Header("Pragma","no-cache")
    c.Header("Access-Control-Allow-Origin","*")
    c.JSON(http.StatusOK,gin.H{"enabled":true,"count":len(unique),"updatedAt":time.Now().UnixMilli()})
}
