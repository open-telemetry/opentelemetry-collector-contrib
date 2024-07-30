package otelcollector

import (
	"github.com/DataDog/datadog-agent/test/new-e2e/pkg/components"
	otelcomp "github.com/DataDog/opentelemetry-collector-contrib/e2e-tests/otel-collector/component"
)

type Kubernetes struct {
	// Components
	KubernetesCluster *components.KubernetesCluster
	FakeIntake        *components.FakeIntake
	OtelCollector     *OtelCollector
}

type OtelCollector struct {
	otelcomp.OtelCollectorOutput
}
