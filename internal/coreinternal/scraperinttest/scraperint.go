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
	"sync"
	"testing"
	"time"
	"unicode"

	"github.com/docker/go-connections/nat"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/receiver/receivertest"
	"go.uber.org/multierr"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/golden"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/pmetrictest"
)

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
	containerRequests      []testcontainers.ContainerRequest
	allowHardcodedHostPort bool
	createContainerTimeout time.Duration

	factory      receiver.Factory
	customConfig customConfigFunc

	expectedFile   string
	compareOptions []pmetrictest.CompareMetricsOption
	compareTimeout time.Duration

	failOnErrorLogs bool
	writeExpected   bool
}

func (it *IntegrationTest) Run(t *testing.T) {
	it.validate(t)
	ci := it.createContainers(t)
	defer ci.terminate(t)

	cfg := it.factory.CreateDefaultConfig()
	it.customConfig(t, cfg, ci)
	sink := new(consumertest.MetricsSink)
	settings := receivertest.NewNopSettings()
	observedZapCore, observedLogs := observer.New(zap.WarnLevel)
	settings.Logger = zap.New(observedZapCore)

	rcvr, err := it.factory.CreateMetrics(context.Background(), settings, cfg, sink)
	require.NoError(t, err, "failed creating metrics receiver")
	require.NoError(t, rcvr.Start(context.Background(), componenttest.NewNopHost()))
	defer func() {
		require.NoError(t, rcvr.Shutdown(context.Background()))
	}()

	var expected pmetric.Metrics
	if !it.writeExpected {
		expected, err = golden.ReadMetrics(it.expectedFile)
		require.NoError(t, err)
	}

	// Defined outside of Eventually so it can be printed if the test fails
	var validateErr error
	defer func() {
		if t.Failed() && validateErr != nil {
			t.Error(validateErr.Error())

			logs := strings.Builder{}
			for _, e := range observedLogs.All() {
				logs.WriteString(e.Message + "\n")
			}
			t.Errorf("full log:\n%s", logs.String())

			if len(sink.AllMetrics()) == 0 {
				t.Error("no data emitted by scraper")
				return
			}
			metricBytes, err := golden.MarshalMetricsYAML(sink.AllMetrics()[len(sink.AllMetrics())-1])
			require.NoError(t, err)
			t.Errorf("latest result:\n%s", metricBytes)
		}
	}()

	require.Eventually(t,
		func() bool {
			allMetrics := sink.AllMetrics()
			if len(allMetrics) == 0 {
				return false
			}
			if it.failOnErrorLogs && len(observedLogs.All()) > 0 {
				logs := strings.Builder{}
				for _, e := range observedLogs.All() {
					logs.WriteString(e.Message + "\n")
				}
				t.Errorf("full log:\n%s", logs.String())
			}

			if it.writeExpected {
				require.NoError(t, golden.WriteMetrics(t, it.expectedFile, allMetrics[0]))
				return true
			}
			validateErr = pmetrictest.CompareMetrics(expected, allMetrics[len(allMetrics)-1], it.compareOptions...)
			return validateErr == nil
		},
		it.compareTimeout, it.compareTimeout/20)
}

func (it *IntegrationTest) createContainers(t *testing.T) *ContainerInfo {
	var wg sync.WaitGroup
	ci := &ContainerInfo{
		containers: make(map[string]testcontainers.Container, len(it.containerRequests)),
	}
	wg.Add(len(it.containerRequests))
	for _, cr := range it.containerRequests {
		go func(req testcontainers.ContainerRequest) {
			var errs error
			assert.Eventuallyf(t, func() bool {
				c, err := testcontainers.GenericContainer(
					context.Background(),
					testcontainers.GenericContainerRequest{
						ContainerRequest: req,
						Started:          true,
					})
				if err != nil {
					errs = multierr.Append(errs, fmt.Errorf("execute container request: %w", err))
					return false
				}
				ci.add(req.Name, c)
				return true
			}, it.createContainerTimeout, time.Second, "create container timeout: %v", errs)
			wg.Done()
		}(cr)
	}
	wg.Wait()
	return ci
}

func (it *IntegrationTest) validate(t *testing.T) {
	containerNames := make(map[string]bool, len(it.containerRequests))
	for _, cr := range it.containerRequests {
		if _, ok := containerNames[cr.Name]; ok {
			require.False(t, ok, "duplicate container name: %q", cr.Name)
		} else {
			containerNames[cr.Name] = true
		}
		if !it.allowHardcodedHostPort {
			for _, port := range cr.ExposedPorts {
				require.False(t, strings.ContainsRune(port, ':'), "exposed port hardcoded to host port: %q", port)
			}
		}
		require.NoError(t, cr.Validate())
	}
}

type TestOption func(*IntegrationTest)

func WithContainerRequest(cr testcontainers.ContainerRequest) TestOption {
	return func(it *IntegrationTest) {
		it.containerRequests = append(it.containerRequests, cr)
	}
}

func AllowHardcodedHostPort() TestOption {
	return func(it *IntegrationTest) {
		it.allowHardcodedHostPort = true
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

// This option is useful for debugging scrapers but should not be used permanently
// because the logs do not correlate to a single scrape interval. In other words,
// when a retryable failure occurs, this setting will likely force a failure anyways.
func FailOnErrorLogs() TestOption {
	return func(it *IntegrationTest) {
		it.failOnErrorLogs = true
	}
}

func WriteExpected() TestOption {
	return func(it *IntegrationTest) {
		it.writeExpected = true
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

type customConfigFunc func(*testing.T, component.Config, *ContainerInfo)

type ContainerInfo struct {
	sync.Mutex
	containers map[string]testcontainers.Container
}

func (ci *ContainerInfo) Host(t *testing.T) string {
	return ci.HostForNamedContainer(t, "")
}

func (ci *ContainerInfo) HostForNamedContainer(t *testing.T, containerName string) string {
	c := ci.container(t, containerName)
	h, err := c.Host(context.Background())
	require.NoErrorf(t, err, "get host for container %q: %v", containerName, err)
	return h
}

func (ci *ContainerInfo) MappedPort(t *testing.T, port string) string {
	return ci.MappedPortForNamedContainer(t, "", port)
}

func (ci *ContainerInfo) MappedPortForNamedContainer(t *testing.T, containerName string, port string) string {
	c := ci.container(t, containerName)
	p, err := c.MappedPort(context.Background(), nat.Port(port))
	require.NoErrorf(t, err, "get port %q for container %q: %v", port, containerName, err)
	return p.Port()
}

func (ci *ContainerInfo) container(t *testing.T, name string) testcontainers.Container {
	require.NotEmpty(t, ci.containers, "no containers in use")
	c, ok := ci.containers[name]
	require.True(t, ok, "container with name %q not found", name)
	return c
}

func (ci *ContainerInfo) add(name string, c testcontainers.Container) {
	ci.Lock()
	defer ci.Unlock()
	ci.containers[name] = c
}

func (ci *ContainerInfo) terminate(t *testing.T) {
	for name, c := range ci.containers {
		require.NoError(t, c.Terminate(context.Background()), "terminate container %q", name)
	}
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
