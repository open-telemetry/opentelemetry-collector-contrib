// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package scraperinttest // import "github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/scraperinttest"

import (
	"bytes"
	"context"
	"encoding/json"
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
	"gopkg.in/yaml.v3"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/golden"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/pmetrictest"
)

const errExposedPort = "exposed container port should not be hardcoded to host port. Use ContainerInfo.MappedPort() instead"

func NewIntegrationTest(f receiver.Factory, opts ...TestOption) *IntegrationTest {
	it := &IntegrationTest{
		factory:                f,
		createContainerTimeout: 5 * time.Minute,
		customConfig:           func(*testing.T, component.Config, *ContainerInfo) {},
		expectedFile:           filepath.Join("testdata", "integration", "expected.yaml"),
		compareTimeout:         time.Minute,
	}
	for _, opt := range opts {
		opt(it)
	}
	return it
}

type IntegrationTest struct {
	containerRequest       *testcontainers.ContainerRequest
	createContainerTimeout time.Duration

	factory      receiver.Factory
	customConfig customConfigFunc

	expectedFile   string
	compareOptions []pmetrictest.CompareMetricsOption
	compareTimeout time.Duration

	dumpActualOnFailure bool
}

func (it *IntegrationTest) Run(t *testing.T) {
	ctx := context.Background()

	var ci *ContainerInfo
	if it.containerRequest != nil {
		require.NoError(t, it.containerRequest.Validate())
		for _, port := range it.containerRequest.ExposedPorts {
			require.False(t, strings.ContainsRune(port, ':'), errExposedPort)
		}
		var container testcontainers.Container
		var err error
		require.Eventually(t, func() bool {
			container, err = testcontainers.GenericContainer(ctx,
				testcontainers.GenericContainerRequest{
					ContainerRequest: *it.containerRequest,
					Started:          true,
				})
			return err == nil
		}, it.createContainerTimeout, time.Second)

		defer func() {
			require.NoError(t, container.Terminate(context.Background()))
		}()

		ci, err = containerInfo(ctx, container, it.containerRequest.ExposedPorts)
		require.NoError(t, err)
	}

	cfg := it.factory.CreateDefaultConfig()
	it.customConfig(t, cfg, ci)
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
			numResults := len(sink.AllMetrics())
			if numResults == 0 {
				t.Error("no data emitted by scraper")
				return
			}
			if it.dumpActualOnFailure {
				// TODO copied from golden package - expose elsewhere
				unmarshaler := &pmetric.JSONMarshaler{}
				fileBytes, err := unmarshaler.MarshalMetrics(sink.AllMetrics()[numResults-1])
				if err != nil {
					t.Errorf("failed to marshal actual metrics to JSON: %v", err)
					return
				}

				var jsonVal map[string]interface{}
				if err = json.Unmarshal(fileBytes, &jsonVal); err != nil {
					t.Errorf("failed to unmarshal actual metrics JSON: %v", err)
					return
				}
				b := &bytes.Buffer{}
				enc := yaml.NewEncoder(b)
				enc.SetIndent(2)
				if err := enc.Encode(jsonVal); err != nil {
					t.Errorf("failed to encode actual metrics to YAML: %v", err)
					return
				}
				t.Errorf("latest result:\n%s", b.Bytes())
			}
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

func WithContainerRequest(cr testcontainers.ContainerRequest) TestOption {
	return func(it *IntegrationTest) {
		it.containerRequest = &cr
	}
}

func WithCreateContainerTimeout(t time.Duration) TestOption {
	return func(it *IntegrationTest) {
		it.createContainerTimeout = t
	}
}

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

func WithDumpActualOnFailure() TestOption {
	return func(it *IntegrationTest) {
		it.dumpActualOnFailure = true
	}
}

type customConfigFunc func(*testing.T, component.Config, *ContainerInfo)

type ContainerInfo struct {
	host  string
	ports map[string]string
}

func (c ContainerInfo) Host(t *testing.T) string {
	require.NotEmpty(t, c.host, "container not in use")
	return c.host
}

func (c ContainerInfo) MappedPort(t *testing.T, port string) string {
	p, ok := c.ports[port]
	require.True(t, ok, "port not exposed %q", port)
	return p
}

func containerInfo(ctx context.Context, c testcontainers.Container, ports []string) (*ContainerInfo, error) {
	h, err := c.Host(ctx)
	if err != nil {
		return nil, fmt.Errorf("get container host: %w", err)
	}

	portMap := make(map[string]string, len(ports))
	for _, internalPort := range ports {
		externalPort, err := c.MappedPort(ctx, nat.Port(internalPort))
		if err != nil {
			return nil, fmt.Errorf("get mapped port for %q: %w", internalPort, err)
		}
		portMap[internalPort] = externalPort.Port()
	}
	return &ContainerInfo{host: h, ports: portMap}, nil
}

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
