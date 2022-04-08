package vmwarevcenterreceiver

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	vmTest "github.com/vmware/govmomi/test"
	"go.opentelemetry.io/collector/receiver/scrapererror"
)

func TestStart(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	driver := vmTest.NewAuthenticatedClient(t)
	client := VmwareVcenterClient{
		vimDriver: driver,
	}
	s := vcenterMetricScraper{
		client: &client,
	}
	errs := scrapererror.ScrapeErrors{}
	s.collectClusters(ctx, &errs)

	require.NoError(t, errs.Combine())
}
