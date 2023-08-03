// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package translator

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/pcommon"

	awsxray "github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/xray"
)

func TestAddAnnotations(t *testing.T) {
	input := make(map[string]interface{})
	input["int"] = 0
	input["int32"] = int32(1)
	input["int64"] = int64(2)
	input["bool"] = false
	input["float32"] = float32(4.5)
	input["float64"] = 5.5

	attrMap := pcommon.NewMap()
	attrMap.EnsureCapacity(initAttrCapacity)
	addAnnotations(input, attrMap)

	expectedAttrMap := pcommon.NewMap()
	expectedAttrMap.PutBool("bool", false)
	expectedAttrMap.PutDouble("float32", 4.5)
	expectedAttrMap.PutDouble("float64", 5.5)
	expectedAttrMap.PutInt("int", 0)
	expectedAttrMap.PutInt("int32", 1)
	expectedAttrMap.PutInt("int64", 2)
	expectedKeys := expectedAttrMap.PutEmptySlice(awsxray.AWSXraySegmentAnnotationsAttribute)
	expectedKeys.AppendEmpty().SetStr("int")
	expectedKeys.AppendEmpty().SetStr("int32")
	expectedKeys.AppendEmpty().SetStr("int64")
	expectedKeys.AppendEmpty().SetStr("bool")
	expectedKeys.AppendEmpty().SetStr("float32")
	expectedKeys.AppendEmpty().SetStr("float64")

	assert.True(t, cmp.Equal(expectedAttrMap.AsRaw(), attrMap.AsRaw(), cmpopts.SortSlices(func(x, y interface{}) bool {
		return x.(string) < y.(string)
	})), "attribute maps differ")
}
