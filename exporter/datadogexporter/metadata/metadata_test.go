// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package metadata

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter/config"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter/utils/cache"
)

func TestGetHostMetadata(t *testing.T) {
	cache.Cache.Flush()
	params := component.ExporterCreateParams{
		Logger: zap.NewNop(),
		ApplicationStartInfo: component.ApplicationStartInfo{
			ExeName: "otelcontribcol",
			Version: "1.0",
		},
	}

	cfg := &config.Config{TagsConfig: config.TagsConfig{
		Hostname: "hostname",
		Env:      "prod",
		Tags:     []string{"key1:tag1", "key2:tag2"},
	}}

	metadata := getHostMetadata(params, cfg)

	assert.Equal(t, metadata.InternalHostname, "hostname")
	assert.Equal(t, metadata.Flavor, "otelcontribcol")
	assert.Equal(t, metadata.Version, "1.0")
	assert.Equal(t, metadata.Meta.Hostname, "hostname")
	assert.ElementsMatch(t, metadata.Tags.OTel, []string{"key1:tag1", "key2:tag2", "env:prod"})
}

func TestPushMetadata(t *testing.T) {

	cfg := &config.Config{API: config.APIConfig{Key: "apikey"}}

	startInfo := component.ApplicationStartInfo{
		ExeName: "otelcontribcol",
		Version: "1.0",
	}

	metadata := HostMetadata{
		InternalHostname: "hostname",
		Flavor:           "otelcontribcol",
		Version:          "1.0",
		Tags:             &HostTags{OTel: []string{"key1:val1"}},
		Meta: &Meta{
			InstanceID:     "i-XXXXXXXXXX",
			EC2Hostname:    "ip-123-45-67-89",
			Hostname:       "hostname",
			SocketHostname: "ip-123-45-67-89",
			SocketFqdn:     "ip-123-45-67-89.internal",
		},
	}

	handler := http.NewServeMux()
	handler.HandleFunc("/intake", func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, r.Header.Get("DD-Api-Key"), "apikey")
		assert.Equal(t, r.Header.Get("User-Agent"), "otelcontribcol/1.0")

		body, err := ioutil.ReadAll(r.Body)
		require.NoError(t, err)

		var recvMetadata HostMetadata
		err = json.Unmarshal(body, &recvMetadata)
		require.NoError(t, err)
		assert.Equal(t, metadata, recvMetadata)
	})

	ts := httptest.NewServer(handler)
	defer ts.Close()
	cfg.Metrics.Endpoint = ts.URL

	err := pushMetadata(cfg, startInfo, &metadata)
	require.NoError(t, err)
}
