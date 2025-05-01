// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package otelcollector // import "github.com/DataDog/opentelemetry-collector-contrib/e2etests/otelcollector"

import (
	"github.com/DataDog/datadog-agent/test/new-e2e/pkg/components"

	otelcomp "github.com/DataDog/opentelemetry-collector-contrib/e2etests/otelcollector/component"
)

type Kubernetes struct {
	// Components
	KubernetesCluster *components.KubernetesCluster
	FakeIntake        *components.FakeIntake
	OTelCollector     *OTelCollector
}

type OTelCollector struct {
	otelcomp.OTelCollectorOutput
}
