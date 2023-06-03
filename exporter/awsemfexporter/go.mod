module github.com/open-telemetry/opentelemetry-collector-contrib/exporter/awsemfexporter

go 1.19

require (
	github.com/aws/aws-sdk-go v1.44.274
	github.com/census-instrumentation/opencensus-proto v0.4.1
	github.com/golang/protobuf v1.5.3
	github.com/google/uuid v1.3.0
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/awsutil v0.78.0
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/cwlogs v0.78.0
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/metrics v0.78.0
	github.com/open-telemetry/opentelemetry-collector-contrib/pkg/resourcetotelemetry v0.78.0
	github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/opencensus v0.78.0
	github.com/stretchr/testify v1.8.4
	go.opentelemetry.io/collector/component v0.78.3-0.20230601234953-deffd4892002
	go.opentelemetry.io/collector/confmap v0.78.3-0.20230601234953-deffd4892002
	go.opentelemetry.io/collector/consumer v0.78.3-0.20230601234953-deffd4892002
	go.opentelemetry.io/collector/exporter v0.78.3-0.20230601234953-deffd4892002
	go.opentelemetry.io/collector/pdata v1.0.0-rcv0012.0.20230601234953-deffd4892002
	go.opentelemetry.io/collector/semconv v0.78.3-0.20230601234953-deffd4892002
	go.uber.org/zap v1.24.0
	golang.org/x/exp v0.0.0-20221217163422-3c43f8badb15
)

require (
	github.com/cenkalti/backoff/v4 v4.2.1 // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/golang/groupcache v0.0.0-20210331224755-41bb18bfe9da // indirect
	github.com/grpc-ecosystem/grpc-gateway/v2 v2.15.2 // indirect
	github.com/jmespath/go-jmespath v0.4.0 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/knadh/koanf v1.5.0 // indirect
	github.com/mitchellh/copystructure v1.2.0 // indirect
	github.com/mitchellh/mapstructure v1.5.0 // indirect
	github.com/mitchellh/reflectwalk v1.0.2 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.2 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal v0.78.0 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/stretchr/objx v0.5.0 // indirect
	go.opencensus.io v0.24.0 // indirect
	go.opentelemetry.io/collector v0.78.3-0.20230601234953-deffd4892002 // indirect
	go.opentelemetry.io/collector/featuregate v1.0.0-rcv0012.0.20230601234953-deffd4892002 // indirect
	go.opentelemetry.io/collector/receiver v0.78.3-0.20230601234953-deffd4892002 // indirect
	go.opentelemetry.io/otel v1.16.0 // indirect
	go.opentelemetry.io/otel/metric v1.16.0 // indirect
	go.opentelemetry.io/otel/trace v1.16.0 // indirect
	go.uber.org/atomic v1.10.0 // indirect
	go.uber.org/multierr v1.11.0 // indirect
	golang.org/x/net v0.10.0 // indirect
	golang.org/x/sys v0.8.0 // indirect
	golang.org/x/text v0.9.0 // indirect
	google.golang.org/genproto v0.0.0-20230410155749-daa745c078e1 // indirect
	google.golang.org/grpc v1.55.0 // indirect
	google.golang.org/protobuf v1.30.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

replace github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/metrics => ../../internal/aws/metrics

replace github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/awsutil => ../../internal/aws/awsutil

replace github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/cwlogs => ../../internal/aws/cwlogs

replace github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal => ../../internal/coreinternal

replace github.com/open-telemetry/opentelemetry-collector-contrib/pkg/resourcetotelemetry => ../../pkg/resourcetotelemetry

replace github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/opencensus => ../../pkg/translator/opencensus

retract (
	v0.76.2
	v0.76.1
	v0.65.0
)

replace github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest => ../../pkg/pdatatest

replace github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatautil => ../../pkg/pdatautil
