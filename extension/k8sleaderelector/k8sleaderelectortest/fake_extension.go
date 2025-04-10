package k8sleaderelectortest

import (
	"context"
	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/k8sleaderelector"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pipeline"
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

// FakeLeaderElection fakes the k8sleaderelector.LeaderElection interface.
type FakeLeaderElection struct {
	OnLeading  func(context.Context)
	OnStopping func()
}

func (fle *FakeLeaderElection) SetCallBackFuncs(onLeading k8sleaderelector.StartCallBack, onStopping k8sleaderelector.StopCallBack) {
	fle.OnLeading = onLeading
	fle.OnStopping = onStopping
}

func (fle *FakeLeaderElection) InvokeOnLeading() {
	if fle.OnLeading != nil {
		fle.OnLeading(context.Background())
	}
}

func (fle *FakeLeaderElection) Start(ctx context.Context, host component.Host) error { return nil }

func (fle *FakeLeaderElection) Shutdown(ctx context.Context) error { return nil }

func (fle *FakeLeaderElection) InvokeOnStopping() {
	if fle.OnStopping != nil {
		fle.OnStopping()
	}
}
