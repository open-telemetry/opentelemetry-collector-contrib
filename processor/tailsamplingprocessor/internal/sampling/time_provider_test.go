// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sampling

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTimeProvider(t *testing.T) {
	clock := MonotonicClock{}
	assert.Greater(t, clock.getCurSecond(), int64(0))
}
