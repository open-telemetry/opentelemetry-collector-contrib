module github.com/open-telemetry/opentelemetry-collector-contrib/confmap/provider/s3provider

go 1.22.0

require (
	github.com/aws/aws-sdk-go-v2 v1.32.8
	github.com/aws/aws-sdk-go-v2/config v1.28.10
	github.com/aws/aws-sdk-go-v2/service/s3 v1.72.2
	github.com/stretchr/testify v1.10.0
	go.opentelemetry.io/collector/confmap v1.23.1-0.20250114172347-71aae791d7f8
	go.uber.org/goleak v1.3.0
	gopkg.in/yaml.v2 v2.4.0
)

require (
	github.com/aws/aws-sdk-go-v2/aws/protocol/eventstream v1.6.7 // indirect
	github.com/aws/aws-sdk-go-v2/credentials v1.17.51 // indirect
	github.com/aws/aws-sdk-go-v2/feature/ec2/imds v1.16.23 // indirect
	github.com/aws/aws-sdk-go-v2/internal/configsources v1.3.27 // indirect
	github.com/aws/aws-sdk-go-v2/internal/endpoints/v2 v2.6.27 // indirect
	github.com/aws/aws-sdk-go-v2/internal/ini v1.8.1 // indirect
	github.com/aws/aws-sdk-go-v2/internal/v4a v1.3.27 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/accept-encoding v1.12.1 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/checksum v1.4.8 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/presigned-url v1.12.8 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/s3shared v1.18.8 // indirect
	github.com/aws/aws-sdk-go-v2/service/sso v1.24.9 // indirect
	github.com/aws/aws-sdk-go-v2/service/ssooidc v1.28.8 // indirect
	github.com/aws/aws-sdk-go-v2/service/sts v1.33.6 // indirect
	github.com/aws/smithy-go v1.22.1 // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/go-viper/mapstructure/v2 v2.2.1 // indirect
	github.com/knadh/koanf/maps v0.1.1 // indirect
	github.com/knadh/koanf/providers/confmap v0.1.0 // indirect
	github.com/knadh/koanf/v2 v2.1.2 // indirect
	github.com/mitchellh/copystructure v1.2.0 // indirect
	github.com/mitchellh/reflectwalk v1.0.2 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	go.uber.org/multierr v1.11.0 // indirect
	go.uber.org/zap v1.27.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

retract (
	v0.76.2
	v0.76.1
	v0.65.0
)
