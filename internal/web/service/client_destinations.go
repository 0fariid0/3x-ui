package service

import (
	"bufio"
	"errors"
	"io"
	"net"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/logger"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	destinationActiveWindow       = 5 * time.Minute
	destinationBucketLimit        = 5
	destinationMaxPerClientBucket = 100
	destinationMaxReportItems     = 200
)

var destinationIngestMu sync.Mutex

// ClientDestinationItem is an aggregate destination shown in client reports.
type ClientDestinationItem struct {
	Key         string `json:"key"`
	Service     string `json:"service"`
	Owner       string `json:"owner"`
	Domain      string `json:"domain,omitempty"`
	IP          string `json:"ip,omitempty"`
	Port        int    `json:"port,omitempty"`
	Protocol    string `json:"protocol,omitempty"`
	Confidence  string `json:"confidence"`
	Connections int64  `json:"connections"`
	FirstSeen   int64  `json:"firstSeen"`
	LastSeen    int64  `json:"lastSeen"`
	Active      bool   `json:"active"`
}

// ClientDestinationSummary groups destination rows by recognized service.
type ClientDestinationSummary struct {
	Service            string `json:"service"`
	Owner              string `json:"owner"`
	Connections        int64  `json:"connections"`
	Destinations       int    `json:"destinations"`
	LastSeen           int64  `json:"lastSeen"`
	Active             bool   `json:"active"`
	ActiveDestinations int    `json:"activeDestinations"`
}

type destinationEvent struct {
	Email       string
	BucketStart int64
	Key         string
	Service     string
	Owner       string
	Domain      string
	IP          string
	Port        int
	Protocol    string
	Confidence  string
	FirstAt     int64
	LastAt      int64
	Count       int64
}

type serviceRule struct {
	Service string
	Owner   string
	Domains []string
}

var destinationServiceRules = []serviceRule{
	{Service: "Instagram", Owner: "Meta", Domains: []string{"instagram.com", "cdninstagram.com"}},
	{Service: "Facebook", Owner: "Meta", Domains: []string{"facebook.com", "facebook.net", "fbcdn.net", "fb.com"}},
	{Service: "WhatsApp", Owner: "Meta", Domains: []string{"whatsapp.com", "whatsapp.net"}},
	{Service: "Telegram", Owner: "Telegram", Domains: []string{"telegram.org", "telegram.me", "t.me", "telesco.pe"}},
	{Service: "YouTube", Owner: "Google", Domains: []string{"youtube.com", "youtu.be", "googlevideo.com", "ytimg.com"}},
	{Service: "Google", Owner: "Google", Domains: []string{"google.com", "googleapis.com", "gstatic.com", "googleusercontent.com", "1e100.net"}},
	{Service: "TikTok", Owner: "ByteDance", Domains: []string{"tiktok.com", "tiktokcdn.com", "byteoversea.com", "ibytedtos.com"}},
	{Service: "X / Twitter", Owner: "X Corp", Domains: []string{"x.com", "twitter.com", "twimg.com", "t.co"}},
	{Service: "Discord", Owner: "Discord", Domains: []string{"discord.com", "discord.gg", "discordapp.com", "discordapp.net"}},
	{Service: "Spotify", Owner: "Spotify", Domains: []string{"spotify.com", "scdn.co", "spotifycdn.com"}},
	{Service: "Netflix", Owner: "Netflix", Domains: []string{"netflix.com", "nflxvideo.net", "nflximg.net", "nflxext.com"}},
	{Service: "Apple", Owner: "Apple", Domains: []string{"apple.com", "icloud.com", "mzstatic.com"}},
	{Service: "Microsoft", Owner: "Microsoft", Domains: []string{"microsoft.com", "live.com", "office.com", "windows.net", "azureedge.net"}},
	{Service: "GitHub", Owner: "GitHub", Domains: []string{"github.com", "githubusercontent.com", "githubassets.com"}},
	{Service: "Cloudflare", Owner: "Cloudflare", Domains: []string{"cloudflare.com", "cloudflare.net", "cloudflare-dns.com"}},
}

func suffixMatch(host, suffix string) bool {
	return host == suffix || strings.HasSuffix(host, "."+suffix)
}

func compactDomain(host string) string {
	host = strings.TrimSuffix(strings.ToLower(strings.TrimSpace(host)), ".")
	for _, rule := range destinationServiceRules {
		for _, suffix := range rule.Domains {
			if suffixMatch(host, suffix) {
				return suffix
			}
		}
	}
	parts := strings.Split(host, ".")
	if len(parts) <= 2 {
		return host
	}
	// Keep one extra label for common country-code registries such as co.uk.
	if len(parts[len(parts)-1]) == 2 && len(parts[len(parts)-2]) <= 3 && len(parts) >= 3 {
		return strings.Join(parts[len(parts)-3:], ".")
	}
	return strings.Join(parts[len(parts)-2:], ".")
}

func classifyDestination(domain, ip string) (service, owner, confidence string) {
	domain = strings.TrimSpace(strings.ToLower(domain))
	ip = strings.TrimSpace(ip)
	if domain != "" {
		for _, rule := range destinationServiceRules {
			for _, suffix := range rule.Domains {
				if suffixMatch(domain, suffix) {
					return rule.Service, rule.Owner, "domain"
				}
			}
		}
		return "Other", "", "domain"
	}
	parsed := parseDestinationIP(ip)
	if parsed != nil {
		if parsed.IsPrivate() || parsed.IsLoopback() || parsed.IsLinkLocalUnicast() {
			return "Private network", "Local", "ip"
		}
		if service, owner, confidence, ok := classifyDestinationIP(parsed); ok {
			return service, owner, confidence
		}
	}
	return "Other", "", "ip"
}

func parseDestinationIP(raw string) net.IP {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	if host, _, err := net.SplitHostPort(raw); err == nil {
		raw = host
	}
	raw = strings.Trim(raw, "[]")
	if zone := strings.LastIndexByte(raw, '%'); zone > 0 {
		raw = raw[:zone]
	}
	return net.ParseIP(strings.TrimSpace(raw))
}

func splitDestination(raw string) (protocol, host string, port int) {
	raw = strings.TrimSpace(strings.TrimLeft(raw, "/"))
	if raw == "" {
		return "", "", 0
	}
	if idx := strings.IndexByte(raw, ':'); idx > 0 {
		prefix := strings.ToLower(raw[:idx])
		if prefix == "tcp" || prefix == "udp" {
			protocol = prefix
			raw = raw[idx+1:]
		}
	}
	if h, p, err := net.SplitHostPort(raw); err == nil {
		host = strings.Trim(h, "[]")
		port, _ = strconv.Atoi(p)
		return protocol, host, port
	}
	// Xray usually logs host:port. Keep a bare host/IP if no port is present.
	return protocol, strings.Trim(raw, "[]"), 0
}

func parseDestinationAccessLine(line string, tracked map[string]struct{}, loc *time.Location) (destinationEvent, bool) {
	parts := strings.Fields(strings.TrimSpace(line))
	if len(parts) < 5 {
		return destinationEvent{}, false
	}
	var email, target string
	for i, part := range parts {
		switch part {
		case "accepted":
			if i+1 < len(parts) {
				target = strings.TrimLeft(parts[i+1], "/")
			}
		case "email:":
			if i+1 < len(parts) {
				email = strings.TrimSpace(parts[i+1])
			}
		}
	}
	if email == "" || target == "" {
		return destinationEvent{}, false
	}
	if _, ok := tracked[email]; !ok {
		return destinationEvent{}, false
	}

	// Use the ingestion instant rather than Xray's timezone-less log prefix.
	// Epoch timestamps then render consistently with the panel's selected clock.
	at := time.Now().In(loc)
	protocol, host, port := splitDestination(target)
	if host == "" {
		return destinationEvent{}, false
	}
	domain, ip := "", ""
	if parsed := net.ParseIP(host); parsed != nil {
		ip = parsed.String()
	} else {
		domain = compactDomain(host)
	}
	service, owner, confidence := classifyDestination(domain, ip)
	bucket := at.Truncate(time.Minute).UnixMilli()
	keyHost := domain
	if keyHost == "" {
		keyHost = ip
	}
	key := protocol + ":" + keyHost + ":" + strconv.Itoa(port)
	return destinationEvent{
		Email: email, BucketStart: bucket, Key: key, Service: service, Owner: owner,
		Domain: domain, IP: ip, Port: port, Protocol: protocol, Confidence: confidence,
		FirstAt: at.UnixMilli(), LastAt: at.UnixMilli(), Count: 1,
	}, true
}

func trackedDestinationEmails(db *gorm.DB) (map[string]struct{}, error) {
	var emails []string
	if err := db.Model(&model.ClientRecord{}).Where("destination_tracking = ?", true).Pluck("email", &emails).Error; err != nil {
		return nil, err
	}
	out := make(map[string]struct{}, len(emails))
	for _, email := range emails {
		if email = strings.TrimSpace(email); email != "" {
			out[email] = struct{}{}
		}
	}
	return out, nil
}

func destinationIsActive(lastSeen int64, now time.Time) bool {
	if lastSeen <= 0 {
		return false
	}
	seen := time.UnixMilli(lastSeen)
	return !seen.After(now.Add(30*time.Second)) && !seen.Before(now.Add(-destinationActiveWindow))
}

func latestDestinationBucketStarts(db *gorm.DB, email string, limit int) ([]int64, error) {
	if db == nil {
		return nil, errors.New("database is not initialized")
	}
	email = strings.TrimSpace(email)
	if email == "" || limit <= 0 {
		return []int64{}, nil
	}

	buckets := make([]int64, 0, limit)
	err := db.Model(&model.ClientDestinationHour{}).
		Distinct("bucket_start").
		Where("email = ?", email).
		Order("bucket_start DESC").
		Limit(limit).
		Pluck("bucket_start", &buckets).Error
	return buckets, err
}

// pruneDestinationBuckets keeps exactly the five newest one-minute buckets for
// a client. It runs only after a new destination bucket is written. Therefore an
// offline client keeps its final five buckets unchanged until new activity creates
// a sixth bucket, at which point only the oldest bucket is removed.
func pruneDestinationBuckets(db *gorm.DB, email string) error {
	buckets, err := latestDestinationBucketStarts(db, email, destinationBucketLimit+1)
	if err != nil {
		return err
	}
	if len(buckets) <= destinationBucketLimit {
		return nil
	}

	oldestKept := buckets[destinationBucketLimit-1]
	return db.Where("email = ? AND bucket_start < ?", strings.TrimSpace(email), oldestKept).
		Delete(&model.ClientDestinationHour{}).Error
}

// AnyDestinationTrackingEnabled reports whether any client opted into persisted destination reporting.
// Exact live inbound attribution uses the runtime access log for every client regardless of this setting.
func AnyDestinationTrackingEnabled() bool {
	var count int64
	err := database.GetDB().Model(&model.ClientRecord{}).Where("destination_tracking = ?", true).Limit(1).Count(&count).Error
	return err == nil && count > 0
}

// IngestDestinationAccessLog tails Xray's bounded access log. Every accepted
// connection refreshes transient exact email -> inbound attribution in memory;
// minute destination aggregates are persisted only for clients that opted in.
func (s *ClientInsightService) IngestDestinationAccessLog() error {
	destinationIngestMu.Lock()
	defer destinationIngestMu.Unlock()

	db := database.GetDB()
	tracked, err := trackedDestinationEmails(db)
	if err != nil {
		return err
	}
	path, err := xray.GetAccessLogPath()
	if err != nil || path == "" || strings.EqualFold(path, "none") {
		(&InboundService{}).RefreshLocalOnlineInbounds(nil)
		return err
	}
	file, err := os.Open(path)
	if err != nil {
		(&InboundService{}).RefreshLocalOnlineInbounds(nil)
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return err
	}

	cursor := model.ClientDestinationCursor{Id: 1}
	if err := db.First(&cursor, 1).Error; err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	if cursor.Path != path || cursor.Offset < 0 || info.Size() < cursor.Offset {
		cursor.Offset = 0
	}
	if _, err := file.Seek(cursor.Offset, io.SeekStart); err != nil {
		return err
	}

	loc := insightPanelLocation()
	reader := bufio.NewReaderSize(file, 64*1024)
	offset := cursor.Offset
	aggregated := make(map[string]destinationEvent)
	// Live inbound attribution is never persisted. Keep only the pairs observed
	// in this bounded access-log pass, then fold them into the process grace
	// window below.
	liveInboundSets := make(map[string]map[string]struct{})
	for {
		raw, readErr := reader.ReadString('\n')
		if len(raw) > 0 {
			// An unterminated final line may still be being written; process it next run.
			if readErr == io.EOF && !strings.HasSuffix(raw, "\n") {
				break
			}
			offset += int64(len(raw))
			entry := parseAccessLogFields(raw)
			email := strings.TrimSpace(entry.Email)
			inboundTag := strings.TrimSpace(entry.Inbound)
			if email != "" && inboundTag != "" {
				set := liveInboundSets[email]
				if set == nil {
					set = make(map[string]struct{})
					liveInboundSets[email] = set
				}
				set[inboundTag] = struct{}{}
			}
			if len(tracked) > 0 {
				if event, ok := parseDestinationAccessLine(raw, tracked, loc); ok {
					mapKey := event.Email + "\x00" + strconv.FormatInt(event.BucketStart, 10) + "\x00" + event.Key
					if current, exists := aggregated[mapKey]; exists {
						current.Count++
						if event.FirstAt < current.FirstAt {
							current.FirstAt = event.FirstAt
						}
						if event.LastAt > current.LastAt {
							current.LastAt = event.LastAt
						}
						aggregated[mapKey] = current
					} else {
						aggregated[mapKey] = event
					}
				}
			}
		}
		if readErr != nil {
			if readErr != io.EOF {
				logger.Debug("[ClientDestinations] read access log failed:", readErr)
			}
			break
		}
	}

	liveInbounds := make(map[string][]string, len(liveInboundSets))
	for email, tags := range liveInboundSets {
		for tag := range tags {
			liveInbounds[email] = append(liveInbounds[email], tag)
		}
	}
	(&InboundService{}).RefreshLocalOnlineInbounds(liveInbounds)

	if len(aggregated) > 0 {
		if err := db.Transaction(func(tx *gorm.DB) error {
			groups := make(map[string][]destinationEvent)
			touchedEmails := make(map[string]struct{})
			for _, event := range aggregated {
				groupKey := event.Email + "\x00" + strconv.FormatInt(event.BucketStart, 10)
				groups[groupKey] = append(groups[groupKey], event)
				touchedEmails[event.Email] = struct{}{}
			}
			for _, events := range groups {
				sort.Slice(events, func(i, j int) bool { return events[i].Count > events[j].Count })
				if len(events) > destinationMaxPerClientBucket {
					events = events[:destinationMaxPerClientBucket]
				}
				for _, event := range events {
					row := model.ClientDestinationHour{
						Email: event.Email, BucketStart: event.BucketStart, Key: event.Key,
						Service: event.Service, Owner: event.Owner, Domain: event.Domain, IP: event.IP,
						Port: event.Port, Protocol: event.Protocol, Confidence: event.Confidence,
						Connections: event.Count, FirstSeen: event.FirstAt, LastSeen: event.LastAt,
					}
					if err := tx.Clauses(clause.OnConflict{
						Columns: []clause.Column{{Name: "email"}, {Name: "bucket_start"}, {Name: "key"}},
						DoUpdates: clause.Assignments(map[string]any{
							// Classification rules can improve between releases. Refresh all
							// descriptive fields when an existing minute row is hit so a row
							// previously stored as Other is repaired by the next connection.
							"service":     event.Service,
							"owner":       event.Owner,
							"domain":      event.Domain,
							"ip":          event.IP,
							"port":        event.Port,
							"protocol":    event.Protocol,
							"confidence":  event.Confidence,
							"connections": gorm.Expr("connections + ?", event.Count),
							"first_seen":  gorm.Expr("CASE WHEN first_seen = 0 OR first_seen > ? THEN ? ELSE first_seen END", event.FirstAt, event.FirstAt),
							"last_seen":   gorm.Expr("CASE WHEN last_seen < ? THEN ? ELSE last_seen END", event.LastAt, event.LastAt),
						}),
					}).Create(&row).Error; err != nil {
						return err
					}
				}
				if len(events) > 0 {
					var ids []int
					if err := tx.Model(&model.ClientDestinationHour{}).
						Where("email = ? AND bucket_start = ?", events[0].Email, events[0].BucketStart).
						Order("connections DESC, last_seen DESC").
						Pluck("id", &ids).Error; err != nil {
						return err
					}
					if len(ids) > destinationMaxPerClientBucket {
						if err := tx.Where("id IN ?", ids[destinationMaxPerClientBucket:]).Delete(&model.ClientDestinationHour{}).Error; err != nil {
							return err
						}
					}
				}
			}
			for email := range touchedEmails {
				if err := pruneDestinationBuckets(tx, email); err != nil {
					return err
				}
			}
			return nil
		}); err != nil {
			return err
		}
	}

	cursor.Path = path
	cursor.Offset = offset
	cursor.ObservedSize = info.Size()

	// The generated destination log may briefly contain lines for all clients
	// because Xray access logging is process-wide. Once every complete line is
	// aggregated, truncate this dedicated file so unselected clients are not
	// retained as raw records. Administrator-configured access logs are never
	// truncated here.
	if base := filepath.Base(filepath.Clean(path)); base == "client-runtime-access.log" || base == "client-destinations-access.log" {
		if latest, statErr := os.Stat(path); statErr == nil && latest.Size() == offset {
			if truncateErr := os.Truncate(path, 0); truncateErr != nil {
				logger.Debug("[ClientDestinations] truncate dedicated access log failed:", truncateErr)
			} else {
				cursor.Offset = 0
				cursor.ObservedSize = 0
			}
		}
	}
	return db.Save(&cursor).Error
}

func reclassifyDestinationItem(item *ClientDestinationItem) {
	if item == nil {
		return
	}
	item.Service, item.Owner, item.Confidence = classifyDestination(item.Domain, item.IP)
	item.Active = destinationIsActive(item.LastSeen, time.Now())
}

// ClearDestinations removes all stored destination aggregates for one client.
// Pending complete access-log lines are ingested first so data generated before
// the administrator pressed reset cannot reappear on the next ingestion run.
func (s *ClientInsightService) ClearDestinations(email string) error {
	email = strings.TrimSpace(email)
	if email == "" {
		return errors.New("client email is required")
	}
	if err := s.IngestDestinationAccessLog(); err != nil {
		return err
	}
	return database.GetDB().Where("email = ?", email).Delete(&model.ClientDestinationHour{}).Error
}

func (s *ClientInsightService) destinationReport(email string, _ int64, _ int64) ([]ClientDestinationItem, []ClientDestinationSummary, error) {
	// Opening a report also schedules a provider-rule refresh. Bundled ranges
	// classify immediately; refreshed ranges are used by subsequent reports.
	RefreshDestinationNetworkRulesIfNeeded()
	items := make([]ClientDestinationItem, 0)
	db := database.GetDB()

	// Destination history is a rolling set of five one-minute activity buckets,
	// not a wall-clock expiry window. Querying by the five newest bucket starts
	// keeps the final snapshot visible when the client goes offline, even after
	// hours or days. A sixth activity minute is what removes the oldest bucket.
	bucketStarts, err := latestDestinationBucketStarts(db, email, destinationBucketLimit)
	if err != nil {
		return nil, nil, err
	}
	if len(bucketStarts) == 0 {
		return items, []ClientDestinationSummary{}, nil
	}

	err = db.Model(&model.ClientDestinationHour{}).
		// Do not group by the stored classification. Older rows may have been
		// saved as Other before a CIDR/domain rule was added. Aggregate by the
		// actual destination and classify every returned item with the current
		// rules, so upgrades repair historical reports immediately.
		Select("key, domain, ip, port, protocol, SUM(connections) AS connections, MIN(first_seen) AS first_seen, MAX(last_seen) AS last_seen").
		Where("email = ? AND bucket_start IN ?", email, bucketStarts).
		Group("key, domain, ip, port, protocol").
		Order("connections DESC, last_seen DESC").
		Limit(destinationMaxReportItems).
		Scan(&items).Error
	if err != nil {
		return nil, nil, err
	}
	for i := range items {
		reclassifyDestinationItem(&items[i])
	}
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].Active != items[j].Active {
			return items[i].Active
		}
		if items[i].Connections != items[j].Connections {
			return items[i].Connections > items[j].Connections
		}
		return items[i].LastSeen > items[j].LastSeen
	})
	type summaryAccumulator struct {
		ClientDestinationSummary
		keys map[string]struct{}
	}
	byService := make(map[string]*summaryAccumulator)
	for _, item := range items {
		name := item.Service
		if name == "" {
			name = "Other"
		}
		acc := byService[name]
		if acc == nil {
			acc = &summaryAccumulator{ClientDestinationSummary: ClientDestinationSummary{Service: name, Owner: item.Owner}, keys: map[string]struct{}{}}
			byService[name] = acc
		}
		acc.Connections += item.Connections
		acc.keys[item.Key] = struct{}{}
		if item.Active {
			acc.Active = true
			acc.ActiveDestinations++
		}
		if item.LastSeen > acc.LastSeen {
			acc.LastSeen = item.LastSeen
		}
	}
	summaries := make([]ClientDestinationSummary, 0, len(byService))
	for _, acc := range byService {
		acc.Destinations = len(acc.keys)
		summaries = append(summaries, acc.ClientDestinationSummary)
	}
	sort.Slice(summaries, func(i, j int) bool {
		if summaries[i].Active != summaries[j].Active {
			return summaries[i].Active
		}
		return summaries[i].Connections > summaries[j].Connections
	})
	return items, summaries, nil
}
