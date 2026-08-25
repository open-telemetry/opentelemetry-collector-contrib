module github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awss3receiver

go 1.26.0

require (
	github.com/aws/aws-sdk-go-v2 v1.43.7
	github.com/aws/aws-sdk-go-v2/config v1.32.38
	github.com/aws/aws-sdk-go-v2/service/s3 v1.107.3
	github.com/aws/aws-sdk-go-v2/service/sqs v1.46.7
	github.com/itchyny/timefmt-go v0.1.8
	github.com/klauspost/compress v1.19.2
	github.com/open-telemetry/opamp-go v0.23.0
	github.com/open-telemetry/opentelemetry-collector-contrib/extension/opampcustommessages v0.159.0
	github.com/stretchr/testify v1.12.1
	go.opentelemetry.io/collector/component v1.65.1-0.20260824174011-67fef8cb7049
	go.opentelemetry.io/collector/component/componenttest v0.159.1-0.20260824174011-67fef8cb7049
	go.opentelemetry.io/collector/confmap v1.65.1-0.20260824174011-67fef8cb7049
	go.opentelemetry.io/collector/consumer v1.65.1-0.20260824174011-67fef8cb7049
	go.opentelemetry.io/collector/consumer/consumertest v0.159.1-0.20260824174011-67fef8cb7049
	go.opentelemetry.io/collector/pdata v1.65.1-0.20260824174011-67fef8cb7049
	go.opentelemetry.io/collector/receiver v1.65.1-0.20260824174011-67fef8cb7049
	go.opentelemetry.io/collector/receiver/receiverhelper v0.159.1-0.20260824174011-67fef8cb7049
	go.opentelemetry.io/collector/receiver/receivertest v0.159.1-0.20260824174011-67fef8cb7049
	go.uber.org/goleak v1.3.0
	go.uber.org/multierr v1.11.0
	go.uber.org/zap v1.28.0
)

require (
	github.com/aws/aws-sdk-go-v2/aws/protocol/eventstream v1.7.18 // indirect
	github.com/aws/aws-sdk-go-v2/credentials v1.19.37 // indirect
	github.com/aws/aws-sdk-go-v2/feature/ec2/imds v1.18.38 // indirect
	github.com/aws/aws-sdk-go-v2/internal/configsources v1.4.38 // indirect
	github.com/aws/aws-sdk-go-v2/internal/endpoints/v2 v2.7.38 // indirect
	github.com/aws/aws-sdk-go-v2/internal/v4a v1.4.39 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/accept-encoding v1.13.17 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/checksum v1.9.31 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/presigned-url v1.13.38 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/s3shared v1.19.39 // indirect
	github.com/aws/aws-sdk-go-v2/service/signin v1.5.7 // indirect
	github.com/aws/aws-sdk-go-v2/service/sso v1.33.7 // indirect
	github.com/aws/aws-sdk-go-v2/service/ssooidc v1.38.7 // indirect
	github.com/aws/aws-sdk-go-v2/service/sts v1.45.7 // indirect
	github.com/aws/smithy-go v1.27.8 // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/go-logr/logr v1.4.4 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/go-viper/mapstructure/v2 v2.5.0 // indirect
	github.com/gobwas/glob v0.2.3 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/hashicorp/go-version v1.9.0 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/knadh/koanf/maps v0.1.3 // indirect
	github.com/knadh/koanf/providers/confmap v1.0.1 // indirect
	github.com/knadh/koanf/v2 v2.3.6 // indirect
	github.com/mitchellh/copystructure v1.2.0 // indirect
	github.com/mitchellh/reflectwalk v1.0.2 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.3-0.20250322232337-35a7c28c31ee // indirect
	github.com/stretchr/objx v0.5.3 // indirect
	go.opentelemetry.io/auto/sdk v1.2.1 // indirect
	go.opentelemetry.io/collector/consumer/consumererror v0.159.1-0.20260824174011-67fef8cb7049 // indirect
	go.opentelemetry.io/collector/consumer/xconsumer v0.159.1-0.20260824174011-67fef8cb7049 // indirect
	go.opentelemetry.io/collector/featuregate v1.65.1-0.20260824174011-67fef8cb7049 // indirect
	go.opentelemetry.io/collector/internal/componentalias v0.159.1-0.20260824174011-67fef8cb7049 // indirect
	go.opentelemetry.io/collector/pdata/pprofile v0.159.1-0.20260824174011-67fef8cb7049 // indirect
	go.opentelemetry.io/collector/pipeline v1.65.1-0.20260824174011-67fef8cb7049 // indirect
	go.opentelemetry.io/collector/pipeline/xpipeline v0.159.1-0.20260824174011-67fef8cb7049 // indirect
	go.opentelemetry.io/collector/receiver/xreceiver v0.159.1-0.20260824174011-67fef8cb7049 // indirect
	go.opentelemetry.io/otel v1.45.0 // indirect
	go.opentelemetry.io/otel/metric v1.45.0 // indirect
	go.opentelemetry.io/otel/sdk v1.45.0 // indirect
	go.opentelemetry.io/otel/sdk/metric v1.45.0 // indirect
	go.opentelemetry.io/otel/trace v1.45.0 // indirect
	go.yaml.in/yaml/v3 v3.0.5 // indirect
	golang.org/x/sys v0.47.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20260610212136-7ab31c22f7ad // indirect
	google.golang.org/grpc v1.83.0 // indirect
	google.golang.org/protobuf v1.36.12 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

replace github.com/open-telemetry/opentelemetry-collector-contrib/extension/opampcustommessages => ../../extension/opampcustommessages
