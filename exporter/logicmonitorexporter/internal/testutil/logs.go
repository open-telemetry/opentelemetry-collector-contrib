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

package testutil // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/logicmonitorexporter/internal/testutil"

import (
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
)

func CreateLogData(numberOfLogs int) plog.Logs {
	logs := plog.NewLogs()
	logs.ResourceLogs().AppendEmpty() // Add an empty ResourceLogs
	rl := logs.ResourceLogs().AppendEmpty()
	rl.Resource().Attributes().PutStr("service.name", "myapp")
	rl.Resource().Attributes().PutStr("hostname", "myapp")
	rl.Resource().Attributes().PutInt("http.statusCode", 200)
	rl.Resource().Attributes().PutBool("isTest", true)
	rl.Resource().Attributes().PutDouble("value", 20.00)
	rl.Resource().Attributes().PutEmptySlice("values")
	rl.Resource().Attributes().PutEmptyMap("valueMap")
	rl.ScopeLogs().AppendEmpty() // Add an empty ScopeLogs
	ill := rl.ScopeLogs().AppendEmpty()

	for i := 0; i < numberOfLogs; i++ {
		ts := pcommon.Timestamp(int64(i) * time.Millisecond.Nanoseconds())
		logRecord := ill.LogRecords().AppendEmpty()
		logRecord.Body().SetStr("mylog")
		logRecord.Attributes().PutStr("my-label", "myapp-type")
		logRecord.Attributes().PutStr("custom", "custom")
		logRecord.SetTimestamp(ts)
	}
	ill.LogRecords().AppendEmpty()

	return logs
}
