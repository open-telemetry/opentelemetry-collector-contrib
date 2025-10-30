// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package googlecloudlogentryencodingextension // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/googlecloudlogentryencodingextension"

import (
	"bufio"
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
	scanner := bufio.NewScanner(bytes.NewReader(buf))
	for scanner.Scan() {
		line := scanner.Bytes()
		if err := ex.handleLogLine(logs, line); err != nil {
			return plog.Logs{}, err
		}
	}

	if err := scanner.Err(); err != nil {
		return plog.Logs{}, fmt.Errorf("error reading log: %w", err)
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
