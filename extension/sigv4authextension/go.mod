module github.com/open-telemetry/opentelemetry-collector-contrib/extension/sigv4authextension

go 1.18

require (
	github.com/aws/aws-sdk-go-v2 v1.17.2
	github.com/aws/aws-sdk-go-v2/config v1.18.0
	github.com/aws/aws-sdk-go-v2/credentials v1.13.0
	github.com/aws/aws-sdk-go-v2/service/sts v1.17.6
	github.com/stretchr/testify v1.8.1
	go.opentelemetry.io/collector v0.66.1-0.20221202005155-1c54042beb70
	go.opentelemetry.io/collector/component v0.66.1-0.20221202005155-1c54042beb70
	go.opentelemetry.io/collector/confmap v0.0.0-20221201172708-2bdff61fa52a
	go.uber.org/zap v1.24.0
	google.golang.org/grpc v1.51.0
)

require (
	github.com/aws/aws-sdk-go-v2/feature/ec2/imds v1.12.19 // indirect
	github.com/aws/aws-sdk-go-v2/internal/configsources v1.1.26 // indirect
	github.com/aws/aws-sdk-go-v2/internal/endpoints/v2 v2.4.20 // indirect
	github.com/aws/aws-sdk-go-v2/internal/ini v1.3.26 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/presigned-url v1.9.20 // indirect
	github.com/aws/aws-sdk-go-v2/service/sso v1.11.25 // indirect
	github.com/aws/aws-sdk-go-v2/service/ssooidc v1.13.8 // indirect
	github.com/aws/smithy-go v1.13.5 // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/golang/protobuf v1.5.2 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/knadh/koanf v1.4.4 // indirect
	github.com/mitchellh/copystructure v1.2.0 // indirect
	github.com/mitchellh/mapstructure v1.5.0 // indirect
	github.com/mitchellh/reflectwalk v1.0.2 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.2 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	go.opentelemetry.io/collector/consumer v0.66.1-0.20221202005155-1c54042beb70 // indirect
	go.opentelemetry.io/collector/featuregate v0.66.1-0.20221202005155-1c54042beb70 // indirect
	go.opentelemetry.io/collector/pdata v0.66.1-0.20221202005155-1c54042beb70 // indirect
	go.opentelemetry.io/otel v1.11.1 // indirect
	go.opentelemetry.io/otel/metric v0.33.0 // indirect
	go.opentelemetry.io/otel/trace v1.11.1 // indirect
	go.uber.org/atomic v1.10.0 // indirect
	go.uber.org/multierr v1.8.0 // indirect
	golang.org/x/net v0.0.0-20220722155237-a158d28d115b // indirect
	golang.org/x/sys v0.2.0 // indirect
	golang.org/x/text v0.4.0 // indirect
	google.golang.org/genproto v0.0.0-20211208223120-3a66f561d7aa // indirect
	google.golang.org/protobuf v1.28.1 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

retract v0.65.0
