module github.com/open-telemetry/opentelemetry-collector-contrib/receiver/syslogreceiver

go 1.14

require (
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/stanza v0.0.0-00010101000000-000000000000
	github.com/open-telemetry/opentelemetry-log-collection v0.16.0
	go.opentelemetry.io/collector v0.21.1-0.20210308033310-65c4c4a1b383
	gopkg.in/yaml.v2 v2.4.0
)

replace github.com/open-telemetry/opentelemetry-collector-contrib/internal/stanza => ../../internal/stanza
