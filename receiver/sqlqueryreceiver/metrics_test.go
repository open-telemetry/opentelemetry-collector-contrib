// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sqlqueryreceiver

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/pmetric"
)

func TestSetDataPointValue(t *testing.T) {
	err := setDataPointValue(MetricCfg{
		ValueType:   MetricValueTypeInt,
		ValueColumn: "some-col",
	}, "", pmetric.NewNumberDataPoint())
	assert.EqualError(t, err, `setDataPointValue: col "some-col": error converting to integer: strconv.Atoi: parsing "": invalid syntax`)
}
