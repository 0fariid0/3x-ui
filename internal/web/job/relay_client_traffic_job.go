package job

import (
	"github.com/mhsanaei/3x-ui/v3/internal/logger"
	"github.com/mhsanaei/3x-ui/v3/internal/web/service"
	"github.com/mhsanaei/3x-ui/v3/internal/web/websocket"
	"sync"
)

// RelayClientTrafficJob keeps the main clients list current without scanning
// every account, logging connections or running the normal traffic pipeline.
type RelayClientTrafficJob struct {
	inbound service.InboundService
}

// A web-only reload can leave the same Xray process running. Keep its
// baseline so totals already saved by the old job are not counted twice.
var relayClientSampler = struct {
	sync.Mutex
	service.RelayClientSampler
}{}

func (j *RelayClientTrafficJob) Run() {
	relayClientSampler.Lock()
	defer relayClientSampler.Unlock()
	emails, ok, err := relayClientSampler.Sample(j.inbound.RecordRelayClientTraffic)
	if err != nil {
		logger.Debug("relay client statistics:", err)
	}
	if ok {
		j.inbound.RefreshLocalOnlineClients(emails, nil)
	}
	if websocket.HasClients() {
		online := j.inbound.GetOnlineClients()
		if online == nil {
			online = []string{}
		}
		websocket.BroadcastTraffic(map[string]any{"onlineClients": online})
	}
}
