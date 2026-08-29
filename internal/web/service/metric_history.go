package service

import (
	"encoding/gob"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/config"
	"github.com/mhsanaei/3x-ui/v3/internal/logger"
)

// MetricSample is one point of any time-series we keep in memory.
// The frontend deserializes both keys, so they must stay short.
type MetricSample struct {
	T int64   `json:"t"`
	V float64 `json:"v"`
}

// tierSpec defines one resolution layer of the rollup ladder: a fixed bucket
// size in seconds and how many buckets to retain. window = resolution*capacity.
type tierSpec struct {
	resolution int
	capacity   int
}

// metricTiers is the rollup ladder applied to every series. High resolution is
// kept only for the recent past; older samples roll up into progressively
// coarser, cheaper layers (RRDtool-style). Per series this totals ~5700 samples
// (~100 KiB) yet spans a live 2s view through ~31 days of history.
var metricTiers = []tierSpec{
	{resolution: 2, capacity: 1800},   // 1h at 2s
	{resolution: 60, capacity: 2880},  // 48h at 1m
	{resolution: 600, capacity: 1008}, // 7d at 10m
	{resolution: 3600, capacity: 744}, // 31d at 1h
}

// tierBuf is one fixed-resolution ring of a series. Samples land in an open
// bucket and are averaged into the ring only when the next bucket begins, so a
// coarse tier carries one mean per bucket instead of every raw point.
type tierBuf struct {
	resolution int
	capacity   int
	sumValues  bool
	samples    []MetricSample
	open       bool
	openStart  int64
	openSum    float64
	openCount  int
}

func (tb *tierBuf) add(unixSec int64, v float64) {
	res := int64(tb.resolution)
	b := (unixSec / res) * res
	if tb.open && b != tb.openStart {
		tb.flush()
	}
	tb.open = true
	tb.openStart = b
	tb.openSum += v
	tb.openCount++
}

func (tb *tierBuf) flush() {
	if tb.openCount == 0 {
		tb.open = false
		return
	}
	value := tb.openSum
	if !tb.sumValues {
		value /= float64(tb.openCount)
	}
	tb.samples = append(tb.samples, MetricSample{T: tb.openStart, V: value})
	if len(tb.samples) > tb.capacity {
		tb.samples = tb.samples[len(tb.samples)-tb.capacity:]
	}
	tb.open = false
	tb.openStart = 0
	tb.openSum = 0
	tb.openCount = 0
}

// readSamples returns a copy of the closed buckets plus the still-open one, so
// the most recent point is visible before its bucket boundary closes.
func (tb *tierBuf) readSamples() []MetricSample {
	out := make([]MetricSample, len(tb.samples), len(tb.samples)+1)
	copy(out, tb.samples)
	if tb.openCount > 0 {
		value := tb.openSum
		if !tb.sumValues {
			value /= float64(tb.openCount)
		}
		out = append(out, MetricSample{T: tb.openStart, V: value})
	}
	return out
}

// series is the rollup ladder for one metric: a sample is fed to every tier.
type series struct {
	tiers     []*tierBuf
	sumValues bool
}

func newSeries(sumValues ...bool) *series {
	isSum := len(sumValues) > 0 && sumValues[0]
	s := &series{tiers: make([]*tierBuf, len(metricTiers)), sumValues: isSum}
	for i, spec := range metricTiers {
		s.tiers[i] = &tierBuf{resolution: spec.resolution, capacity: spec.capacity, sumValues: isSum}
	}
	return s
}

func (s *series) add(unixSec int64, v float64) {
	for _, tb := range s.tiers {
		tb.add(unixSec, v)
	}
}

// pickTier returns the finest tier whose window covers spanSeconds, falling back
// to the coarsest (longest-window) tier when nothing covers it.
func (s *series) pickTier(spanSeconds int64) *tierBuf {
	for _, tb := range s.tiers {
		if int64(tb.resolution)*int64(tb.capacity) >= spanSeconds {
			return tb
		}
	}
	return s.tiers[len(s.tiers)-1]
}

// metricHistory is a thread-safe, in-memory store of tiered series keyed by
// arbitrary strings. Three singletons live below: system-wide host metrics,
// per-node metrics, and xray expvar metrics.
type metricHistory struct {
	mu     sync.Mutex
	series map[string]*series
}

func newMetricHistory() *metricHistory {
	return &metricHistory{series: map[string]*series{}}
}

// append stores a single sample for the given metric across all tiers.
func (h *metricHistory) append(metric string, t time.Time, v float64) {
	h.appendValue(metric, t, v, false)
}

// appendSum stores an increment rather than a gauge. Rollup tiers add these
// values instead of averaging them, which preserves traffic byte totals even
// in the 31-day archive.
func (h *metricHistory) appendSum(metric string, t time.Time, v float64) {
	h.appendValue(metric, t, v, true)
}

func (h *metricHistory) appendValue(metric string, t time.Time, v float64, sumValues bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	s := h.series[metric]
	if s == nil {
		s = newSeries(sumValues)
		h.series[metric] = s
	}
	s.add(t.Unix(), v)
}

// aggregateSumRange returns byte totals and chart-rate points for an increment
// series over [from,to). Coarse archive buckets that only partially overlap a
// requested boundary are prorated; this limits rolling-window boundary error
// without inventing traffic outside the retained samples.
func (h *metricHistory) aggregateSumRange(metric string, from, to int64, bucketSeconds int) (float64, []MetricSample) {
	if from >= to || bucketSeconds <= 0 {
		return 0, []MetricSample{}
	}

	h.mu.Lock()
	s := h.series[metric]
	if s == nil || !s.sumValues {
		h.mu.Unlock()
		return 0, []MetricSample{}
	}
	tier := s.pickTier(to - from)
	raw := tier.readSamples()
	resolution := int64(tier.resolution)
	openStart := int64(-1)
	openEnd := int64(0)
	if tier.openCount > 0 {
		openStart = tier.openStart
		openEnd = time.Now().Unix()
		if openEnd > openStart+resolution {
			openEnd = openStart + resolution
		}
	}
	h.mu.Unlock()

	values := make(map[int64]float64)
	var total float64
	for _, p := range raw {
		pointEnd := p.T + resolution
		if p.T == openStart && openEnd > p.T {
			pointEnd = openEnd
		}
		start := max(p.T, from)
		end := min(pointEnd, to)
		if start >= end || pointEnd <= p.T {
			continue
		}
		weighted := p.V * float64(end-start) / float64(pointEnd-p.T)
		total += weighted

		// A retained tier is always finer than the display buckets used by the
		// traffic endpoint, so one source point normally maps to one chart point.
		// Split defensively in case a future caller asks for a smaller bucket.
		cursor := start
		for cursor < end {
			bucketStart := from + ((cursor - from) / int64(bucketSeconds) * int64(bucketSeconds))
			bucketEnd := min(bucketStart+int64(bucketSeconds), end)
			part := weighted * float64(bucketEnd-cursor) / float64(end-start)
			values[bucketStart] += part
			cursor = bucketEnd
		}
	}

	points := make([]MetricSample, 0, len(values))
	for ts := from; ts < to; ts += int64(bucketSeconds) {
		if value, ok := values[ts]; ok {
			duration := min(int64(bucketSeconds), to-ts)
			points = append(points, MetricSample{T: ts, V: value / float64(duration)})
		}
	}
	return total, points
}

// drop removes the entire history for one metric. Used when a node is deleted so
// its old samples don't linger forever in the singleton.
func (h *metricHistory) drop(metric string) {
	h.mu.Lock()
	delete(h.series, metric)
	h.mu.Unlock()
}

// aggregate returns up to maxPoints buckets of size bucketSeconds, each carrying
// the arithmetic mean of the underlying samples from the finest tier that covers
// the requested span. Bucket alignment is to absolute Unix-second boundaries so
// two concurrent calls see identical x-axes.
func (h *metricHistory) aggregate(metric string, bucketSeconds int, maxPoints int) []map[string]any {
	empty := []map[string]any{}
	if bucketSeconds <= 0 || maxPoints <= 0 {
		return empty
	}
	span := int64(bucketSeconds) * int64(maxPoints)
	cutoff := time.Now().Unix() - span

	h.mu.Lock()
	s := h.series[metric]
	if s == nil {
		h.mu.Unlock()
		return empty
	}
	raw := s.pickTier(span).readSamples()
	h.mu.Unlock()

	startIdx := len(raw)
	for i, r := range slices.Backward(raw) {
		if r.T < cutoff {
			break
		}
		startIdx = i
	}
	tmp := raw[startIdx:]
	if len(tmp) == 0 {
		return empty
	}

	bSize := int64(bucketSeconds)
	curBucket := (tmp[0].T / bSize) * bSize
	var out []map[string]any
	var acc []float64
	flush := func(ts int64) {
		if len(acc) == 0 {
			return
		}
		sum := 0.0
		for _, v := range acc {
			sum += v
		}
		out = append(out, map[string]any{"t": ts, "v": sum / float64(len(acc))})
		acc = acc[:0]
	}
	for _, p := range tmp {
		b := (p.T / bSize) * bSize
		if b != curBucket {
			flush(curBucket)
			curBucket = b
		}
		acc = append(acc, p.V)
	}
	flush(curBucket)
	if len(out) > maxPoints {
		out = out[len(out)-maxPoints:]
	}
	if out == nil {
		return empty
	}
	return out
}

// persistedTier and persistedSeries are the on-disk shape of a series. Tiers are
// matched back by resolution on restore, so changing the ladder degrades
// gracefully (unmatched layers are dropped) instead of corrupting state.
type persistedTier struct {
	Resolution int
	Samples    []MetricSample
}

type persistedSeries struct {
	Tiers     []persistedTier
	SumValues bool
}

// snapshot returns a deep copy of every series' closed buckets, safe to
// serialize without holding the lock during disk I/O.
func (h *metricHistory) snapshot() map[string]persistedSeries {
	h.mu.Lock()
	defer h.mu.Unlock()
	out := make(map[string]persistedSeries, len(h.series))
	for k, s := range h.series {
		ps := persistedSeries{Tiers: make([]persistedTier, len(s.tiers)), SumValues: s.sumValues}
		for i, tb := range s.tiers {
			stored := tb.samples
			// Persist the current partial traffic bucket as well, limiting
			// restart loss to the most recent 2-second collection interval.
			// Gauge series keep their historical closed-bucket behavior.
			if s.sumValues {
				stored = tb.readSamples()
			}
			cp := make([]MetricSample, len(stored))
			copy(cp, stored)
			ps.Tiers[i] = persistedTier{Resolution: tb.resolution, Samples: cp}
		}
		out[k] = ps
	}
	return out
}

// restore replaces the in-memory series with a previously persisted set,
// re-applying each tier's capacity cap so a tampered or oversized file can't grow
// the working set unbounded.
func (h *metricHistory) restore(data map[string]persistedSeries) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for k, ps := range data {
		s := newSeries(ps.SumValues)
		for _, pt := range ps.Tiers {
			for _, tb := range s.tiers {
				if tb.resolution != pt.Resolution {
					continue
				}
				samples := pt.Samples
				if len(samples) > tb.capacity {
					samples = samples[len(samples)-tb.capacity:]
				}
				tb.samples = samples
				break
			}
		}
		h.series[k] = s
	}
}

// systemMetrics holds whole-host time series (cpu, mem, netUp, etc.) fed by
// ServerService.RefreshStatus every 2s. nodeMetrics holds per-node CPU/Mem fed
// by NodeHeartbeatJob. xrayMetrics holds xray expvar series. Only systemMetrics
// is persisted; the others rebuild from live connections.
var (
	systemMetrics = newMetricHistory()
	nodeMetrics   = newMetricHistory()
	xrayMetrics   = newMetricHistory()
)

// SystemMetricKeys lists the metric names ServerService writes on every status
// sample. Exposed for documentation/test purposes; the controller validates
// incoming names against an allow-list.
var SystemMetricKeys = []string{
	"cpu", "mem", "swap", "netUp", "netDown", "pktUp", "pktDown", "diskRead", "diskWrite", "diskUsage", "tcpCount", "udpCount", "online", "load1", "load5", "load15",
}

// NodeMetricKeys lists the per-node metric names NodeHeartbeatJob writes.
var NodeMetricKeys = []string{"cpu", "mem", "netUp", "netDown"}

// XrayMetricKeys lists series sourced from xray's /debug/vars expvar endpoint.
var XrayMetricKeys = []string{
	"xrAlloc", "xrSys", "xrHeapObjects", "xrNumGC", "xrPauseNs",
}

// systemMetricsStorePath is where the host time-series is persisted between
// restarts. It lives next to the database so a single volume mount carries both.
func systemMetricsStorePath() string {
	return filepath.Join(config.GetDBFolderPath(), "system_metrics.gob")
}

// PersistSystemMetrics writes the host time-series to disk via a temp file +
// rename so a crash mid-write can't corrupt the previous snapshot. Called on a
// timer and at shutdown.
func PersistSystemMetrics() error {
	path := systemMetricsStorePath()
	tmp := path + ".tmp"
	f, err := os.Create(tmp)
	if err != nil {
		return err
	}
	if err := gob.NewEncoder(f).Encode(systemMetrics.snapshot()); err != nil {
		f.Close()
		os.Remove(tmp)
		return err
	}
	if err := f.Close(); err != nil {
		os.Remove(tmp)
		return err
	}
	return os.Rename(tmp, path)
}

// RestoreSystemMetrics loads a previously persisted host time-series on startup.
// A missing file is not an error (first boot). A pre-tier flat snapshot is
// migrated by replaying its samples through the rollup.
func RestoreSystemMetrics() {
	path := systemMetricsStorePath()
	f, err := os.Open(path)
	if err != nil {
		if !os.IsNotExist(err) {
			logger.Warning("restore system metrics failed:", err)
		}
		return
	}
	var data map[string]persistedSeries
	decErr := gob.NewDecoder(f).Decode(&data)
	f.Close()
	if decErr == nil {
		systemMetrics.restore(data)
		return
	}
	if migrateLegacySystemMetrics(path) {
		return
	}
	logger.Warning("decode system metrics failed:", decErr)
}

// migrateLegacySystemMetrics loads a pre-tier flat snapshot
// (map[string][]MetricSample) and replays it through append so the new tiers are
// seeded from the existing history instead of starting empty.
func migrateLegacySystemMetrics(path string) bool {
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer f.Close()
	var legacy map[string][]MetricSample
	if err := gob.NewDecoder(f).Decode(&legacy); err != nil {
		return false
	}
	for metric, samples := range legacy {
		for _, p := range samples {
			systemMetrics.append(metric, time.Unix(p.T, 0), p.V)
		}
	}
	return true
}
