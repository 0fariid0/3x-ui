package job

import (
	"github.com/mhsanaei/3x-ui/v3/internal/logger"
	"github.com/mhsanaei/3x-ui/v3/internal/web/service"
)

// ClientDestinationJob tails Xray access records for two purposes: transient
// exact email -> inbound attribution for every client, and privacy-preserving
// destination aggregates only for clients that individually opted in. The
// generated runtime access log is truncated after complete ingestion.
type ClientDestinationJob struct {
	insights       service.ClientInsightService
	relayLightMode bool
}

func NewClientDestinationJob(relayLightMode bool) *ClientDestinationJob {
	return &ClientDestinationJob{relayLightMode: relayLightMode}
}

func (j *ClientDestinationJob) Run() {
	if !j.relayLightMode && service.AnyDestinationTrackingEnabled() {
		service.RefreshDestinationNetworkRulesIfNeeded()
	}
	if err := j.insights.IngestDestinationAccessLogWithHistory(!j.relayLightMode); err != nil {
		logger.Debug("[ClientDestinations] ingest failed:", err)
	}
}
