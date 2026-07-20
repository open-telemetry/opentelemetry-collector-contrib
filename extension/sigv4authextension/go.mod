module github.com/open-telemetry/opentelemetry-collector-contrib/extension/sigv4authextension

go 1.25.0

require (
	github.com/aws/aws-sdk-go-v2 v1.42.1
	github.com/aws/aws-sdk-go-v2/config v1.32.29
	github.com/aws/aws-sdk-go-v2/credentials v1.19.28
	github.com/aws/aws-sdk-go-v2/service/sts v1.44.0
	github.com/stretchr/testify v1.11.1
	go.opentelemetry.io/collector/component v1.62.1-0.20260720131437-2ab65ae99cb1
	go.opentelemetry.io/collector/component/componenttest v0.156.1-0.20260720131437-2ab65ae99cb1
	go.opentelemetry.io/collector/confmap v1.62.1-0.20260720131437-2ab65ae99cb1
	go.opentelemetry.io/collector/confmap/xconfmap v0.156.1-0.20260720131437-2ab65ae99cb1
	go.opentelemetry.io/collector/extension v1.62.1-0.20260720131437-2ab65ae99cb1
	go.opentelemetry.io/collector/extension/extensionauth v1.62.1-0.20260720131437-2ab65ae99cb1
	go.opentelemetry.io/collector/extension/extensiontest v0.156.1-0.20260720131437-2ab65ae99cb1
	go.uber.org/goleak v1.3.0
	go.uber.org/zap v1.28.0
)

require (
	github.com/aws/aws-sdk-go-v2/feature/ec2/imds v1.18.30 // indirect
	github.com/aws/aws-sdk-go-v2/internal/configsources v1.4.30 // indirect
	github.com/aws/aws-sdk-go-v2/internal/endpoints/v2 v2.7.30 // indirect
	github.com/aws/aws-sdk-go-v2/internal/v4a v1.4.31 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/accept-encoding v1.13.13 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/presigned-url v1.13.30 // indirect
	github.com/aws/aws-sdk-go-v2/service/signin v1.4.0 // indirect
	github.com/aws/aws-sdk-go-v2/service/sso v1.32.0 // indirect
	github.com/aws/aws-sdk-go-v2/service/ssooidc v1.37.0 // indirect
	github.com/aws/smithy-go v1.27.3 // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/go-logr/logr v1.4.3 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/go-viper/mapstructure/v2 v2.5.0 // indirect
	github.com/gobwas/glob v0.2.3 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/hashicorp/go-version v1.9.0 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/knadh/koanf/maps v0.1.2 // indirect
	github.com/knadh/koanf/providers/confmap v1.0.0 // indirect
	github.com/knadh/koanf/v2 v2.3.5 // indirect
	github.com/mitchellh/copystructure v1.2.0 // indirect
	github.com/mitchellh/reflectwalk v1.0.2 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.3-0.20250322232337-35a7c28c31ee // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	go.opentelemetry.io/auto/sdk v1.2.1 // indirect
	go.opentelemetry.io/collector/featuregate v1.62.1-0.20260720131437-2ab65ae99cb1 // indirect
	go.opentelemetry.io/collector/internal/componentalias v0.156.1-0.20260720131437-2ab65ae99cb1 // indirect
	go.opentelemetry.io/collector/pdata v1.62.1-0.20260720131437-2ab65ae99cb1 // indirect
	go.opentelemetry.io/otel v1.44.0 // indirect
	go.opentelemetry.io/otel/metric v1.44.0 // indirect
	go.opentelemetry.io/otel/sdk v1.44.0 // indirect
	go.opentelemetry.io/otel/sdk/metric v1.44.0 // indirect
	go.opentelemetry.io/otel/trace v1.44.0 // indirect
	go.uber.org/multierr v1.11.0 // indirect
	go.yaml.in/yaml/v3 v3.0.4 // indirect
	golang.org/x/sys v0.45.0 // indirect
	google.golang.org/grpc v1.82.0 // indirect
	google.golang.org/protobuf v1.36.11 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

retract (
	v0.76.2
	v0.76.1
	v0.65.0
)
