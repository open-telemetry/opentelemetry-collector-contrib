module github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awss3receiver

go 1.23.0

require (
	github.com/aws/aws-sdk-go-v2 v1.36.3
	github.com/aws/aws-sdk-go-v2/config v1.29.10
	github.com/aws/aws-sdk-go-v2/feature/s3/manager v1.17.67
	github.com/aws/aws-sdk-go-v2/service/s3 v1.78.2
	github.com/open-telemetry/opamp-go v0.19.0
	github.com/open-telemetry/opentelemetry-collector-contrib/extension/opampcustommessages v0.122.0
	github.com/stretchr/testify v1.10.0
	go.opentelemetry.io/collector/component v1.28.2-0.20250319144947-41a9ea7f7402
	go.opentelemetry.io/collector/component/componenttest v0.122.2-0.20250319144947-41a9ea7f7402
	go.opentelemetry.io/collector/confmap v1.28.2-0.20250319144947-41a9ea7f7402
	go.opentelemetry.io/collector/confmap/xconfmap v0.122.2-0.20250319144947-41a9ea7f7402
	go.opentelemetry.io/collector/consumer v1.28.2-0.20250319144947-41a9ea7f7402
	go.opentelemetry.io/collector/consumer/consumertest v0.122.2-0.20250319144947-41a9ea7f7402
	go.opentelemetry.io/collector/pdata v1.28.2-0.20250319144947-41a9ea7f7402
	go.opentelemetry.io/collector/receiver v1.28.2-0.20250319144947-41a9ea7f7402
	go.opentelemetry.io/collector/receiver/receiverhelper v0.122.2-0.20250319144947-41a9ea7f7402
	go.opentelemetry.io/collector/receiver/receivertest v0.122.2-0.20250319144947-41a9ea7f7402
	go.opentelemetry.io/collector/semconv v0.122.2-0.20250319144947-41a9ea7f7402
	go.uber.org/goleak v1.3.0
	go.uber.org/multierr v1.11.0
	go.uber.org/zap v1.27.0
)

require (
	github.com/aws/aws-sdk-go-v2/aws/protocol/eventstream v1.6.10 // indirect
	github.com/aws/aws-sdk-go-v2/credentials v1.17.63 // indirect
	github.com/aws/aws-sdk-go-v2/feature/ec2/imds v1.16.30 // indirect
	github.com/aws/aws-sdk-go-v2/internal/configsources v1.3.34 // indirect
	github.com/aws/aws-sdk-go-v2/internal/endpoints/v2 v2.6.34 // indirect
	github.com/aws/aws-sdk-go-v2/internal/ini v1.8.3 // indirect
	github.com/aws/aws-sdk-go-v2/internal/v4a v1.3.34 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/accept-encoding v1.12.3 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/checksum v1.7.0 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/presigned-url v1.12.15 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/s3shared v1.18.15 // indirect
	github.com/aws/aws-sdk-go-v2/service/sso v1.25.1 // indirect
	github.com/aws/aws-sdk-go-v2/service/ssooidc v1.29.2 // indirect
	github.com/aws/aws-sdk-go-v2/service/sts v1.33.17 // indirect
	github.com/aws/smithy-go v1.22.2 // indirect
	github.com/davecgh/go-spew v1.1.2-0.20180830191138-d8f796af33cc // indirect
	github.com/go-logr/logr v1.4.2 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/go-viper/mapstructure/v2 v2.2.1 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/hashicorp/go-version v1.7.0 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/knadh/koanf/maps v0.1.1 // indirect
	github.com/knadh/koanf/providers/confmap v0.1.0 // indirect
	github.com/knadh/koanf/v2 v2.1.2 // indirect
	github.com/mitchellh/copystructure v1.2.0 // indirect
	github.com/mitchellh/reflectwalk v1.0.2 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.2 // indirect
	github.com/pmezard/go-difflib v1.0.1-0.20181226105442-5d4384ee4fb2 // indirect
	go.opentelemetry.io/auto/sdk v1.1.0 // indirect
	go.opentelemetry.io/collector/consumer/consumererror v0.122.2-0.20250319144947-41a9ea7f7402 // indirect
	go.opentelemetry.io/collector/consumer/xconsumer v0.122.2-0.20250319144947-41a9ea7f7402 // indirect
	go.opentelemetry.io/collector/featuregate v1.28.2-0.20250319144947-41a9ea7f7402 // indirect
	go.opentelemetry.io/collector/pdata/pprofile v0.122.2-0.20250319144947-41a9ea7f7402 // indirect
	go.opentelemetry.io/collector/pipeline v0.122.2-0.20250319144947-41a9ea7f7402 // indirect
	go.opentelemetry.io/collector/receiver/xreceiver v0.122.2-0.20250319144947-41a9ea7f7402 // indirect
	go.opentelemetry.io/otel v1.35.0 // indirect
	go.opentelemetry.io/otel/metric v1.35.0 // indirect
	go.opentelemetry.io/otel/sdk v1.35.0 // indirect
	go.opentelemetry.io/otel/sdk/metric v1.35.0 // indirect
	go.opentelemetry.io/otel/trace v1.35.0 // indirect
	golang.org/x/net v0.37.0 // indirect
	golang.org/x/sys v0.31.0 // indirect
	golang.org/x/text v0.23.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20250115164207-1a7da9e5054f // indirect
	google.golang.org/grpc v1.71.0 // indirect
	google.golang.org/protobuf v1.36.5 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

replace github.com/open-telemetry/opentelemetry-collector-contrib/extension/opampcustommessages => ../../extension/opampcustommessages
