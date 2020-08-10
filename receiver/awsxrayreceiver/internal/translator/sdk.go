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

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awsxrayreceiver/internal/tracesegment"
)

func populateInstrumentationLibrary(seg *tracesegment.Segment, ils *pdata.InstrumentationLibrarySpans) {
	if seg.AWS != nil && seg.AWS.XRay != nil {
		xr := seg.AWS.XRay
		il := ils.InstrumentationLibrary()
		il.InitEmpty()
		il.SetName(*xr.SDK)
		il.SetVersion(*xr.SDKVersion)
	}
}
