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

package splunkhecreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/splunkhecreceiver"

import (
	"bufio"
	"net/url"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/splunk"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
)

const (
	// splunk metadata
	index      = "index"
	source     = "source"
	sourcetype = "sourcetype"
	host       = "host"
)

// SplunkHecRawToLogData transforms raw splunk event into log
func splunkHecRawToLogData(sc *bufio.Scanner, query url.Values, resourceCustomizer func(pcommon.Resource), config *splunk.HecToOtelAttrs) (plog.Logs, int) {
	ld := plog.NewLogs()
	rl := ld.ResourceLogs().AppendEmpty()
	appendSplunkMetadata(rl, config, query.Get(host), query.Get(source), query.Get(sourcetype), query.Get(index))
	if resourceCustomizer != nil {
		resourceCustomizer(rl.Resource())
	}

	sl := rl.ScopeLogs().AppendEmpty()
	for sc.Scan() {
		logRecord := sl.LogRecords().AppendEmpty()
		logLine := sc.Text()
		logRecord.Body().SetStr(logLine)
	}

	return ld, sl.LogRecords().Len()
}

func appendSplunkMetadata(rl plog.ResourceLogs, attrs *splunk.HecToOtelAttrs, host, source, sourceType, index string) {
	if host != "" {
		rl.Resource().Attributes().PutStr(attrs.Host, host)
	}
	if source != "" {
		rl.Resource().Attributes().PutStr(attrs.Source, source)
	}
	if sourceType != "" {
		rl.Resource().Attributes().PutStr(attrs.SourceType, sourceType)
	}
	if index != "" {
		rl.Resource().Attributes().PutStr(attrs.Index, index)
	}
}
