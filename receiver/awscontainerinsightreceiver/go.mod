module github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awscontainerinsightreceiver

go 1.16

require (
	github.com/aws/aws-sdk-go v1.38.64
	github.com/google/cadvisor v0.39.2
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/awsutil v0.0.0-00010101000000-000000000000
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/containerinsight v0.0.0-00010101000000-000000000000
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/k8s v0.0.0-00010101000000-000000000000
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/metrics v0.0.0-00010101000000-000000000000
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/k8sconfig v0.0.0-00010101000000-000000000000
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/kubelet v0.0.0-00010101000000-000000000000
	github.com/shirou/gopsutil v3.21.5+incompatible
	github.com/stretchr/testify v1.7.0
	go.opentelemetry.io/collector v0.28.1-0.20210616151306-cdc163427b8e
	go.uber.org/zap v1.17.0
	k8s.io/api v0.21.1
	k8s.io/apimachinery v0.21.1
	k8s.io/client-go v0.21.1
	k8s.io/klog v1.0.0
)

replace github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/metrics => ./../../internal/aws/metrics

replace github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/containerinsight => ./../../internal/aws/containerinsight

replace github.com/open-telemetry/opentelemetry-collector-contrib/internal/k8sconfig => ../../internal/k8sconfig

replace github.com/open-telemetry/opentelemetry-collector-contrib/internal/kubelet => ../../internal/kubelet

replace github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/awsutil => ./../../internal/aws/awsutil

replace github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/k8s => ./../../internal/aws/k8s
