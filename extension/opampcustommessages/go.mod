module github.com/open-telemetry/opentelemetry-collector-contrib/extension/opampcustommessages

go 1.24.0

require github.com/open-telemetry/opamp-go v0.22.0

require google.golang.org/protobuf v1.36.7 // indirect

replace go.opentelemetry.io/collector/config/configopaque => github.com/jade-guiton-dd/opentelemetry-collector/config/configopaque v0.0.0-20251023145606-417796e61f83

replace go.opentelemetry.io/collector/config/configgrpc => github.com/jade-guiton-dd/opentelemetry-collector/config/configgrpc v0.0.0-20251023145606-417796e61f83

replace go.opentelemetry.io/collector/config/confighttp => github.com/jade-guiton-dd/opentelemetry-collector/config/confighttp v0.0.0-20251023145606-417796e61f83

replace go.opentelemetry.io/collector/exporter/otlpexporter => github.com/jade-guiton-dd/opentelemetry-collector/exporter/otlpexporter v0.0.0-20251023145606-417796e61f83

replace go.opentelemetry.io/collector/exporter/otlphttpexporter => github.com/jade-guiton-dd/opentelemetry-collector/exporter/otlphttpexporter v0.0.0-20251023145606-417796e61f83
