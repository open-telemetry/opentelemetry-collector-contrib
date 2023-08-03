// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package datadogreceiver

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/cenkalti/backoff"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/receiver/receivertest"
	"go.uber.org/multierr"
)

func TestDatadogReceiver_Lifecycle(t *testing.T) {
	factory := NewFactory()
	ddr, err := factory.CreateTracesReceiver(context.Background(), receivertest.NewNopCreateSettings(), factory.CreateDefaultConfig(), consumertest.NewNop())
	assert.NoError(t, err, "Receiver should be created")

	err = startComponentWithRetries(context.Background(), ddr, componenttest.NewNopHost())
	assert.NoError(t, err, "Server should start")

	err = ddr.Shutdown(context.Background())
	assert.NoError(t, err, "Server should stop")
}

func TestDatadogServer(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	dd, err := newDataDogReceiver(
		cfg,
		consumertest.NewNop(),
		receivertest.NewNopCreateSettings(),
	)
	require.NoError(t, err, "Must not error when creating receiver")

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	require.NoError(t, startComponentWithRetries(ctx, dd, componenttest.NewNopHost()))
	t.Cleanup(func() {
		require.NoError(t, dd.Shutdown(ctx), "Must not error shutting down")
	})

	for _, tc := range []struct {
		name string
		op   io.Reader

		expectCode    int
		expectContent string
	}{
		{
			name:          "invalid data",
			op:            strings.NewReader("{"),
			expectCode:    http.StatusBadRequest,
			expectContent: "Unable to unmarshal reqs\n",
		},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			req, err := http.NewRequest(
				http.MethodPost,
				fmt.Sprintf("http://%s/v0.7/traces", cfg.Endpoint),
				tc.op,
			)
			require.NoError(t, err, "Must not error when creating request")

			resp, err := http.DefaultClient.Do(req)
			require.NoError(t, err, "Must not error performing request")

			actual, err := io.ReadAll(resp.Body)
			require.NoError(t, multierr.Combine(err, resp.Body.Close()), "Must not error when reading body")

			assert.Equal(t, tc.expectContent, string(actual))
			assert.Equal(t, tc.expectCode, resp.StatusCode, "Must match the expected status code")
		})
	}
}

// Throughout the repo, many tests are invoked with go test ... -parallel ...
// This means that in addition to explicitly parallelizable tests (t.Parallel()), most test cases can be executed
// in parallel. Components that statically try to bind a port (e.g., from an integration test) can therefore fail
// and cause flaky builds. This function wraps Component.start with a simple ≤5s retry schedule.
func startComponentWithRetries(ctx context.Context, component component.Component, host component.Host) error {
	return backoff.Retry(func() error {
		return component.Start(ctx, host)
	}, backoff.WithMaxRetries(backoff.NewConstantBackOff(time.Second), 5))
}
