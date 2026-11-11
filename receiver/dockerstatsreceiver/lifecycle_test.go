// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package dockerstatsreceiver

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/receiver/receivertest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/dockerstatsreceiver/internal/metadata"
)

var mockFolder = filepath.Join("testdata", "mock")

// TestLifecycle stands in for the mdatagen lifecycle test, which needs a live Docker
// daemon. Talking to a mock daemon lets it run on every supported platform in CI.
func TestLifecycle(t *testing.T) {
	containerID := "10b703fb312b25e8368ab5a3bce3a1610d1cee5d71a94920f1a7adbc5b0cb326"
	mockDockerEngine, err := dockerMockServer(&map[string]string{
		"/v" + defaultDockerAPIVersion + "/containers/json":                      filepath.Join(mockFolder, "single_container", "containers.json"),
		"/v" + defaultDockerAPIVersion + "/containers/" + containerID + "/json":  filepath.Join(mockFolder, "single_container", "container.json"),
		"/v" + defaultDockerAPIVersion + "/containers/" + containerID + "/stats": filepath.Join(mockFolder, "single_container", "stats.json"),
	})
	require.NoError(t, err)
	defer mockDockerEngine.Close()

	factory := NewFactory()
	cfg := factory.CreateDefaultConfig().(*Config)
	cfg.Config.Endpoint = mockDockerEngine.URL

	for range 2 {
		rcvr, err := factory.CreateMetrics(t.Context(), receivertest.NewNopSettings(metadata.Type), cfg, consumertest.NewNop())
		require.NoError(t, err)
		require.NoError(t, rcvr.Start(t.Context(), componenttest.NewNopHost()))
		require.NoError(t, rcvr.Shutdown(t.Context()))
	}
}

func dockerMockServer(urlToFile *map[string]string) (*httptest.Server, error) {
	urlToFileContents := make(map[string][]byte, len(*urlToFile))
	for urlPath, filePath := range *urlToFile {
		fileContents, err := os.ReadFile(filepath.Clean(filePath))
		if err != nil {
			return nil, err
		}
		urlToFileContents[urlPath] = fileContents
	}

	return httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		data, ok := urlToFileContents[req.URL.Path]
		if !ok {
			rw.WriteHeader(http.StatusNotFound)
			return
		}
		rw.WriteHeader(http.StatusOK)
		_, _ = rw.Write(data)
	})), nil
}
