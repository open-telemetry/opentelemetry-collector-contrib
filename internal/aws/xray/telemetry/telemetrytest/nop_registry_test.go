// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package telemetrytest

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/component"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/xray/telemetry"
)

func TestNopRegistry(t *testing.T) {
	assert.Same(t, nopRegistryInstance, NewNopRegistry())
	r := NewNopRegistry()
	assert.NotPanics(t, func() {
		recorder := r.Register(component.MustNewID("a"), telemetry.Config{}, nil)
		assert.Same(t, recorder, r.Load(component.MustNewID("b")))
		r.LoadOrStore(component.MustNewID("c"), recorder)
		r.LoadOrNop(component.MustNewID("d"))
	})
}
