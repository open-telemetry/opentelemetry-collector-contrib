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

package instrumentationlibrary

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/model/pdata"
)

func TestTagsFromInstrumentationLibraryMetadata(t *testing.T) {
	tests := []struct {
		name         string
		version      string
		expectedTags []string
	}{
		{"test-il", "1.0.0", []string{fmt.Sprintf("%s:%s", instrumentationLibraryTag, "test-il"), fmt.Sprintf("%s:%s", instrumentationLibraryVersionTag, "1.0.0")}},
		{"test-il", "", []string{fmt.Sprintf("%s:%s", instrumentationLibraryTag, "test-il"), fmt.Sprintf("%s:%s", instrumentationLibraryVersionTag, "n/a")}},
		{"", "1.0.0", []string{fmt.Sprintf("%s:%s", instrumentationLibraryTag, "n/a"), fmt.Sprintf("%s:%s", instrumentationLibraryVersionTag, "1.0.0")}},
		{"", "", []string{fmt.Sprintf("%s:%s", instrumentationLibraryTag, "n/a"), fmt.Sprintf("%s:%s", instrumentationLibraryVersionTag, "n/a")}},
	}

	for _, testInstance := range tests {
		il := pdata.NewInstrumentationLibrary()
		il.SetName(testInstance.name)
		il.SetVersion(testInstance.version)
		tags := TagsFromInstrumentationLibraryMetadata(il)

		assert.ElementsMatch(t, testInstance.expectedTags, tags)
	}
}
