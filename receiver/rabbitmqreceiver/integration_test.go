// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

//go:build integration
// +build integration

package rabbitmqreceiver

import (
	"context"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"testing"
	"time"
	"unicode"

	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/receiver/receivertest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/golden"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/pmetrictest"
)

var (
	containerRequest3_9 = testcontainers.ContainerRequest{
		FromDockerfile: testcontainers.FromDockerfile{
			Context:    filepath.Join("testdata", "integration"),
			Dockerfile: "Dockerfile.rabbitmq.3_9",
		},
		ExposedPorts: []string{"15672:15672"},
		Hostname:     "localhost",
		WaitingFor:   waitStrategy{},
	}
)

func TestRabbitmqIntegration(t *testing.T) {
	t.Skip("See https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/17201")
	t.Run("Running rabbitmq 3.9", func(t *testing.T) {
		t.Parallel()
		container := getContainer(t, containerRequest3_9)
		defer func() {
			require.NoError(t, container.Terminate(context.Background()))
		}()
		require.NoError(t, container.Start(context.Background()))

		hostname, err := container.Host(context.Background())
		require.NoError(t, err)

		f := NewFactory()
		cfg := f.CreateDefaultConfig().(*Config)
		cfg.Endpoint = fmt.Sprintf("http://%s:15672", hostname)
		cfg.Username = "otelu"
		cfg.Password = "otelp"

		consumer := new(consumertest.MetricsSink)
		settings := receivertest.NewNopCreateSettings()
		rcvr, err := f.CreateMetricsReceiver(context.Background(), settings, cfg, consumer)
		require.NoError(t, err, "failed creating metrics receiver")

		require.NoError(t, rcvr.Start(context.Background(), componenttest.NewNopHost()))
		require.Eventuallyf(t, func() bool {
			return len(consumer.AllMetrics()) > 0
		}, 2*time.Minute, 1*time.Second, "failed to receive more than 0 metrics")
		require.NoError(t, rcvr.Shutdown(context.Background()))

		actualMetrics := consumer.AllMetrics()[0]

		expectedFile := filepath.Join("testdata", "integration", "expected.3_9.json")
		expectedMetrics, err := golden.ReadMetrics(expectedFile)
		require.NoError(t, err)

		require.NoError(t, pmetrictest.CompareMetrics(expectedMetrics, actualMetrics, pmetrictest.IgnoreMetricValues()))
	})
}

func getContainer(t *testing.T, req testcontainers.ContainerRequest) testcontainers.Container {
	require.NoError(t, req.Validate())
	container, err := testcontainers.GenericContainer(
		context.Background(),
		testcontainers.GenericContainerRequest{
			ContainerRequest: req,
			Started:          true,
		})
	require.NoError(t, err)
	return container
}

type waitStrategy struct{}

func (ws waitStrategy) WaitUntilReady(ctx context.Context, st wait.StrategyTarget) error {
	if err := wait.ForListeningPort("15672").
		WithStartupTimeout(2*time.Minute).
		WaitUntilReady(ctx, st); err != nil {
		return err
	}

	code, r, err := st.Exec(context.Background(), []string{"./setup.sh"})
	if err != nil {
		return err
	}
	if code == 0 {
		return nil
	}

	// Try to read the error message for the sake of debugging
	if errBytes, readerErr := io.ReadAll(r); readerErr == nil {
		// Error message may have non-printable chars, so clean it up
		errStr := strings.Map(func(r rune) rune {
			if unicode.IsPrint(r) {
				return r
			}
			return -1
		}, string(errBytes))
		return errors.New(strings.TrimSpace(errStr))
	}
	return errors.New("setup script returned non-zero exit code")
}
