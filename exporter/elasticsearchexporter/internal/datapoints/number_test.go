// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package datapoints

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/pmetric"
)

func TestNumberTemporality(t *testing.T) {
	m := pmetric.NewMetric()
	m.SetName("test")

	dp := pmetric.NewNumberDataPoint()
	dp.SetDoubleValue(30.0)

	esSum := NewNumber(m, dp)

	actual := esSum.Temporality()
	assert.Empty(t, actual, "number doesn't have a temporality")
}
