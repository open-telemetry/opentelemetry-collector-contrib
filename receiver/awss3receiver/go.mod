module github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awss3receiver

go 1.25.0

require (
	github.com/aws/aws-sdk-go-v2 v1.41.7
	github.com/aws/aws-sdk-go-v2/config v1.32.17
	github.com/aws/aws-sdk-go-v2/service/s3 v1.100.1
	github.com/aws/aws-sdk-go-v2/service/sqs v1.42.27
	github.com/itchyny/timefmt-go v0.1.8
	github.com/klauspost/compress v1.18.6
	github.com/open-telemetry/opamp-go v0.23.0
	github.com/open-telemetry/opentelemetry-collector-contrib/extension/opampcustommessages v0.151.0
	github.com/stretchr/testify v1.11.1
	go.opentelemetry.io/collector/component v1.57.1-0.20260511090956-a434cb0a4028
	go.opentelemetry.io/collector/component/componenttest v0.151.1-0.20260511090956-a434cb0a4028
	go.opentelemetry.io/collector/confmap v1.57.1-0.20260511090956-a434cb0a4028
	go.opentelemetry.io/collector/confmap/xconfmap v0.151.1-0.20260511090956-a434cb0a4028
	go.opentelemetry.io/collector/consumer v1.57.1-0.20260511090956-a434cb0a4028
	go.opentelemetry.io/collector/consumer/consumertest v0.151.1-0.20260511090956-a434cb0a4028
	go.opentelemetry.io/collector/pdata v1.57.1-0.20260511090956-a434cb0a4028
	go.opentelemetry.io/collector/receiver v1.57.1-0.20260511090956-a434cb0a4028
	go.opentelemetry.io/collector/receiver/receiverhelper v0.151.1-0.20260511090956-a434cb0a4028
	go.opentelemetry.io/collector/receiver/receivertest v0.151.1-0.20260511090956-a434cb0a4028
	go.uber.org/goleak v1.3.0
	go.uber.org/multierr v1.11.0
	go.uber.org/zap v1.28.0
)

require (
	github.com/aws/aws-sdk-go-v2/aws/protocol/eventstream v1.7.10 // indirect
	github.com/aws/aws-sdk-go-v2/credentials v1.19.16 // indirect
	github.com/aws/aws-sdk-go-v2/feature/ec2/imds v1.18.23 // indirect
	github.com/aws/aws-sdk-go-v2/internal/configsources v1.4.23 // indirect
	github.com/aws/aws-sdk-go-v2/internal/endpoints/v2 v2.7.23 // indirect
	github.com/aws/aws-sdk-go-v2/internal/v4a v1.4.24 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/accept-encoding v1.13.9 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/checksum v1.9.15 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/presigned-url v1.13.23 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/s3shared v1.19.23 // indirect
	github.com/aws/aws-sdk-go-v2/service/signin v1.0.11 // indirect
	github.com/aws/aws-sdk-go-v2/service/sso v1.30.17 // indirect
	github.com/aws/aws-sdk-go-v2/service/ssooidc v1.35.21 // indirect
	github.com/aws/aws-sdk-go-v2/service/sts v1.42.1 // indirect
	github.com/aws/smithy-go v1.25.1 // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/davecgh/go-spew v1.1.2-0.20180830191138-d8f796af33cc // indirect
	github.com/go-logr/logr v1.4.3 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/go-viper/mapstructure/v2 v2.5.0 // indirect
	github.com/gobwas/glob v0.2.3 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/hashicorp/go-version v1.9.0 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/knadh/koanf/maps v0.1.2 // indirect
	github.com/knadh/koanf/providers/confmap v1.0.0 // indirect
	github.com/knadh/koanf/v2 v2.3.4 // indirect
	github.com/mitchellh/copystructure v1.2.0 // indirect
	github.com/mitchellh/reflectwalk v1.0.2 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.3-0.20250322232337-35a7c28c31ee // indirect
	github.com/pmezard/go-difflib v1.0.1-0.20181226105442-5d4384ee4fb2 // indirect
	github.com/stretchr/objx v0.5.2 // indirect
	go.opentelemetry.io/auto/sdk v1.2.1 // indirect
	go.opentelemetry.io/collector/consumer/consumererror v0.151.1-0.20260511090956-a434cb0a4028 // indirect
	go.opentelemetry.io/collector/consumer/xconsumer v0.151.1-0.20260511090956-a434cb0a4028 // indirect
	go.opentelemetry.io/collector/featuregate v1.57.1-0.20260511090956-a434cb0a4028 // indirect
	go.opentelemetry.io/collector/internal/componentalias v0.151.1-0.20260511090956-a434cb0a4028 // indirect
	go.opentelemetry.io/collector/pdata/pprofile v0.151.1-0.20260511090956-a434cb0a4028 // indirect
	go.opentelemetry.io/collector/pipeline v1.57.1-0.20260511090956-a434cb0a4028 // indirect
	go.opentelemetry.io/collector/pipeline/xpipeline v0.151.1-0.20260511090956-a434cb0a4028 // indirect
	go.opentelemetry.io/collector/receiver/xreceiver v0.151.1-0.20260511090956-a434cb0a4028 // indirect
	go.opentelemetry.io/otel v1.43.0 // indirect
	go.opentelemetry.io/otel/metric v1.43.0 // indirect
	go.opentelemetry.io/otel/sdk v1.43.0 // indirect
	go.opentelemetry.io/otel/sdk/metric v1.43.0 // indirect
	go.opentelemetry.io/otel/trace v1.43.0 // indirect
	go.yaml.in/yaml/v3 v3.0.4 // indirect
	golang.org/x/sys v0.42.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20260226221140-a57be14db171 // indirect
	google.golang.org/grpc v1.81.0 // indirect
	google.golang.org/protobuf v1.36.11 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

replace github.com/open-telemetry/opentelemetry-collector-contrib/extension/opampcustommessages => ../../extension/opampcustommessages
