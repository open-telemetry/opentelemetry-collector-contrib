// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0
package alias

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTypeAliases(t *testing.T) {
	t.Parallel()

	assert.IsType(t, (*string)(nil), (*AttributeKey)(nil))
	assert.IsType(t, (*string)(nil), (*SignalName)(nil))
}
