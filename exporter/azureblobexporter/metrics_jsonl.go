// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package azureblobexporter

import (
	"bytes"

	"go.opentelemetry.io/collector/pdata/pmetric"
)

type metricsJSONLMarshaler struct{}

func (*metricsJSONLMarshaler) MarshalMetrics(md pmetric.Metrics) ([]byte, error) {
	marshaler := &pmetric.JSONMarshaler{}
	var buf bytes.Buffer

	rms := md.ResourceMetrics()

	for i := 0; i < rms.Len(); i++ {
		rm := rms.At(i)
		resource := rm.Resource()

		sms := rm.ScopeMetrics()
		for j := 0; j < sms.Len(); j++ {
			sm := sms.At(j)
			scope := sm.Scope()

			metrics := sm.Metrics()
			for k := 0; k < metrics.Len(); k++ {
				metric := metrics.At(k)

				tempMd := pmetric.NewMetrics()
				tempRm := tempMd.ResourceMetrics().AppendEmpty()
				resource.CopyTo(tempRm.Resource())
				tempSm := tempRm.ScopeMetrics().AppendEmpty()
				scope.CopyTo(tempSm.Scope())
				metric.CopyTo(tempSm.Metrics().AppendEmpty())

				jsonBytes, err := marshaler.MarshalMetrics(tempMd)
				if err != nil {
					return nil, err
				}

				// Remove null values to prevent issues with downstream consumers
				cleanedJSON := removeNullValues(jsonBytes)
				buf.Write(cleanedJSON)
				buf.WriteByte('\n')
			}
		}
	}

	return buf.Bytes(), nil
}
