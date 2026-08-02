package service

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/logger"
	"github.com/mhsanaei/3x-ui/v3/internal/web/entity"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const insightBytesPerMB int64 = 1024 * 1024

// ClientInsightService owns durable per-client reporting and abnormal usage
// detection. The zero value is ready to use, matching the other panel services.
type ClientInsightService struct{}

type ClientDailyUsage struct {
	Day   string `json:"day"`
	Up    int64  `json:"up"`
	Down  int64  `json:"down"`
	Total int64  `json:"total"`
}

type ClientHourlyUsage struct {
	Hour  int   `json:"hour"`
	Up    int64 `json:"up"`
	Down  int64 `json:"down"`
	Total int64 `json:"total"`
	Bytes int64 `json:"bytes"` // Backward-compatible alias of Total.
}

type ClientReportApp struct {
	ID           int    `json:"id"`
	AppName      string `json:"appName"`
	Version      string `json:"version,omitempty"`
	OS           string `json:"os,omitempty"`
	UserAgent    string `json:"userAgent,omitempty"`
	Format       string `json:"format,omitempty"`
	RequestCount int64  `json:"requestCount"`
	FirstSeen    int64  `json:"firstSeen"`
	LastSeen     int64  `json:"lastSeen"`
}

type ClientReportHost struct {
	ID        int    `json:"id"`
	InboundID int    `json:"inboundId"`
	Remark    string `json:"remark"`
	Address   string `json:"address"`
	Port      int    `json:"port"`
	LastSeen  int64  `json:"lastSeen"`
}

type ClientInsightReport struct {
	Email             string                  `json:"email"`
	Days              int                     `json:"days"`
	LastOnline        int64                   `json:"lastOnline"`
	RecentIPCount     int                     `json:"recentIpCount"`
	RecentIPs         []model.ClientIPHistory `json:"recentIps"`
	Apps              []ClientReportApp       `json:"apps"`
	Hosts             []ClientReportHost      `json:"hosts"`
	DailyUsage        []ClientDailyUsage      `json:"dailyUsage"`
	HourlyUsage       []ClientHourlyUsage     `json:"hourlyUsage"`
	TotalUp           int64                   `json:"totalUp"`
	TotalDown         int64                   `json:"totalDown"`
	TotalUsage        int64                   `json:"totalUsage"`
	AverageDaily      int64                   `json:"averageDaily"`
	PeakDay           string                  `json:"peakDay"`
	PeakDayBytes      int64                   `json:"peakDayBytes"`
	PeakHour          int                     `json:"peakHour"`
	PeakHourBytes     int64                   `json:"peakHourBytes"`
	PeakMinuteBytes   int64                   `json:"peakMinuteBytes"`
	LatestMinuteBytes int64                   `json:"latestMinuteBytes"`
	ActiveDays        int                     `json:"activeDays"`
	ActiveMinutes     int                     `json:"activeMinutes"`
	FirstDataAt       int64                   `json:"firstDataAt"`
	LastDataAt        int64                   `json:"lastDataAt"`
	Events            []model.ClientEvent     `json:"events"`
	Anomalies         []model.ClientAnomaly   `json:"anomalies"`
}

type ClientUsageAlert struct {
	Email             string  `json:"email"`
	TotalUp           int64   `json:"totalUp"`
	TotalDown         int64   `json:"totalDown"`
	TotalUsage        int64   `json:"totalUsage"`
	AverageDaily      int64   `json:"averageDaily"`
	PeakMinuteBytes   int64   `json:"peakMinuteBytes"`
	ActiveMinutes     int     `json:"activeMinutes"`
	RecentIPCount     int     `json:"recentIpCount"`
	LastOnline        int64   `json:"lastOnline"`
	AnomalyCount      int     `json:"anomalyCount"`
	LastAnomalyKind   string  `json:"lastAnomalyKind,omitempty"`
	LastAnomalyStatus string  `json:"lastAnomalyStatus,omitempty"`
	Severity          string  `json:"severity"`
	QuotaBytes        int64   `json:"quotaBytes"`
	UsagePercent      float64 `json:"usagePercent"`
}

type ClientUsageAlerts struct {
	Days      int                `json:"days"`
	Generated int64              `json:"generatedAt"`
	Items     []ClientUsageAlert `json:"items"`
}

func (s *ClientInsightService) RecordTraffic(clientTraffics []*xray.ClientTraffic, at time.Time) error {
	if len(clientTraffics) == 0 {
		return nil
	}
	bucket := at.Truncate(time.Minute).UnixMilli()
	db := database.GetDB()
	return db.Transaction(func(tx *gorm.DB) error {
		for _, traffic := range clientTraffics {
			if traffic == nil || strings.TrimSpace(traffic.Email) == "" || traffic.Up+traffic.Down <= 0 {
				continue
			}
			row := model.ClientTrafficBucket{
				Email:       traffic.Email,
				BucketStart: bucket,
				Up:          traffic.Up,
				Down:        traffic.Down,
				Samples:     1,
			}
			if err := tx.Clauses(clause.OnConflict{
				Columns: []clause.Column{{Name: "email"}, {Name: "bucket_start"}},
				DoUpdates: clause.Assignments(map[string]any{
					"up":         gorm.Expr("up + ?", traffic.Up),
					"down":       gorm.Expr("down + ?", traffic.Down),
					"samples":    gorm.Expr("samples + 1"),
					"updated_at": at.UnixMilli(),
				}),
			}).Create(&row).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

func (s *ClientInsightService) RecordIPHistory(observed map[string][]model.ClientIpEntry) error {
	if len(observed) == 0 {
		return nil
	}
	now := time.Now().UnixMilli()
	return database.GetDB().Transaction(func(tx *gorm.DB) error {
		for email, entries := range observed {
			if strings.TrimSpace(email) == "" {
				continue
			}
			for _, entry := range entries {
				ip := strings.TrimSpace(entry.IP)
				if ip == "" {
					continue
				}
				seen := entry.Timestamp
				if seen <= 0 {
					seen = now
				} else if seen < 10_000_000_000 {
					seen *= 1000
				}
				row := model.ClientIPHistory{Email: email, IP: ip, FirstSeen: seen, LastSeen: seen, SeenCount: 1}
				if err := tx.Clauses(clause.OnConflict{
					Columns: []clause.Column{{Name: "email"}, {Name: "ip"}},
					DoUpdates: clause.Assignments(map[string]any{
						"last_seen":  gorm.Expr("CASE WHEN last_seen > ? THEN last_seen ELSE ? END", seen, seen),
						"seen_count": gorm.Expr("seen_count + 1"),
					}),
				}).Create(&row).Error; err != nil {
					return err
				}
			}
		}
		return nil
	})
}

func (s *ClientInsightService) RecordEvent(email, kind, summary string, details any) error {
	email = strings.TrimSpace(email)
	if email == "" {
		return nil
	}
	var detailText string
	if details != nil {
		if b, err := json.Marshal(details); err == nil {
			detailText = string(b)
		}
	}
	return database.GetDB().Create(&model.ClientEvent{
		Email: email, Kind: strings.TrimSpace(kind), Summary: strings.TrimSpace(summary), Details: detailText,
	}).Error
}

// RenameClientHistory keeps the reporting timeline attached to a client when
// its email (the panel's stable public identifier) is changed.
func (s *ClientInsightService) RenameClientHistory(oldEmail, newEmail string) error {
	oldEmail = strings.TrimSpace(oldEmail)
	newEmail = strings.TrimSpace(newEmail)
	if oldEmail == "" || newEmail == "" || oldEmail == newEmail {
		return nil
	}
	db := database.GetDB()
	return db.Transaction(func(tx *gorm.DB) error {
		var buckets []model.ClientTrafficBucket
		if err := tx.Where("email = ?", oldEmail).Find(&buckets).Error; err != nil {
			return err
		}
		for _, bucket := range buckets {
			row := model.ClientTrafficBucket{Email: newEmail, BucketStart: bucket.BucketStart, Up: bucket.Up, Down: bucket.Down, Samples: bucket.Samples}
			if err := tx.Clauses(clause.OnConflict{
				Columns: []clause.Column{{Name: "email"}, {Name: "bucket_start"}},
				DoUpdates: clause.Assignments(map[string]any{
					"up":      gorm.Expr("up + ?", bucket.Up),
					"down":    gorm.Expr("down + ?", bucket.Down),
					"samples": gorm.Expr("samples + ?", bucket.Samples),
				}),
			}).Create(&row).Error; err != nil {
				return err
			}
		}
		if err := tx.Where("email = ?", oldEmail).Delete(&model.ClientTrafficBucket{}).Error; err != nil {
			return err
		}

		var ips []model.ClientIPHistory
		if err := tx.Where("email = ?", oldEmail).Find(&ips).Error; err != nil {
			return err
		}
		for _, ip := range ips {
			row := model.ClientIPHistory{Email: newEmail, IP: ip.IP, FirstSeen: ip.FirstSeen, LastSeen: ip.LastSeen, SeenCount: ip.SeenCount}
			if err := tx.Clauses(clause.OnConflict{
				Columns: []clause.Column{{Name: "email"}, {Name: "ip"}},
				DoUpdates: clause.Assignments(map[string]any{
					"first_seen": gorm.Expr("CASE WHEN first_seen < ? THEN first_seen ELSE ? END", ip.FirstSeen, ip.FirstSeen),
					"last_seen":  gorm.Expr("CASE WHEN last_seen > ? THEN last_seen ELSE ? END", ip.LastSeen, ip.LastSeen),
					"seen_count": gorm.Expr("seen_count + ?", ip.SeenCount),
				}),
			}).Create(&row).Error; err != nil {
				return err
			}
		}
		if err := tx.Where("email = ?", oldEmail).Delete(&model.ClientIPHistory{}).Error; err != nil {
			return err
		}
		if err := tx.Model(&model.ClientEvent{}).Where("email = ?", oldEmail).Update("email", newEmail).Error; err != nil {
			return err
		}
		if err := tx.Model(&model.ClientAnomaly{}).Where("email = ?", oldEmail).Update("email", newEmail).Error; err != nil {
			return err
		}
		return nil
	})
}

func detectClientOS(userAgent string) string {
	ua := strings.ToLower(userAgent)
	switch {
	case strings.Contains(ua, "android"):
		return "Android"
	case strings.Contains(ua, "iphone"), strings.Contains(ua, "ipad"), strings.Contains(ua, "ios"):
		return "iOS/iPadOS"
	case strings.Contains(ua, "windows"):
		return "Windows"
	case strings.Contains(ua, "macintosh"), strings.Contains(ua, "mac os"):
		return "macOS"
	case strings.Contains(ua, "linux"):
		return "Linux"
	default:
		return ""
	}
}

func (s *ClientInsightService) GetReport(email string, days int) (*ClientInsightReport, error) {
	if days < 1 {
		days = 30
	}
	if days > 365 {
		days = 365
	}
	db := database.GetDB()
	var rec model.ClientRecord
	if err := db.Where("email = ?", email).First(&rec).Error; err != nil {
		return nil, err
	}
	now := time.Now()
	localMidnight := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	start := localMidnight.AddDate(0, 0, -(days - 1))
	var buckets []model.ClientTrafficBucket
	if err := db.Where("email = ? AND bucket_start >= ?", email, start.UnixMilli()).Order("bucket_start ASC").Find(&buckets).Error; err != nil {
		return nil, err
	}

	dailyMap := make(map[string]*ClientDailyUsage, days)
	hourly := make([]ClientHourlyUsage, 24)
	for hour := range hourly {
		hourly[hour].Hour = hour
	}
	for i := 0; i < days; i++ {
		day := start.AddDate(0, 0, i).Format("2006-01-02")
		dailyMap[day] = &ClientDailyUsage{Day: day}
	}
	for _, bucket := range buckets {
		t := time.UnixMilli(bucket.BucketStart).Local()
		day := t.Format("2006-01-02")
		if row := dailyMap[day]; row != nil {
			row.Up += bucket.Up
			row.Down += bucket.Down
			row.Total += bucket.Up + bucket.Down
		}
		hourly[t.Hour()].Up += bucket.Up
		hourly[t.Hour()].Down += bucket.Down
		hourly[t.Hour()].Total += bucket.Up + bucket.Down
		hourly[t.Hour()].Bytes = hourly[t.Hour()].Total
	}
	daily := make([]ClientDailyUsage, 0, days)
	var totalUp, totalDown, peakDayBytes, peakMinuteBytes, latestMinuteBytes int64
	peakDay := ""
	activeDays := 0
	for i := 0; i < days; i++ {
		day := start.AddDate(0, 0, i).Format("2006-01-02")
		row := *dailyMap[day]
		daily = append(daily, row)
		totalUp += row.Up
		totalDown += row.Down
		if row.Total > 0 {
			activeDays++
		}
		if row.Total > peakDayBytes {
			peakDay, peakDayBytes = row.Day, row.Total
		}
	}
	peakHour, peakBytes := 0, int64(0)
	for _, row := range hourly {
		if row.Bytes > peakBytes {
			peakHour, peakBytes = row.Hour, row.Bytes
		}
	}
	for i, bucket := range buckets {
		minuteBytes := bucket.Up + bucket.Down
		if minuteBytes > peakMinuteBytes {
			peakMinuteBytes = minuteBytes
		}
		if i == len(buckets)-1 {
			latestMinuteBytes = minuteBytes
		}
	}
	totalUsage := totalUp + totalDown
	averageDaily := int64(0)
	if days > 0 {
		averageDaily = totalUsage / int64(days)
	}

	var traffic xray.ClientTraffic
	_ = db.Where("email = ?", email).First(&traffic).Error
	var recentIPCount int64
	_ = db.Model(&model.ClientIPHistory{}).Where("email = ? AND last_seen >= ?", email, start.UnixMilli()).Count(&recentIPCount).Error
	var ips []model.ClientIPHistory
	_ = db.Where("email = ? AND last_seen >= ?", email, start.UnixMilli()).Order("last_seen DESC").Limit(20).Find(&ips).Error
	var agents []model.ClientSubscriptionAgent
	_ = db.Where("client_id = ?", rec.Id).Order("last_seen DESC, id DESC").Limit(maxRecentSubscriptionApps).Find(&agents).Error
	apps := make([]ClientReportApp, 0, len(agents))
	for _, agent := range agents {
		apps = append(apps, ClientReportApp{ID: agent.Id, AppName: agent.AppName, Version: agent.Version, OS: detectClientOS(agent.UserAgent), UserAgent: agent.UserAgent, Format: agent.Format, RequestCount: agent.RequestCount, FirstSeen: agent.FirstSeen, LastSeen: agent.LastSeen})
	}

	var hosts []ClientReportHost
	_ = db.Table("hosts").
		Select("hosts.id, hosts.inbound_id, hosts.remark, hosts.address, hosts.port").
		Joins("JOIN client_inbounds ON client_inbounds.inbound_id = hosts.inbound_id").
		Where("client_inbounds.client_id = ? AND hosts.is_disabled = ? AND hosts.is_hidden = ?", rec.Id, false, false).
		Order("hosts.sort_order ASC, hosts.id ASC").Scan(&hosts).Error
	if len(buckets) > 0 {
		lastSeen := buckets[len(buckets)-1].BucketStart
		for i := range hosts {
			hosts[i].LastSeen = lastSeen
		}
	}
	var events []model.ClientEvent
	_ = db.Where("email = ?", email).Order("created_at DESC, id DESC").Limit(100).Find(&events).Error
	var anomalies []model.ClientAnomaly
	_ = db.Where("email = ?", email).Order("created_at DESC, id DESC").Limit(50).Find(&anomalies).Error

	firstDataAt, lastDataAt := int64(0), int64(0)
	if len(buckets) > 0 {
		firstDataAt = buckets[0].BucketStart
		lastDataAt = buckets[len(buckets)-1].BucketStart
	}
	return &ClientInsightReport{
		Email: email, Days: days, LastOnline: traffic.LastOnline, RecentIPCount: int(recentIPCount), RecentIPs: ips,
		Apps: apps, Hosts: hosts, DailyUsage: daily, HourlyUsage: hourly,
		TotalUp: totalUp, TotalDown: totalDown, TotalUsage: totalUsage, AverageDaily: averageDaily,
		PeakDay: peakDay, PeakDayBytes: peakDayBytes, PeakHour: peakHour, PeakHourBytes: peakBytes,
		PeakMinuteBytes: peakMinuteBytes, LatestMinuteBytes: latestMinuteBytes,
		ActiveDays: activeDays, ActiveMinutes: len(buckets), FirstDataAt: firstDataAt, LastDataAt: lastDataAt,
		Events: events, Anomalies: anomalies,
	}, nil
}

func normalizeInsightDays(days int) int {
	if days < 1 {
		return 7
	}
	if days > 365 {
		return 365
	}
	return days
}

func normalizeInsightLimit(limit int) int {
	if limit < 1 {
		return 8
	}
	if limit > 50 {
		return 50
	}
	return limit
}

func (s *ClientInsightService) GetUsageAlerts(days, limit int) (*ClientUsageAlerts, error) {
	days = normalizeInsightDays(days)
	limit = normalizeInsightLimit(limit)
	start := time.Now().AddDate(0, 0, -days).UnixMilli()
	db := database.GetDB()
	type aggregate struct {
		Email         string `gorm:"column:email"`
		TotalUp       int64  `gorm:"column:total_up"`
		TotalDown     int64  `gorm:"column:total_down"`
		TotalUsage    int64  `gorm:"column:total_usage"`
		PeakMinute    int64  `gorm:"column:peak_minute"`
		ActiveMinutes int    `gorm:"column:active_minutes"`
	}
	var rows []aggregate
	if err := db.Model(&model.ClientTrafficBucket{}).
		Select("email, COALESCE(SUM(up),0) AS total_up, COALESCE(SUM(down),0) AS total_down, COALESCE(SUM(up + down),0) AS total_usage, COALESCE(MAX(up + down),0) AS peak_minute, COUNT(*) AS active_minutes").
		Where("bucket_start >= ?", start).
		Group("email").
		Order("total_usage DESC").
		Limit(limit * 3).
		Scan(&rows).Error; err != nil {
		return nil, err
	}
	items := make([]ClientUsageAlert, 0, limit)
	for _, row := range rows {
		if len(items) >= limit {
			break
		}
		var rec model.ClientRecord
		if err := db.Where("email = ?", row.Email).First(&rec).Error; err != nil {
			continue
		}
		var traffic xray.ClientTraffic
		_ = db.Where("email = ?", row.Email).First(&traffic).Error
		var recentIPCount int64
		_ = db.Model(&model.ClientIPHistory{}).Where("email = ? AND last_seen >= ?", row.Email, start).Count(&recentIPCount).Error
		var anomalyCount int64
		_ = db.Model(&model.ClientAnomaly{}).Where("email = ? AND created_at >= ?", row.Email, start).Count(&anomalyCount).Error
		var latest model.ClientAnomaly
		_ = db.Where("email = ? AND created_at >= ?", row.Email, start).Order("created_at DESC, id DESC").First(&latest).Error
		severity := "info"
		if anomalyCount > 0 {
			severity = "warning"
		}
		if latest.Status == "acted" || latest.Status == "open" {
			severity = "critical"
		}
		usagePercent := float64(0)
		if rec.TotalGB > 0 {
			usagePercent = float64(row.TotalUsage) * 100 / float64(rec.TotalGB)
		}
		items = append(items, ClientUsageAlert{
			Email: row.Email, TotalUp: row.TotalUp, TotalDown: row.TotalDown, TotalUsage: row.TotalUsage,
			AverageDaily: row.TotalUsage / int64(days), PeakMinuteBytes: row.PeakMinute,
			ActiveMinutes: row.ActiveMinutes, RecentIPCount: int(recentIPCount), LastOnline: traffic.LastOnline,
			AnomalyCount: int(anomalyCount), LastAnomalyKind: latest.Kind, LastAnomalyStatus: latest.Status,
			Severity: severity, QuotaBytes: rec.TotalGB, UsagePercent: usagePercent,
		})
	}
	return &ClientUsageAlerts{Days: days, Generated: time.Now().UnixMilli(), Items: items}, nil
}

func currentIPCount(db *gorm.DB, email string) int {
	var row model.InboundClientIps
	if err := db.Where("client_email = ?", email).First(&row).Error; err != nil {
		return 0
	}
	var entries []map[string]any
	if json.Unmarshal([]byte(row.Ips), &entries) == nil {
		return len(entries)
	}
	var stringsOnly []string
	if json.Unmarshal([]byte(row.Ips), &stringsOnly) == nil {
		return len(stringsOnly)
	}
	return 0
}

func anomalySummary(kind string, observed, threshold int64, ipCount int) string {
	switch kind {
	case "sharing":
		return fmt.Sprintf("Possible subscription sharing detected (%d recent IPs)", ipCount)
	case "sustained":
		return fmt.Sprintf("Sustained high usage detected (%d MB/min)", observed/insightBytesPerMB)
	default:
		return fmt.Sprintf("Sudden traffic spike detected (%d MB/min)", observed/insightBytesPerMB)
	}
}

func (s *ClientInsightService) recentAnomalyExists(db *gorm.DB, email, kind string, since int64) bool {
	var count int64
	_ = db.Model(&model.ClientAnomaly{}).Where("email = ? AND kind = ? AND created_at >= ?", email, kind, since).Count(&count).Error
	return count > 0
}

func (s *ClientInsightService) createAnomaly(email, kind string, observed, threshold int64, ipCount int, settings *entity.AllSetting, inboundSvc *InboundService, xraySvc *XrayService) error {
	db := database.GetDB()
	cooldown := max(settings.AnomalyCooldownMinutes, 1)
	if s.recentAnomalyExists(db, email, kind, time.Now().Add(-time.Duration(cooldown)*time.Minute).UnixMilli()) {
		return nil
	}
	rec, err := (&ClientService{}).GetRecordByEmail(nil, email)
	if err != nil {
		return err
	}
	inboundIDs, _ := (&ClientService{}).GetInboundIdsForRecord(rec.Id)
	previousIDs, _ := json.Marshal(inboundIDs)
	action := settings.AnomalyAction
	if action != "disable" && action != "throttle" {
		action = "alert"
	}
	details := anomalySummary(kind, observed, threshold, ipCount)
	if action != "alert" {
		var activeActions int64
		_ = db.Model(&model.ClientAnomaly{}).Where("email = ? AND status = ? AND action_until > ?", email, "acted", time.Now().UnixMilli()).Count(&activeActions).Error
		if activeActions > 0 {
			action = "alert"
			details += ": another temporary action is already active"
		}
	}
	row := model.ClientAnomaly{
		Email: email, Kind: kind, Severity: "warning", ObservedBytesPerMin: observed,
		ThresholdBytesPerMin: threshold, IPCount: ipCount, Action: action, Status: "open",
		PreviousEnable: rec.Enable, PreviousInboundIDs: string(previousIDs), Details: details,
	}
	if action == "throttle" {
		row.AppliedInboundID = settings.AnomalyThrottleInboundId
	}
	if action != "alert" {
		row.ActionUntil = time.Now().Add(time.Duration(max(settings.AnomalyActionMinutes, 1)) * time.Minute).UnixMilli()
	}
	if err := db.Create(&row).Error; err != nil {
		return err
	}

	clientSvc := &ClientService{}
	switch action {
	case "disable":
		updated := rec.ToClient()
		updated.Enable = false
		needRestart, updateErr := clientSvc.UpdateByEmail(inboundSvc, email, *updated)
		if updateErr != nil {
			db.Model(&row).Updates(map[string]any{"status": "action_failed", "details": row.Details + ": " + updateErr.Error()})
			return updateErr
		}
		if needRestart {
			xraySvc.SetToNeedRestart()
		}
		db.Model(&row).Update("status", "acted")
	case "throttle":
		throttleID := settings.AnomalyThrottleInboundId
		if throttleID <= 0 {
			db.Model(&row).Updates(map[string]any{"action": "alert", "status": "open", "action_until": 0, "details": row.Details + ": throttle inbound is not configured"})
			break
		}
		if len(inboundIDs) > 0 {
			if needRestart, detachErr := clientSvc.DetachByEmailMany(inboundSvc, email, inboundIDs); detachErr != nil {
				db.Model(&row).Updates(map[string]any{"status": "action_failed", "details": row.Details + ": " + detachErr.Error()})
				return detachErr
			} else if needRestart {
				xraySvc.SetToNeedRestart()
			}
		}
		if needRestart, attachErr := clientSvc.AttachByEmail(inboundSvc, email, []int{throttleID}); attachErr != nil {
			if len(inboundIDs) > 0 {
				if restoreRestart, restoreErr := clientSvc.AttachByEmail(inboundSvc, email, inboundIDs); restoreErr != nil {
					logger.Warning("rollback anomaly throttle inbounds failed:", restoreErr)
				} else if restoreRestart {
					xraySvc.SetToNeedRestart()
				}
			}
			db.Model(&row).Updates(map[string]any{"status": "action_failed", "details": row.Details + ": " + attachErr.Error()})
			return attachErr
		} else if needRestart {
			xraySvc.SetToNeedRestart()
		}
		db.Model(&row).Update("status", "acted")
	}
	_ = s.RecordEvent(email, "anomaly", row.Details, map[string]any{"kind": kind, "action": action, "until": row.ActionUntil})
	logger.Warningf("[Anomaly] client=%s kind=%s action=%s observed=%d threshold=%d ips=%d", email, kind, action, observed, threshold, ipCount)
	return nil
}

func (s *ClientInsightService) RestoreExpiredActions(inboundSvc *InboundService, xraySvc *XrayService) error {
	db := database.GetDB()
	var rows []model.ClientAnomaly
	if err := db.Where("status = ? AND action_until > 0 AND action_until <= ?", "acted", time.Now().UnixMilli()).Order("action_until ASC").Find(&rows).Error; err != nil {
		return err
	}
	clientSvc := &ClientService{}
	for i := range rows {
		row := &rows[i]
		rec, err := clientSvc.GetRecordByEmail(nil, row.Email)
		if err != nil {
			db.Model(row).Updates(map[string]any{"status": "resolved", "resolved_at": time.Now().UnixMilli(), "details": row.Details + ": client no longer exists"})
			continue
		}
		var manualChanges int64
		_ = db.Model(&model.ClientEvent{}).Where("email = ? AND created_at > ? AND kind NOT IN ?", row.Email, row.CreatedAt, []string{"anomaly", "anomaly_restored"}).Count(&manualChanges).Error
		if manualChanges > 0 {
			now := time.Now().UnixMilli()
			db.Model(row).Updates(map[string]any{"status": "resolved_manual_change", "resolved_at": now, "details": row.Details + ": automatic restore skipped after a later client change"})
			_ = s.RecordEvent(row.Email, "anomaly_restore_skipped", "Automatic anomaly restore skipped after a later client change", map[string]any{"anomalyId": row.Id, "action": row.Action})
			continue
		}
		switch row.Action {
		case "disable":
			if row.PreviousEnable && !rec.Enable {
				updated := rec.ToClient()
				updated.Enable = true
				if needRestart, err := clientSvc.UpdateByEmail(inboundSvc, row.Email, *updated); err != nil {
					logger.Warning("restore anomaly disabled client failed:", err)
					continue
				} else if needRestart {
					xraySvc.SetToNeedRestart()
				}
			}
		case "throttle":
			var previous []int
			_ = json.Unmarshal([]byte(row.PreviousInboundIDs), &previous)
			current, _ := clientSvc.GetInboundIdsForRecord(rec.Id)
			expected := []int{row.AppliedInboundID}
			if row.AppliedInboundID <= 0 || !equalIntSets(current, expected) {
				now := time.Now().UnixMilli()
				db.Model(row).Updates(map[string]any{"status": "resolved_manual_change", "resolved_at": now, "details": row.Details + ": throttle attachment changed before restore"})
				_ = s.RecordEvent(row.Email, "anomaly_restore_skipped", "Throttle restore skipped because inbound attachments changed", map[string]any{"anomalyId": row.Id})
				continue
			}
			if len(current) > 0 {
				if needRestart, err := clientSvc.DetachByEmailMany(inboundSvc, row.Email, current); err != nil {
					logger.Warning("detach anomaly throttle inbound failed:", err)
					continue
				} else if needRestart {
					xraySvc.SetToNeedRestart()
				}
			}
			if len(previous) > 0 {
				if needRestart, err := clientSvc.AttachByEmail(inboundSvc, row.Email, previous); err != nil {
					logger.Warning("restore anomaly inbounds failed:", err)
					continue
				} else if needRestart {
					xraySvc.SetToNeedRestart()
				}
			}
		}
		now := time.Now().UnixMilli()
		db.Model(row).Updates(map[string]any{"status": "resolved", "resolved_at": now})
		_ = s.RecordEvent(row.Email, "anomaly_restored", "Temporary anomaly action was restored", map[string]any{"anomalyId": row.Id, "action": row.Action})
	}
	return nil
}

func (s *ClientInsightService) EvaluateAnomalies(clientTraffics []*xray.ClientTraffic, settings *entity.AllSetting, inboundSvc *InboundService, xraySvc *XrayService) error {
	if settings == nil || !settings.AnomalyEnable {
		return nil
	}
	db := database.GetDB()
	now := time.Now()
	bucketStart := now.Truncate(time.Minute).UnixMilli()
	windowMinutes := max(settings.AnomalySustainedMinutes, 1)
	for _, traffic := range clientTraffics {
		if traffic == nil || traffic.Up+traffic.Down <= 0 || strings.TrimSpace(traffic.Email) == "" {
			continue
		}
		var current model.ClientTrafficBucket
		_ = db.Where("email = ? AND bucket_start = ?", traffic.Email, bucketStart).First(&current).Error
		currentRate := current.Up + current.Down
		ipCount := currentIPCount(db, traffic.Email)
		sharingThreshold := settings.AnomalySharedIPThreshold
		if sharingThreshold > 0 && ipCount >= sharingThreshold {
			if err := s.createAnomaly(traffic.Email, "sharing", 0, 0, ipCount, settings, inboundSvc, xraySvc); err != nil {
				logger.Warning("create sharing anomaly failed:", err)
			}
			continue
		}
		spikeThreshold := int64(settings.AnomalySpikeMBPerMinute) * insightBytesPerMB
		if spikeThreshold > 0 && currentRate >= spikeThreshold {
			if err := s.createAnomaly(traffic.Email, "spike", currentRate, spikeThreshold, ipCount, settings, inboundSvc, xraySvc); err != nil {
				logger.Warning("create spike anomaly failed:", err)
			}
			continue
		}
		var window struct {
			BucketCount int   `gorm:"column:bucket_count"`
			MinRate     int64 `gorm:"column:min_rate"`
			AvgRate     int64 `gorm:"column:avg_rate"`
		}
		from := now.Add(-time.Duration(windowMinutes-1) * time.Minute).Truncate(time.Minute).UnixMilli()
		if err := db.Model(&model.ClientTrafficBucket{}).
			Select("COUNT(*) AS bucket_count, COALESCE(MIN(up + down),0) AS min_rate, COALESCE(SUM(up + down),0) / CASE WHEN COUNT(*) = 0 THEN 1 ELSE COUNT(*) END AS avg_rate").
			Where("email = ? AND bucket_start >= ?", traffic.Email, from).Scan(&window).Error; err != nil {
			continue
		}
		avgRate := window.AvgRate
		sustainedThreshold := int64(settings.AnomalySustainedMBPerMinute) * insightBytesPerMB
		if sustainedThreshold > 0 && window.BucketCount >= windowMinutes && window.MinRate >= sustainedThreshold {
			if err := s.createAnomaly(traffic.Email, "sustained", avgRate, sustainedThreshold, ipCount, settings, inboundSvc, xraySvc); err != nil {
				logger.Warning("create sustained anomaly failed:", err)
			}
		}
	}
	return nil
}

func (s *ClientInsightService) Cleanup(historyDays int) error {
	if historyDays < 1 {
		historyDays = 90
	}
	cutoff := time.Now().AddDate(0, 0, -historyDays).UnixMilli()
	db := database.GetDB()
	return db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("bucket_start < ?", cutoff).Delete(&model.ClientTrafficBucket{}).Error; err != nil {
			return err
		}
		if err := tx.Where("last_seen < ?", cutoff).Delete(&model.ClientIPHistory{}).Error; err != nil {
			return err
		}

		// Active temporary actions need their later client events to decide
		// whether an administrator changed the client before auto-restore.
		eventCutoff := cutoff
		var oldestActive struct {
			CreatedAt int64 `gorm:"column:created_at"`
		}
		_ = tx.Model(&model.ClientAnomaly{}).
			Select("MIN(created_at) AS created_at").
			Where("status = ?", "acted").Scan(&oldestActive).Error
		if oldestActive.CreatedAt > 0 && oldestActive.CreatedAt < eventCutoff {
			eventCutoff = oldestActive.CreatedAt
		}
		if err := tx.Where("created_at < ?", eventCutoff).Delete(&model.ClientEvent{}).Error; err != nil {
			return err
		}
		// Never remove an acted row before its temporary state has been safely
		// restored (or deliberately skipped after a manual change).
		if err := tx.Where("created_at < ? AND status <> ?", cutoff, "acted").Delete(&model.ClientAnomaly{}).Error; err != nil {
			return err
		}
		return nil
	})
}

func equalIntSets(left, right []int) bool {
	left = sortedUniqueInts(left)
	right = sortedUniqueInts(right)
	if len(left) != len(right) {
		return false
	}
	for i := range left {
		if left[i] != right[i] {
			return false
		}
	}
	return true
}

func sortedUniqueInts(values []int) []int {
	seen := make(map[int]struct{}, len(values))
	out := make([]int, 0, len(values))
	for _, value := range values {
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	sort.Ints(out)
	return out
}
