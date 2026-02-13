module github.com/open-telemetry/opentelemetry-collector-contrib/processor/metricsgenerationprocessor

go 1.25.0

require (
	github.com/open-telemetry/opentelemetry-collector-contrib/pkg/golden v0.145.0
	github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest v0.145.0
	github.com/stretchr/testify v1.11.1
	go.opentelemetry.io/collector/component v1.51.1-0.20260212100729-5a059d1d6718
	go.opentelemetry.io/collector/component/componenttest v0.145.1-0.20260212100729-5a059d1d6718
	go.opentelemetry.io/collector/confmap v1.51.1-0.20260212100729-5a059d1d6718
	go.opentelemetry.io/collector/confmap/xconfmap v0.145.1-0.20260212100729-5a059d1d6718
	go.opentelemetry.io/collector/consumer v1.51.1-0.20260212100729-5a059d1d6718
	go.opentelemetry.io/collector/consumer/consumertest v0.145.1-0.20260212100729-5a059d1d6718
	go.opentelemetry.io/collector/featuregate v1.51.1-0.20260212100729-5a059d1d6718
	go.opentelemetry.io/collector/pdata v1.51.1-0.20260212100729-5a059d1d6718
	go.opentelemetry.io/collector/processor v1.51.1-0.20260212100729-5a059d1d6718
	go.opentelemetry.io/collector/processor/processorhelper v0.145.1-0.20260212100729-5a059d1d6718
	go.opentelemetry.io/collector/processor/processortest v0.145.1-0.20260212100729-5a059d1d6718
	go.uber.org/goleak v1.3.0
	go.uber.org/zap v1.27.1
)

require (
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/go-logr/logr v1.4.3 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/go-viper/mapstructure/v2 v2.5.0 // indirect
	github.com/gobwas/glob v0.2.3 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/hashicorp/go-version v1.8.0 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/knadh/koanf/maps v0.1.2 // indirect
	github.com/knadh/koanf/providers/confmap v1.0.0 // indirect
	github.com/knadh/koanf/v2 v2.3.2 // indirect
	github.com/mitchellh/copystructure v1.2.0 // indirect
	github.com/mitchellh/reflectwalk v1.0.2 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.3-0.20250322232337-35a7c28c31ee // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatautil v0.145.0 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	go.opentelemetry.io/auto/sdk v1.2.1 // indirect
	go.opentelemetry.io/collector/component/componentstatus v0.145.1-0.20260212100729-5a059d1d6718 // indirect
	go.opentelemetry.io/collector/consumer/xconsumer v0.145.1-0.20260212100729-5a059d1d6718 // indirect
	go.opentelemetry.io/collector/internal/componentalias v0.145.1-0.20260212100729-5a059d1d6718 // indirect
	go.opentelemetry.io/collector/pdata/pprofile v0.145.1-0.20260212100729-5a059d1d6718 // indirect
	go.opentelemetry.io/collector/pdata/testdata v0.145.1-0.20260212100729-5a059d1d6718 // indirect
	go.opentelemetry.io/collector/pipeline v1.51.1-0.20260212100729-5a059d1d6718 // indirect
	go.opentelemetry.io/collector/processor/xprocessor v0.145.1-0.20260212100729-5a059d1d6718 // indirect
	go.opentelemetry.io/otel v1.40.0 // indirect
	go.opentelemetry.io/otel/metric v1.40.0 // indirect
	go.opentelemetry.io/otel/sdk v1.40.0 // indirect
	go.opentelemetry.io/otel/sdk/metric v1.40.0 // indirect
	go.opentelemetry.io/otel/trace v1.40.0 // indirect
	go.uber.org/multierr v1.11.0 // indirect
	go.yaml.in/yaml/v3 v3.0.4 // indirect
	golang.org/x/sys v0.40.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

retract (
	v0.76.2
	v0.76.1
	v0.65.0
)

replace github.com/open-telemetry/opentelemetry-collector-contrib/pkg/golden => ../../pkg/golden

replace github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest => ../../pkg/pdatatest

replace github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatautil => ../../pkg/pdatautil
