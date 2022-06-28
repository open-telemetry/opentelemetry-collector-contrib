// Copyright  The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package logzioexporter

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"testing"

	"github.com/jaegertracing/jaeger/model"
)

func TestTransformToLogzioSpanBytes(tester *testing.T) {
	inStr, err := ioutil.ReadFile("./testdata/span.json")
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
	m := make(map[string]interface{})
	err = json.Unmarshal(newSpan, &m)
	if err != nil {
		tester.Fatalf(err.Error())
	}
	if _, ok := m["JaegerTag"]; !ok {
		tester.Error("error converting span to logzioSpan, JaegerTag is not found")
	}
}

func TestTransformToDbModelSpan(tester *testing.T) {
	inStr, err := ioutil.ReadFile("./testdata/span.json")
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
