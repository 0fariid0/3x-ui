package job

import (
	"github.com/mhsanaei/3x-ui/v3/internal/logger"
	"github.com/mhsanaei/3x-ui/v3/internal/web/service"
)

// ClientDestinationJob ingests privacy-preserving destination aggregates for
// clients that individually opted in. The underlying access log is already
// size-bounded by PruneXrayLogsJob and wiped daily.
type ClientDestinationJob struct {
	insights service.ClientInsightService
}

func NewClientDestinationJob() *ClientDestinationJob { return &ClientDestinationJob{} }

func (j *ClientDestinationJob) Run() {
	if err := j.insights.IngestDestinationAccessLog(); err != nil {
		logger.Debug("[ClientDestinations] ingest failed:", err)
	}
}
