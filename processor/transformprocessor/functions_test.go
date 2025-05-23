// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package transformprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor"

import (
	"testing"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/stretchr/testify/assert"
)

func Test_createFunctionsMap_EmptyFunctions(t *testing.T) {
	functions := []ottl.Factory[bool]{}
	expected := map[string]ottl.Factory[bool]{}

	actual := createFunctionsMap(functions)

	for k, v := range actual {
		assert.Equal(t, v.Name(), expected[k].Name())
	}
}

func Test_createFunctionsMap_OneFunction(t *testing.T) {
	functions := []ottl.Factory[bool]{
		createTestFuncFactory[bool]("DefaultFunc1"),
	}
	expected := map[string]ottl.Factory[bool]{
		"DefaultFunc1": createTestFuncFactory[bool]("DefaultFunc1"),
	}

	actual := createFunctionsMap(functions)

	for k, v := range actual {
		assert.Equal(t, v.Name(), expected[k].Name())
	}
}

func Test_createFunctionsMap_MultipleFunctions(t *testing.T) {
	functions := []ottl.Factory[bool]{
		createTestFuncFactory[bool]("DefaultFunc1"),
		createTestFuncFactory[bool]("DefaultFunc2"),
		createTestFuncFactory[bool]("DefaultFunc3"),
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
