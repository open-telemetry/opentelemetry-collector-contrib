// Copyright  observIQ, Inc.
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

package logdeduplicationprocessor

import (
	"fmt"
	"testing"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatautil"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
)

func Test_newFieldRemover(t *testing.T) {
	fieldKeys := []string{
		"single_field",
		"compound.field.one",
		"escaped\\.field",
		"escaped\\.compound.field",
	}

	expected := &fieldRemover{
		fields: []*field{
			{
				keyParts: []string{"single_field"},
			},
			{
				keyParts: []string{"compound", "field", "one"},
			},
			{
				keyParts: []string{"escaped.field"},
			},
			{
				keyParts: []string{"escaped.compound", "field"},
			},
		},
	}

	actual := newFieldRemover(fieldKeys)
	require.Equal(t, expected, actual)
}

// TestRemoveFieldsAttributes tests when a remove field is attributes
func TestRemoveFieldsAttributes(t *testing.T) {
	fields := []string{attributeField}
	remover := newFieldRemover(fields)

	expectedBody := "test body"
	logRecord := generateTestLogRecord(t, expectedBody)

	remover.RemoveFields(logRecord)
	require.Equal(t, expectedBody, logRecord.Body().AsString())
	require.Equal(t, 0, logRecord.Attributes().Len())
}

func TestRemoveFields(t *testing.T) {
	fields := []string{
		fmt.Sprintf("%s.nested\\.map.bool", bodyField),
		fmt.Sprintf("%s.bool", attributeField),
		fmt.Sprintf("%s.nested", attributeField),
		fmt.Sprintf("%s.not_present", bodyField),
	}
	remover := newFieldRemover(fields)

	logRecord := plog.NewLogRecord()

	// Fill attribute map
	logRecord.Attributes().PutBool("bool", true)
	logRecord.Attributes().PutStr("str", "attr str")
	nestedAttrMap := logRecord.Attributes().PutEmptyMap("nested")
	nestedAttrMap.PutInt("int", 2)

	// Expected attribut map
	expectedAttrsMap := pcommon.NewMap()
	expectedAttrsMap.PutStr("str", "attr str")
	expectedAttrHash := pdatautil.MapHash(expectedAttrsMap)

	// Fill body map
	bodyMap := logRecord.Body().SetEmptyMap()
	bodyMap.PutInt("safe", 10)
	nestedBodyMap := bodyMap.PutEmptyMap("nested.map")
	nestedBodyMap.PutBool("bool", true)

	// expected body map
	expectedBodyMap := pcommon.NewMap()
	expectedBodyMap.PutEmptyMap("nested.map")
	expectedBodyMap.PutInt("safe", 10)
	expectedBodyHash := pdatautil.MapHash(expectedBodyMap)

	remover.RemoveFields(logRecord)

	actualAttrHash := pdatautil.MapHash(logRecord.Attributes())
	actualBodyHash := pdatautil.MapHash(logRecord.Body().Map())

	require.Equal(t, expectedAttrHash, actualAttrHash)
	require.Equal(t, expectedBodyHash, actualBodyHash)
}
