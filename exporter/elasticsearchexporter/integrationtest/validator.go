// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package integrationtest // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/elasticsearchexporter/integrationtest"

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/open-telemetry/opentelemetry-collector-contrib/testbed/testbed"
)

// countValidator provides a testbed validator that only asserts for counts.
type countValidator struct {
	t            testing.TB
	dataProvider testbed.DataProvider
}

// newCountValidator creates a new instance of the CountValidator.
func newCountValidator(tb testing.TB, provider testbed.DataProvider) *countValidator {
	return &countValidator{
		t:            tb,
		dataProvider: provider,
	}
}

func (v *countValidator) Validate(tc *testbed.TestCase) {
	itemsSent := int64(tc.LoadGenerator.DataItemsSent()) - int64(tc.LoadGenerator.PermanentErrors())
	assert.Equal(v.t,
		itemsSent,
		int64(tc.MockBackend.DataItemsReceived()),
		"Received and sent counters do not match.",
	)
}

func (v *countValidator) RecordResults(_ *testbed.TestCase) {}
