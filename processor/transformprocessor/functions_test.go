// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package transformprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor"

import (
	"testing"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

var (
	DefaultFunctions = map[string]ottl.Factory[bool]{
		"DefaultFunc1": createTestFuncFactory[bool]("DefaultFunc1"),
		"DefaultFunc2": createTestFuncFactory[bool]("DefaultFunc2"),
		"DefaultFunc3": createTestFuncFactory[bool]("DefaultFunc3"),
	}
)

func Test_mergeAdditionalFunctions_NoAdditionalFunctions(t *testing.T) {
	defaultFunctions := DefaultFunctions
	additionalFunctions := []ottl.Factory[bool]{}

	expectedFunctions := map[string]ottl.Factory[bool]{
		"DefaultFunc1": createTestFuncFactory[bool]("DefaultFunc1"),
		"DefaultFunc2": createTestFuncFactory[bool]("DefaultFunc2"),
		"DefaultFunc3": createTestFuncFactory[bool]("DefaultFunc3"),
	}

	actualFunctions := mergeAdditionalFunctions(defaultFunctions, additionalFunctions, zap.NewNop())
	assert.Len(t, actualFunctions, len(expectedFunctions))
	for k, v := range actualFunctions {
		assert.Equal(t, v.Name(), expectedFunctions[k].Name())
	}
}

func Test_mergeAdditionalFunctions_AddOneFunction(t *testing.T) {
	defaultFunctions := DefaultFunctions
	additionalFunctions := []ottl.Factory[bool]{createTestFuncFactory[bool]("TestFunc1")}

	expectedFunctions := map[string]ottl.Factory[bool]{
		"DefaultFunc1": createTestFuncFactory[bool]("DefaultFunc1"),
		"DefaultFunc2": createTestFuncFactory[bool]("DefaultFunc2"),
		"DefaultFunc3": createTestFuncFactory[bool]("DefaultFunc3"),
		"TestFunc1":    createTestFuncFactory[bool]("TestFunc1"),
	}

	actualFunctions := mergeAdditionalFunctions(defaultFunctions, additionalFunctions, zap.NewNop())
	assert.Len(t, actualFunctions, len(expectedFunctions))
	for k, v := range actualFunctions {
		assert.Equal(t, v.Name(), expectedFunctions[k].Name())
	}
}

func Test_mergeAdditionalFunctions_AddMultipleFunctions(t *testing.T) {
	defaultFunctions := DefaultFunctions
	additionalFunctions := []ottl.Factory[bool]{createTestFuncFactory[bool]("TestFunc1"), createTestFuncFactory[bool]("TestFunc2")}

	expectedFunctions := map[string]ottl.Factory[bool]{
		"DefaultFunc1": createTestFuncFactory[bool]("DefaultFunc1"),
		"DefaultFunc2": createTestFuncFactory[bool]("DefaultFunc2"),
		"DefaultFunc3": createTestFuncFactory[bool]("DefaultFunc3"),
		"TestFunc1":    createTestFuncFactory[bool]("TestFunc1"),
		"TestFunc2":    createTestFuncFactory[bool]("TestFunc2"),
	}

	actualFunctions := mergeAdditionalFunctions(defaultFunctions, additionalFunctions, zap.NewNop())
	assert.Len(t, actualFunctions, len(expectedFunctions))
	for k, v := range actualFunctions {
		assert.Equal(t, v.Name(), expectedFunctions[k].Name())
	}
}

func Test_mergeAdditionalFunctions_OverrideDefaultFunction(t *testing.T) {
	defaultFunctions := DefaultFunctions
	additionalFunctions := []ottl.Factory[bool]{createTestFuncFactory[bool]("TestFunc1"), createTestFuncFactory[bool]("DefaultFunc2")}

	expectedFunctions := map[string]ottl.Factory[bool]{
		"DefaultFunc1": createTestFuncFactory[bool]("DefaultFunc1"),
		"DefaultFunc2": createTestFuncFactory[bool]("DefaultFunc2"),
		"DefaultFunc3": createTestFuncFactory[bool]("DefaultFunc3"),
		"TestFunc1":    createTestFuncFactory[bool]("TestFunc1"),
	}

	actualFunctions := mergeAdditionalFunctions(defaultFunctions, additionalFunctions, zap.NewNop())
	assert.Len(t, actualFunctions, len(expectedFunctions))
	for k, v := range actualFunctions {
		assert.Equal(t, v.Name(), expectedFunctions[k].Name())
	}
}

func Test_mergeAdditionalFunctions_DuplicateAdditionalFunctions(t *testing.T) {
	defaultFunctions := DefaultFunctions
	additionalFunctions := []ottl.Factory[bool]{createTestFuncFactory[bool]("TestFunc2"), createTestFuncFactory[bool]("TestFunc2")}

	expectedFunctions := map[string]ottl.Factory[bool]{
		"DefaultFunc1": createTestFuncFactory[bool]("DefaultFunc1"),
		"DefaultFunc2": createTestFuncFactory[bool]("DefaultFunc2"),
		"DefaultFunc3": createTestFuncFactory[bool]("DefaultFunc3"),
		"TestFunc2":    createTestFuncFactory[bool]("TestFunc2"),
	}

	actualFunctions := mergeAdditionalFunctions(defaultFunctions, additionalFunctions, zap.NewNop())
	assert.Len(t, actualFunctions, len(expectedFunctions))
	for k, v := range actualFunctions {
		assert.Equal(t, v.Name(), expectedFunctions[k].Name())
	}
}
