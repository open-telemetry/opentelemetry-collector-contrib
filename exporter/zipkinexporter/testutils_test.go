// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package zipkinexporter

import (
	"encoding/json"
	"testing"

	zipkinmodel "github.com/openzipkin/zipkin-go/model"
	"github.com/stretchr/testify/require"
)

func unmarshalZipkinSpanArrayToMap(t *testing.T, jsonStr string) map[zipkinmodel.ID]*zipkinmodel.SpanModel {
	var i any

	err := json.Unmarshal([]byte(jsonStr), &i)
	require.NoError(t, err)

	results := make(map[zipkinmodel.ID]*zipkinmodel.SpanModel)

	switch x := i.(type) {
	case []any:
		for _, j := range x {
			span := jsonToSpan(t, j)
			results[span.ID] = span
		}
	default:
		span := jsonToSpan(t, x)
		results[span.ID] = span
	}
	return results
}

func jsonToSpan(t *testing.T, j any) *zipkinmodel.SpanModel {
	b, err := json.Marshal(j)
	require.NoError(t, err)
	span := &zipkinmodel.SpanModel{}
	err = span.UnmarshalJSON(b)
	require.NoError(t, err)
	return span
}
