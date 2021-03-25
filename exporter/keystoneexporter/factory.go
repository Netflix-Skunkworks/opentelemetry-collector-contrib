package keystoneexporter

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configmodels"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
)

const (
	typeStr = "keystone"
)

func NewFactory() component.ExporterFactory {
	return exporterhelper.NewFactory(
		typeStr,
		createDefaultConfig,
		exporterhelper.WithMetrics(createMetricsExporter))
}

func createDefaultConfig() configmodels.Exporter {
	return &Config{
		ExporterSettings: configmodels.ExporterSettings{
			TypeVal: typeStr,
			NameVal: typeStr,
		},
		KeystoneUrlFormat: "ksgateway-titus-metrics-%s.prod.netflix.net",
	}
}

func createMetricsExporter(_ context.Context, params component.ExporterCreateParams, config configmodels.Exporter) (component.MetricsExporter, error) {
	return &keystoneExporter{
		log: params.Logger,
		config: config.(*Config),
	}, nil
}
