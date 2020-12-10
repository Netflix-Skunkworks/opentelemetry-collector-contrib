package simplespectatordexporter

import (
	"context"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/spectatord"

	metricspb "github.com/census-instrumentation/opencensus-proto/gen-go/metrics/v1"

	"github.com/hashicorp/go-multierror"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer/pdata"
	"go.opentelemetry.io/collector/translator/internaldata"
	"go.uber.org/zap"
)

type spectatorExporter struct {
	log          *zap.Logger
	specAdapater *spectatord.Adapter
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
	return nil
}

func (e *spectatorExporter) Shutdown(context.Context) error {
	return nil
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
	e.log.Debug("Pushing metric to spectatord", zap.String("name", descriptor.Name))
	return e.specAdapater.UpdateTimeSeries(descriptor, series)
}
