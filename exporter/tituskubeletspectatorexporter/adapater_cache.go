package tituskubeletspectatorexporter

import (
	"fmt"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/spectatord"
	"go.uber.org/zap"
	"sync"
)

type SpectatorAdapaterCache struct {
	log   *zap.Logger
	cache sync.Map
}

func (s *SpectatorAdapaterCache) GetAdapter(podName string) (*spectatord.Adapter, error) {
	if val, ok := s.cache.Load(podName); ok {
		return val.(*spectatord.Adapter), nil
	} else {
		adapter, err := NewSpectatorAdapter(s.log, podName)
		if err != nil {
			s.log.Error(fmt.Sprintf("failed to construct SpectatorAdapter", err))
			return nil, err
		}

		s.cache.Store(podName, adapter)
		return adapter, nil
	}
}

func (s *SpectatorAdapaterCache) Shutdown() error {
	s.cache.Range(func(key, value interface{}) bool {
		value.(*spectatord.Adapter).Shutdown()
		return true
	})

	return nil
}
