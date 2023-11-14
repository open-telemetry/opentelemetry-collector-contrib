// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottlfuncs

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func Test_Now(t *testing.T) {
	exprFunc, err := now[any]()
	assert.NoError(t, err)

	value, err := exprFunc(nil, nil)
	assert.NoError(t, err)
	// There should be basically no difference between the value of time.Now() returned by the ottlfunc vs time.Now() run in the test.
	n := time.Now()
	assert.LessOrEqual(t, n.Sub(value.(time.Time)).Seconds(), 1.0)
}
