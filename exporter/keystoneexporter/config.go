package keystoneexporter

import (
	"go.opentelemetry.io/collector/config/configmodels"
)

// Config defines configuration for logging exporter.
type Config struct {
	configmodels.ExporterSettings `mapstructure:",squash"` // squash ensures fields are correctly decoded in embedded struct.
}
