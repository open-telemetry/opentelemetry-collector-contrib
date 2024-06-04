// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package batch // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/awskinesisexporter/internal/batch"

import (
	"bytes"

	"go.opentelemetry.io/collector/pdata/plog"
)

type rawMarshaler struct{}

// TODO: implement this func, raw all or raw specific attribute
func (*rawMarshaler) MarshalLogs(ld plog.Logs) ([]byte, error) {
	buf := bytes.Buffer{}
	return buf.Bytes(), nil
}

var _ plog.Unmarshaler = (*rawUnmarshaler)(nil)

type rawUnmarshaler struct{}

// TODO: Implement this func
func (*rawUnmarshaler) UnmarshalLogs(buf []byte) (plog.Logs, error) {
	ld := plog.NewLogs()
	return ld, nil
}
