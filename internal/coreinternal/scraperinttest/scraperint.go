// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package scraperinttest // import "github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/scraperinttest"

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

	"github.com/docker/go-connections/nat"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/receiver/receivertest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/golden"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/pmetrictest"
)

func EqualsLatestMetrics(expected pmetric.Metrics, sink *consumertest.MetricsSink, compareOpts []pmetrictest.CompareMetricsOption) func() bool {
	return func() bool {
		allMetrics := sink.AllMetrics()
		return len(allMetrics) > 0 && nil == pmetrictest.CompareMetrics(expected, allMetrics[len(allMetrics)-1], compareOpts...)
	}
}

func NewIntegrationTest(f receiver.Factory, cr testcontainers.ContainerRequest, opts ...TestOption) *IntegrationTest {
	it := &IntegrationTest{
		factory:          f,
		containerRequest: cr,
		customConfig:     func(component.Config, string, MappedPortFunc) {},
		expectedFile:     filepath.Join("testdata", "integration", "expected.yaml"),
		compareTimeout:   time.Minute,
	}
	for _, opt := range opts {
		opt(it)
	}
	return it
}

type IntegrationTest struct {
	containerRequest testcontainers.ContainerRequest

	factory      receiver.Factory
	customConfig customConfigFunc

	expectedFile   string
	compareOptions []pmetrictest.CompareMetricsOption
	compareTimeout time.Duration
}

func (it *IntegrationTest) Run(t *testing.T) {
	ctx := context.Background()

	require.NoError(t, it.containerRequest.Validate())
	container, err := testcontainers.GenericContainer(ctx,
		testcontainers.GenericContainerRequest{
			ContainerRequest: it.containerRequest,
			Started:          true,
		})
	require.NoError(t, err)
	defer func() {
		require.NoError(t, container.Terminate(context.Background()))
	}()

	cfg := it.factory.CreateDefaultConfig()
	host, mappedPort := containerInfo(ctx, t, container)
	it.customConfig(cfg, host, mappedPort)

	sink := new(consumertest.MetricsSink)
	settings := receivertest.NewNopCreateSettings()

	rcvr, err := it.factory.CreateMetricsReceiver(ctx, settings, cfg, sink)
	require.NoError(t, err, "failed creating metrics receiver")
	require.NoError(t, rcvr.Start(ctx, componenttest.NewNopHost()))
	defer func() {
		require.NoError(t, rcvr.Shutdown(ctx))
	}()

	expected, err := golden.ReadMetrics(it.expectedFile)
	require.NoError(t, err)

	// Defined outside of Eventually so it can be printed if the test fails
	var validateErr error
	defer func() {
		if t.Failed() {
			t.Error(validateErr.Error())
		}
	}()

	require.Eventually(t,
		func() bool {
			allMetrics := sink.AllMetrics()
			if len(allMetrics) == 0 {
				return false
			}
			validateErr = pmetrictest.CompareMetrics(expected, allMetrics[len(allMetrics)-1], it.compareOptions...)
			return validateErr == nil
		},
		it.compareTimeout, it.compareTimeout/20)
}

type TestOption func(*IntegrationTest)

func WithCustomConfig(c customConfigFunc) TestOption {
	return func(it *IntegrationTest) {
		it.customConfig = c
	}
}

func WithExpectedFile(f string) TestOption {
	return func(it *IntegrationTest) {
		it.expectedFile = f
	}
}

func WithCompareOptions(opts ...pmetrictest.CompareMetricsOption) TestOption {
	return func(it *IntegrationTest) {
		it.compareOptions = opts
	}
}

func WithCompareTimeout(t time.Duration) TestOption {
	return func(it *IntegrationTest) {
		it.compareTimeout = t
	}
}

type customConfigFunc func(cfg component.Config, host string, mappedPort MappedPortFunc)

func containerInfo(ctx context.Context, t *testing.T, container testcontainers.Container) (string, MappedPortFunc) {
	h, err := container.Host(ctx)
	require.NoError(t, err)
	return h, func(port string) string {
		p, err := container.MappedPort(ctx, nat.Port(port))
		require.NoError(t, err)
		return p.Port()
	}
}

type MappedPortFunc func(port string) string

func RunScript(script []string) testcontainers.ContainerHook {
	return func(ctx context.Context, container testcontainers.Container) error {
		code, r, err := container.Exec(ctx, script)
		if err != nil {
			return err
		}
		if code == 0 {
			return nil
		}

		// Try to read the error message for the sake of debugging
		errBytes, readerErr := io.ReadAll(r)
		if readerErr != nil {
			return fmt.Errorf("setup script returned non-zero exit code: %d", code)
		}

		// Error message may have non-printable chars, so clean it up
		errStr := strings.Map(func(r rune) rune {
			if unicode.IsPrint(r) {
				return r
			}
			return -1
		}, string(errBytes))
		return errors.New(strings.TrimSpace(errStr))
	}
}
