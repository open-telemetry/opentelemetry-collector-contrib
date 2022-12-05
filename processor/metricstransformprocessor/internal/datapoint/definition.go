// Copyright  The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package datapoint

import (
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
)

// Datapoint represents the lowest common interface that is shared
// across each implemented pmetric type.
type Datapoint interface {
	Attributes() pcommon.Map
}

var (
	_ Datapoint = (*pmetric.NumberDataPoint)(nil)
	_ Datapoint = (*pmetric.HistogramDataPoint)(nil)
	_ Datapoint = (*pmetric.ExponentialHistogramDataPoint)(nil)
	_ Datapoint = (*pmetric.SummaryDataPoint)(nil)
)
