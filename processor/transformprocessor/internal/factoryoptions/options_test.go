// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package factoryoptions

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
)

func createTestFuncFactory[K any](name string) ottl.Factory[K] {
	type TestFuncArguments[K any] struct{}
	createFunc := func(_ ottl.FunctionContext, _ ottl.Arguments) (ottl.ExprFunc[K], error) {
		return func(_ context.Context, _ K) (any, error) {
			return nil, nil
		}, nil
	}
	return ottl.NewFactory(name, &TestFuncArguments[K]{}, createFunc)
}

func Test_MergeFunctionsToMap_MergeToEmptyMap(t *testing.T) {
	emptyMap := map[string]ottl.Factory[bool]{}
	functions := []ottl.Factory[bool]{
		createTestFuncFactory[bool]("TestFunc1"),
	}
	expected := map[string]ottl.Factory[bool]{
		"TestFunc1": createTestFuncFactory[bool]("TestFunc1"),
	}

	actual := MergeFunctionsToMap(emptyMap, functions)

	for k, v := range expected {
		assert.Contains(t, actual, k)
		assert.Equal(t, v.Name(), actual[k].Name())
	}
}

func Test_MergeFunctionsToMap_MergeNoFunctionsToDefaultFunctions(t *testing.T) {
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

	actual := MergeFunctionsToMap(functionsMap, functions)

	for k, v := range expected {
		assert.Contains(t, actual, k)
		assert.Equal(t, v.Name(), actual[k].Name())
	}
}

func Test_MergeFunctionsToMap_MergeOneFunctionToDefaultFunctions(t *testing.T) {
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

	actual := MergeFunctionsToMap(functionsMap, functions)

	for k, v := range expected {
		assert.Contains(t, actual, k)
		assert.Equal(t, v.Name(), actual[k].Name())
	}
}

func Test_MergeFunctionsToMap_MergeMultipleFunctionToDefaultFunctions(t *testing.T) {
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

	actual := MergeFunctionsToMap(functionsMap, functions)

	for k, v := range expected {
		assert.Contains(t, actual, k)
		assert.Equal(t, v.Name(), actual[k].Name())
	}
}

func Test_MergeFunctionsToMap_OverridingExistingFunction(t *testing.T) {
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

	actual := MergeFunctionsToMap(functionsMap, functions)

	for k, v := range expected {
		assert.Contains(t, actual, k)
		assert.Equal(t, v.Name(), actual[k].Name())
	}
}
