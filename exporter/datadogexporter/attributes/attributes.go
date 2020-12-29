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

package attributes

import (
	"go.opentelemetry.io/collector/consumer/pdata"
	"go.opentelemetry.io/collector/translator/conventions"
)

// TagsFromAttributes converts a selected list of attributes
// to a tag list that can be added to metrics.
func TagsFromAttributes(attrs pdata.AttributeMap) []string {
	tags := make([]string, 0, attrs.Len())

	var processAttributes processAttributes
	var systemAttributes systemAttributes

	attrs.ForEach(func(key string, value pdata.AttributeValue) {
		switch key {
		// Process attributes
		case conventions.AttributeProcessExecutableName:
			processAttributes.ExecutableName = value.StringVal()
		case conventions.AttributeProcessExecutablePath:
			processAttributes.ExecutablePath = value.StringVal()
		case conventions.AttributeProcessCommand:
			processAttributes.Command = value.StringVal()
		case conventions.AttributeProcessCommandLine:
			processAttributes.CommandLine = value.StringVal()
		case conventions.AttributeProcessID:
			processAttributes.PID = value.IntVal()
		case conventions.AttributeProcessOwner:
			processAttributes.Owner = value.StringVal()

		// System attributes
		case conventions.AttributeOSType:
			systemAttributes.OSType = value.StringVal()
		}
	})

	tags = append(tags, processAttributes.extractTags()...)
	tags = append(tags, systemAttributes.extractTags()...)

	return tags
}
