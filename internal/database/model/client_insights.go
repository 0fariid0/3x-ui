package model

// ClientTrafficBucket stores the traffic delta accumulated inside one
// one-minute bucket. Buckets make the per-client charts and anomaly detector
// durable without writing one row for every five-second Xray poll.
type ClientTrafficBucket struct {
	Id          int    `json:"id" gorm:"primaryKey;autoIncrement"`
	Email       string `json:"email" gorm:"uniqueIndex:idx_client_traffic_bucket,priority:1;index:idx_client_traffic_bucket_email;size:255;not null"`
	BucketStart int64  `json:"bucketStart" gorm:"uniqueIndex:idx_client_traffic_bucket,priority:2;index:idx_client_traffic_bucket_time;not null"`
	Up          int64  `json:"up" gorm:"default:0"`
	Down        int64  `json:"down" gorm:"default:0"`
	Samples     int    `json:"samples" gorm:"default:1"`
	CreatedAt   int64  `json:"createdAt" gorm:"autoCreateTime:milli"`
	UpdatedAt   int64  `json:"updatedAt" gorm:"autoUpdateTime:milli"`
}

func (ClientTrafficBucket) TableName() string { return "client_traffic_buckets" }

// ClientIPHistory keeps a bounded historical view of addresses observed for a
// client. inbound_client_ips remains the short-lived live set used by Limit IP;
// this table is reporting-only and is retained for the configured history span.
type ClientIPHistory struct {
	Id        int    `json:"id" gorm:"primaryKey;autoIncrement"`
	Email     string `json:"email" gorm:"uniqueIndex:idx_client_ip_history,priority:1;index:idx_client_ip_history_email;size:255;not null"`
	IP        string `json:"ip" gorm:"uniqueIndex:idx_client_ip_history,priority:2;size:128;not null"`
	FirstSeen int64  `json:"firstSeen" gorm:"index"`
	LastSeen  int64  `json:"lastSeen" gorm:"index"`
	SeenCount int64  `json:"seenCount" gorm:"default:1"`
}

func (ClientIPHistory) TableName() string { return "client_ip_history" }

// ClientEvent is an append-only client timeline used for renewals, edits,
// attachment changes, resets, and automatic anomaly actions.
type ClientEvent struct {
	Id        int    `json:"id" gorm:"primaryKey;autoIncrement"`
	Email     string `json:"email" gorm:"index:idx_client_event_email_time,priority:1;size:255;not null"`
	Kind      string `json:"kind" gorm:"index;size:48;not null"`
	Summary   string `json:"summary" gorm:"size:512"`
	Details   string `json:"details" gorm:"type:text"`
	CreatedAt int64  `json:"createdAt" gorm:"index:idx_client_event_email_time,priority:2;autoCreateTime:milli"`
}

func (ClientEvent) TableName() string { return "client_events" }

// ClientAnomaly stores each detected abnormal-usage incident and the state
// needed to reverse a temporary automatic action.
type ClientAnomaly struct {
	Id                   int    `json:"id" gorm:"primaryKey;autoIncrement"`
	Email                string `json:"email" gorm:"index:idx_client_anomaly_email_time,priority:1;size:255;not null"`
	Kind                 string `json:"kind" gorm:"index;size:32;not null"`
	Severity             string `json:"severity" gorm:"size:16;default:warning"`
	ObservedBytesPerMin  int64  `json:"observedBytesPerMin"`
	ThresholdBytesPerMin int64  `json:"thresholdBytesPerMin"`
	IPCount              int    `json:"ipCount"`
	Action               string `json:"action" gorm:"size:24;default:alert"`
	Status               string `json:"status" gorm:"index;size:24;default:open"`
	PreviousEnable       bool   `json:"previousEnable"`
	PreviousInboundIDs   string `json:"previousInboundIds" gorm:"type:text"`
	AppliedInboundID     int    `json:"appliedInboundId"`
	ActionUntil          int64  `json:"actionUntil" gorm:"index"`
	Details              string `json:"details" gorm:"type:text"`
	CreatedAt            int64  `json:"createdAt" gorm:"index:idx_client_anomaly_email_time,priority:2;autoCreateTime:milli"`
	ResolvedAt           int64  `json:"resolvedAt"`
}

func (ClientAnomaly) TableName() string { return "client_anomalies" }

// ClientTrafficHourBucket is the compact hourly rollup used by longer charts.
// One active client creates at most 24 rows per day, keeping the database small
// while the raw minute table is retained only for the recent 25-hour window.
type ClientTrafficHourBucket struct {
	Id              int    `json:"id" gorm:"primaryKey;autoIncrement"`
	Email           string `json:"email" gorm:"uniqueIndex:idx_client_traffic_hour,priority:1;index:idx_client_traffic_hour_email;size:255;not null"`
	BucketStart     int64  `json:"bucketStart" gorm:"uniqueIndex:idx_client_traffic_hour,priority:2;index:idx_client_traffic_hour_time;not null"`
	Up              int64  `json:"up" gorm:"default:0"`
	Down            int64  `json:"down" gorm:"default:0"`
	ActiveMinutes   int    `json:"activeMinutes" gorm:"default:0"`
	PeakMinuteBytes int64  `json:"peakMinuteBytes" gorm:"default:0"`
	LastMinuteStart int64  `json:"-" gorm:"default:0"`
	LastMinuteBytes int64  `json:"-" gorm:"default:0"`
	CreatedAt       int64  `json:"createdAt" gorm:"autoCreateTime:milli"`
	UpdatedAt       int64  `json:"updatedAt" gorm:"autoUpdateTime:milli"`
}

func (ClientTrafficHourBucket) TableName() string { return "client_traffic_hour_buckets" }

// ClientTrafficDayBucket is the long-term daily rollup. It allows 30–365 day
// usage charts without retaining millions of minute-level rows.
type ClientTrafficDayBucket struct {
	Id              int    `json:"id" gorm:"primaryKey;autoIncrement"`
	Email           string `json:"email" gorm:"uniqueIndex:idx_client_traffic_day,priority:1;index:idx_client_traffic_day_email;size:255;not null"`
	BucketStart     int64  `json:"bucketStart" gorm:"uniqueIndex:idx_client_traffic_day,priority:2;index:idx_client_traffic_day_time;not null"`
	Day             string `json:"day" gorm:"index:idx_client_traffic_day_key;size:10;not null"`
	Up              int64  `json:"up" gorm:"default:0"`
	Down            int64  `json:"down" gorm:"default:0"`
	ActiveMinutes   int    `json:"activeMinutes" gorm:"default:0"`
	PeakMinuteBytes int64  `json:"peakMinuteBytes" gorm:"default:0"`
	LastMinuteStart int64  `json:"-" gorm:"default:0"`
	LastMinuteBytes int64  `json:"-" gorm:"default:0"`
	CreatedAt       int64  `json:"createdAt" gorm:"autoCreateTime:milli"`
	UpdatedAt       int64  `json:"updatedAt" gorm:"autoUpdateTime:milli"`
}

func (ClientTrafficDayBucket) TableName() string { return "client_traffic_day_buckets" }

// ClientDestinationHour stores privacy-preserving hourly destination aggregates.
// No URL path, payload, message content, or DNS query body is retained.
type ClientDestinationHour struct {
	Id          int    `json:"id" gorm:"primaryKey;autoIncrement"`
	Email       string `json:"email" gorm:"uniqueIndex:idx_client_destination_hour,priority:1;index:idx_client_destination_email;size:255;not null"`
	BucketStart int64  `json:"bucketStart" gorm:"uniqueIndex:idx_client_destination_hour,priority:2;index:idx_client_destination_time;not null"`
	Key         string `json:"key" gorm:"uniqueIndex:idx_client_destination_hour,priority:3;size:320;not null"`
	Service     string `json:"service" gorm:"index;size:64"`
	Owner       string `json:"owner" gorm:"size:96"`
	Domain      string `json:"domain" gorm:"size:255"`
	IP          string `json:"ip" gorm:"size:128"`
	Port        int    `json:"port"`
	Protocol    string `json:"protocol" gorm:"size:12"`
	Confidence  string `json:"confidence" gorm:"size:16"`
	Connections int64  `json:"connections" gorm:"default:1"`
	FirstSeen   int64  `json:"firstSeen" gorm:"index"`
	LastSeen    int64  `json:"lastSeen" gorm:"index"`
	CreatedAt   int64  `json:"createdAt" gorm:"autoCreateTime:milli"`
	UpdatedAt   int64  `json:"updatedAt" gorm:"autoUpdateTime:milli"`
}

func (ClientDestinationHour) TableName() string { return "client_destination_hours" }

// ClientDestinationCursor persists the access-log tail offset across panel restarts.
type ClientDestinationCursor struct {
	Id            int    `json:"id" gorm:"primaryKey"`
	Path          string `json:"path" gorm:"size:1024"`
	Offset        int64  `json:"offset"`
	ObservedSize  int64  `json:"observedSize"`
	LastCleanupAt int64  `json:"lastCleanupAt"`
	UpdatedAt     int64  `json:"updatedAt" gorm:"autoUpdateTime:milli"`
}

func (ClientDestinationCursor) TableName() string { return "client_destination_cursor" }
