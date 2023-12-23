// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package configschema

import (
	"os"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFieldComments(t *testing.T) {
	v := reflect.ValueOf(testStruct{})
	wd, err := os.Getwd()
	require.NoError(t, err)
	comments, err := commentsForStruct(v, wd)
	assert.NoError(t, err)
	assert.Equal(t, "embedded, package qualified comment\n", comments["Duration"])
	assert.Equal(t, "testStruct comment\n", comments["_struct"])
}
