// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package transformprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor"

import (
	"testing"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/stretchr/testify/assert"
)

var (
	DefaultFunctions = []ottl.Factory[bool]{
		createTestFuncFactory[bool]("DefaultFunc1"),
		createTestFuncFactory[bool]("DefaultFunc2"),
		createTestFuncFactory[bool]("DefaultFunc3"),
	}
)

func Test_createFunctionsMap_EmptyFunctions(t *testing.T) {
	functions := [][]ottl.Factory[bool]{}
	expected := map[string]ottl.Factory[bool]{}

	actual := createFunctionsMap(functions)

	for k, v := range actual {
		assert.Equal(t, v.Name(), expected[k].Name())
	}
}

func Test_createFunctionsMap_NoDefaultsAndOneFunction(t *testing.T) {
	functions := [][]ottl.Factory[bool]{
		{
			createTestFuncFactory[bool]("TestFunc1"),
		},
	}
	expected := map[string]ottl.Factory[bool]{
		"TestFunc1": createTestFuncFactory[bool]("TestFunc1"),
	}

	actual := createFunctionsMap(functions)

	for k, v := range actual {
		assert.Equal(t, v.Name(), expected[k].Name())
	}
}

func Test_createFunctionsMap_DefaultsAndOneFunction(t *testing.T) {
	functions := [][]ottl.Factory[bool]{
		DefaultFunctions,
		{
			createTestFuncFactory[bool]("TestFunc1"),
		},
	}
	expected := map[string]ottl.Factory[bool]{
		"DefaultFunc1": createTestFuncFactory[bool]("DefaultFunc1"),
		"DefaultFunc2": createTestFuncFactory[bool]("DefaultFunc2"),
		"DefaultFunc3": createTestFuncFactory[bool]("DefaultFunc3"),
		"TestFunc1":    createTestFuncFactory[bool]("TestFunc1"),
	}

	actual := createFunctionsMap(functions)

	for k, v := range actual {
		assert.Equal(t, v.Name(), expected[k].Name())
	}
}

func Test_createFunctionsMap_OverrideDefaultFunction(t *testing.T) {
	functions := [][]ottl.Factory[bool]{
		DefaultFunctions,
		{
			createTestFuncFactory[bool]("DefaultFunc1"),
		},
	}
	expected := map[string]ottl.Factory[bool]{
		"DefaultFunc1": createTestFuncFactory[bool]("DefaultFunc1"),
		"DefaultFunc2": createTestFuncFactory[bool]("DefaultFunc2"),
		"DefaultFunc3": createTestFuncFactory[bool]("DefaultFunc3"),
	}

	actual := createFunctionsMap(functions)

	for k, v := range actual {
		assert.Equal(t, v.Name(), expected[k].Name())
	}
}

func Test_createFunctionsMapeDefaultsAndMultipleFunctions(t *testing.T) {
	functions := [][]ottl.Factory[bool]{
		DefaultFunctions,
		{
			createTestFuncFactory[bool]("TestFunc1"),
			createTestFuncFactory[bool]("TestFunc2"),
			createTestFuncFactory[bool]("TestFunc3"),
		},
	}
	expected := map[string]ottl.Factory[bool]{
		"DefaultFunc1": createTestFuncFactory[bool]("DefaultFunc1"),
		"DefaultFunc2": createTestFuncFactory[bool]("DefaultFunc2"),
		"DefaultFunc3": createTestFuncFactory[bool]("DefaultFunc3"),
		"TestFunc1":    createTestFuncFactory[bool]("TestFunc1"),
		"TestFunc2":    createTestFuncFactory[bool]("TestFunc2"),
		"TestFunc3":    createTestFuncFactory[bool]("TestFunc3"),
	}

	actual := createFunctionsMap(functions)

	for k, v := range actual {
		assert.Equal(t, v.Name(), expected[k].Name())
	}
}
