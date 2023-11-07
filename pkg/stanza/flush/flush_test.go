// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package flush

import (
	"bufio"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/trim"
)

func TestFlusher(t *testing.T) {

	// bufio.ScanWords is a simple split function which with tokenize based on newlines.
	// It will return a partial token if atEOF=true. In order to test the flusher,
	// we don't want the split func to return partial tokens on its own. Instead, we only
	// want the flusher to force the partial out based on its own behavior. Therefore, we
	// always use atEOF=false.

	flushPeriod := 100 * time.Millisecond
	f := WithPeriod(bufio.ScanWords, trim.Nop, flushPeriod)

	content := []byte("foo bar hellowo")

	// The first token is complete
	advance, token, err := f(content, false)
	assert.NoError(t, err)
	assert.Equal(t, 4, advance)
	assert.Equal(t, []byte("foo"), token)

	// The second token is also complete
	advance, token, err = f(content[4:], false)
	assert.NoError(t, err)
	assert.Equal(t, 4, advance)
	assert.Equal(t, []byte("bar"), token)

	// We find a partial token, but we just updated, so don't flush it yet
	advance, token, err = f(content[8:], false)
	assert.NoError(t, err)
	assert.Equal(t, 0, advance)
	assert.Equal(t, []byte(nil), token)

	// We find the same partial token, but we updated quite recently, so still don't flush it yet
	advance, token, err = f(content[8:], false)
	assert.NoError(t, err)
	assert.Equal(t, 0, advance)
	assert.Equal(t, []byte(nil), token)

	time.Sleep(2 * flushPeriod)

	// Now it's been a while, so we should just flush the partial token
	advance, token, err = f(content[8:], false)
	assert.NoError(t, err)
	assert.Equal(t, 7, advance)
	assert.Equal(t, []byte("hellowo"), token)
}

func TestNoFlushPeriod(t *testing.T) {
	// Same test as above, but with a flush period of 0 we should never force flush.
	// In other words, we should expect exactly the behavior of bufio.ScanWords.

	flushPeriod := time.Duration(0)
	f := WithPeriod(bufio.ScanWords, trim.Nop, flushPeriod)

	content := []byte("foo bar hellowo")

	// The first token is complete
	advance, token, err := f(content, false)
	assert.NoError(t, err)
	assert.Equal(t, 4, advance)
	assert.Equal(t, []byte("foo"), token)

	// The second token is also complete
	advance, token, err = f(content[4:], false)
	assert.NoError(t, err)
	assert.Equal(t, 4, advance)
	assert.Equal(t, []byte("bar"), token)

	// We find a partial token, but we're using flushPeriod = 0 so we should never flush
	advance, token, err = f(content[8:], false)
	assert.NoError(t, err)
	assert.Equal(t, 0, advance)
	assert.Equal(t, []byte(nil), token)

	// We find the same partial token, but we're using flushPeriod = 0 so we should never flush
	advance, token, err = f(content[8:], false)
	assert.NoError(t, err)
	assert.Equal(t, 0, advance)
	assert.Equal(t, []byte(nil), token)

	time.Sleep(2 * flushPeriod)

	// Now it's been a while, but we are using flushPeriod=0, so we should never not flush
	advance, token, err = f(content[8:], false)
	assert.NoError(t, err)
	assert.Equal(t, 0, advance)
	assert.Equal(t, []byte(nil), token)
}
