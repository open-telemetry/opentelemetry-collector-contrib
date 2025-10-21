module github.com/open-telemetry/opentelemetry-collector-contrib/internal/grpcutil

go 1.24.0

require github.com/stretchr/testify v1.11.1

require (
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/kr/pretty v0.3.1 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/rogpeppe/go-internal v1.10.0 // indirect
	gopkg.in/check.v1 v1.0.0-20201130134442-10cb98267c6c // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

replace go.opentelemetry.io/collector/config/configopaque => github.com/jade-guiton-dd/opentelemetry-collector/config/configopaque v0.0.0-20251023145606-417796e61f83

replace go.opentelemetry.io/collector/config/configgrpc => github.com/jade-guiton-dd/opentelemetry-collector/config/configgrpc v0.0.0-20251023145606-417796e61f83

replace go.opentelemetry.io/collector/config/confighttp => github.com/jade-guiton-dd/opentelemetry-collector/config/confighttp v0.0.0-20251023145606-417796e61f83

replace go.opentelemetry.io/collector/exporter/otlpexporter => github.com/jade-guiton-dd/opentelemetry-collector/exporter/otlpexporter v0.0.0-20251023145606-417796e61f83

replace go.opentelemetry.io/collector/exporter/otlphttpexporter => github.com/jade-guiton-dd/opentelemetry-collector/exporter/otlphttpexporter v0.0.0-20251023145606-417796e61f83
