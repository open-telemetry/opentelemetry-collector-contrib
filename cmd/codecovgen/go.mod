module github.com/open-telemetry/opentelemetry-collector-contrib/cmd/codecovgen

go 1.24.0

require (
	github.com/bmatcuk/doublestar/v4 v4.9.1
	go.yaml.in/yaml/v3 v3.0.4
	golang.org/x/mod v0.28.0
)

replace go.opentelemetry.io/collector/config/configopaque => github.com/jade-guiton-dd/opentelemetry-collector/config/configopaque v0.0.0-20251023145606-417796e61f83

replace go.opentelemetry.io/collector/config/configgrpc => github.com/jade-guiton-dd/opentelemetry-collector/config/configgrpc v0.0.0-20251023145606-417796e61f83

replace go.opentelemetry.io/collector/config/confighttp => github.com/jade-guiton-dd/opentelemetry-collector/config/confighttp v0.0.0-20251023145606-417796e61f83

replace go.opentelemetry.io/collector/exporter/otlpexporter => github.com/jade-guiton-dd/opentelemetry-collector/exporter/otlpexporter v0.0.0-20251023145606-417796e61f83

replace go.opentelemetry.io/collector/exporter/otlphttpexporter => github.com/jade-guiton-dd/opentelemetry-collector/exporter/otlphttpexporter v0.0.0-20251023145606-417796e61f83
