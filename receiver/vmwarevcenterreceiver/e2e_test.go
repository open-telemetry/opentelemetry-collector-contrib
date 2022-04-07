package vmwarevcenterreceiver

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	vmTest "github.com/vmware/govmomi/test"
)

func TestEndtoEnd(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	driver := vmTest.NewAuthenticatedClient(t)
	client := VmwareVcenterClient{
		vimDriver: driver,
	}
	clusters, err := client.Clusters(ctx)
	require.NoError(t, err)
	require.Greater(t, len(clusters), 0)
}
