module github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/proxy

go 1.25.0

require (
	github.com/aws/aws-sdk-go-v2 v1.41.6
	github.com/aws/aws-sdk-go-v2/feature/ec2/imds v1.18.22
	github.com/aws/aws-sdk-go-v2/service/sts v1.42.0
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/awsutil v0.150.0
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/common v0.150.0
	github.com/stretchr/testify v1.11.1
	go.opentelemetry.io/collector/config/confignet v1.56.1-0.20260424074859-d91c0edd1da5
	go.opentelemetry.io/collector/config/configtls v1.56.1-0.20260424074859-d91c0edd1da5
	go.uber.org/zap v1.27.1
)

require (
	github.com/aws/aws-sdk-go-v2/config v1.32.16 // indirect
	github.com/aws/aws-sdk-go-v2/credentials v1.19.15 // indirect
	github.com/aws/aws-sdk-go-v2/internal/configsources v1.4.22 // indirect
	github.com/aws/aws-sdk-go-v2/internal/endpoints/v2 v2.7.22 // indirect
	github.com/aws/aws-sdk-go-v2/internal/v4a v1.4.23 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/accept-encoding v1.13.8 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/presigned-url v1.13.22 // indirect
	github.com/aws/aws-sdk-go-v2/service/signin v1.0.10 // indirect
	github.com/aws/aws-sdk-go-v2/service/sso v1.30.16 // indirect
	github.com/aws/aws-sdk-go-v2/service/ssooidc v1.35.20 // indirect
	github.com/aws/smithy-go v1.25.0 // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/foxboron/go-tpm-keyfiles v0.0.0-20250903184740-5d135037bd4d // indirect
	github.com/fsnotify/fsnotify v1.9.0 // indirect
	github.com/go-viper/mapstructure/v2 v2.5.0 // indirect
	github.com/gobwas/glob v0.2.3 // indirect
	github.com/google/go-tpm v0.9.8 // indirect
	github.com/hashicorp/go-version v1.9.0 // indirect
	github.com/knadh/koanf/maps v0.1.2 // indirect
	github.com/knadh/koanf/providers/confmap v1.0.0 // indirect
	github.com/knadh/koanf/v2 v2.3.4 // indirect
	github.com/mitchellh/copystructure v1.2.0 // indirect
	github.com/mitchellh/reflectwalk v1.0.2 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	go.opentelemetry.io/collector/config/configopaque v1.56.1-0.20260424074859-d91c0edd1da5 // indirect
	go.opentelemetry.io/collector/confmap v1.56.1-0.20260424074859-d91c0edd1da5 // indirect
	go.opentelemetry.io/collector/confmap/xconfmap v0.150.1-0.20260424074859-d91c0edd1da5 // indirect
	go.opentelemetry.io/collector/featuregate v1.56.1-0.20260424074859-d91c0edd1da5 // indirect
	go.uber.org/multierr v1.11.0 // indirect
	go.yaml.in/yaml/v3 v3.0.4 // indirect
	golang.org/x/crypto v0.50.0 // indirect
	golang.org/x/net v0.53.0 // indirect
	golang.org/x/sys v0.43.0 // indirect
	golang.org/x/text v0.36.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

replace github.com/open-telemetry/opentelemetry-collector-contrib/internal/common => ../../../internal/common

replace github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/awsutil => ../../../internal/aws/awsutil

retract (
	v0.76.2
	v0.76.1
	v0.65.0
)
