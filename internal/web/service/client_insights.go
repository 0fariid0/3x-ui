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

const (
	insightBytesPerMB              int64 = 1024 * 1024
	insightRawRetention                  = 25 * time.Hour
	insightHourlyRetention               = 31 * 24 * time.Hour
	insightMaxDailyRetentionDays         = 365
	insightRollupBackfillKey             = "clientInsightRollupBackfilledV366"
	clientReportRecentInboundLimit       = 3
)

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

// ClientTimelineUsage represents one absolute hour on rolling 12h/24h charts.
type ClientTimelineUsage struct {
	BucketStart int64 `json:"bucketStart"`
	Up          int64 `json:"up"`
	Down        int64 `json:"down"`
	Total       int64 `json:"total"`
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

type ClientReportInbound struct {
	ID       int    `json:"id"`
	Tag      string `json:"tag"`
	Remark   string `json:"remark"`
	Protocol string `json:"protocol"`
	Port     int    `json:"port"`
	NodeID   *int   `json:"nodeId,omitempty"`
	Online   bool   `json:"online"`
	LastSeen int64  `json:"lastSeen"`
}

type ClientInsightReport struct {
	Email                string                     `json:"email"`
	Days                 int                        `json:"days"`
	Hours                int                        `json:"hours"`
	RangeStart           int64                      `json:"rangeStart"`
	RangeEnd             int64                      `json:"rangeEnd"`
	LastOnline           int64                      `json:"lastOnline"`
	RecentIPCount        int                        `json:"recentIpCount"`
	RecentIPs            []model.ClientIPHistory    `json:"recentIps"`
	Apps                 []ClientReportApp          `json:"apps"`
	Hosts                []ClientReportHost         `json:"hosts"` // backward-compatible; UI uses ConnectedInbounds
	ConnectedInbounds    []ClientReportInbound      `json:"connectedInbounds"`
	DailyUsage           []ClientDailyUsage         `json:"dailyUsage"`
	HourlyUsage          []ClientHourlyUsage        `json:"hourlyUsage"`
	TimelineUsage        []ClientTimelineUsage      `json:"timelineUsage"`
	TotalUp              int64                      `json:"totalUp"`
	TotalDown            int64                      `json:"totalDown"`
	TotalUsage           int64                      `json:"totalUsage"`
	AverageDaily         int64                      `json:"averageDaily"`
	PeakDay              string                     `json:"peakDay"`
	PeakDayBytes         int64                      `json:"peakDayBytes"`
	PeakHour             int                        `json:"peakHour"`
	PeakHourBytes        int64                      `json:"peakHourBytes"`
	PeakMinuteBytes      int64                      `json:"peakMinuteBytes"`
	LatestMinuteBytes    int64                      `json:"latestMinuteBytes"`
	ActiveDays           int                        `json:"activeDays"`
	ActiveMinutes        int                        `json:"activeMinutes"`
	FirstDataAt          int64                      `json:"firstDataAt"`
	LastDataAt           int64                      `json:"lastDataAt"`
	Events               []model.ClientEvent        `json:"events"`
	Anomalies            []model.ClientAnomaly      `json:"anomalies"`
	DestinationTracking  bool                       `json:"destinationTracking"`
	DestinationSummaries []ClientDestinationSummary `json:"destinationSummaries"`
	Destinations         []ClientDestinationItem    `json:"destinations"`
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

func insightPanelLocation() *time.Location {
	timezone, err := (&SettingService{}).GetScheduledRestartTimezone()
	if err != nil {
		return time.Local
	}
	loc, _, err := entity.ScheduledRestartLocation(timezone)
	if err != nil || loc == nil {
		return time.Local
	}
	return loc
}

func insightHourStart(at time.Time, loc *time.Location) time.Time {
	local := at.In(loc)
	return time.Date(local.Year(), local.Month(), local.Day(), local.Hour(), 0, 0, 0, loc)
}

func insightDayStart(at time.Time, loc *time.Location) time.Time {
	local := at.In(loc)
	return time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, loc)
}

func compactRetentionDays(historyDays int) int {
	if historyDays < 1 {
		historyDays = 30
	}
	if historyDays > insightMaxDailyRetentionDays {
		historyDays = insightMaxDailyRetentionDays
	}
	return historyDays
}

func rollupAssignments(minuteStart, deltaUp, deltaDown, updatedAt int64) map[string]any {
	deltaTotal := deltaUp + deltaDown
	currentMinute := "CASE WHEN last_minute_start = ? THEN last_minute_bytes + ? ELSE ? END"
	return map[string]any{
		"up":                gorm.Expr("up + ?", deltaUp),
		"down":              gorm.Expr("down + ?", deltaDown),
		"active_minutes":    gorm.Expr("active_minutes + CASE WHEN last_minute_start = ? THEN 0 ELSE 1 END", minuteStart),
		"peak_minute_bytes": gorm.Expr("CASE WHEN peak_minute_bytes >= ("+currentMinute+") THEN peak_minute_bytes ELSE ("+currentMinute+") END", minuteStart, deltaTotal, deltaTotal, minuteStart, deltaTotal, deltaTotal),
		"last_minute_start": minuteStart,
		"last_minute_bytes": gorm.Expr(currentMinute, minuteStart, deltaTotal, deltaTotal),
		"updated_at":        updatedAt,
	}
}

func (s *ClientInsightService) RecordTraffic(clientTraffics []*xray.ClientTraffic, at time.Time) error {
	if len(clientTraffics) == 0 {
		return nil
	}
	loc := insightPanelLocation()
	minuteStart := at.Truncate(time.Minute).UnixMilli()
	hourStart := insightHourStart(at, loc).UnixMilli()
	dayStartTime := insightDayStart(at, loc)
	dayStart := dayStartTime.UnixMilli()
	dayKey := dayStartTime.Format("2006-01-02")
	updatedAt := at.UnixMilli()
	db := database.GetDB()
	return db.Transaction(func(tx *gorm.DB) error {
		for _, traffic := range clientTraffics {
			if traffic == nil || strings.TrimSpace(traffic.Email) == "" || traffic.Up+traffic.Down <= 0 {
				continue
			}
			email := strings.TrimSpace(traffic.Email)
			minute := model.ClientTrafficBucket{
				Email: email, BucketStart: minuteStart, Up: traffic.Up, Down: traffic.Down, Samples: 1,
			}
			if err := tx.Clauses(clause.OnConflict{
				Columns: []clause.Column{{Name: "email"}, {Name: "bucket_start"}},
				DoUpdates: clause.Assignments(map[string]any{
					"up":         gorm.Expr("up + ?", traffic.Up),
					"down":       gorm.Expr("down + ?", traffic.Down),
					"samples":    gorm.Expr("samples + 1"),
					"updated_at": updatedAt,
				}),
			}).Create(&minute).Error; err != nil {
				return err
			}

			deltaTotal := traffic.Up + traffic.Down
			hour := model.ClientTrafficHourBucket{
				Email: email, BucketStart: hourStart, Up: traffic.Up, Down: traffic.Down,
				ActiveMinutes: 1, PeakMinuteBytes: deltaTotal, LastMinuteStart: minuteStart, LastMinuteBytes: deltaTotal,
			}
			if err := tx.Clauses(clause.OnConflict{
				Columns:   []clause.Column{{Name: "email"}, {Name: "bucket_start"}},
				DoUpdates: clause.Assignments(rollupAssignments(minuteStart, traffic.Up, traffic.Down, updatedAt)),
			}).Create(&hour).Error; err != nil {
				return err
			}

			day := model.ClientTrafficDayBucket{
				Email: email, BucketStart: dayStart, Day: dayKey, Up: traffic.Up, Down: traffic.Down,
				ActiveMinutes: 1, PeakMinuteBytes: deltaTotal, LastMinuteStart: minuteStart, LastMinuteBytes: deltaTotal,
			}
			if err := tx.Clauses(clause.OnConflict{
				Columns:   []clause.Column{{Name: "email"}, {Name: "bucket_start"}},
				DoUpdates: clause.Assignments(rollupAssignments(minuteStart, traffic.Up, traffic.Down, updatedAt)),
			}).Create(&day).Error; err != nil {
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

// RecordInboundActivity persists exact client -> inbound observations from the
// access log. The input is email -> inbound tag -> last observed timestamp in
// milliseconds. Share hosts and IPs are intentionally ignored here: the report
// needs the inbound that was actually used, not every generated link for it.
func (s *ClientInsightService) RecordInboundActivity(observed map[string]map[string]int64) error {
	if len(observed) == 0 {
		return nil
	}

	tagSet := make(map[string]struct{})
	now := time.Now().UnixMilli()
	for email, byTag := range observed {
		if strings.TrimSpace(email) == "" {
			continue
		}
		for tag := range byTag {
			if tag = strings.TrimSpace(tag); tag != "" {
				tagSet[tag] = struct{}{}
			}
		}
	}
	if len(tagSet) == 0 {
		return nil
	}

	tags := make([]string, 0, len(tagSet))
	for tag := range tagSet {
		tags = append(tags, tag)
	}

	db := database.GetDB()
	var inbounds []*model.Inbound
	if err := db.Model(&model.Inbound{}).
		Select("id, tag, origin_node_guid").
		Where("tag IN ?", tags).
		Find(&inbounds).Error; err != nil {
		return err
	}
	if len(inbounds) == 0 {
		return nil
	}
	(&InboundService{}).annotateLocalOriginGuid(inbounds)

	byTag := make(map[string][]*model.Inbound, len(inbounds))
	for _, inbound := range inbounds {
		if inbound == nil || strings.TrimSpace(inbound.Tag) == "" {
			continue
		}
		byTag[inbound.Tag] = append(byTag[inbound.Tag], inbound)
	}

	return db.Transaction(func(tx *gorm.DB) error {
		for email, observedTags := range observed {
			email = strings.TrimSpace(email)
			if email == "" {
				continue
			}
			for tag, seen := range observedTags {
				tag = strings.TrimSpace(tag)
				if tag == "" {
					continue
				}
				if seen <= 0 {
					seen = now
				} else if seen < 10_000_000_000 {
					seen *= 1000
				}
				for _, inbound := range byTag[tag] {
					if inbound == nil || inbound.Id <= 0 {
						continue
					}
					row := model.ClientInboundHistory{
						Email: email, InboundID: inbound.Id, InboundTag: inbound.Tag,
						OriginNodeGuid: inbound.OriginNodeGuid, FirstSeen: seen, LastSeen: seen, SeenCount: 1,
					}
					if err := tx.Clauses(clause.OnConflict{
						Columns: []clause.Column{{Name: "email"}, {Name: "inbound_id"}},
						DoUpdates: clause.Assignments(map[string]any{
							"inbound_tag":      inbound.Tag,
							"origin_node_guid": inbound.OriginNodeGuid,
							"first_seen":       gorm.Expr("CASE WHEN first_seen < ? THEN first_seen ELSE ? END", seen, seen),
							"last_seen":        gorm.Expr("CASE WHEN last_seen > ? THEN last_seen ELSE ? END", seen, seen),
							"seen_count":       gorm.Expr("seen_count + 1"),
						}),
					}).Create(&row).Error; err != nil {
						return err
					}
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

		var hourBuckets []model.ClientTrafficHourBucket
		if err := tx.Where("email = ?", oldEmail).Find(&hourBuckets).Error; err != nil {
			return err
		}
		for _, bucket := range hourBuckets {
			row := bucket
			row.Id = 0
			row.Email = newEmail
			if err := tx.Clauses(clause.OnConflict{
				Columns: []clause.Column{{Name: "email"}, {Name: "bucket_start"}},
				DoUpdates: clause.Assignments(map[string]any{
					"up":                gorm.Expr("up + ?", bucket.Up),
					"down":              gorm.Expr("down + ?", bucket.Down),
					"active_minutes":    gorm.Expr("active_minutes + ?", bucket.ActiveMinutes),
					"peak_minute_bytes": gorm.Expr("CASE WHEN peak_minute_bytes >= ? THEN peak_minute_bytes ELSE ? END", bucket.PeakMinuteBytes, bucket.PeakMinuteBytes),
				}),
			}).Create(&row).Error; err != nil {
				return err
			}
		}
		if err := tx.Where("email = ?", oldEmail).Delete(&model.ClientTrafficHourBucket{}).Error; err != nil {
			return err
		}

		var dayBuckets []model.ClientTrafficDayBucket
		if err := tx.Where("email = ?", oldEmail).Find(&dayBuckets).Error; err != nil {
			return err
		}
		for _, bucket := range dayBuckets {
			row := bucket
			row.Id = 0
			row.Email = newEmail
			if err := tx.Clauses(clause.OnConflict{
				Columns: []clause.Column{{Name: "email"}, {Name: "bucket_start"}},
				DoUpdates: clause.Assignments(map[string]any{
					"up":                gorm.Expr("up + ?", bucket.Up),
					"down":              gorm.Expr("down + ?", bucket.Down),
					"active_minutes":    gorm.Expr("active_minutes + ?", bucket.ActiveMinutes),
					"peak_minute_bytes": gorm.Expr("CASE WHEN peak_minute_bytes >= ? THEN peak_minute_bytes ELSE ? END", bucket.PeakMinuteBytes, bucket.PeakMinuteBytes),
				}),
			}).Create(&row).Error; err != nil {
				return err
			}
		}
		if err := tx.Where("email = ?", oldEmail).Delete(&model.ClientTrafficDayBucket{}).Error; err != nil {
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

		var inboundHistory []model.ClientInboundHistory
		if err := tx.Where("email = ?", oldEmail).Find(&inboundHistory).Error; err != nil {
			return err
		}
		for _, item := range inboundHistory {
			row := item
			row.Id = 0
			row.Email = newEmail
			if err := tx.Clauses(clause.OnConflict{
				Columns: []clause.Column{{Name: "email"}, {Name: "inbound_id"}},
				DoUpdates: clause.Assignments(map[string]any{
					"inbound_tag":      item.InboundTag,
					"origin_node_guid": item.OriginNodeGuid,
					"first_seen":       gorm.Expr("CASE WHEN first_seen < ? THEN first_seen ELSE ? END", item.FirstSeen, item.FirstSeen),
					"last_seen":        gorm.Expr("CASE WHEN last_seen > ? THEN last_seen ELSE ? END", item.LastSeen, item.LastSeen),
					"seen_count":       gorm.Expr("seen_count + ?", item.SeenCount),
				}),
			}).Create(&row).Error; err != nil {
				return err
			}
		}
		if err := tx.Where("email = ?", oldEmail).Delete(&model.ClientInboundHistory{}).Error; err != nil {
			return err
		}

		var destinations []model.ClientDestinationHour
		if err := tx.Where("email = ?", oldEmail).Find(&destinations).Error; err != nil {
			return err
		}
		for _, destination := range destinations {
			row := destination
			row.Id = 0
			row.Email = newEmail
			if err := tx.Clauses(clause.OnConflict{
				Columns: []clause.Column{{Name: "email"}, {Name: "bucket_start"}, {Name: "key"}},
				DoUpdates: clause.Assignments(map[string]any{
					"connections": gorm.Expr("connections + ?", destination.Connections),
					"first_seen":  gorm.Expr("CASE WHEN first_seen < ? THEN first_seen ELSE ? END", destination.FirstSeen, destination.FirstSeen),
					"last_seen":   gorm.Expr("CASE WHEN last_seen > ? THEN last_seen ELSE ? END", destination.LastSeen, destination.LastSeen),
				}),
			}).Create(&row).Error; err != nil {
				return err
			}
		}
		if err := tx.Where("email = ?", oldEmail).Delete(&model.ClientDestinationHour{}).Error; err != nil {
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
	return s.GetReportForRange(email, days, 0)
}

func (s *ClientInsightService) GetReportForRange(email string, days, hours int) (*ClientInsightReport, error) {
	if hours != 12 && hours != 24 {
		hours = 0
	}
	if days < 1 {
		days = 30
	}
	if days > insightMaxDailyRetentionDays {
		days = insightMaxDailyRetentionDays
	}

	loc := insightPanelLocation()
	db := database.GetDB()
	if err := ensureInsightRollups(db, loc); err != nil {
		return nil, err
	}
	var rec model.ClientRecord
	if err := db.Where("email = ?", email).First(&rec).Error; err != nil {
		return nil, err
	}

	now := time.Now().In(loc)
	currentHour := insightHourStart(now, loc)
	localMidnight := insightDayStart(now, loc)
	start := localMidnight.AddDate(0, 0, -(days - 1))
	if hours > 0 {
		start = currentHour.Add(-time.Duration(hours-1) * time.Hour)
		days = 1
	}
	rangeStart := start.UnixMilli()
	rangeEnd := now.UnixMilli()

	var dayBuckets []model.ClientTrafficDayBucket
	dayQueryStart := insightDayStart(start, loc).UnixMilli()
	if err := db.Where("email = ? AND bucket_start >= ? AND bucket_start <= ?", email, dayQueryStart, rangeEnd).
		Order("bucket_start ASC").Find(&dayBuckets).Error; err != nil {
		return nil, err
	}

	dailyMap := make(map[string]*ClientDailyUsage, days)
	for i := 0; i < days; i++ {
		day := localMidnight.AddDate(0, 0, -(days-1)+i).Format("2006-01-02")
		dailyMap[day] = &ClientDailyUsage{Day: day}
	}
	if hours > 0 {
		day := now.Format("2006-01-02")
		dailyMap = map[string]*ClientDailyUsage{day: {Day: day}}
	}
	for _, bucket := range dayBuckets {
		if row := dailyMap[bucket.Day]; row != nil {
			row.Up += bucket.Up
			row.Down += bucket.Down
			row.Total += bucket.Up + bucket.Down
		}
	}

	// Hour-of-day pattern is intentionally retained for 31 days only. This
	// gives useful behavioural detail without keeping minute-level logs for the
	// full 365-day reporting window.
	hourly := make([]ClientHourlyUsage, 24)
	for hour := range hourly {
		hourly[hour].Hour = hour
	}
	hourPatternStart := rangeStart
	retainedHourStart := now.Add(-insightHourlyRetention).UnixMilli()
	if hourPatternStart < retainedHourStart {
		hourPatternStart = retainedHourStart
	}
	var hourBuckets []model.ClientTrafficHourBucket
	if err := db.Where("email = ? AND bucket_start >= ? AND bucket_start <= ?", email, hourPatternStart, rangeEnd).
		Order("bucket_start ASC").Find(&hourBuckets).Error; err != nil {
		return nil, err
	}
	for _, bucket := range hourBuckets {
		h := time.UnixMilli(bucket.BucketStart).In(loc).Hour()
		hourly[h].Up += bucket.Up
		hourly[h].Down += bucket.Down
		hourly[h].Total += bucket.Up + bucket.Down
		hourly[h].Bytes = hourly[h].Total
	}

	timeline := make([]ClientTimelineUsage, 0)
	if hours > 0 {
		byStart := make(map[int64]model.ClientTrafficHourBucket, len(hourBuckets))
		for _, bucket := range hourBuckets {
			if bucket.BucketStart >= rangeStart {
				byStart[bucket.BucketStart] = bucket
			}
		}
		timeline = make([]ClientTimelineUsage, 0, hours)
		for i := 0; i < hours; i++ {
			bucketStart := start.Add(time.Duration(i) * time.Hour).UnixMilli()
			bucket := byStart[bucketStart]
			timeline = append(timeline, ClientTimelineUsage{
				BucketStart: bucketStart,
				Up:          bucket.Up,
				Down:        bucket.Down,
				Total:       bucket.Up + bucket.Down,
			})
		}
	}

	daily := make([]ClientDailyUsage, 0, len(dailyMap))
	var totalUp, totalDown, peakDayBytes, peakMinuteBytes, latestMinuteBytes int64
	peakDay := ""
	activeDays := 0
	activeMinutes := 0
	firstDataAt, lastDataAt := int64(0), int64(0)
	if hours > 0 {
		rollingDays := make(map[string]*ClientDailyUsage, 2)
		rollingDayOrder := make([]string, 0, 2)
		for _, row := range timeline {
			totalUp += row.Up
			totalDown += row.Down
			day := time.UnixMilli(row.BucketStart).In(loc).Format("2006-01-02")
			dayRow := rollingDays[day]
			if dayRow == nil {
				dayRow = &ClientDailyUsage{Day: day}
				rollingDays[day] = dayRow
				rollingDayOrder = append(rollingDayOrder, day)
			}
			dayRow.Up += row.Up
			dayRow.Down += row.Down
			dayRow.Total += row.Total
			if row.Total > 0 {
				if firstDataAt == 0 {
					firstDataAt = row.BucketStart
				}
				lastDataAt = row.BucketStart
			}
		}
		for _, bucket := range hourBuckets {
			if bucket.BucketStart < rangeStart {
				continue
			}
			activeMinutes += bucket.ActiveMinutes
			if bucket.PeakMinuteBytes > peakMinuteBytes {
				peakMinuteBytes = bucket.PeakMinuteBytes
			}
		}
		for _, day := range rollingDayOrder {
			row := *rollingDays[day]
			daily = append(daily, row)
			if row.Total > 0 {
				activeDays++
			}
			if row.Total > peakDayBytes {
				peakDay, peakDayBytes = row.Day, row.Total
			}
		}
	} else {
		for i := 0; i < days; i++ {
			day := localMidnight.AddDate(0, 0, -(days-1)+i).Format("2006-01-02")
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
		for _, bucket := range dayBuckets {
			if bucket.BucketStart < rangeStart {
				continue
			}
			activeMinutes += bucket.ActiveMinutes
			if bucket.PeakMinuteBytes > peakMinuteBytes {
				peakMinuteBytes = bucket.PeakMinuteBytes
			}
			if bucket.Up+bucket.Down > 0 {
				if firstDataAt == 0 {
					firstDataAt = bucket.BucketStart
				}
				lastDataAt = bucket.BucketStart
			}
		}
	}

	peakHour, peakHourBytes := 0, int64(0)
	for _, row := range hourly {
		if row.Total > peakHourBytes {
			peakHour, peakHourBytes = row.Hour, row.Total
		}
	}

	var latestBucket model.ClientTrafficBucket
	if err := db.Where("email = ? AND bucket_start >= ? AND bucket_start <= ?", email, rangeStart, rangeEnd).
		Order("bucket_start DESC").First(&latestBucket).Error; err == nil {
		latestMinuteBytes = latestBucket.Up + latestBucket.Down
		if lastDataAt == 0 || latestBucket.BucketStart > lastDataAt {
			lastDataAt = latestBucket.BucketStart
		}
		if firstDataAt == 0 {
			firstDataAt = latestBucket.BucketStart
		}
	}

	totalUsage := totalUp + totalDown
	averageDaily := totalUsage
	if hours > 0 {
		averageDaily = totalUsage * 24 / int64(hours)
	} else if days > 0 {
		averageDaily = totalUsage / int64(days)
	}

	var traffic xray.ClientTraffic
	_ = db.Where("email = ?", email).First(&traffic).Error
	var recentIPCount int64
	_ = db.Model(&model.ClientIPHistory{}).Where("email = ? AND last_seen >= ?", email, rangeStart).Count(&recentIPCount).Error
	var ips []model.ClientIPHistory
	_ = db.Where("email = ? AND last_seen >= ?", email, rangeStart).Order("last_seen DESC").Limit(20).Find(&ips).Error
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
	// Keep legacy host rows for API compatibility, but do not stamp every host
	// with the client's last traffic time. One inbound may have several host/IP
	// choices, and that timestamp falsely implied that every choice was used.

	// Report inbound usage at inbound granularity only. One inbound may expose
	// several share hosts/IPs, but those are links, not distinct connections.
	// Show every currently-online inbound plus the three most recent offline
	// inbounds with their last observed connection time.
	var attachedInbounds []*model.Inbound
	_ = db.Model(&model.Inbound{}).
		Select("inbounds.id, inbounds.tag, inbounds.remark, inbounds.protocol, inbounds.port, inbounds.node_id, inbounds.origin_node_guid").
		Joins("JOIN client_inbounds ON client_inbounds.inbound_id = inbounds.id").
		Where("client_inbounds.client_id = ?", rec.Id).
		Order("inbounds.id ASC").Find(&attachedInbounds).Error
	inboundSvc := &InboundService{}
	inboundSvc.annotateLocalOriginGuid(attachedInbounds)
	onlineInbounds := inboundSvc.GetOnlineInboundsByGuid()
	onlineByInboundID := make(map[int]bool)
	for _, inbound := range attachedInbounds {
		if inbound == nil || inbound.Tag == "" || inbound.OriginNodeGuid == "" {
			continue
		}
		byEmail, supported := onlineInbounds[inbound.OriginNodeGuid]
		if !supported {
			continue
		}
		for _, tag := range byEmail[email] {
			if tag == inbound.Tag {
				onlineByInboundID[inbound.Id] = true
				break
			}
		}
	}

	inboundByID := make(map[int]*model.Inbound, len(attachedInbounds))
	attachedIDs := make([]int, 0, len(attachedInbounds))
	for _, inbound := range attachedInbounds {
		if inbound == nil || inbound.Id <= 0 {
			continue
		}
		inboundByID[inbound.Id] = inbound
		attachedIDs = append(attachedIDs, inbound.Id)
	}

	var inboundHistory []model.ClientInboundHistory
	if len(attachedIDs) > 0 {
		_ = db.Where("email = ? AND inbound_id IN ?", email, attachedIDs).
			Order("last_seen DESC, id DESC").Find(&inboundHistory).Error
	}
	lastSeenByInboundID := make(map[int]int64, len(inboundHistory))
	for _, item := range inboundHistory {
		if item.InboundID > 0 && item.LastSeen > lastSeenByInboundID[item.InboundID] {
			lastSeenByInboundID[item.InboundID] = item.LastSeen
		}
	}

	selected := make(map[int]ClientReportInbound)
	appendInbound := func(inbound *model.Inbound, online bool, lastSeen int64) {
		if inbound == nil || inbound.Id <= 0 {
			return
		}
		if lastSeen <= 0 && online {
			lastSeen = rangeEnd
		}
		selected[inbound.Id] = ClientReportInbound{
			ID: inbound.Id, Tag: inbound.Tag, Remark: inbound.Remark,
			Protocol: string(inbound.Protocol), Port: inbound.Port, NodeID: inbound.NodeID,
			Online: online, LastSeen: lastSeen,
		}
	}

	for _, inbound := range attachedInbounds {
		if onlineByInboundID[inbound.Id] {
			appendInbound(inbound, true, lastSeenByInboundID[inbound.Id])
		}
	}
	recentCount := 0
	for _, item := range inboundHistory {
		if recentCount >= clientReportRecentInboundLimit {
			break
		}
		if _, exists := selected[item.InboundID]; exists {
			continue
		}
		inbound := inboundByID[item.InboundID]
		if inbound == nil {
			continue
		}
		appendInbound(inbound, false, item.LastSeen)
		recentCount++
	}

	connectedInbounds := make([]ClientReportInbound, 0, len(selected))
	for _, item := range selected {
		connectedInbounds = append(connectedInbounds, item)
	}
	sort.SliceStable(connectedInbounds, func(i, j int) bool {
		if connectedInbounds[i].Online != connectedInbounds[j].Online {
			return connectedInbounds[i].Online
		}
		if connectedInbounds[i].LastSeen != connectedInbounds[j].LastSeen {
			return connectedInbounds[i].LastSeen > connectedInbounds[j].LastSeen
		}
		return connectedInbounds[i].ID < connectedInbounds[j].ID
	})
	var events []model.ClientEvent
	_ = db.Where("email = ? AND created_at >= ?", email, rangeStart).Order("created_at DESC, id DESC").Limit(100).Find(&events).Error
	var anomalies []model.ClientAnomaly
	_ = db.Where("email = ? AND created_at >= ?", email, rangeStart).Order("created_at DESC, id DESC").Limit(50).Find(&anomalies).Error
	destinations, destinationSummaries, destinationErr := s.destinationReport(email, rangeStart, rangeEnd)
	if destinationErr != nil {
		return nil, destinationErr
	}

	return &ClientInsightReport{
		Email: email, Days: days, Hours: hours, RangeStart: rangeStart, RangeEnd: rangeEnd,
		LastOnline: traffic.LastOnline, RecentIPCount: int(recentIPCount), RecentIPs: ips,
		Apps: apps, Hosts: hosts, ConnectedInbounds: connectedInbounds,
		DailyUsage: daily, HourlyUsage: hourly, TimelineUsage: timeline,
		TotalUp: totalUp, TotalDown: totalDown, TotalUsage: totalUsage, AverageDaily: averageDaily,
		PeakDay: peakDay, PeakDayBytes: peakDayBytes, PeakHour: peakHour, PeakHourBytes: peakHourBytes,
		PeakMinuteBytes: peakMinuteBytes, LatestMinuteBytes: latestMinuteBytes,
		ActiveDays: activeDays, ActiveMinutes: activeMinutes, FirstDataAt: firstDataAt, LastDataAt: lastDataAt,
		Events: events, Anomalies: anomalies, DestinationTracking: rec.DestinationTracking,
		DestinationSummaries: destinationSummaries, Destinations: destinations,
	}, nil
}

func normalizeInsightDays(days int) int {
	if days < 1 {
		return 1
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
	loc := insightPanelLocation()
	now := time.Now().In(loc)
	start := insightDayStart(now, loc).AddDate(0, 0, -(days - 1)).UnixMilli()
	db := database.GetDB()
	if err := ensureInsightRollups(db, loc); err != nil {
		return nil, err
	}
	type aggregate struct {
		Email         string `gorm:"column:email"`
		TotalUp       int64  `gorm:"column:total_up"`
		TotalDown     int64  `gorm:"column:total_down"`
		TotalUsage    int64  `gorm:"column:total_usage"`
		PeakMinute    int64  `gorm:"column:peak_minute"`
		ActiveMinutes int    `gorm:"column:active_minutes"`
	}
	var rows []aggregate
	aggregateQuery := db.Model(&model.ClientTrafficDayBucket{})
	if days == 1 {
		start = insightHourStart(now, loc).Add(-23 * time.Hour).UnixMilli()
		aggregateQuery = db.Model(&model.ClientTrafficHourBucket{})
	}
	if err := aggregateQuery.
		Select("email, COALESCE(SUM(up),0) AS total_up, COALESCE(SUM(down),0) AS total_down, COALESCE(SUM(up + down),0) AS total_usage, COALESCE(MAX(peak_minute_bytes),0) AS peak_minute, COALESCE(SUM(active_minutes),0) AS active_minutes").
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

func rollupMarkerExists(db *gorm.DB) bool {
	var count int64
	_ = db.Model(&model.Setting{}).Where("key = ? AND value = ?", insightRollupBackfillKey, "true").Count(&count).Error
	return count > 0
}

func saveRollupMarker(db *gorm.DB) error {
	return db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("key = ?", insightRollupBackfillKey).Delete(&model.Setting{}).Error; err != nil {
			return err
		}
		return tx.Create(&model.Setting{Key: insightRollupBackfillKey, Value: "true"}).Error
	})
}

// ensureInsightRollups performs a one-time upgrade of v3.6.4/v3.6.5 minute
// history into the compact hourly/daily tables. If an earlier attempt was
// interrupted, the absent marker causes a clean rebuild on the next run.
func ensureInsightRollups(db *gorm.DB, loc *time.Location) error {
	if rollupMarkerExists(db) {
		return nil
	}
	if err := db.Session(&gorm.Session{AllowGlobalUpdate: true}).Delete(&model.ClientTrafficHourBucket{}).Error; err != nil {
		return err
	}
	if err := db.Session(&gorm.Session{AllowGlobalUpdate: true}).Delete(&model.ClientTrafficDayBucket{}).Error; err != nil {
		return err
	}

	rows, err := db.Model(&model.ClientTrafficBucket{}).Order("email ASC, bucket_start ASC").Rows()
	if err != nil {
		return err
	}
	defer rows.Close()

	currentEmail := ""
	hours := make(map[int64]*model.ClientTrafficHourBucket)
	days := make(map[int64]*model.ClientTrafficDayBucket)
	flush := func() error {
		if currentEmail == "" {
			return nil
		}
		hourRows := make([]model.ClientTrafficHourBucket, 0, len(hours))
		for _, row := range hours {
			hourRows = append(hourRows, *row)
		}
		dayRows := make([]model.ClientTrafficDayBucket, 0, len(days))
		for _, row := range days {
			dayRows = append(dayRows, *row)
		}
		if len(hourRows) > 0 {
			if err := db.CreateInBatches(&hourRows, 500).Error; err != nil {
				return err
			}
		}
		if len(dayRows) > 0 {
			if err := db.CreateInBatches(&dayRows, 500).Error; err != nil {
				return err
			}
		}
		hours = make(map[int64]*model.ClientTrafficHourBucket)
		days = make(map[int64]*model.ClientTrafficDayBucket)
		return nil
	}

	for rows.Next() {
		var bucket model.ClientTrafficBucket
		if err := db.ScanRows(rows, &bucket); err != nil {
			return err
		}
		if currentEmail != "" && bucket.Email != currentEmail {
			if err := flush(); err != nil {
				return err
			}
		}
		currentEmail = bucket.Email
		at := time.UnixMilli(bucket.BucketStart).In(loc)
		hourStart := insightHourStart(at, loc).UnixMilli()
		dayStartTime := insightDayStart(at, loc)
		dayStart := dayStartTime.UnixMilli()
		total := bucket.Up + bucket.Down

		hour := hours[hourStart]
		if hour == nil {
			hour = &model.ClientTrafficHourBucket{Email: bucket.Email, BucketStart: hourStart}
			hours[hourStart] = hour
		}
		hour.Up += bucket.Up
		hour.Down += bucket.Down
		hour.ActiveMinutes++
		if total > hour.PeakMinuteBytes {
			hour.PeakMinuteBytes = total
		}
		hour.LastMinuteStart = bucket.BucketStart
		hour.LastMinuteBytes = total

		day := days[dayStart]
		if day == nil {
			day = &model.ClientTrafficDayBucket{Email: bucket.Email, BucketStart: dayStart, Day: dayStartTime.Format("2006-01-02")}
			days[dayStart] = day
		}
		day.Up += bucket.Up
		day.Down += bucket.Down
		day.ActiveMinutes++
		if total > day.PeakMinuteBytes {
			day.PeakMinuteBytes = total
		}
		day.LastMinuteStart = bucket.BucketStart
		day.LastMinuteBytes = total
	}
	if err := rows.Err(); err != nil {
		return err
	}
	if err := flush(); err != nil {
		return err
	}
	return saveRollupMarker(db)
}

func (s *ClientInsightService) Cleanup(historyDays int) error {
	historyDays = compactRetentionDays(historyDays)
	loc := insightPanelLocation()
	db := database.GetDB()
	if err := ensureInsightRollups(db, loc); err != nil {
		return err
	}

	now := time.Now()
	rawCutoff := now.Add(-insightRawRetention).UnixMilli()
	hourCutoff := now.Add(-insightHourlyRetention).UnixMilli()
	dayCutoff := insightDayStart(now.In(loc), loc).AddDate(0, 0, -(historyDays - 1)).UnixMilli()
	if err := db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("bucket_start < ?", rawCutoff).Delete(&model.ClientTrafficBucket{}).Error; err != nil {
			return err
		}
		if err := tx.Where("bucket_start < ?", hourCutoff).Delete(&model.ClientTrafficHourBucket{}).Error; err != nil {
			return err
		}
		if err := tx.Where("bucket_start < ?", dayCutoff).Delete(&model.ClientTrafficDayBucket{}).Error; err != nil {
			return err
		}
		if err := tx.Where("last_seen < ?", dayCutoff).Delete(&model.ClientIPHistory{}).Error; err != nil {
			return err
		}
		if err := tx.Where("last_seen < ?", dayCutoff).Delete(&model.ClientInboundHistory{}).Error; err != nil {
			return err
		}

		// Active temporary actions need their later client events to decide
		// whether an administrator changed the client before auto-restore.
		eventCutoff := dayCutoff
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
		if err := tx.Where("created_at < ? AND status <> ?", dayCutoff, "acted").Delete(&model.ClientAnomaly{}).Error; err != nil {
			return err
		}
		return nil
	}); err != nil {
		return err
	}

	// Bound SQLite's WAL sidecar and refresh planner statistics. Deleted pages
	// remain reusable by SQLite, preventing continued disk growth without the
	// long exclusive lock of a full VACUUM on busy production panels.
	if !database.IsPostgres() {
		_ = db.Exec("PRAGMA wal_checkpoint(TRUNCATE)").Error
		_ = db.Exec("PRAGMA optimize").Error
	}
	return nil
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
