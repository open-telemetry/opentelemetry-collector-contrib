module github.com/open-telemetry/opentelemetry-collector-contrib/exporter/awskinesisexporter

go 1.18

require (
	github.com/aws/aws-sdk-go-v2 v1.16.14
	github.com/aws/aws-sdk-go-v2/config v1.15.14
	github.com/aws/aws-sdk-go-v2/credentials v1.12.18
	github.com/aws/aws-sdk-go-v2/service/kinesis v1.15.17
	github.com/aws/aws-sdk-go-v2/service/sts v1.16.17
	github.com/gogo/protobuf v1.3.2
	github.com/google/uuid v1.3.0
	github.com/jaegertracing/jaeger v1.37.0
	github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/jaeger v0.59.0
	github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/zipkin v0.59.0
	github.com/stretchr/testify v1.8.0
	go.opentelemetry.io/collector v0.59.1-0.20220909192754-8d66f408a79a
	go.opentelemetry.io/collector/pdata v0.59.1-0.20220909192754-8d66f408a79a
	go.uber.org/multierr v1.8.0
	go.uber.org/zap v1.23.0
)

require (
	github.com/apache/thrift v0.16.0 // indirect
	github.com/aws/aws-sdk-go-v2/aws/protocol/eventstream v1.4.7 // indirect
	github.com/aws/aws-sdk-go-v2/feature/ec2/imds v1.12.15 // indirect
	github.com/aws/aws-sdk-go-v2/internal/configsources v1.1.21 // indirect
	github.com/aws/aws-sdk-go-v2/internal/endpoints/v2 v2.4.15 // indirect
	github.com/aws/aws-sdk-go-v2/internal/ini v1.3.15 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/presigned-url v1.9.15 // indirect
	github.com/aws/aws-sdk-go-v2/service/sso v1.11.21 // indirect
	github.com/aws/aws-sdk-go-v2/service/ssooidc v1.13.3 // indirect
	github.com/aws/smithy-go v1.13.2 // indirect
	github.com/benbjohnson/clock v1.3.0 // indirect
	github.com/cenkalti/backoff/v4 v4.1.3 // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/golang/protobuf v1.5.2 // indirect
	github.com/jmespath/go-jmespath v0.4.0 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/knadh/koanf v1.4.3 // indirect
	github.com/mitchellh/copystructure v1.2.0 // indirect
	github.com/mitchellh/mapstructure v1.5.0 // indirect
	github.com/mitchellh/reflectwalk v1.0.2 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.2 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal v0.59.0 // indirect
	github.com/opentracing/opentracing-go v1.2.0 // indirect
	github.com/openzipkin/zipkin-go v0.4.0 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/uber/jaeger-client-go v2.30.0+incompatible // indirect
	github.com/uber/jaeger-lib v2.4.1+incompatible // indirect
	go.opencensus.io v0.23.0 // indirect
	go.opentelemetry.io/collector/semconv v0.59.1-0.20220909192754-8d66f408a79a // indirect
	go.opentelemetry.io/otel v1.9.0 // indirect
	go.opentelemetry.io/otel/metric v0.31.0 // indirect
	go.opentelemetry.io/otel/trace v1.9.0 // indirect
	go.uber.org/atomic v1.10.0 // indirect
	golang.org/x/net v0.0.0-20220520000938-2e3eb7b945c2 // indirect
	golang.org/x/sys v0.0.0-20220808155132-1c4a2a72c664 // indirect
	golang.org/x/text v0.3.7 // indirect
	google.golang.org/genproto v0.0.0-20220519153652-3a47de7e79bd // indirect
	google.golang.org/grpc v1.49.0 // indirect
	google.golang.org/protobuf v1.28.1 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

replace github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/jaeger => ../../pkg/translator/jaeger

replace github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/zipkin => ../../pkg/translator/zipkin

replace github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal => ../../internal/coreinternal

replace github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/opencensus => ../../pkg/translator/opencensus
