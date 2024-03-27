// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package batch

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBatchRecordsCapacity(t *testing.T) {
	t.Parallel()

	b := New()

	assert.Equal(t, MaxBatchedRecords, cap(b.records), "Must have correct value set")
}
