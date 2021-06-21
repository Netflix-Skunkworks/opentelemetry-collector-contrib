module github.com/open-telemetry/opentelemetry-collector-contrib/exporter/stackdriverexporter

go 1.14

require (
	contrib.go.opencensus.io/exporter/stackdriver v0.13.4
	github.com/GoogleCloudPlatform/opentelemetry-operations-go/exporter/trace v0.10.1-0.20200828070728-ba72e89a4087
	github.com/census-instrumentation/opencensus-proto v0.3.0
	github.com/stretchr/testify v1.6.1
	go.opencensus.io v0.23.0
	go.opentelemetry.io/collector v0.10.1-0.20200922190504-eb2127131b29
	go.opentelemetry.io/otel v0.11.0
	go.opentelemetry.io/otel/sdk v0.11.0
	go.uber.org/zap v1.16.0
	google.golang.org/api v0.48.0
	google.golang.org/genproto v0.0.0-20210604141403-392c879c8b08
	google.golang.org/grpc v1.38.0
	google.golang.org/grpc/examples v0.0.0-20200728194956-1c32b02682df // indirect
	google.golang.org/protobuf v1.26.0
)
