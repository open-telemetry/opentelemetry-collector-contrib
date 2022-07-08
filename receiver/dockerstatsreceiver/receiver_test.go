// Copyright 2020 OpenTelemetry Authors
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

//go:build !windows
// +build !windows

// TODO review if tests should succeed on Windows

package dockerstatsreceiver

import (
	"context"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver/scraperhelper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/scrapertest"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/scrapertest/golden"
)

var mockFolder = filepath.Join("testdata", "mock")

func newMockServer(tb testing.TB) *httptest.Server {
	statsFile, err := os.Open(filepath.Join(mockFolder, "stats.json"))
	assert.NoError(tb, err)
	stats, err := ioutil.ReadAll(statsFile)
	assert.NoError(tb, err)
	_ = statsFile.Close()

	containersFile, err := os.Open(filepath.Join(mockFolder, "containers.json"))
	assert.NoError(tb, err)
	containers, err := ioutil.ReadAll(containersFile)
	assert.NoError(tb, err)
	_ = containersFile.Close()

	detailedContainerFile, err := os.Open(filepath.Join(mockFolder, "container.json"))
	assert.NoError(tb, err)
	detailedContainer, err := ioutil.ReadAll(detailedContainerFile)
	assert.NoError(tb, err)
	_ = detailedContainerFile.Close()

	return httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		containerID := "10b703fb312b25e8368ab5a3bce3a1610d1cee5d71a94920f1a7adbc5b0cb326"
		if strings.Contains(req.URL.Path, "/containers/json") {
			rw.WriteHeader(http.StatusOK)
			_, err := rw.Write(containers)
			require.NoError(tb, err)
		} else if strings.Contains(req.URL.Path, "containers/"+containerID+"/json") {
			rw.WriteHeader(http.StatusOK)
			_, err := rw.Write(detailedContainer)
			require.NoError(tb, err)
		} else if strings.Contains(req.URL.Path, "stats") {
			rw.WriteHeader(http.StatusOK)
			_, err := rw.Write(stats)
			require.NoError(tb, err)
		} else {
			rw.WriteHeader(http.StatusNotFound)
		}
		return
	}))
}

func TestNewReceiver(t *testing.T) {
	cfg := &Config{
		ScraperControllerSettings: scraperhelper.ScraperControllerSettings{
			CollectionInterval: 1 * time.Second,
		},
		Endpoint:         "unix:///run/some.sock",
		DockerAPIVersion: defaultDockerAPIVersion,
	}
	mr := newReceiver(componenttest.NewNopReceiverCreateSettings(), cfg)
	assert.NotNil(t, mr)
}

func TestErrorsInStart(t *testing.T) {
	unreachable := "unix:///not/a/thing.sock"
	cfg := &Config{
		ScraperControllerSettings: scraperhelper.ScraperControllerSettings{
			CollectionInterval: 1 * time.Second,
		},
		Endpoint:         unreachable,
		DockerAPIVersion: defaultDockerAPIVersion,
	}
	recv := newReceiver(componenttest.NewNopReceiverCreateSettings(), cfg)
	assert.NotNil(t, recv)

	cfg.Endpoint = "..not/a/valid/endpoint"
	err := recv.start(context.Background(), componenttest.NewNopHost())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unable to parse docker host")

	cfg.Endpoint = unreachable
	err = recv.start(context.Background(), componenttest.NewNopHost())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "context deadline exceeded")
}

func TestScrapes(t *testing.T) {
	ms := newMockServer(t)
	defer ms.Close()

	testCases := []struct {
		desc                string
		scrape              func(*receiver) (pmetric.Metrics, error)
		expectedMetricsFile string
		mockDockerEngine    *httptest.Server
	}{
		{
			desc: "scrape v1 (no mdatagen)",
			scrape: func(rcv *receiver) (pmetric.Metrics, error) {
				return rcv.scrape(context.Background())
			},
			expectedMetricsFile: "expected_metrics.json",
			mockDockerEngine:    ms,
		},
		{
			desc: "scrape v2 (uses mdatagen)",
			scrape: func(rcv *receiver) (pmetric.Metrics, error) {
				return rcv.scrapeV2(context.Background())
			},
			expectedMetricsFile: "expected_metrics.json",
			mockDockerEngine:    ms,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			cfg := createDefaultConfig().(*Config)
			cfg.Endpoint = tc.mockDockerEngine.URL
			cfg.EnvVarsToMetricLabels = map[string]string{"ENV_VAR": "env-var-metric-label"}
			cfg.ContainerLabelsToMetricLabels = map[string]string{"container.label": "container-metric-label"}

			receiver := newReceiver(componenttest.NewNopReceiverCreateSettings(), cfg)
			err := receiver.start(context.Background(), componenttest.NewNopHost())
			require.NoError(t, err)

			actualMetrics, err := tc.scrape(receiver)
			require.NoError(t, err)
			assert.Equal(t, 1, actualMetrics.ResourceMetrics().Len())

			expectedMetrics, err := golden.ReadMetrics(filepath.Join(mockFolder, tc.expectedMetricsFile))
			assert.NoError(t, err)
			assert.NoError(t, scrapertest.CompareMetrics(expectedMetrics, actualMetrics))
		})
	}
}
