package simplespectatordexporter

import (
	"context"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/spectatord"
	"net"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configmodels"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
)

const (
	typeStr = "spectatord"
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
	}
}

func createMetricsExporter(
	_ context.Context,
	params component.ExporterCreateParams,
	cfg configmodels.Exporter,
) (component.MetricsExporter, error) {
	conn, err := net.Dial("udp", "localhost:1234")
	if err != nil{
		return nil, err
	}

	adapter, err := spectatord.NewAdapter(params.Logger, conn)
	if err != nil {
		return nil, err
	}

	return &spectatorExporter{
		log:          params.Logger,
		specAdapater: adapter,
	}, nil
}
