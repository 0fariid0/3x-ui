package service

import (
	"encoding/json"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"
	"gorm.io/gorm"
)

type clientLimitChange struct {
	Version           int    `json:"version"`
	Source            string `json:"source"`
	PreviousEmail     string `json:"previousEmail,omitempty"`
	PreviousTotal     int64  `json:"previousTotalBytes"`
	Total             int64  `json:"totalBytes"`
	AddedBytes        int64  `json:"addedBytes"`
	PreviousExpiry    int64  `json:"previousExpiryTime"`
	Expiry            int64  `json:"expiryTime"`
	AddedMilliseconds *int64 `json:"addedMilliseconds"`
}

func recordClientTrafficChange(tx *gorm.DB, previous xray.ClientTraffic, upload, download int64, kind string) error {
	if previous.Up == upload && previous.Down == download {
		return nil
	}
	details, err := json.Marshal(map[string]any{
		"version": 1, "source": "panel_or_api", "previousUpload": previous.Up, "previousDownload": previous.Down,
		"upload": upload, "download": download, "previousUsedBytes": previous.Up + previous.Down, "usedBytes": upload + download,
	})
	if err != nil {
		return err
	}
	return tx.Create(&model.ClientEvent{
		Email: previous.Email, Kind: kind, Summary: "Client traffic counters changed", Details: string(details), CreatedAt: time.Now().UnixMilli(),
	}).Error
}

func recordClientTrafficResets(tx *gorm.DB, emails []string) error {
	for _, batch := range chunkStrings(emails, sqlInChunk) {
		var previous []xray.ClientTraffic
		if err := tx.Where("email IN ?", batch).Find(&previous).Error; err != nil {
			return err
		}
		for _, traffic := range previous {
			if err := recordClientTrafficChange(tx, traffic, 0, 0, "traffic_reset"); err != nil {
				return err
			}
		}
	}
	return nil
}

func recordClientLimitChange(tx *gorm.DB, previous, current model.ClientRecord) error {
	if previous.TotalGB == current.TotalGB && previous.ExpiryTime == current.ExpiryTime {
		return nil
	}
	change := clientLimitChange{
		Version: 1, Source: "panel_or_api", PreviousEmail: previous.Email,
		PreviousTotal: previous.TotalGB, Total: current.TotalGB, AddedBytes: current.TotalGB - previous.TotalGB,
		PreviousExpiry: previous.ExpiryTime, Expiry: current.ExpiryTime,
	}
	kind := "limits_changed"
	summary := "Client quota or expiry changed"
	if previous.ExpiryTime < 0 && current.ExpiryTime > 0 {
		kind, summary = "activated", "Client first-use expiry activated"
	} else if previous.ExpiryTime != 0 && current.ExpiryTime != 0 && (previous.ExpiryTime < 0) == (current.ExpiryTime < 0) {
		delta := current.ExpiryTime - previous.ExpiryTime
		if previous.ExpiryTime < 0 {
			delta = -delta
		}
		change.AddedMilliseconds = &delta
	}
	if kind != "activated" && (change.AddedBytes > 0 || (change.AddedMilliseconds != nil && *change.AddedMilliseconds > 0)) {
		kind, summary = "renewed", "Client renewed or quota increased"
	}
	details, err := json.Marshal(change)
	if err != nil {
		return err
	}
	return tx.Create(&model.ClientEvent{
		Email: current.Email, Kind: kind, Summary: summary, Details: string(details), CreatedAt: time.Now().UnixMilli(),
	}).Error
}
