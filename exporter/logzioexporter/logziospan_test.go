// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package logzioexporter

import (
	"encoding/json"
	"fmt"
	"os"
	"testing"

	"github.com/jaegertracing/jaeger/model"
	"github.com/stretchr/testify/require"
)

func TestTransformToLogzioSpanBytes(tester *testing.T) {
	inStr, err := os.ReadFile("./testdata/span.json")
	require.NoError(tester, err, "error opening sample span file")

	var span model.Span
	err = json.Unmarshal(inStr, &span)
	if err != nil {
		fmt.Println("json.Unmarshal")
	}
	newSpan, err := transformToLogzioSpanBytes(&span)
	require.NoError(tester, err)
	m := make(map[string]any)
	err = json.Unmarshal(newSpan, &m)
	require.NoError(tester, err)
	if _, ok := m["JaegerTag"]; !ok {
		tester.Error("error converting span to logzioSpan, JaegerTag is not found")
	}
}

func TestTransformToDbModelSpan(tester *testing.T) {
	inStr, err := os.ReadFile("./testdata/span.json")
	require.NoError(tester, err, "error opening sample span file")
	var span model.Span
	err = json.Unmarshal(inStr, &span)
	if err != nil {
		fmt.Println("json.Unmarshal")
	}
	newSpan, err := transformToLogzioSpanBytes(&span)
	require.NoError(tester, err)
	var testLogzioSpan logzioSpan
	err = json.Unmarshal(newSpan, &testLogzioSpan)
	require.NoError(tester, err)
	dbModelSpan := testLogzioSpan.transformToDbModelSpan()
	require.Len(tester, dbModelSpan.References, 3, "Error converting logzio span to dbmodel span")
}
