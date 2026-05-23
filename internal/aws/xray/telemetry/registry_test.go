// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package telemetry

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/component"
)

func TestRegistry(t *testing.T) {
	r := NewRegistry()
	newID := component.MustNewID("new")
	contribID := component.MustNewID("contrib")
	notCreatedID := component.MustNewID("not_created")
	original := r.Register(
		newID,
		Config{
			IncludeMetadata: false,
			Contributors:    []component.ID{contribID},
		},
		&mockXRayClient{},
	)
	withSameID := r.Register(
		newID,
		Config{
			IncludeMetadata: true,
			Contributors:    []component.ID{notCreatedID},
		},
		&mockXRayClient{},
		WithResourceARN("arn"),
	)
	// still the same recorder
	assert.Same(t, original, withSameID)
	// contributors have access to same recorder
	contrib := r.Load(contribID)
	assert.NotNil(t, contrib)
	assert.Same(t, original, contrib)
	// second attempt with same ID did not give contributors access
	assert.Nil(t, r.Load(notCreatedID))
	nop := r.LoadOrNop(notCreatedID)
	assert.NotNil(t, nop)
	assert.Equal(t, NewNopSender(), nop)
}

func TestGlobalRegistry(t *testing.T) {
	assert.Same(t, globalRegistry, GlobalRegistry())
}
