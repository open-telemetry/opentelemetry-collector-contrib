// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package k8sleaderelectortest // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/k8sleaderelector/k8sleaderelectortest"

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pipeline"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/k8sleaderelector"
)

type FakeHost struct {
	FakeLeaderElection *FakeLeaderElection
}

func (fh *FakeHost) GetExtensions() map[component.ID]component.Component {
	extID := component.MustNewID("k8s_leader_elector")
	return map[component.ID]component.Component{
		extID: fh.FakeLeaderElection,
	}
}

func (fh *FakeHost) GetExporters() map[pipeline.Signal]map[component.ID]component.Component {
	return nil
}

type FakeLeaderElection struct {
	OnLeading  func(context.Context)
	OnStopping func()
}

func (fle *FakeLeaderElection) SetCallBackFuncs(onLeading k8sleaderelector.StartCallback, onStopping k8sleaderelector.StopCallback) {
	fle.OnLeading = onLeading
	fle.OnStopping = onStopping
}

func (fle *FakeLeaderElection) InvokeOnLeading() {
	if fle.OnLeading != nil {
		fle.OnLeading(context.Background())
	}
}

func (fle *FakeLeaderElection) Start(_ context.Context, _ component.Host) error { return nil }

func (fle *FakeLeaderElection) Shutdown(_ context.Context) error { return nil }

func (fle *FakeLeaderElection) InvokeOnStopping() {
	if fle.OnStopping != nil {
		fle.OnStopping()
	}
}
