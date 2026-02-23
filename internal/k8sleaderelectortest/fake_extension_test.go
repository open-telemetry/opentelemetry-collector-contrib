// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package k8sleaderelectortest

import (
	"context"
	"testing"

	"go.opentelemetry.io/collector/component"
)

func TestFakeHost_GetExtensions(t *testing.T) {
	fakeLE := &FakeLeaderElection{}
	host := &FakeHost{FakeLeaderElection: fakeLE}
	exts := host.GetExtensions()
	extID := component.MustNewID("k8s_leader_elector")
	if exts[extID] != fakeLE {
		t.Errorf("expected extension to be fakeLE, got %v", exts[extID])
	}
}

func TestFakeHost_GetExporters(t *testing.T) {
	host := &FakeHost{}
	if host.GetExporters() != nil {
		t.Error("expected GetExporters to return nil")
	}
}

func TestFakeLeaderElection_SetCallBackFuncs_And_Invoke(t *testing.T) {
	var leadingCalled, stoppingCalled bool
	fle := &FakeLeaderElection{}
	fle.SetCallBackFuncs(
		func(_ context.Context) { leadingCalled = true },
		func() { stoppingCalled = true },
	)
	fle.InvokeOnLeading()
	if !leadingCalled {
		t.Error("expected OnLeading callback to be called")
	}
	fle.InvokeOnStopping()
	if !stoppingCalled {
		t.Error("expected OnStopping callback to be called")
	}
}

func TestFakeLeaderElection_Start_Shutdown(t *testing.T) {
	fle := &FakeLeaderElection{}
	if err := fle.Start(t.Context(), nil); err != nil {
		t.Errorf("expected Start to return nil, got %v", err)
	}
	if err := fle.Shutdown(t.Context()); err != nil {
		t.Errorf("expected Shutdown to return nil, got %v", err)
	}
}
