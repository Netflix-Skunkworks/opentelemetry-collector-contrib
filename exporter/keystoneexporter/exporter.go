package keystoneexporter

import (
	"context"
	metricspb "github.com/census-instrumentation/opencensus-proto/gen-go/metrics/v1"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer/pdata"
	"go.opentelemetry.io/collector/translator/internaldata"
	"go.uber.org/zap"
)

type keystoneExporter struct {
	log *zap.Logger
}

func (k keystoneExporter) Start(ctx context.Context, host component.Host) error {
	return nil
}

func (k keystoneExporter) Shutdown(ctx context.Context) error {
	return nil
}

func (k keystoneExporter) ConsumeMetrics(ctx context.Context, md pdata.Metrics) error {
	return k.publishMessages(md)
}

func (k keystoneExporter) publishMessages(md pdata.Metrics) (error) {
	events := make([]KsEvent, 0)

	ocmds := internaldata.MetricsToOC(md)
	k.log.Debug("constructing keystone message for metricsdata objects", zap.Int("count", len(ocmds)))
	for _, ocmd := range ocmds {
		k.log.Debug("constructing keystone message for metrics", zap.Int("count", len(ocmd.Metrics)))
		for _, metric := range ocmd.Metrics {
			if metric.MetricDescriptor == nil {
				k.log.Error("no metric descriptor present")
				continue
			}

			if !isValidMetricType(metric.MetricDescriptor.Type) {
				k.log.Error("invalid type", zap.String("metric", metric.MetricDescriptor.Name), zap.String("type", metric.MetricDescriptor.Type.String()))
				continue
			}

			if evts, err := GetEvents(metric); err == nil {
				events = append(events, evts...)
			} else {
				k.log.Error("failed to construct event from metric", zap.String("name", metric.GetMetricDescriptor().Name))
			}
		}
	}

	return PublishMessages(events)
}

func isValidMetricType(metricType metricspb.MetricDescriptor_Type) bool {
	switch metricType {
	case
		metricspb.MetricDescriptor_CUMULATIVE_DOUBLE,
		metricspb.MetricDescriptor_GAUGE_DOUBLE:
		return true
	default:
		return false
	}
}
