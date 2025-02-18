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
	for i := range rls.Len() {
		rl := rls.At(i)

		ills := rl.ScopeLogs()
		for j := range ills.Len() {
			ils := ills.At(j)
			logs := ils.LogRecords()
			for k := range logs.Len() {
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
