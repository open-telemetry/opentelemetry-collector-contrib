// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package serializeprofiles

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTraceHashSprintf(t *testing.T) {
	origHash := newTraceHash(0x0001C03F8D6B8520, 0xEDEAEEA9460BEEBB)

	marshaled := fmt.Sprintf("%d", origHash)
	//nolint:goconst
	expected := "{492854164817184 17143777342331285179}"
	assert.Equal(t, expected, marshaled)

	marshaled = fmt.Sprintf("%s", origHash)
	expected = "{%!s(uint64=492854164817184) %!s(uint64=17143777342331285179)}"
	assert.Equal(t, expected, marshaled)

	marshaled = fmt.Sprintf("%v", origHash)
	//nolint:goconst
	expected = "{492854164817184 17143777342331285179}"
	assert.Equal(t, expected, marshaled)

	marshaled = fmt.Sprintf("%#v", origHash)
	expected = "0x1c03f8d6b8520edeaeea9460beebb"
	assert.Equal(t, expected, marshaled)

	// Values were chosen to test non-zero-padded output
	traceHash := newTraceHash(42, 100)

	marshaled = fmt.Sprintf("%x", traceHash)
	expected = "2a0000000000000064"
	assert.Equal(t, expected, marshaled)

	marshaled = fmt.Sprintf("%X", traceHash)
	expected = "2A0000000000000064"
	assert.Equal(t, expected, marshaled)

	marshaled = fmt.Sprintf("%#x", traceHash)
	expected = "0x2a0000000000000064"
	assert.Equal(t, expected, marshaled)

	marshaled = fmt.Sprintf("%#X", traceHash)
	expected = "0x2A0000000000000064"
	assert.Equal(t, expected, marshaled)
}
