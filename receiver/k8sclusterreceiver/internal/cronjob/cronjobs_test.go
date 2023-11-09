// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package cronjob

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/receiver/receivertest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/golden"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/pmetrictest"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver/internal/testutils"
)

func TestCronJobMetrics(t *testing.T) {
	cj := testutils.NewCronJob("1")

	ts := pcommon.Timestamp(time.Now().UnixNano())
	mb := metadata.NewMetricsBuilder(metadata.DefaultMetricsBuilderConfig(), receivertest.NewNopCreateSettings())
	RecordMetrics(mb, cj, ts)
	m := mb.Emit()

	expected, err := golden.ReadMetrics(filepath.Join("testdata", "expected.yaml"))
	require.NoError(t, err)
	require.NoError(t, pmetrictest.CompareMetrics(expected, m,
		pmetrictest.IgnoreTimestamp(),
		pmetrictest.IgnoreStartTimestamp(),
		pmetrictest.IgnoreResourceMetricsOrder(),
		pmetrictest.IgnoreMetricsOrder(),
		pmetrictest.IgnoreScopeMetricsOrder(),
	),
	)
}

func TestCronJobMetadata(t *testing.T) {
	cj := testutils.NewCronJob("1")

	actualMetadata := GetMetadata(cj)

	require.Equal(t, 1, len(actualMetadata))

	// Assert metadata from Pod.
	require.Equal(t,
		metadata.KubernetesMetadata{
			EntityType:    "k8s.cronjob",
			ResourceIDKey: "k8s.cronjob.uid",
			ResourceID:    "test-cronjob-1-uid",
			Metadata: map[string]string{
				"cronjob.creation_timestamp": "0001-01-01T00:00:00Z",
				"foo":                        "bar",
				"foo1":                       "",
				"schedule":                   "schedule",
				"concurrency_policy":         "concurrency_policy",
				"k8s.workload.kind":          "CronJob",
				"k8s.workload.name":          "test-cronjob-1",
			},
		},
		*actualMetadata["test-cronjob-1-uid"],
	)
}
