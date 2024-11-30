package cgroupruntimeextension

import (
	"context"
	"os"
	"testing"

	"github.com/containerd/cgroups/v3/cgroup2"
	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/extension/extensiontest"
)

func TestCgroupV2Integration(t *testing.T) {
	res := cgroup2.Resources{}
	m, err := cgroup2.NewSystemd("/", "my-cgroup-abc.slice", -1, &res)
	if err != nil {
		t.Error(err)
	}
	t.Cleanup(func() {
		_ = m.Delete()
	})

	m.AddProc(uint64(os.Getpid()))

	factory := NewFactory()
	ctx := context.Background()
	extension, err := factory.Create(ctx, extensiontest.NewNopSettings(), factory.CreateDefaultConfig())
	assert.NoError(t, err)

	err = extension.Start(ctx, componenttest.NewNopHost())
	assert.NoError(t, err)
}
