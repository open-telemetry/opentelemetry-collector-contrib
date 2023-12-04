// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package hierarchicalresourcequota

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver/receivertest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/golden"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/pmetrictest"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver/internal/testutils"
)

func TestHierarchicalRequestQuotaMetrics(t *testing.T) {
	hrq := testutils.NewHierarchicalResourceQuota("1")
	ts := pcommon.Timestamp(time.Now().UnixNano())
	mbc := metadata.DefaultMetricsBuilderConfig()
	mb := metadata.NewMetricsBuilder(mbc, receivertest.NewNopCreateSettings())
	RecordMetrics(mb, hrq, ts)
	m := mb.Emit()

	expected := pmetric.NewMetrics()
	require.NoError(t, pmetrictest.CompareMetrics(expected, m))
}

func TestHierarchicalRequestQuotaMetricsEnabled(t *testing.T) {
	hrq := testutils.NewHierarchicalResourceQuota("1")
	ts := pcommon.Timestamp(time.Now().UnixNano())
	mbc := metadata.DefaultMetricsBuilderConfig()
	mbc.ResourceAttributes.K8sHierarchicalresourcequotaName.Enabled = true
	mbc.ResourceAttributes.K8sHierarchicalresourcequotaUID.Enabled = true
	mbc.Metrics.K8sHierarchicalResourceQuotaHardLimit.Enabled = true
	mbc.Metrics.K8sHierarchicalResourceQuotaUsed.Enabled = true
	mb := metadata.NewMetricsBuilder(mbc, receivertest.NewNopCreateSettings())
	RecordMetrics(mb, hrq, ts)
	m := mb.Emit()

	expected, err := golden.ReadMetrics(filepath.Join("testdata", "expected.yaml"))
	require.NoError(t, err)
	require.NoError(t, pmetrictest.CompareMetrics(expected, m,
		pmetrictest.IgnoreTimestamp(),
		pmetrictest.IgnoreStartTimestamp(),
		pmetrictest.IgnoreResourceMetricsOrder(),
		pmetrictest.IgnoreMetricsOrder(),
		pmetrictest.IgnoreScopeMetricsOrder(),
		pmetrictest.IgnoreMetricDataPointsOrder(),
	),
	)
}
