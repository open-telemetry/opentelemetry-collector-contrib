// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awsxray

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

var (
	emptyString = ""
	testString  = "TEST"
)

func TestUtilWithNormalString(t *testing.T) {
	res := String(testString)

	assert.Equal(t, &testString, res)
}

func TestUtilWithEmptyString(t *testing.T) {
	res := String(emptyString)

	assert.Nil(t, res)
}

func TestStringOrEmptyWithNormalString(t *testing.T) {
	res := StringOrEmpty(&testString)

	assert.Equal(t, testString, res)
}

func TestStringOrEmptyWithNil(t *testing.T) {
	res := StringOrEmpty(nil)

	assert.Equal(t, emptyString, res)
}
