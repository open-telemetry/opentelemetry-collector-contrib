// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package hostmetadata

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/DataDog/opentelemetry-mapping-go/pkg/otlp/attributes"
	"github.com/DataDog/opentelemetry-mapping-go/pkg/otlp/attributes/azure"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/exporter/exportertest"
	"go.opentelemetry.io/collector/pdata/pcommon"
	conventions "go.opentelemetry.io/collector/semconv/v1.6.1"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter/internal/testutil"
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

	mockBuildInfo = component.BuildInfo{
		Command: "otelcontribcol",
		Version: "1.0",
	}

	mockExporterCreateSettings = exporter.CreateSettings{
		TelemetrySettings: componenttest.NewNopTelemetrySettings(),
		BuildInfo:         mockBuildInfo,
	}
)

func TestFillHostMetadata(t *testing.T) {
	params := exportertest.NewNopCreateSettings()
	params.BuildInfo = mockBuildInfo

	pcfg := PusherConfig{
		ConfigHostname: "hostname",
		ConfigTags:     []string{"key1:tag1", "key2:tag2", "env:prod"},
	}

	hostProvider, err := GetSourceProvider(componenttest.NewNopTelemetrySettings(), "hostname")
	require.NoError(t, err)

	metadata := &HostMetadata{Meta: &Meta{}, Tags: &HostTags{}}
	fillHostMetadata(params, pcfg, hostProvider, metadata)

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

	fillHostMetadata(params, pcfg, hostProvider, metadataWithVals)
	assert.Equal(t, metadataWithVals.InternalHostname, "my-custom-hostname")
	assert.Equal(t, metadataWithVals.Flavor, "otelcontribcol")
	assert.Equal(t, metadataWithVals.Version, "1.0")
	assert.Equal(t, metadataWithVals.Meta.Hostname, "my-custom-hostname")
	assert.ElementsMatch(t, metadataWithVals.Tags.OTel, []string{"key1:tag1", "key2:tag2", "env:prod"})
}

func TestMetadataFromAttributes(t *testing.T) {
	tests := []struct {
		name     string
		attrs    pcommon.Map
		expected *HostMetadata
	}{
		{
			name: "AWS",
			attrs: testutil.NewAttributeMap(map[string]string{
				conventions.AttributeCloudProvider: conventions.AttributeCloudProviderAWS,
				conventions.AttributeHostID:        "host-id",
				conventions.AttributeHostName:      "ec2amaz-host-name",
				"ec2.tag.tag1":                     "val1",
				"ec2.tag.tag2":                     "val2",
			}),
			expected: &HostMetadata{
				InternalHostname: "host-id",
				Meta: &Meta{
					Hostname:    "host-id",
					InstanceID:  "host-id",
					EC2Hostname: "ec2amaz-host-name",
				},
				Tags: &HostTags{OTel: []string{"tag1:val1", "tag2:val2"}},
			},
		},
		{
			name: "GCP",
			attrs: testutil.NewAttributeMap(map[string]string{
				conventions.AttributeCloudProvider:         conventions.AttributeCloudProviderGCP,
				conventions.AttributeHostID:                "host-id",
				conventions.AttributeCloudAccountID:        "project-id",
				conventions.AttributeHostName:              "host-name",
				conventions.AttributeHostType:              "host-type",
				conventions.AttributeCloudAvailabilityZone: "cloud-zone",
			}),
			expected: &HostMetadata{
				InternalHostname: "host-name.project-id",
				Meta: &Meta{
					Hostname: "host-name.project-id",
				},
				Tags: &HostTags{
					GCP: []string{"instance-id:host-id", "project:project-id", "zone:cloud-zone", "instance-type:host-type"},
				},
			},
		},
		{
			name: "Azure",
			attrs: testutil.NewAttributeMap(map[string]string{
				conventions.AttributeCloudProvider:  conventions.AttributeCloudProviderAzure,
				conventions.AttributeHostName:       "azure-host-name",
				conventions.AttributeCloudRegion:    "location",
				conventions.AttributeHostID:         "azure-vm-id",
				conventions.AttributeCloudAccountID: "subscriptionID",
				azure.AttributeResourceGroupName:    "resourceGroup",
			}),
			expected: &HostMetadata{
				InternalHostname: "azure-vm-id",
				Meta: &Meta{
					Hostname: "azure-vm-id",
				},
				Tags: &HostTags{},
			},
		},
		{
			name: "Custom name",
			attrs: testutil.NewAttributeMap(map[string]string{
				attributes.AttributeDatadogHostname: "custom-name",
			}),
			expected: &HostMetadata{
				InternalHostname: "custom-name",
				Meta: &Meta{
					Hostname: "custom-name",
				},
				Tags: &HostTags{},
			},
		},
	}

	for _, testInstance := range tests {
		t.Run(testInstance.name, func(t *testing.T) {
			metadata := metadataFromAttributes(testInstance.attrs)
			assert.Equal(t, testInstance.expected.InternalHostname, metadata.InternalHostname)
			assert.Equal(t, testInstance.expected.Meta, metadata.Meta)
			assert.ElementsMatch(t, testInstance.expected.Tags.GCP, metadata.Tags.GCP)
			assert.ElementsMatch(t, testInstance.expected.Tags.OTel, metadata.Tags.OTel)
		})
	}
}

func TestPushMetadata(t *testing.T) {
	pcfg := PusherConfig{
		APIKey: "apikey",
	}

	handler := http.NewServeMux()
	handler.HandleFunc("/intake", func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, r.Header.Get("DD-Api-Key"), "apikey")
		assert.Equal(t, r.Header.Get("User-Agent"), "otelcontribcol/1.0")

		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)

		var recvMetadata HostMetadata
		err = json.Unmarshal(body, &recvMetadata)
		require.NoError(t, err)
		assert.Equal(t, mockMetadata, recvMetadata)
	})

	ts := httptest.NewServer(handler)
	defer ts.Close()
	pcfg.MetricsEndpoint = ts.URL

	err := pushMetadata(pcfg, mockExporterCreateSettings, &mockMetadata)
	require.NoError(t, err)
}

func TestFailPushMetadata(t *testing.T) {
	pcfg := PusherConfig{
		APIKey: "apikey",
	}
	handler := http.NewServeMux()
	handler.Handle("/intake", http.NotFoundHandler())

	ts := httptest.NewServer(handler)
	defer ts.Close()
	pcfg.MetricsEndpoint = ts.URL

	err := pushMetadata(pcfg, mockExporterCreateSettings, &mockMetadata)
	require.Error(t, err)
}

func TestPusher(t *testing.T) {
	pcfg := PusherConfig{
		APIKey:              "apikey",
		UseResourceMetadata: true,
	}
	params := exportertest.NewNopCreateSettings()
	params.BuildInfo = mockBuildInfo

	hostProvider, err := GetSourceProvider(componenttest.NewNopTelemetrySettings(), "")
	require.NoError(t, err)

	attrs := testutil.NewAttributeMap(map[string]string{
		attributes.AttributeDatadogHostname: "datadog-hostname",
	})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	server := testutil.DatadogServerMock()
	defer server.Close()
	pcfg.MetricsEndpoint = server.URL

	go Pusher(ctx, params, pcfg, hostProvider, attrs)

	body := <-server.MetadataChan
	var recvMetadata HostMetadata
	err = json.Unmarshal(body, &recvMetadata)
	require.NoError(t, err)
	assert.Equal(t, recvMetadata.InternalHostname, "datadog-hostname")
	assert.Equal(t, recvMetadata.Version, mockBuildInfo.Version)
	assert.Equal(t, recvMetadata.Flavor, mockBuildInfo.Command)
	require.NotNil(t, recvMetadata.Meta)
	hostname, err := os.Hostname()
	require.NoError(t, err)
	assert.Equal(t, recvMetadata.Meta.SocketHostname, hostname)
}
