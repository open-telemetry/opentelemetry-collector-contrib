module github.com/open-telemetry/opentelemetry-collector-contrib/internal/stanza

go 1.15

require (
	github.com/open-telemetry/opentelemetry-collector-contrib/extension/storage v0.0.0-00010101000000-000000000000
	github.com/open-telemetry/opentelemetry-log-collection v0.17.1-0.20210409145101-a881ed8b0592
	github.com/spf13/viper v1.7.1 // indirect
	github.com/stretchr/testify v1.7.0
	go.opentelemetry.io/collector v0.25.1-0.20210504213219-970b76cc794a
	go.uber.org/multierr v1.6.0
	go.uber.org/zap v1.16.0
	gopkg.in/yaml.v2 v2.4.0
)

replace github.com/open-telemetry/opentelemetry-collector-contrib/extension/storage => ../../extension/storage

// WIP update for otelcol changes
replace go.opentelemetry.io/collector => github.com/pmatyjasek-sumo/opentelemetry-collector v0.25.1-0.20210428081312-72ef9d6ccfe5
