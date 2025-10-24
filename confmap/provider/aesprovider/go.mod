module github.com/open-telemetry/opentelemetry-collector-contrib/confmap/provider/aesprovider

go 1.24.0

require (
	github.com/stretchr/testify v1.11.1
	go.opentelemetry.io/collector/confmap v1.44.1-0.20251021231522-c657d5d4e920
	go.uber.org/goleak v1.3.0
	go.uber.org/zap v1.27.0
)

require (
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/go-viper/mapstructure/v2 v2.4.0 // indirect
	github.com/gobwas/glob v0.2.3 // indirect
	github.com/hashicorp/go-version v1.7.0 // indirect
	github.com/knadh/koanf/maps v0.1.2 // indirect
	github.com/knadh/koanf/providers/confmap v1.0.0 // indirect
	github.com/knadh/koanf/v2 v2.3.0 // indirect
	github.com/mitchellh/copystructure v1.2.0 // indirect
	github.com/mitchellh/reflectwalk v1.0.2 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	go.opentelemetry.io/collector/featuregate v1.44.1-0.20251021231522-c657d5d4e920 // indirect
	go.uber.org/multierr v1.11.0 // indirect
	go.yaml.in/yaml/v3 v3.0.4 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

replace go.opentelemetry.io/collector/config/configopaque => github.com/jade-guiton-dd/opentelemetry-collector/config/configopaque v0.0.0-20251023145606-417796e61f83

replace go.opentelemetry.io/collector/config/configgrpc => github.com/jade-guiton-dd/opentelemetry-collector/config/configgrpc v0.0.0-20251023145606-417796e61f83

replace go.opentelemetry.io/collector/config/confighttp => github.com/jade-guiton-dd/opentelemetry-collector/config/confighttp v0.0.0-20251023145606-417796e61f83

replace go.opentelemetry.io/collector/exporter/otlpexporter => github.com/jade-guiton-dd/opentelemetry-collector/exporter/otlpexporter v0.0.0-20251023145606-417796e61f83

replace go.opentelemetry.io/collector/exporter/otlphttpexporter => github.com/jade-guiton-dd/opentelemetry-collector/exporter/otlphttpexporter v0.0.0-20251023145606-417796e61f83
