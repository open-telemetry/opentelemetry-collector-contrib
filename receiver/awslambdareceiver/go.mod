module github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awslambdareceiver

go 1.25.0

require (
	github.com/aws/aws-lambda-go v1.52.0
	github.com/aws/aws-sdk-go-v2/config v1.32.7
	github.com/aws/aws-sdk-go-v2/service/s3 v1.96.0
	github.com/goccy/go-json v0.10.5
	github.com/open-telemetry/opentelemetry-collector-contrib/pkg/golden v0.145.0
	github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest v0.145.0
	github.com/stretchr/testify v1.11.1
	go.opentelemetry.io/collector/component v1.51.1-0.20260217094636-d8d07c01912d
	go.opentelemetry.io/collector/component/componenttest v0.145.1-0.20260217094636-d8d07c01912d
	go.opentelemetry.io/collector/confmap v1.51.1-0.20260217094636-d8d07c01912d
	go.opentelemetry.io/collector/confmap/xconfmap v0.145.1-0.20260217094636-d8d07c01912d
	go.opentelemetry.io/collector/consumer v1.51.1-0.20260217094636-d8d07c01912d
	go.opentelemetry.io/collector/consumer/consumererror v0.145.1-0.20260217094636-d8d07c01912d
	go.opentelemetry.io/collector/consumer/consumertest v0.145.1-0.20260217094636-d8d07c01912d
	go.opentelemetry.io/collector/pdata v1.51.1-0.20260217094636-d8d07c01912d
	go.opentelemetry.io/collector/receiver v1.51.1-0.20260217094636-d8d07c01912d
	go.opentelemetry.io/collector/receiver/receivertest v0.145.1-0.20260217094636-d8d07c01912d
	go.opentelemetry.io/otel v1.40.0
	go.uber.org/goleak v1.3.0
	go.uber.org/mock v0.6.0
	go.uber.org/zap v1.27.1
)

require (
	github.com/aws/aws-sdk-go-v2 v1.41.1 // indirect
	github.com/aws/aws-sdk-go-v2/aws/protocol/eventstream v1.7.4 // indirect
	github.com/aws/aws-sdk-go-v2/credentials v1.19.7 // indirect
	github.com/aws/aws-sdk-go-v2/feature/ec2/imds v1.18.17 // indirect
	github.com/aws/aws-sdk-go-v2/internal/configsources v1.4.17 // indirect
	github.com/aws/aws-sdk-go-v2/internal/endpoints/v2 v2.7.17 // indirect
	github.com/aws/aws-sdk-go-v2/internal/ini v1.8.4 // indirect
	github.com/aws/aws-sdk-go-v2/internal/v4a v1.4.17 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/accept-encoding v1.13.4 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/checksum v1.9.8 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/presigned-url v1.13.17 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/s3shared v1.19.17 // indirect
	github.com/aws/aws-sdk-go-v2/service/signin v1.0.5 // indirect
	github.com/aws/aws-sdk-go-v2/service/sso v1.30.9 // indirect
	github.com/aws/aws-sdk-go-v2/service/ssooidc v1.35.13 // indirect
	github.com/aws/aws-sdk-go-v2/service/sts v1.41.6 // indirect
	github.com/aws/smithy-go v1.24.0 // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/davecgh/go-spew v1.1.2-0.20180830191138-d8f796af33cc // indirect
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
	github.com/pmezard/go-difflib v1.0.1-0.20181226105442-5d4384ee4fb2 // indirect
	go.opentelemetry.io/auto/sdk v1.2.1 // indirect
	go.opentelemetry.io/collector/consumer/xconsumer v0.145.1-0.20260217094636-d8d07c01912d // indirect
	go.opentelemetry.io/collector/featuregate v1.51.1-0.20260217094636-d8d07c01912d // indirect
	go.opentelemetry.io/collector/internal/componentalias v0.145.1-0.20260217094636-d8d07c01912d // indirect
	go.opentelemetry.io/collector/pdata/pprofile v0.145.1-0.20260217094636-d8d07c01912d // indirect
	go.opentelemetry.io/collector/pipeline v1.51.1-0.20260217094636-d8d07c01912d // indirect
	go.opentelemetry.io/collector/receiver/xreceiver v0.145.1-0.20260217094636-d8d07c01912d // indirect
	go.opentelemetry.io/otel/metric v1.40.0 // indirect
	go.opentelemetry.io/otel/sdk v1.40.0 // indirect
	go.opentelemetry.io/otel/sdk/metric v1.40.0 // indirect
	go.opentelemetry.io/otel/trace v1.40.0 // indirect
	go.uber.org/multierr v1.11.0 // indirect
	go.yaml.in/yaml/v3 v3.0.4 // indirect
	golang.org/x/mod v0.27.0 // indirect
	golang.org/x/sync v0.19.0 // indirect
	golang.org/x/sys v0.40.0 // indirect
	golang.org/x/tools v0.36.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20251222181119-0a764e51fe1b // indirect
	google.golang.org/grpc v1.79.1 // indirect
	google.golang.org/protobuf v1.36.11 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

replace github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatautil => ../../pkg/pdatautil

replace github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest => ../../pkg/pdatatest

replace github.com/open-telemetry/opentelemetry-collector-contrib/pkg/golden => ../../pkg/golden

tool go.uber.org/mock/mockgen
