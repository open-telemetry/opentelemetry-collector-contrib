// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

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
