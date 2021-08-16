module github.com/open-telemetry/opentelemetry-collector-contrib/exporter/awsemfexporter

go 1.16

require (
	github.com/aws/aws-sdk-go v1.40.19
	github.com/census-instrumentation/opencensus-proto v0.3.0
	github.com/golang/protobuf v1.5.2
	github.com/google/gofuzz v1.2.0 // indirect
	github.com/google/uuid v1.3.0
	github.com/imdario/mergo v0.3.11 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/awsutil v0.0.0-00010101000000-000000000000
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/containerinsight v0.0.0-00010101000000-000000000000
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/k8s v0.0.0-00010101000000-000000000000
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/metrics v0.0.0-00010101000000-000000000000
	github.com/stretchr/testify v1.7.0
	go.opentelemetry.io/collector v0.31.1-0.20210810171211-8038673eba9e
	go.opentelemetry.io/collector/model v0.31.1-0.20210810171211-8038673eba9e
	go.uber.org/zap v1.19.0
	golang.org/x/tools v0.1.4 // indirect
	google.golang.org/protobuf v1.27.1
	k8s.io/client-go v0.22.0
)

replace github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/metrics => ./../../internal/aws/metrics

replace github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/containerinsight => ./../../internal/aws/containerinsight

replace github.com/open-telemetry/opentelemetry-collector-contrib/internal/k8sconfig => ../../internal/k8sconfig

replace github.com/open-telemetry/opentelemetry-collector-contrib/internal/kubelet => ../../internal/kubelet

replace github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/awsutil => ./../../internal/aws/awsutil

replace github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/k8s => ./../../internal/aws/k8s
