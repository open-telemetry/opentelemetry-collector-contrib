// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package integrationtest

import (
	"testing"

	"github.com/open-telemetry/opentelemetry-collector-contrib/testbed/testbed"
	"github.com/stretchr/testify/assert"
)

// CountValidator provides a testbed validator that only asserts for counts.
type CountValidator struct {
	t            testing.TB
	dataProvider testbed.DataProvider
}

// NewCountValidator creates a new instance of the CountValidator.
func NewCountValidator(t testing.TB, provider testbed.DataProvider) *CountValidator {
	return &CountValidator{
		t:            t,
		dataProvider: provider,
	}
}

func (v *CountValidator) Validate(tc *testbed.TestCase) {
	itemsSent := int64(tc.LoadGenerator.DataItemsSent()) - int64(tc.LoadGenerator.PermanentErrors())
	assert.Equal(v.t,
		itemsSent,
		int64(tc.MockBackend.DataItemsReceived()),
		"Received and sent counters do not match.",
	)
}

func (v *CountValidator) RecordResults(tc *testbed.TestCase) {}
