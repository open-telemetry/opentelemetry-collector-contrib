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
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/common/awsxray"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/common/awsxray/util"
)

func TestConvertStackFramesToStackTraceStr(t *testing.T) {
	excp := awsxray.Exception{
		Type:    util.String("exceptionType"),
		Message: util.String("exceptionMessage"),
		Stack: []awsxray.StackFrame{
			{
				Path:  util.String("path0"),
				Line:  util.Int(10),
				Label: util.String("label0"),
			},
			{
				Path:  util.String("path1"),
				Line:  util.Int(11),
				Label: util.String("label1"),
			},
		},
	}
	actual := convertStackFramesToStackTraceStr(excp)
	assert.Equal(t, actual, "exceptionType: exceptionMessage\n\tat label0(path0: 10)\n\tat label1(path1: 11)\n")
}
