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

package translator // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awsxrayreceiver/internal/translator"

import (
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

func addStartTime(startTime *float64, span *ptrace.Span) {
	span.SetStartTimestamp(floatSecToNanoEpoch(startTime))
}

func addEndTime(endTime *float64, span *ptrace.Span) {
	if endTime != nil {
		span.SetEndTimestamp(floatSecToNanoEpoch(endTime))
	}
}

func floatSecToNanoEpoch(epochSec *float64) pcommon.Timestamp {
	return pcommon.Timestamp((*epochSec) * float64(time.Second))
}
