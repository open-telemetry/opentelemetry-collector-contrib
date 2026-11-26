// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package datapoints // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/elasticsearchexporter/internal/datapoints"

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/pmetric"
)

func TestSummaryTemporality(t *testing.T) {
	m := pmetric.NewMetric()
	m.SetName("test")

	dp := pmetric.NewSummaryDataPoint()
	dp.SetCount(10)

	esSummary := NewSummary(m, dp)

	actual := esSummary.Temporality()
	assert.Empty(t, actual, "summary doesn't have a temporality")
}

// Summary doesn't contain Exemplars so here we are verifying its empty
func TestSummaryExemplars(t *testing.T) {
	m := pmetric.NewMetric()
	m.SetName("test")

	dp := pmetric.NewSummaryDataPoint()
	dp.SetCount(10)

	esHist := NewSummary(m, dp)
	actual := esHist.Exemplars()
	assert.Equal(t, 0, actual.Len())
}
