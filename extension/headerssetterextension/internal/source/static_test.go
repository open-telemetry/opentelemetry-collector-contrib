// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package source

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestStaticSource(t *testing.T) {
	ts := &StaticSource{Value: "acme"}
	tenant, err := ts.Get(t.Context())
	assert.NoError(t, err)
	assert.Equal(t, "acme", tenant)
}
