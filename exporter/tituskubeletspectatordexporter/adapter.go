package tituskubeletspectatordexporter

import (
	"context"
	"fmt"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/spectatord"
	"go.uber.org/zap"
)

func NewSpectatorAdapter(log *zap.Logger, podName string) (*spectatord.Adapter, error) {
	netNsPath, err := GetNetworkNamespacePath(podName)
	if err != nil {
		log.Error(fmt.Sprintf("failed to get network namespace path: %v", err))
		return nil, err
	}

	nsDialer, err := NewNsDialer(netNsPath)
	if err != nil{
		log.Error(fmt.Sprintf("failed to construct namespace dialer: %v", err))
		return nil, err
	}

	conn, err := nsDialer.DialContext(context.Background(), "udp", "localhost:1234")
	if err != nil {
		log.Error(fmt.Sprintf("failed to construct namespaced connection: %v", err))
		return nil, err
	}

	return spectatord.NewAdapter(log, conn)
}
