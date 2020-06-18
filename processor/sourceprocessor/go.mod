module github.com/open-telemetry/opentelemetry-collector-contrib/processor/sourceprocessor

go 1.14

require (
	github.com/stretchr/testify v1.5.1
	go.opencensus.io v0.22.3
	go.opentelemetry.io/collector v0.4.0
	go.uber.org/zap v1.13.0 // indirect
)

replace go.opentelemetry.io/collector => github.com/SumoLogic/opentelemetry-collector v0.2.7-0.20200618143706-98e95f47a6d5
