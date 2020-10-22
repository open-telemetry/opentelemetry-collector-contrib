module github.com/open-telemetry/opentelemetry-collector-contrib/extension/jmxmetricextension

go 1.14

require (
	github.com/Sirupsen/logrus v0.0.0-00010101000000-000000000000 // indirect
	github.com/containerd/containerd v1.4.0 // indirect
	github.com/davecgh/go-spew v1.1.1
	github.com/prometheus/common v0.14.0
	github.com/prometheus/prometheus v1.8.2-0.20200827201422-1195cc24e3c8
	github.com/shirou/gopsutil v2.20.6+incompatible
	github.com/stretchr/testify v1.6.1
	github.com/testcontainers/testcontainers-go v0.8.0
	go.opentelemetry.io/collector v0.13.1-0.20201020175630-99cb5b244aad
	go.uber.org/atomic v1.7.0
	go.uber.org/zap v1.16.0
)

replace github.com/open-telemetry/opentelemetry-collector-contrib/internal/common => ../../internal/common

// Needed until testcontainers,docker logrus and docker/errdef dep issues are resolved
replace github.com/Sirupsen/logrus => github.com/sirupsen/logrus v1.2.0

replace github.com/docker/docker => github.com/docker/engine v17.12.0-ce-rc1.0.20200309214505-aa6a9891b09c+incompatible
