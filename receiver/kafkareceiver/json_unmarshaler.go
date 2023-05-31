// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package kafkareceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/kafkareceiver"
import (
	"encoding/json"
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
)

type jsonLogsUnmarshaler struct {
}

func newJSONLogsUnmarshaler() LogsUnmarshaler {
	return &jsonLogsUnmarshaler{}
}

func (r *jsonLogsUnmarshaler) Unmarshal(buf []byte) (plog.Logs, error) {
	// create a new Logs struct to be populated with log data and returned
	p := plog.NewLogs()

	// get json logs from the buffer
	jsonVal := map[string]interface{}{}
	if err := json.Unmarshal(buf, &jsonVal); err != nil {
		return p, err
	}

	// dig down to the Log Records level of the Logs struct
	logRecords := p.ResourceLogs().AppendEmpty().ScopeLogs().AppendEmpty().LogRecords().AppendEmpty()
	logRecords.SetObservedTimestamp(pcommon.NewTimestampFromTime(time.Now()))

	// Set the unmarshaled jsonVal as the body of the log record
	logRecords.Body().SetEmptyMap().FromRaw(jsonVal)

	return p, nil
}

func (r *jsonLogsUnmarshaler) Encoding() string {
	return "json"
}
