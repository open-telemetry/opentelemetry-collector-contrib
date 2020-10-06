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

package translator

import (
	"go.opentelemetry.io/collector/consumer/pdata"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/awsxray"
)

func addParentSpanID(seg *awsxray.Segment, parentID *string, span *pdata.Span) {
	if parentID != nil {
		// `seg` is an embedded subsegment. Please refer to:
		// https://docs.aws.amazon.com/xray/latest/devguide/xray-api-segmentdocuments.html#api-segmentdocuments-subsegments
		// for the difference between an embedded and an independent subsegment.
		span.SetParentSpanID(pdata.NewSpanID([]byte(*parentID)))
	} else if seg.ParentID != nil {
		// `seg` is an independent subsegment
		span.SetParentSpanID(pdata.NewSpanID([]byte(*seg.ParentID)))
	}
	// else: `seg` is the root segment with no parent segment.
}
