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
	insights service.ClientInsightService
}

func NewClientDestinationJob() *ClientDestinationJob { return &ClientDestinationJob{} }

func (j *ClientDestinationJob) Run() {
	if service.AnyDestinationTrackingEnabled() {
		service.RefreshDestinationNetworkRulesIfNeeded()
	}
	if err := j.insights.IngestDestinationAccessLog(); err != nil {
		logger.Debug("[ClientDestinations] ingest failed:", err)
	}
}
