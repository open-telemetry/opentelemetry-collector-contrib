// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build integration

package kubeletstatsreceiver

// These tests validate the metrics that the receiver emits against the
// OpenTelemetry semantic conventions, using a Weaver live-check container via
// pkg/semconvtest. The check runs against the released semantic-conventions
// version that the receiver declares as sem_conv_version in metadata.yaml,
// pinned with semconvtest.WithRegistry, so that unreleased changes on the
// main branch of the registry do not affect the result.

import (
	"slices"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver/receivertest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/semconvtest"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/kubeletstatsreceiver/internal/kubelet"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/kubeletstatsreceiver/internal/metadata"
)

// semconvRegistry must match sem_conv_version in metadata.yaml.
const semconvRegistry = "https://github.com/open-telemetry/semantic-conventions.git@v1.43.0[model]"

// semconvCompliantMetrics lists the receiver metrics whose name, type, unit
// and attributes already match a release_candidate (or stable) definition
// in a released version of the semantic conventions. Metrics that still need
// a migration (type change, rename) are intentionally left out and must be
// added here once they are promoted.
var semconvCompliantMetrics = []string{
	"container.cpu.time",
}

// TestSemconvComplianceContainerMetrics checks the container metrics, and the
// resource attributes they carry, against semantic conventions.
func TestSemconvComplianceContainerMetrics(t *testing.T) {
	r, err := newKubeletScraper(
		&fakeRestClient{},
		receivertest.NewNopSettings(metadata.Type),
		&scraperOptions{
			metricGroupsToCollect: map[kubelet.MetricGroup]bool{
				kubelet.ContainerMetricGroup: true,
			},
		},
		metadata.NewDefaultMetricsBuilderConfig(),
		"worker-42",
	)
	require.NoError(t, err)

	md, err := r.ScrapeMetrics(t.Context())
	require.NoError(t, err)

	keepSemconvCompliantMetrics(md)
	require.Positive(t, md.MetricCount(), "expected at least one semconv-compliant metric")

	semconvtest.TestMetrics(t, md, semconvtest.WithRegistry(semconvRegistry))
}

// keepSemconvCompliantMetrics drops every metric that is not listed in
// semconvCompliantMetrics, along with the scopes and resources left empty.
func keepSemconvCompliantMetrics(md pmetric.Metrics) {
	md.ResourceMetrics().RemoveIf(func(rm pmetric.ResourceMetrics) bool {
		rm.ScopeMetrics().RemoveIf(func(sm pmetric.ScopeMetrics) bool {
			sm.Metrics().RemoveIf(func(m pmetric.Metric) bool {
				return !slices.Contains(semconvCompliantMetrics, m.Name())
			})
			return sm.Metrics().Len() == 0
		})
		return rm.ScopeMetrics().Len() == 0
	})
}
