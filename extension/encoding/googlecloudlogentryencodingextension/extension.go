// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package googlecloudlogentryencodingextension // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/googlecloudlogentryencodingextension"

import (
	"context"
	"fmt"

	gojson "github.com/goccy/go-json"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/plog"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding"
)

var _ encoding.LogsUnmarshalerExtension = (*ext)(nil)

type ext struct {
	config Config
}

func newExtension(cfg *Config) *ext {
	return &ext{config: *cfg}
}

func (*ext) Start(_ context.Context, _ component.Host) error {
	return nil
}

func (*ext) Shutdown(context.Context) error {
	return nil
}

func (ex *ext) UnmarshalLogs(buf []byte) (plog.Logs, error) {
	logs, err := ex.translateLogEntry(buf)
	if err != nil {
		return plog.Logs{}, err
	}

	return logs, nil
}

// TranslateLogEntry translates a JSON-encoded LogEntry message into plog.Logs,
// trying to keep as close as possible to the GCP original names.
//
// For maximum fidelity, the decoding is done according to the protobuf message
// schema; this ensures that a numeric value in the input is correctly
// translated to either an integer or a double in the output. It falls back to
// plain JSON decoding if payload type is not available in the proto registry.
func (ex *ext) translateLogEntry(data []byte) (plog.Logs, error) {
	var log logEntry
	if err := gojson.Unmarshal(data, &log); err != nil {
		return plog.Logs{}, fmt.Errorf("failed to unmarshal log entry: %w", err)
	}

	logs := plog.NewLogs()
	resourceLogs := logs.ResourceLogs().AppendEmpty()
	resource := resourceLogs.Resource()
	logRecord := resourceLogs.ScopeLogs().AppendEmpty().LogRecords().AppendEmpty()

	if err := handleLogEntryFields(resource.Attributes(), logRecord, log, ex.config); err != nil {
		return plog.Logs{}, fmt.Errorf("failed to handle log entry: %w", err)
	}
	return logs, nil
}
