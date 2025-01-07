module github.com/open-telemetry/opentelemetry-collector-contrib/cmd/githubgen

go 1.22.0

require (
	github.com/google/go-github/v68 v68.0.0
	go.opentelemetry.io/collector/confmap v1.22.1-0.20250107062214-ced38e8af2ae
	go.opentelemetry.io/collector/confmap/provider/fileprovider v1.22.1-0.20250107062214-ced38e8af2ae
	gopkg.in/yaml.v3 v3.0.1
)

require (
	github.com/go-viper/mapstructure/v2 v2.2.1 // indirect
	github.com/google/go-querystring v1.1.0 // indirect
	github.com/knadh/koanf/maps v0.1.1 // indirect
	github.com/knadh/koanf/providers/confmap v0.1.0 // indirect
	github.com/knadh/koanf/v2 v2.1.2 // indirect
	github.com/mitchellh/copystructure v1.2.0 // indirect
	github.com/mitchellh/reflectwalk v1.0.2 // indirect
	go.uber.org/multierr v1.11.0 // indirect
	go.uber.org/zap v1.27.0 // indirect
)

replace go.opentelemetry.io/collector/scraper/scraperhelper v0.116.0 => go.opentelemetry.io/collector/scraper/scraperhelper v0.0.0-20250107062214-ced38e8af2ae

replace go.opentelemetry.io/collector/extension/xextension v0.116.0 => go.opentelemetry.io/collector/extension/xextension v0.0.0-20250107062214-ced38e8af2ae
