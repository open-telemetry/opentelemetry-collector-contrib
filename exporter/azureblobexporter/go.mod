module github.com/open-telemetry/opentelemetry-collector-contrib/exporter/azureblobexporter

go 1.25.0

require (
	github.com/Azure/azure-sdk-for-go/sdk/azcore v1.21.0
	github.com/Azure/azure-sdk-for-go/sdk/azidentity v1.13.1
	github.com/Azure/azure-sdk-for-go/sdk/storage/azblob v1.6.4
	github.com/stretchr/testify v1.11.1
	go.opentelemetry.io/collector/component v1.51.1-0.20260212100729-5a059d1d6718
	go.opentelemetry.io/collector/component/componenttest v0.145.1-0.20260212100729-5a059d1d6718
	go.opentelemetry.io/collector/confmap v1.51.1-0.20260212100729-5a059d1d6718
	go.opentelemetry.io/collector/confmap/xconfmap v0.145.1-0.20260212100729-5a059d1d6718
	go.opentelemetry.io/collector/consumer v1.51.1-0.20260212100729-5a059d1d6718
	go.opentelemetry.io/collector/exporter v1.51.1-0.20260212100729-5a059d1d6718
	go.opentelemetry.io/collector/exporter/exporterhelper v0.145.1-0.20260212100729-5a059d1d6718
	go.opentelemetry.io/collector/exporter/exportertest v0.145.1-0.20260212100729-5a059d1d6718
	go.opentelemetry.io/collector/pdata v1.51.1-0.20260212100729-5a059d1d6718
	go.opentelemetry.io/collector/pipeline v1.51.1-0.20260212100729-5a059d1d6718
	go.uber.org/zap v1.27.1
)

require (
	github.com/cenkalti/backoff/v5 v5.0.3 // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/gobwas/glob v0.2.3 // indirect
	github.com/stretchr/objx v0.5.2 // indirect
	go.opentelemetry.io/collector/client v1.51.1-0.20260212100729-5a059d1d6718 // indirect
	go.opentelemetry.io/collector/config/configoptional v1.51.1-0.20260212100729-5a059d1d6718 // indirect
	go.opentelemetry.io/collector/internal/componentalias v0.145.1-0.20260212100729-5a059d1d6718 // indirect
	go.opentelemetry.io/collector/pdata/xpdata v0.145.1-0.20260212100729-5a059d1d6718 // indirect
	go.opentelemetry.io/collector/pipeline/xpipeline v0.145.1-0.20260212100729-5a059d1d6718 // indirect
	go.yaml.in/yaml/v3 v3.0.4 // indirect
)

require (
	github.com/Azure/azure-sdk-for-go/sdk/internal v1.11.2 // indirect
	github.com/AzureAD/microsoft-authentication-library-for-go v1.6.0 // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/go-logr/logr v1.4.3 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/go-viper/mapstructure/v2 v2.5.0 // indirect
	github.com/golang-jwt/jwt/v5 v5.3.0 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/hashicorp/go-version v1.8.0 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/knadh/koanf/maps v0.1.2 // indirect
	github.com/knadh/koanf/providers/confmap v1.0.0 // indirect
	github.com/knadh/koanf/v2 v2.3.2 // indirect
	github.com/kylelemons/godebug v1.1.0 // indirect
	github.com/mitchellh/copystructure v1.2.0 // indirect
	github.com/mitchellh/reflectwalk v1.0.2 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.3-0.20250322232337-35a7c28c31ee // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal v0.145.0
	github.com/pkg/browser v0.0.0-20240102092130-5ac0b6a4141c // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/tj/assert v0.0.3
	go.opentelemetry.io/auto/sdk v1.2.1 // indirect
	go.opentelemetry.io/collector/config/configretry v1.51.1-0.20260212100729-5a059d1d6718
	go.opentelemetry.io/collector/consumer/consumererror v0.145.1-0.20260212100729-5a059d1d6718 // indirect
	go.opentelemetry.io/collector/consumer/consumertest v0.145.1-0.20260212100729-5a059d1d6718 // indirect
	go.opentelemetry.io/collector/consumer/xconsumer v0.145.1-0.20260212100729-5a059d1d6718 // indirect
	go.opentelemetry.io/collector/exporter/xexporter v0.145.1-0.20260212100729-5a059d1d6718 // indirect
	go.opentelemetry.io/collector/extension v1.51.1-0.20260212100729-5a059d1d6718 // indirect
	go.opentelemetry.io/collector/extension/xextension v0.145.1-0.20260212100729-5a059d1d6718 // indirect
	go.opentelemetry.io/collector/featuregate v1.51.1-0.20260212100729-5a059d1d6718 // indirect
	go.opentelemetry.io/collector/pdata/pprofile v0.145.1-0.20260212100729-5a059d1d6718 // indirect
	go.opentelemetry.io/collector/receiver v1.51.1-0.20260212100729-5a059d1d6718 // indirect
	go.opentelemetry.io/collector/receiver/receivertest v0.145.1-0.20260212100729-5a059d1d6718 // indirect
	go.opentelemetry.io/collector/receiver/xreceiver v0.145.1-0.20260212100729-5a059d1d6718 // indirect
	go.opentelemetry.io/otel v1.40.0 // indirect
	go.opentelemetry.io/otel/metric v1.40.0 // indirect
	go.opentelemetry.io/otel/sdk v1.40.0 // indirect
	go.opentelemetry.io/otel/sdk/metric v1.40.0 // indirect
	go.opentelemetry.io/otel/trace v1.40.0 // indirect
	go.uber.org/multierr v1.11.0 // indirect
	golang.org/x/crypto v0.47.0 // indirect
	golang.org/x/net v0.49.0 // indirect
	golang.org/x/sys v0.40.0 // indirect
	golang.org/x/text v0.34.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20251222181119-0a764e51fe1b // indirect
	google.golang.org/grpc v1.78.0 // indirect
	google.golang.org/protobuf v1.36.11 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

retract (
	v0.76.2
	v0.76.1
)

replace github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest => ../../pkg/pdatatest

replace github.com/open-telemetry/opentelemetry-collector-contrib/pkg/golden => ../../pkg/golden

replace github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal => ../../internal/coreinternal

replace github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatautil => ../../pkg/pdatautil
