// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package datapoints

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
