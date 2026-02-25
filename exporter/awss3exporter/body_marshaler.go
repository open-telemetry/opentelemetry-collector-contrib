// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awss3exporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/awss3exporter"

import (
	"bytes"
	"fmt"

	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

type bodyMarshaler struct{}

func (*bodyMarshaler) format() string {
	return "txt"
}

func newbodyMarshaler() bodyMarshaler {
	return bodyMarshaler{}
}

func (bodyMarshaler) MarshalLogs(ld plog.Logs) ([]byte, error) {
	buf := bytes.Buffer{}
	rls := ld.ResourceLogs()
	for i := 0; i < rls.Len(); i++ {
		rl := rls.At(i)

		ills := rl.ScopeLogs()
		for j := 0; j < ills.Len(); j++ {
			ils := ills.At(j)
			logs := ils.LogRecords()
			for k := 0; k < logs.Len(); k++ {
				lr := logs.At(k)
				body := lr.Body()
				buf.WriteString(body.AsString())
				buf.WriteString("\n")
			}
		}
	}
	return buf.Bytes(), nil
}

func (s bodyMarshaler) MarshalTraces(_ ptrace.Traces) ([]byte, error) {
	return nil, fmt.Errorf("traces can't be marshaled into %s format", s.format())
}

func (s bodyMarshaler) MarshalMetrics(_ pmetric.Metrics) ([]byte, error) {
	return nil, fmt.Errorf("metrics can't be marshaled into %s format", s.format())
}
