// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package azureblobexporter

import (
	"bytes"

	"go.opentelemetry.io/collector/pdata/plog"
)

type logsJSONLMarshaler struct{}

func (*logsJSONLMarshaler) MarshalLogs(ld plog.Logs) ([]byte, error) {
	marshaler := &plog.JSONMarshaler{}
	var buf bytes.Buffer

	rls := ld.ResourceLogs()

	for i := 0; i < rls.Len(); i++ {
		rl := rls.At(i)
		resource := rl.Resource()

		sls := rl.ScopeLogs()
		for j := 0; j < sls.Len(); j++ {
			sl := sls.At(j)
			scope := sl.Scope()

			lrs := sl.LogRecords()
			for k := 0; k < lrs.Len(); k++ {
				lr := lrs.At(k)

				tempLd := plog.NewLogs()
				tempRl := tempLd.ResourceLogs().AppendEmpty()
				resource.CopyTo(tempRl.Resource())
				tempSl := tempRl.ScopeLogs().AppendEmpty()
				scope.CopyTo(tempSl.Scope())
				lr.CopyTo(tempSl.LogRecords().AppendEmpty())

				jsonBytes, err := marshaler.MarshalLogs(tempLd)
				if err != nil {
					return nil, err
				}

				buf.Write(jsonBytes)
				buf.WriteByte('\n')
			}
		}
	}

	return buf.Bytes(), nil
}
