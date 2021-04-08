module github.com/open-telemetry/opentelemetry-collector-contrib/receiver/syslogreceiver

go 1.14

require (
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/stanza v0.0.0-00010101000000-000000000000
	github.com/open-telemetry/opentelemetry-log-collection v0.17.0
	github.com/stretchr/testify v1.7.0
	go.opentelemetry.io/collector v0.24.0
	go.uber.org/zap v1.16.0
	gopkg.in/yaml.v2 v2.4.0
)

replace github.com/open-telemetry/opentelemetry-collector-contrib/internal/stanza => ../../internal/stanza
