// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package tracesegment

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTraceSegmentHeaderIsValid(t *testing.T) {
	header := Header{
		Format:  "json",
		Version: 1,
	}

	valid := header.IsValid()

	assert.True(t, valid)
}

func TestTraceSegmentHeaderIsValidCaseInsensitive(t *testing.T) {
	header := Header{
		Format:  "jSoN",
		Version: 1,
	}

	valid := header.IsValid()

	assert.True(t, valid)
}

func TestTraceSegmentHeaderIsValidWrongVersion(t *testing.T) {
	header := Header{
		Format:  "json",
		Version: 2,
	}

	valid := header.IsValid()

	assert.False(t, valid)
}

func TestTraceSegmentHeaderIsValidWrongFormat(t *testing.T) {
	header := Header{
		Format:  "xml",
		Version: 1,
	}

	valid := header.IsValid()

	assert.False(t, valid)
}

func TestTraceSegmentHeaderIsValidWrongFormatVersion(t *testing.T) {
	header := Header{
		Format:  "xml",
		Version: 2,
	}

	valid := header.IsValid()

	assert.False(t, valid)
}
