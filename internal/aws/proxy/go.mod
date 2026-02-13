module github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/proxy

go 1.25.0

require (
	github.com/aws/aws-sdk-go v1.55.8
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/common v0.145.0
	github.com/stretchr/testify v1.11.1
	go.opentelemetry.io/collector/config/confignet v1.51.1-0.20260212100729-5a059d1d6718
	go.opentelemetry.io/collector/config/configtls v1.51.1-0.20260212100729-5a059d1d6718
	go.uber.org/zap v1.27.1
)

require (
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/foxboron/go-tpm-keyfiles v0.0.0-20250903184740-5d135037bd4d // indirect
	github.com/fsnotify/fsnotify v1.9.0 // indirect
	github.com/go-viper/mapstructure/v2 v2.5.0 // indirect
	github.com/gobwas/glob v0.2.3 // indirect
	github.com/google/go-tpm v0.9.8 // indirect
	github.com/hashicorp/go-version v1.8.0 // indirect
	github.com/jmespath/go-jmespath v0.4.0 // indirect
	github.com/knadh/koanf/maps v0.1.2 // indirect
	github.com/knadh/koanf/providers/confmap v1.0.0 // indirect
	github.com/knadh/koanf/v2 v2.3.2 // indirect
	github.com/mitchellh/copystructure v1.2.0 // indirect
	github.com/mitchellh/reflectwalk v1.0.2 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	go.opentelemetry.io/collector/config/configopaque v1.51.1-0.20260212100729-5a059d1d6718 // indirect
	go.opentelemetry.io/collector/confmap v1.51.1-0.20260212100729-5a059d1d6718 // indirect
	go.opentelemetry.io/collector/confmap/xconfmap v0.145.1-0.20260212100729-5a059d1d6718 // indirect
	go.opentelemetry.io/collector/featuregate v1.51.1-0.20260212100729-5a059d1d6718 // indirect
	go.uber.org/multierr v1.11.0 // indirect
	go.yaml.in/yaml/v3 v3.0.4 // indirect
	golang.org/x/crypto v0.45.0 // indirect
	golang.org/x/sys v0.38.0 // indirect
	gopkg.in/yaml.v2 v2.4.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

replace github.com/open-telemetry/opentelemetry-collector-contrib/internal/common => ../../../internal/common

retract (
	v0.76.2
	v0.76.1
	v0.65.0
)
