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

package testdata

import "go.opentelemetry.io/collector/pdata/pcommon"

const (
	TestLabelKey1       = "label-1"
	TestLabelValue1     = "label-value-1"
	TestLabelKey2       = "label-2"
	TestLabelValue2     = "label-value-2"
	TestLabelKey3       = "label-3"
	TestLabelValue3     = "label-value-3"
	TestAttachmentKey   = "exemplar-attachment"
	TestAttachmentValue = "exemplar-attachment-value"
)

func initMetricAttachment(dest pcommon.Map) {
	dest.PutStr(TestAttachmentKey, TestAttachmentValue)
}

func initMetricAttributes1(dest pcommon.Map) {
	dest.PutStr(TestLabelKey1, TestLabelValue1)
}

func initMetricAttributes12(dest pcommon.Map) {
	dest.PutStr(TestLabelKey1, TestLabelValue1)
	dest.PutStr(TestLabelKey2, TestLabelValue2)
	dest.Sort()
}

func initMetricAttributes13(dest pcommon.Map) {
	dest.PutStr(TestLabelKey1, TestLabelValue1)
	dest.PutStr(TestLabelKey3, TestLabelValue3)
	dest.Sort()
}

func initMetricAttributes2(dest pcommon.Map) {
	dest.PutStr(TestLabelKey2, TestLabelValue2)
}
