// Package job provides background job implementations for the 3x-ui web panel,
// including traffic monitoring, system checks, and periodic maintenance tasks.
package job

import (
	"sync"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/eventbus"
	"github.com/mhsanaei/3x-ui/v3/internal/logger"
	"github.com/mhsanaei/3x-ui/v3/internal/web/service"
)

// EventBus is set from web layer to publish events.
var EventBus *eventbus.Bus

type xrayHealthPolicy struct {
	enabled          bool
	failureThreshold int
	cooldown         time.Duration
	maxRestarts      int
	window           time.Duration
}

func defaultXrayHealthPolicy() xrayHealthPolicy {
	return xrayHealthPolicy{
		enabled:          true,
		failureThreshold: 2,
		cooldown:         5 * time.Minute,
		maxRestarts:      3,
		window:           30 * time.Minute,
	}
}

type xrayHealthState struct {
	consecutiveFailures int
	restartAttempts     []time.Time
	cooldownUntil       time.Time
	blockedUntil        time.Time
}

// shouldRestart observes one health result and decides whether an automatic
// restart is allowed. Restart attempts are rate-limited by both a cooldown and
// a rolling-window circuit breaker, preventing a broken config from creating a
// permanent restart loop.
func (s *xrayHealthState) shouldRestart(now time.Time, crashed bool, policy xrayHealthPolicy) (bool, string) {
	if !policy.enabled || !crashed {
		s.consecutiveFailures = 0
		if !policy.enabled {
			s.restartAttempts = nil
			s.cooldownUntil = time.Time{}
			s.blockedUntil = time.Time{}
		}
		return false, ""
	}

	s.consecutiveFailures++
	if s.consecutiveFailures < policy.failureThreshold {
		return false, "waiting for consecutive failure confirmation"
	}
	s.consecutiveFailures = 0

	if now.Before(s.blockedUntil) {
		return false, "restart circuit breaker is open"
	}
	if now.Before(s.cooldownUntil) {
		return false, "restart cooldown is active"
	}

	cutoff := now.Add(-policy.window)
	kept := s.restartAttempts[:0]
	for _, attemptedAt := range s.restartAttempts {
		if attemptedAt.After(cutoff) || attemptedAt.Equal(cutoff) {
			kept = append(kept, attemptedAt)
		}
	}
	s.restartAttempts = kept
	if len(s.restartAttempts) >= policy.maxRestarts {
		oldest := s.restartAttempts[0]
		s.blockedUntil = oldest.Add(policy.window)
		if !s.blockedUntil.After(now) {
			s.blockedUntil = now.Add(policy.cooldown)
		}
		return false, "maximum restart attempts reached"
	}

	s.restartAttempts = append(s.restartAttempts, now)
	s.cooldownUntil = now.Add(policy.cooldown)
	return true, ""
}

// CheckXrayRunningJob monitors Xray process health and restarts it only after a
// confirmed crash. Policy is refreshed periodically from panel settings.
type CheckXrayRunningJob struct {
	mu             sync.Mutex
	xrayService    *service.XrayService
	settingService *service.SettingService
	state          xrayHealthState
	policy         xrayHealthPolicy
	policyLoadedAt time.Time
	lastSkipReason string
}

// NewCheckXrayRunningJob creates a new Xray health check job instance.
func NewCheckXrayRunningJob(xrayService *service.XrayService, settingService *service.SettingService) *CheckXrayRunningJob {
	return &CheckXrayRunningJob{
		xrayService:    xrayService,
		settingService: settingService,
		policy:         defaultXrayHealthPolicy(),
	}
}

func (j *CheckXrayRunningJob) loadPolicy(now time.Time) xrayHealthPolicy {
	if !j.policyLoadedAt.IsZero() && now.Sub(j.policyLoadedAt) < 10*time.Second {
		return j.policy
	}
	j.policyLoadedAt = now
	settings, err := j.settingService.GetAllSetting()
	if err != nil || settings == nil {
		if err != nil {
			logger.Warning("xray health monitor: unable to read settings:", err)
		}
		return j.policy
	}
	policy := defaultXrayHealthPolicy()
	policy.enabled = settings.XrayHealthEnable
	if settings.XrayHealthFailureThreshold > 0 {
		policy.failureThreshold = settings.XrayHealthFailureThreshold
	}
	if settings.XrayHealthRestartCooldown > 0 {
		policy.cooldown = time.Duration(settings.XrayHealthRestartCooldown) * time.Minute
	}
	if settings.XrayHealthMaxRestarts > 0 {
		policy.maxRestarts = settings.XrayHealthMaxRestarts
	}
	if settings.XrayHealthWindowMinutes > 0 {
		policy.window = time.Duration(settings.XrayHealthWindowMinutes) * time.Minute
	}
	j.policy = policy
	return policy
}

// Run checks whether Xray actually crashed. Manual stops do not count as a
// crash because DidXrayCrash excludes them.
func (j *CheckXrayRunningJob) Run() {
	j.mu.Lock()
	defer j.mu.Unlock()

	now := time.Now()
	policy := j.loadPolicy(now)
	shouldRestart, reason := j.state.shouldRestart(now, j.xrayService.DidXrayCrash(), policy)
	if !shouldRestart {
		if reason != "" && reason != "waiting for consecutive failure confirmation" && reason != j.lastSkipReason {
			logger.Warning("xray health monitor:", reason)
		}
		j.lastSkipReason = reason
		return
	}
	j.lastSkipReason = ""
	logger.Warning("xray health monitor: confirmed crash; attempting automatic restart")
	if err := j.xrayService.RestartXray(false); err != nil {
		logger.Error("xray health monitor: automatic restart failed:", err)
		return
	}
	logger.Info("xray health monitor: Xray restarted successfully")
}
