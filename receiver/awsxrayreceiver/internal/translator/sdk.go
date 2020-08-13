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
	"strings"

	"go.opentelemetry.io/collector/consumer/pdata"
	"go.opentelemetry.io/collector/translator/conventions"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/common/awsxray"
)

func populateInstrumentationLibrary(seg *awsxray.Segment, ils *pdata.InstrumentationLibrarySpans) {
	if seg.AWS != nil && seg.AWS.XRay != nil {
		xr := seg.AWS.XRay
		il := ils.InstrumentationLibrary()
		il.InitEmpty()
		il.SetName(*xr.SDK)
		il.SetVersion(*xr.SDKVersion)
	}
}

func addSdkToResource(seg *awsxray.Segment, attrs *pdata.AttributeMap) {
	if seg.AWS != nil && seg.AWS.XRay != nil {
		xr := seg.AWS.XRay
		addString(xr.SDKVersion, conventions.AttributeTelemetrySDKVersion, attrs)
		if xr.SDK != nil {
			attrs.UpsertString(conventions.AttributeTelemetrySDKName, *xr.SDK)
			if seg.Cause != nil && len(seg.Cause.Exceptions) > 0 {
				// https://github.com/open-telemetry/opentelemetry-collector-contrib/blob/master/exporter/awsxrayexporter/translator/cause.go#L163
				// x-ray exporter only supports Java stack trace for now
				// TODO: Update this once the exporter is more flexible
				attrs.UpsertString(conventions.AttributeTelemetrySDKLanguage, "java")
			} else {
				res := strings.Split(*xr.SDK, "for ")
				if len(res) == 2 {
					attrs.UpsertString(conventions.AttributeTelemetrySDKLanguage, res[1])
				}
			}
		}
	}
}
