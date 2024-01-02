// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package carbonreceiver

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/obsreport/obsreporttest"
	"go.opentelemetry.io/collector/receiver"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/carbonreceiver/internal/metadata"
)

func TestReporterObservability(t *testing.T) {
	receiverID := component.NewIDWithName(metadata.Type, "fake_receiver")
	tt, err := obsreporttest.SetupTelemetry(receiverID)
	require.NoError(t, err)
	defer func() {
		require.NoError(t, tt.Shutdown(context.Background()))
	}()

	reporter, err := newReporter(receiver.CreateSettings{ID: receiverID, TelemetrySettings: tt.TelemetrySettings, BuildInfo: component.NewDefaultBuildInfo()})
	require.NoError(t, err)

	ctx := reporter.OnDataReceived(context.Background())

	reporter.OnMetricsProcessed(ctx, 17, nil)

	require.NoError(t, tt.CheckReceiverMetrics("tcp", 17, 0))

	// Below just exercise the error paths.
	err = errors.New("fake error for tests")
	reporter.OnTranslationError(ctx, err)
	reporter.OnMetricsProcessed(ctx, 10, err)

	require.NoError(t, tt.CheckReceiverMetrics("tcp", 17, 10))
}
