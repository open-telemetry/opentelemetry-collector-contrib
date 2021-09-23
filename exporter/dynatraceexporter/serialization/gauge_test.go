// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package serialization

import (
	"testing"
	"time"

	"github.com/dynatrace-oss/dynatrace-metric-utils-go/metric/dimensions"
	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/model/pdata"
)

func Test_serializeGauge(t *testing.T) {
	t.Run("float with prefix and dimension", func(t *testing.T) {
		dp := pdata.NewNumberDataPoint()
		dp.SetDoubleVal(5.5)
		dp.SetTimestamp(pdata.Timestamp(time.Date(2021, 07, 16, 12, 30, 0, 0, time.UTC).UnixNano()))

		got, err := serializeGauge("dbl_gauge", "prefix", dimensions.NewNormalizedDimensionList(dimensions.NewDimension("key", "value")), dp)
		assert.NoError(t, err)
		assert.Equal(t, "prefix.dbl_gauge,key=value gauge,5.5 1626438600000", got)
	})

	t.Run("int with prefix and dimension", func(t *testing.T) {
		dp := pdata.NewNumberDataPoint()
		dp.SetIntVal(5)
		dp.SetTimestamp(pdata.Timestamp(time.Date(2021, 07, 16, 12, 30, 0, 0, time.UTC).UnixNano()))

		got, err := serializeGauge("int_gauge", "prefix", dimensions.NewNormalizedDimensionList(dimensions.NewDimension("key", "value")), dp)
		assert.NoError(t, err)
		assert.Equal(t, "prefix.int_gauge,key=value gauge,5 1626438600000", got)
	})

	t.Run("without timestamp", func(t *testing.T) {
		dp := pdata.NewNumberDataPoint()
		dp.SetIntVal(5)

		got, err := serializeGauge("int_gauge", "prefix", dimensions.NewNormalizedDimensionList(), dp)
		assert.NoError(t, err)
		assert.Equal(t, "prefix.int_gauge gauge,5", got)
	})
}
