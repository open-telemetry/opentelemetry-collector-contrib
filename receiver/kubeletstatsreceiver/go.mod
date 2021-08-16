module github.com/open-telemetry/opentelemetry-collector-contrib/receiver/kubeletstatsreceiver

go 1.16

require (
	github.com/census-instrumentation/opencensus-proto v0.3.0
	github.com/containerd/containerd v1.4.4 // indirect
	github.com/docker/docker v20.10.8+incompatible // indirect
	github.com/onsi/ginkgo v1.14.1 // indirect
	github.com/onsi/gomega v1.10.2 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/interval v0.0.0-00010101000000-000000000000
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/k8sconfig v0.0.0-00010101000000-000000000000
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/kubelet v0.0.0-00010101000000-000000000000
	github.com/stretchr/testify v1.7.0
	go.opentelemetry.io/collector v0.31.1-0.20210810171211-8038673eba9e
	go.opentelemetry.io/collector/model v0.31.1-0.20210810171211-8038673eba9e
	go.uber.org/zap v1.19.0
	google.golang.org/protobuf v1.27.1
	gopkg.in/yaml.v2 v2.4.0
	k8s.io/api v0.22.0
	k8s.io/apimachinery v0.22.0
	k8s.io/client-go v0.22.0
	k8s.io/kubelet v0.22.0
)

replace (
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/common => ../../internal/common
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/interval => ../../internal/interval
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/k8sconfig => ../../internal/k8sconfig
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/kubelet => ../../internal/kubelet
)
