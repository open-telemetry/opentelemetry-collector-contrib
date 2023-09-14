// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package splitter

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"golang.org/x/text/encoding/unicode"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/split"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/trim"
)

func TestSplitFuncError(t *testing.T) {
	sCfg := split.Config{
		LineStartPattern: "START",
		LineEndPattern:   "END",
	}
	factory := NewSplitFuncFactory(sCfg, unicode.UTF8, 1024, trim.Nop, 0)
	splitFunc, err := factory.SplitFunc()
	assert.Error(t, err)
	assert.Nil(t, splitFunc)
}

func TestSplitFunc(t *testing.T) {
	factory := NewSplitFuncFactory(split.Config{}, unicode.UTF8, 1024, trim.Nop, 0)
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

func TestSplitFuncWithTrim(t *testing.T) {
	factory := NewSplitFuncFactory(split.Config{}, unicode.UTF8, 1024, trim.Whitespace, 0)
	splitFunc, err := factory.SplitFunc()
	assert.NoError(t, err)
	assert.NotNil(t, splitFunc)

	input := []byte(" hello \n world \n extra ")

	advance, token, err := splitFunc(input, false)
	assert.NoError(t, err)
	assert.Equal(t, 8, advance)
	assert.Equal(t, []byte("hello"), token)

	advance, token, err = splitFunc(input[8:], false)
	assert.NoError(t, err)
	assert.Equal(t, 8, advance)
	assert.Equal(t, []byte("world"), token)

	advance, token, err = splitFunc(input[16:], false)
	assert.NoError(t, err)
	assert.Equal(t, 0, advance)
	assert.Nil(t, token)
}

func TestSplitFuncWithFlush(t *testing.T) {
	flushPeriod := 100 * time.Millisecond
	factory := NewSplitFuncFactory(split.Config{}, unicode.UTF8, 1024, trim.Nop, flushPeriod)
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

func TestSplitFuncWithFlushTrim(t *testing.T) {
	flushPeriod := 100 * time.Millisecond
	factory := NewSplitFuncFactory(split.Config{}, unicode.UTF8, 1024, trim.Whitespace, flushPeriod)
	splitFunc, err := factory.SplitFunc()
	assert.NoError(t, err)
	assert.NotNil(t, splitFunc)

	input := []byte(" hello \n world \n extra ")

	advance, token, err := splitFunc(input, false)
	assert.NoError(t, err)
	assert.Equal(t, 8, advance)
	assert.Equal(t, []byte("hello"), token)

	advance, token, err = splitFunc(input[8:], false)
	assert.NoError(t, err)
	assert.Equal(t, 8, advance)
	assert.Equal(t, []byte("world"), token)

	advance, token, err = splitFunc(input[16:], false)
	assert.NoError(t, err)
	assert.Equal(t, 0, advance)
	assert.Nil(t, token)

	time.Sleep(2 * flushPeriod)

	advance, token, err = splitFunc(input[16:], false)
	assert.NoError(t, err)
	assert.Equal(t, 7, advance)
	assert.Equal(t, []byte("extra"), token) // Ensure trim applies to flushed token
}
