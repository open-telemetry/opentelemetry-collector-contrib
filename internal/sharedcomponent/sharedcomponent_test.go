// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sharedcomponent

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
)

var id = component.NewID("test")

func TestNewSharedComponents(t *testing.T) {
	comps := NewSharedComponents()
	assert.Len(t, comps.comps, 0)
}

type mockComponent struct {
	component.StartFunc
	component.ShutdownFunc
}

func TestSharedComponents_GetOrAdd(t *testing.T) {
	nop := &mockComponent{}
	createNop := func() component.Component { return nop }

	comps := NewSharedComponents()
	got := comps.GetOrAdd(id, createNop)
	assert.Len(t, comps.comps, 1)
	assert.Same(t, nop, got.Unwrap())
	assert.Same(t, got, comps.GetOrAdd(id, createNop))

	// Shutdown nop will remove
	assert.NoError(t, got.Shutdown(context.Background()))
	assert.Len(t, comps.comps, 0)
	assert.NotSame(t, got, comps.GetOrAdd(id, createNop))
}

func TestSharedComponent(t *testing.T) {
	wantErr := errors.New("my error")
	calledStart := 0
	calledStop := 0
	comp := &mockComponent{
		StartFunc: func(ctx context.Context, host component.Host) error {
			calledStart++
			return wantErr
		},
		ShutdownFunc: func(ctx context.Context) error {
			calledStop++
			return wantErr
		},
	}
	createComp := func() component.Component { return comp }

	comps := NewSharedComponents()
	got := comps.GetOrAdd(id, createComp)
	assert.Equal(t, wantErr, got.Start(context.Background(), componenttest.NewNopHost()))
	assert.Equal(t, 1, calledStart)
	// Second time is not called anymore.
	assert.NoError(t, got.Start(context.Background(), componenttest.NewNopHost()))
	assert.Equal(t, 1, calledStart)
	assert.Equal(t, wantErr, got.Shutdown(context.Background()))
	assert.Equal(t, 1, calledStop)
	// Second time is not called anymore.
	assert.NoError(t, got.Shutdown(context.Background()))
	assert.Equal(t, 1, calledStop)
}
