module github.com/open-telemetry/opentelemetry-collector-contrib/receiver/jmxreceiver

go 1.14

require (
	github.com/containerd/containerd v1.4.0 // indirect
	github.com/shirou/gopsutil v3.20.10+incompatible
	github.com/stretchr/testify v1.6.1
	github.com/testcontainers/testcontainers-go v0.8.0
	go.opentelemetry.io/collector v0.15.1-0.20201120151746-8ceddba7ea03
	go.uber.org/atomic v1.7.0
	go.uber.org/zap v1.16.0
)

replace github.com/open-telemetry/opentelemetry-collector-contrib/internal/common => ../../internal/common

// Needed until testcontainers,docker logrus and docker/errdef dep issues are resolved
replace github.com/Sirupsen/logrus => github.com/sirupsen/logrus v1.2.0

replace github.com/docker/docker => github.com/docker/engine v17.12.0-ce-rc1.0.20200309214505-aa6a9891b09c+incompatible
