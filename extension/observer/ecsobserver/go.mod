module github.com/open-telemetry/opentelemetry-collector-contrib/extension/observer/ecsobserver

go 1.24.0

require (
	github.com/aws/aws-sdk-go-v2 v1.39.6
	github.com/aws/aws-sdk-go-v2/config v1.31.17
	github.com/aws/aws-sdk-go-v2/service/ec2 v1.261.1
	github.com/aws/aws-sdk-go-v2/service/ecs v1.67.2
	github.com/hashicorp/golang-lru/v2 v2.0.7
	github.com/stretchr/testify v1.11.1
	go.opentelemetry.io/collector/component v1.45.1-0.20251106125304-a6a176660925
	go.opentelemetry.io/collector/component/componentstatus v0.139.1-0.20251106125304-a6a176660925
	go.opentelemetry.io/collector/component/componenttest v0.139.1-0.20251106125304-a6a176660925
	go.opentelemetry.io/collector/confmap v1.45.1-0.20251106125304-a6a176660925
	go.opentelemetry.io/collector/confmap/xconfmap v0.139.1-0.20251106125304-a6a176660925
	go.opentelemetry.io/collector/extension v1.45.1-0.20251106125304-a6a176660925
	go.opentelemetry.io/collector/extension/extensiontest v0.139.1-0.20251106125304-a6a176660925
	go.uber.org/goleak v1.3.0
	go.uber.org/multierr v1.11.0
	go.uber.org/zap v1.27.0
	gopkg.in/yaml.v3 v3.0.1
)

require (
	github.com/aws/aws-sdk-go-v2/credentials v1.18.21 // indirect
	github.com/aws/aws-sdk-go-v2/feature/ec2/imds v1.18.13 // indirect
	github.com/aws/aws-sdk-go-v2/internal/configsources v1.4.13 // indirect
	github.com/aws/aws-sdk-go-v2/internal/endpoints/v2 v2.7.13 // indirect
	github.com/aws/aws-sdk-go-v2/internal/ini v1.8.4 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/accept-encoding v1.13.3 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/presigned-url v1.13.13 // indirect
	github.com/aws/aws-sdk-go-v2/service/sso v1.30.1 // indirect
	github.com/aws/aws-sdk-go-v2/service/ssooidc v1.35.5 // indirect
	github.com/aws/aws-sdk-go-v2/service/sts v1.39.1 // indirect
	github.com/aws/smithy-go v1.23.2 // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/go-logr/logr v1.4.3 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/go-viper/mapstructure/v2 v2.4.0 // indirect
	github.com/gobwas/glob v0.2.3 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/hashicorp/go-version v1.7.0 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/knadh/koanf/maps v0.1.2 // indirect
	github.com/knadh/koanf/providers/confmap v1.0.0 // indirect
	github.com/knadh/koanf/v2 v2.3.0 // indirect
	github.com/mitchellh/copystructure v1.2.0 // indirect
	github.com/mitchellh/reflectwalk v1.0.2 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.3-0.20250322232337-35a7c28c31ee // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	go.opentelemetry.io/auto/sdk v1.1.0 // indirect
	go.opentelemetry.io/collector/featuregate v1.45.1-0.20251106125304-a6a176660925 // indirect
	go.opentelemetry.io/collector/pdata v1.45.1-0.20251106125304-a6a176660925 // indirect
	go.opentelemetry.io/collector/pipeline v1.45.1-0.20251106125304-a6a176660925 // indirect
	go.opentelemetry.io/otel v1.38.0 // indirect
	go.opentelemetry.io/otel/metric v1.38.0 // indirect
	go.opentelemetry.io/otel/sdk v1.38.0 // indirect
	go.opentelemetry.io/otel/sdk/metric v1.38.0 // indirect
	go.opentelemetry.io/otel/trace v1.38.0 // indirect
	go.yaml.in/yaml/v3 v3.0.4 // indirect
	golang.org/x/sys v0.35.0 // indirect
)

retract (
	v0.76.2
	v0.76.1
	v0.65.0
)
