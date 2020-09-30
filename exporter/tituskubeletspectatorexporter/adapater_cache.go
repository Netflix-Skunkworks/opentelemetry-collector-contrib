package tituskubeletspectatorexporter

import (
	"fmt"
	"sync"
	"time"

	"github.com/hashicorp/go-multierror"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/spectatord"
	"go.uber.org/zap"
)

type SpectatorAdapaterCache struct {
	log   *zap.Logger
	cache sync.Map
	mutex sync.Mutex
	quit  chan struct{}
}

func NewSpectatorAdapaterCache(log *zap.Logger) *SpectatorAdapaterCache {
	return &SpectatorAdapaterCache{
		log: log,
	}
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

func (s *SpectatorAdapaterCache) Start() error {

	// Instead of trying to track the last accessed time of every adapter
	// we clear the cache every 10 minutes.  Construction of new adapters
	// on demand is not that expensive.
	ticker := time.NewTicker(1 * time.Minute)
	go func() {
		for {
			select {
			case <-ticker.C:
				s.clearCache()
			case <-s.quit:
				s.log.Info("stopping cache clearing ticker...")
				ticker.Stop()
				s.log.Info("stopped cache clearing ticker")
				return
			}
		}
	}()

	return nil
}

func (s *SpectatorAdapaterCache) Shutdown() error {
	s.log.Info("shutting down spectatord adapater cache...")
	var result *multierror.Error

	s.cache.Range(func(key, value interface{}) bool {
		if err := value.(*spectatord.Adapter).Shutdown(); err != nil {
			result = multierror.Append(result, err)
		}
		return true
	})

	if s.quit != nil {
		close(s.quit)
	}

	s.log.Info("shut down spectatord adapater cache")
	if result != nil {
		return result.ErrorOrNil()
	} else {
		return nil
	}
}

func (s *SpectatorAdapaterCache) clearCache() {
	s.log.Info("clearing spectatord adapter cache...")
	s.cache.Range(func(key interface{}, value interface{}) bool {
		s.cache.Delete(key)
		return true
	})
	s.log.Info("cleared spectatord adapter cache...")
}
