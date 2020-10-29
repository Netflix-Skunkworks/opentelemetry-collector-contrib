package keystoneexporter

import (
	"context"

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
	msg, err := k.getMessage(md)
	if err != nil {
		k.log.Error("failed to construct message", zap.Error(err))
		return err
	}

	err = PublishMessage(msg)
	if err != nil {
		k.log.Error("failed to publish message", zap.Error(err))
		return err
	}

	return nil
}

func (k keystoneExporter) getMessage(md pdata.Metrics) (KsMessage, error) {
	events := make([]KsEvent, 0)

	ocmds := internaldata.MetricsToOC(md)
	k.log.Debug("constructing keystone message for metricsdata objects", zap.Int("count", len(ocmds)))
	for _, ocmd := range ocmds {
		k.log.Debug("constructing keystone message for metrics", zap.Int("count", len(ocmd.Metrics)))
		for _, metric := range ocmd.Metrics {
			if event, err := GetEvent(metric); err == nil {
				events = append(events, event)
			} else {
				k.log.Error("failed to construct event from metric", zap.String("name", metric.GetMetricDescriptor().Name))
			}
		}
	}

	return GetMessage(events)
}
