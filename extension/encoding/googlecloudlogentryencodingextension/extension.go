// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package googlecloudlogentryencodingextension // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/googlecloudlogentryencodingextension"

import (
	"bytes"
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
	logs := plog.NewLogs()

	// each line corresponds to a log
	for len(buf) > 0 {
		var line []byte
		if i := bytes.IndexByte(buf, '\n'); i >= 0 {
			line, buf = buf[:i], buf[i+1:]
		} else {
			line, buf = buf, nil
		}
		if n := len(line); n > 0 && line[n-1] == '\r' {
			line = line[:n-1]
		}
		if err := ex.handleLogLine(logs, line); err != nil {
			return plog.Logs{}, err
		}
	}

	return logs, nil
}

func (ex *ext) handleLogLine(logs plog.Logs, logLine []byte) error {
	var log logEntry
	if err := gojson.Unmarshal(logLine, &log); err != nil {
		return fmt.Errorf("failed to unmarshal log entry: %w", err)
	}

	rl := logs.ResourceLogs().AppendEmpty()
	r := rl.Resource()
	scopeLogs := rl.ScopeLogs().AppendEmpty()

	if err := handleLogEntryFields(r.Attributes(), scopeLogs, log, ex.config); err != nil {
		return fmt.Errorf("failed to handle log entry: %w", err)
	}

	return nil
}
