// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package splitter

import (
	"bufio"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestCustom(t *testing.T) {
	factory := NewCustomFactory(bufio.ScanLines, 0)
	splitFunc, err := factory.SplitFunc()
	assert.NoError(t, err)
	assert.NotNil(t, splitFunc)

	input := []byte(" hello \n world \n extra ")

	advance, token, err := splitFunc(input, false)
	assert.NoError(t, err)
	assert.Equal(t, 8, advance)
	assert.Equal(t, []byte(" hello "), token)

	advance, token, err = splitFunc(input[8:], false)
	assert.NoError(t, err)
	assert.Equal(t, 8, advance)
	assert.Equal(t, []byte(" world "), token)

	advance, token, err = splitFunc(input[16:], false)
	assert.NoError(t, err)
	assert.Equal(t, 0, advance)
	assert.Nil(t, token)
}

func TestCustomWithFlush(t *testing.T) {
	flushPeriod := 100 * time.Millisecond
	factory := NewCustomFactory(bufio.ScanLines, flushPeriod)
	splitFunc, err := factory.SplitFunc()
	assert.NoError(t, err)
	assert.NotNil(t, splitFunc)

	input := []byte(" hello \n world \n extra ")

	advance, token, err := splitFunc(input, false)
	assert.NoError(t, err)
	assert.Equal(t, 8, advance)
	assert.Equal(t, []byte(" hello "), token)

	advance, token, err = splitFunc(input[8:], false)
	assert.NoError(t, err)
	assert.Equal(t, 8, advance)
	assert.Equal(t, []byte(" world "), token)

	advance, token, err = splitFunc(input[16:], false)
	assert.NoError(t, err)
	assert.Equal(t, 0, advance)
	assert.Nil(t, token)

	time.Sleep(2 * flushPeriod)

	advance, token, err = splitFunc(input[16:], false)
	assert.NoError(t, err)
	assert.Equal(t, 7, advance)
	assert.Equal(t, []byte(" extra "), token)
}
