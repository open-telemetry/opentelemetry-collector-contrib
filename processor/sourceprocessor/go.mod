module github.com/open-telemetry/opentelemetry-collector-contrib/processor/sourceprocessor

go 1.14

require (
	github.com/stretchr/testify v1.5.1
	go.opencensus.io v0.22.3
	go.opentelemetry.io/collector v0.4.0
)

replace go.opentelemetry.io/collector => github.com/SumoLogic/opentelemetry-collector v0.2.7-0.20200709094216-57486cde1244
