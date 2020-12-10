package simplespectatordexporter

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configcheck"
)

func TestCreateDefaultConfig(t *testing.T) {
	cfg := createDefaultConfig()
	assert.NotNil(t, cfg, "failed to create default config")
	assert.NoError(t, configcheck.ValidateConfig(cfg))
}

func TestCreateMetricsExporter(t *testing.T) {
	cfg := createDefaultConfig()
	exp, err := createMetricsExporter(
		context.Background(),
		component.ExporterCreateParams{Logger: zap.NewNop()},
		cfg)
	assert.Error(t, err)
	require.Nil(t, exp)
}
