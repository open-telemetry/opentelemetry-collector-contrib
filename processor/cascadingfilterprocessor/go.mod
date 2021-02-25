module github.com/open-telemetry/opentelemetry-collector-contrib/processor/cascadingfilterprocessor

go 1.14

require (
	github.com/google/uuid v1.2.0
	github.com/stretchr/testify v1.7.0
	go.opencensus.io v0.22.6
	go.opentelemetry.io/collector v0.19.0
	go.uber.org/zap v1.16.0
)

replace go.opentelemetry.io/collector => github.com/SumoLogic/opentelemetry-collector v0.21.0-sumo
