// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package logzioexporter

import (
	"encoding/json"
	"fmt"
	"os"
	"testing"

	"github.com/jaegertracing/jaeger/model"
)

func TestTransformToLogzioSpanBytes(tester *testing.T) {
	inStr, err := os.ReadFile("./testdata/span.json")
	if err != nil {
		tester.Fatalf(fmt.Sprintf("error opening sample span file %s", err.Error()))
	}

	var span model.Span
	err = json.Unmarshal(inStr, &span)
	if err != nil {
		fmt.Println("json.Unmarshal")
	}
	newSpan, err := transformToLogzioSpanBytes(&span)
	if err != nil {
		tester.Fatalf(err.Error())
	}
	m := make(map[string]any)
	err = json.Unmarshal(newSpan, &m)
	if err != nil {
		tester.Fatalf(err.Error())
	}
	if _, ok := m["JaegerTag"]; !ok {
		tester.Error("error converting span to logzioSpan, JaegerTag is not found")
	}
}

func TestTransformToDbModelSpan(tester *testing.T) {
	inStr, err := os.ReadFile("./testdata/span.json")
	if err != nil {
		tester.Fatalf(fmt.Sprintf("error opening sample span file %s", err.Error()))
	}
	var span model.Span
	err = json.Unmarshal(inStr, &span)
	if err != nil {
		fmt.Println("json.Unmarshal")
	}
	newSpan, err := transformToLogzioSpanBytes(&span)
	if err != nil {
		tester.Fatalf(err.Error())
	}
	var testLogzioSpan logzioSpan
	err = json.Unmarshal(newSpan, &testLogzioSpan)
	if err != nil {
		tester.Fatalf(err.Error())
	}
	dbModelSpan := testLogzioSpan.transformToDbModelSpan()
	if len(dbModelSpan.References) != 3 {
		tester.Fatalf("Error converting logzio span to dbmodel span")
	}
}
