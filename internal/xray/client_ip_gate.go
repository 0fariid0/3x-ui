package xray

import (
	"context"
	"fmt"
	"google.golang.org/protobuf/types/known/structpb"
)

// ClientIPControl is available only in the bundled Fara core. Never silently
// substitute a host-wide firewall ban when this capability is unavailable.
func (x *XrayAPI) ClientIPControl(ctx context.Context, action, email, ip string, seconds int) (map[string]any, error) {
	if x.grpcClient == nil {
		return nil, fmt.Errorf("Xray API unavailable")
	}
	req, err := structpb.NewStruct(map[string]any{"action": action, "email": email, "ip": ip, "seconds": seconds})
	if err != nil {
		return nil, err
	}
	result := new(structpb.Struct)
	if err = x.grpcClient.Invoke(ctx, "/fara.core.ClientIP/Control", req, result); err != nil {
		return nil, err
	}
	return result.AsMap(), nil
}
