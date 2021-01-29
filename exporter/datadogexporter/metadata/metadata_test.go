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
	"context"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/translator/conventions"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter/config"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter/testutils"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter/utils/cache"
)

var (
	mockMetadata = HostMetadata{
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

	mockStartInfo = component.ApplicationStartInfo{
		ExeName: "otelcontribcol",
		Version: "1.0",
	}
)

func TestFillHostMetadata(t *testing.T) {
	cache.Cache.Flush()
	params := component.ExporterCreateParams{
		Logger:               zap.NewNop(),
		ApplicationStartInfo: mockStartInfo,
	}

	cfg := &config.Config{TagsConfig: config.TagsConfig{
		Hostname: "hostname",
		Env:      "prod",
		Tags:     []string{"key1:tag1", "key2:tag2"},
	}}

	metadata := &HostMetadata{Meta: &Meta{}, Tags: &HostTags{}}
	fillHostMetadata(params, cfg, metadata)

	assert.Equal(t, metadata.InternalHostname, "hostname")
	assert.Equal(t, metadata.Flavor, "otelcontribcol")
	assert.Equal(t, metadata.Version, "1.0")
	assert.Equal(t, metadata.Meta.Hostname, "hostname")
	assert.ElementsMatch(t, metadata.Tags.OTel, []string{"key1:tag1", "key2:tag2", "env:prod"})

	metadataWithVals := &HostMetadata{
		InternalHostname: "my-custom-hostname",
		Meta:             &Meta{Hostname: "my-custom-hostname"},
		Tags:             &HostTags{},
	}

	fillHostMetadata(params, cfg, metadataWithVals)
	assert.Equal(t, metadataWithVals.InternalHostname, "my-custom-hostname")
	assert.Equal(t, metadataWithVals.Flavor, "otelcontribcol")
	assert.Equal(t, metadataWithVals.Version, "1.0")
	assert.Equal(t, metadataWithVals.Meta.Hostname, "my-custom-hostname")
	assert.ElementsMatch(t, metadataWithVals.Tags.OTel, []string{"key1:tag1", "key2:tag2", "env:prod"})
}

func TestMetadataFromAttributes(t *testing.T) {
	// AWS
	attrsAWS := testutils.NewAttributeMap(map[string]string{
		conventions.AttributeCloudProvider: conventions.AttributeCloudProviderAWS,
		conventions.AttributeHostID:        "host-id",
		conventions.AttributeHostName:      "ec2amaz-host-name",
		"ec2.tag.tag1":                     "val1",
		"ec2.tag.tag2":                     "val2",
	})
	metadataAWS := metadataFromAttributes(attrsAWS)
	assert.Equal(t, metadataAWS.InternalHostname, "host-id")
	assert.Equal(t, metadataAWS.Meta,
		&Meta{
			Hostname:    "host-id",
			InstanceID:  "host-id",
			EC2Hostname: "ec2amaz-host-name",
		})
	assert.ElementsMatch(t, metadataAWS.Tags.OTel, []string{"tag1:val1", "tag2:val2"})

	// GCP
	attrsGCP := testutils.NewAttributeMap(map[string]string{
		conventions.AttributeCloudProvider: conventions.AttributeCloudProviderGCP,
		conventions.AttributeHostID:        "host-id",
		conventions.AttributeHostName:      "host-name",
		conventions.AttributeHostType:      "host-type",
		conventions.AttributeCloudZone:     "cloud-zone",
	})
	metadataGCP := metadataFromAttributes(attrsGCP)
	assert.Equal(t, metadataGCP.InternalHostname, "host-name")
	assert.Equal(t, metadataGCP.Meta.Hostname, "host-name")
	assert.ElementsMatch(t, metadataGCP.Meta.HostAliases, []string{"host-id"})
	assert.ElementsMatch(t, metadataGCP.Tags.GCP,
		[]string{"instance-id:host-id", "zone:cloud-zone", "instance-type:host-type"})

	// Other
	attrsOther := testutils.NewAttributeMap(map[string]string{
		AttributeDatadogHostname: "custom-name",
	})
	metadataOther := metadataFromAttributes(attrsOther)
	assert.Equal(t, metadataOther.InternalHostname, "custom-name")
	assert.Equal(t, metadataOther.Meta, &Meta{Hostname: "custom-name"})
	assert.Equal(t, metadataOther.Tags, &HostTags{})

}

func TestPushMetadata(t *testing.T) {
	cfg := &config.Config{API: config.APIConfig{Key: "apikey"}}

	handler := http.NewServeMux()
	handler.HandleFunc("/intake", func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, r.Header.Get("DD-Api-Key"), "apikey")
		assert.Equal(t, r.Header.Get("User-Agent"), "otelcontribcol/1.0")

		body, err := ioutil.ReadAll(r.Body)
		require.NoError(t, err)

		var recvMetadata HostMetadata
		err = json.Unmarshal(body, &recvMetadata)
		require.NoError(t, err)
		assert.Equal(t, mockMetadata, recvMetadata)
	})

	ts := httptest.NewServer(handler)
	defer ts.Close()
	cfg.Metrics.Endpoint = ts.URL

	err := pushMetadata(cfg, mockStartInfo, &mockMetadata)
	require.NoError(t, err)
}

func TestFailPushMetadata(t *testing.T) {
	cfg := &config.Config{API: config.APIConfig{Key: "apikey"}}

	handler := http.NewServeMux()
	handler.Handle("/intake", http.NotFoundHandler())

	ts := httptest.NewServer(handler)
	defer ts.Close()
	cfg.Metrics.Endpoint = ts.URL

	err := pushMetadata(cfg, mockStartInfo, &mockMetadata)
	require.Error(t, err)
}

func TestPusher(t *testing.T) {
	cfg := &config.Config{
		API:                 config.APIConfig{Key: "apikey"},
		UseResourceMetadata: true,
	}
	mockParams := component.ExporterCreateParams{
		Logger:               zap.NewNop(),
		ApplicationStartInfo: mockStartInfo,
	}
	attrs := testutils.NewAttributeMap(map[string]string{
		AttributeDatadogHostname: "datadog-hostname",
	})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	server := testutils.DatadogServerMock()
	defer server.Close()
	cfg.Metrics.Endpoint = server.URL

	go Pusher(ctx, mockParams, cfg, attrs)

	body := <-server.MetadataChan
	var recvMetadata HostMetadata
	err := json.Unmarshal(body, &recvMetadata)
	require.NoError(t, err)
	assert.Equal(t, recvMetadata.InternalHostname, "datadog-hostname")
	assert.Equal(t, recvMetadata.Version, mockStartInfo.Version)
	assert.Equal(t, recvMetadata.Flavor, mockStartInfo.ExeName)
	require.NotNil(t, recvMetadata.Meta)
	hostname, err := os.Hostname()
	require.NoError(t, err)
	assert.Equal(t, recvMetadata.Meta.SocketHostname, hostname)
}
