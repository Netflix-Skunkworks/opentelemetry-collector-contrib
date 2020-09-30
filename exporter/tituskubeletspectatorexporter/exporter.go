// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package tituskubeletspectatorexporter

import (
	"context"
	"fmt"

	metricspb "github.com/census-instrumentation/opencensus-proto/gen-go/metrics/v1"

	"github.com/hashicorp/go-multierror"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer/pdata"
	"go.opentelemetry.io/collector/translator/internaldata"
	"go.uber.org/zap"
)

type spectatorExporter struct {
	log   *zap.Logger
	cache *SpectatorAdapaterCache
}

func (e *spectatorExporter) ConsumeMetrics(_ context.Context, md pdata.Metrics) error {
	var result *multierror.Error

	ocmds := internaldata.MetricsToOC(md)
	for _, ocmd := range ocmds {
		for _, metric := range ocmd.Metrics {
			err := e.updateMetric(metric)
			if err != nil {
				result = multierror.Append(result, err)
			}
		}
	}

	if result != nil {
		return result.ErrorOrNil()
	} else {
		return nil
	}
}

func (e *spectatorExporter) Start(ctx context.Context, host component.Host) error {
	return e.cache.Start()
}

// Shutdown stops the exporter and is invoked during shutdown.
func (e *spectatorExporter) Shutdown(context.Context) error {
	return e.cache.Shutdown()
}

func (e *spectatorExporter) updateMetric(m *metricspb.Metric) error {
	var result *multierror.Error

	descriptor := m.GetMetricDescriptor()
	for _, series := range m.Timeseries {
		err := e.updateTimeSeries(descriptor, series)
		if err != nil {
			result = multierror.Append(result, err)
		}
	}

	if result != nil {
		return result.ErrorOrNil()
	} else {
		return nil
	}
}

func (e *spectatorExporter) updateTimeSeries(descriptor *metricspb.MetricDescriptor, series *metricspb.TimeSeries) error {
	podName, err := e.getPodName(descriptor, series)
	if err != nil {
		return err
	}

	adapter, err := e.cache.GetAdapter(podName)
	if err != nil {
		return err
	}

	return adapter.UpdateTimeSeries(descriptor, series)
}

func (e *spectatorExporter) getPodName(descriptor *metricspb.MetricDescriptor, series *metricspb.TimeSeries) (string, error) {
	labelKeys := descriptor.GetLabelKeys()
	labelValues := series.GetLabelValues()
	for i, key := range labelKeys {
		if key.GetKey() == "pod" && labelValues[i].HasValue {
			return labelValues[i].GetValue(), nil
		}
	}

	return "", fmt.Errorf("failed to find pod label for metric: %s", descriptor.Name)
}
