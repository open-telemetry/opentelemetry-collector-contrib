// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package transformprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor"

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
)

func Test_mergeFunctionsToMap_MergeToEmptyMap(t *testing.T) {
	emptyMap := map[string]ottl.Factory[bool]{}
	functions := []ottl.Factory[bool]{
		createTestFuncFactory[bool]("TestFunc1"),
	}
	expected := map[string]ottl.Factory[bool]{
		"TestFunc1": createTestFuncFactory[bool]("TestFunc1"),
	}

	actual := mergeFunctionsToMap(emptyMap, functions)

	for k, v := range expected {
		assert.Contains(t, actual, k)
		assert.Equal(t, v.Name(), actual[k].Name())
	}
}

func Test_mergeFunctionsToMap_MergeNoFunctionsToDefaultFunctions(t *testing.T) {
	functionsMap := map[string]ottl.Factory[bool]{
		"DefaultFunc1": createTestFuncFactory[bool]("DefaultFunc1"),
		"DefaultFunc2": createTestFuncFactory[bool]("DefaultFunc2"),
		"DefaultFunc3": createTestFuncFactory[bool]("DefaultFunc3"),
	}
	functions := []ottl.Factory[bool]{}
	expected := map[string]ottl.Factory[bool]{
		"DefaultFunc1": createTestFuncFactory[bool]("DefaultFunc1"),
		"DefaultFunc2": createTestFuncFactory[bool]("DefaultFunc2"),
		"DefaultFunc3": createTestFuncFactory[bool]("DefaultFunc3"),
	}

	actual := mergeFunctionsToMap(functionsMap, functions)

	for k, v := range expected {
		assert.Contains(t, actual, k)
		assert.Equal(t, v.Name(), actual[k].Name())
	}
}

func Test_mergeFunctionsToMap_MergeOneFunctionToDefaultFunctions(t *testing.T) {
	functionsMap := map[string]ottl.Factory[bool]{
		"DefaultFunc1": createTestFuncFactory[bool]("DefaultFunc1"),
		"DefaultFunc2": createTestFuncFactory[bool]("DefaultFunc2"),
		"DefaultFunc3": createTestFuncFactory[bool]("DefaultFunc3"),
	}
	functions := []ottl.Factory[bool]{
		createTestFuncFactory[bool]("TestFunc1"),
	}
	expected := map[string]ottl.Factory[bool]{
		"DefaultFunc1": createTestFuncFactory[bool]("DefaultFunc1"),
		"DefaultFunc2": createTestFuncFactory[bool]("DefaultFunc2"),
		"DefaultFunc3": createTestFuncFactory[bool]("DefaultFunc3"),
		"TestFunc1":    createTestFuncFactory[bool]("TestFunc1"),
	}

	actual := mergeFunctionsToMap(functionsMap, functions)

	for k, v := range expected {
		assert.Contains(t, actual, k)
		assert.Equal(t, v.Name(), actual[k].Name())
	}
}

func Test_mergeFunctionsToMap_MergeMultipleFunctionToDefaultFunctions(t *testing.T) {
	functionsMap := map[string]ottl.Factory[bool]{
		"DefaultFunc1": createTestFuncFactory[bool]("DefaultFunc1"),
		"DefaultFunc2": createTestFuncFactory[bool]("DefaultFunc2"),
		"DefaultFunc3": createTestFuncFactory[bool]("DefaultFunc3"),
	}
	functions := []ottl.Factory[bool]{
		createTestFuncFactory[bool]("TestFunc1"),
		createTestFuncFactory[bool]("TestFunc2"),
		createTestFuncFactory[bool]("TestFunc3"),
	}
	expected := map[string]ottl.Factory[bool]{
		"DefaultFunc1": createTestFuncFactory[bool]("DefaultFunc1"),
		"DefaultFunc2": createTestFuncFactory[bool]("DefaultFunc2"),
		"DefaultFunc3": createTestFuncFactory[bool]("DefaultFunc3"),
		"TestFunc1":    createTestFuncFactory[bool]("TestFunc1"),
		"TestFunc2":    createTestFuncFactory[bool]("TestFunc2"),
		"TestFunc3":    createTestFuncFactory[bool]("TestFunc3"),
	}

	actual := mergeFunctionsToMap(functionsMap, functions)

	for k, v := range expected {
		assert.Contains(t, actual, k)
		assert.Equal(t, v.Name(), actual[k].Name())
	}
}

func Test_mergeFunctionsToMap_OverridingExistingFunction(t *testing.T) {
	functionsMap := map[string]ottl.Factory[bool]{
		"DefaultFunc1": createTestFuncFactory[bool]("DefaultFunc1"),
		"DefaultFunc2": createTestFuncFactory[bool]("DefaultFunc2"),
		"DefaultFunc3": createTestFuncFactory[bool]("DefaultFunc3"),
	}
	functions := []ottl.Factory[bool]{
		createTestFuncFactory[bool]("TestFunc1"),
		createTestFuncFactory[bool]("DefaultFunc2"),
		createTestFuncFactory[bool]("TestFunc3"),
	}
	expected := map[string]ottl.Factory[bool]{
		"DefaultFunc1": createTestFuncFactory[bool]("DefaultFunc1"),
		"DefaultFunc2": createTestFuncFactory[bool]("DefaultFunc2"),
		"DefaultFunc3": createTestFuncFactory[bool]("DefaultFunc3"),
		"TestFunc1":    createTestFuncFactory[bool]("TestFunc1"),
		"TestFunc3":    createTestFuncFactory[bool]("TestFunc3"),
	}

	actual := mergeFunctionsToMap(functionsMap, functions)

	for k, v := range expected {
		assert.Contains(t, actual, k)
		assert.Equal(t, v.Name(), actual[k].Name())
	}
}
