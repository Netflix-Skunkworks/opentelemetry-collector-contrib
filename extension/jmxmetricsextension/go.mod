module github.com/open-telemetry/opentelemetry-collector-contrib/extension/jmxmetricsextension

go 1.14

require (
	github.com/stretchr/testify v1.7.0
	go.opentelemetry.io/collector v0.10.1-0.20200922190504-eb2127131b29
	go.uber.org/zap v1.18.1
)

replace github.com/open-telemetry/opentelemetry-collector-contrib/internal/common => ../../internal/common
