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

package sumologicsyslogprocessor

import (
	"context"
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/model/pdata"
)

func TestProcessLogs(t *testing.T) {
	lines := []string{
		`<13> Example log`,
		`<334> Another example log`,
		`Plain text`,
	}

	facilities := []string{
		`user-level messages`,
		`syslog`,
		`syslog`,
	}

	logs := pdata.NewLogs()
	rls := logs.ResourceLogs().AppendEmpty()
	rls.InstrumentationLibraryLogs().EnsureCapacity(len(lines))
	ills := rls.InstrumentationLibraryLogs().AppendEmpty()

	for _, line := range lines {
		lr := ills.Logs().AppendEmpty()
		lr.Body().SetStringVal(line)
	}
	ills.Logs().At(1).Attributes().InsertString("facility_name", "pre filled facility")

	ctx := context.Background()
	processor := &sumologicSyslogProcessor{
		syslogFacilityAttrName: "facility_name",
		syslogFacilityRegex:    regexp.MustCompile(`^<(?P<number>\d+)>`),
	}

	result, err := processor.ProcessLogs(ctx, logs)
	require.NoError(t, err)

	for i, line := range facilities {
		attrs := result.ResourceLogs().At(0).InstrumentationLibraryLogs().At(0).Logs().At(i).Attributes()
		attr, ok := attrs.Get("facility_name")
		require.True(t, ok)
		assert.Equal(t, line, attr.StringVal())
	}
}
