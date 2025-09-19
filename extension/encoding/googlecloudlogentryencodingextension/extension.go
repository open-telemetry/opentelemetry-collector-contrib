// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package googlecloudlogentryencodingextension // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/googlecloudlogentryencodingextension"

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"

	gojson "github.com/goccy/go-json"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/plog"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding"
)

var (
	_ encoding.LogsUnmarshalerExtension          = (*ext)(nil)
	_ encoding.StreamingLogsUnmarshalerExtension = (*ext)(nil)
)

type ext struct {
	config Config
}

type logsDecoder struct {
	scanner *bufio.Scanner
	ext     *ext
}

var _ encoding.LogsDecoder = (*logsDecoder)(nil)

func (ld *logsDecoder) DecodeLogs(to plog.Logs) error {
	if !ld.scanner.Scan() {
		if err := ld.scanner.Err(); err != nil {
			return fmt.Errorf("error reading log: %w", err)
		}
		return io.EOF
	}
	line := ld.scanner.Bytes()
	return ld.ext.handleLogLine(to, line)
}

func (ex *ext) NewLogsDecoder(r io.Reader) (encoding.LogsDecoder, error) {
	return &logsDecoder{
		scanner: bufio.NewScanner(r),
		ext:     ex,
	}, nil
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
	logRecord := rl.ScopeLogs().AppendEmpty().LogRecords().AppendEmpty()
	if err := handleLogEntryFields(r.Attributes(), logRecord, log, ex.config); err != nil {
		return fmt.Errorf("failed to handle log entry: %w", err)
	}

	return nil
}
