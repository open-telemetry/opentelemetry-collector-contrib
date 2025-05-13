// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package hostmetadata

import (
	"compress/gzip"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/DataDog/datadog-agent/comp/otelcol/otlp/testutil"
	"github.com/DataDog/opentelemetry-mapping-go/pkg/inframetadata"
	"github.com/DataDog/opentelemetry-mapping-go/pkg/inframetadata/payload"
	"github.com/DataDog/opentelemetry-mapping-go/pkg/otlp/attributes"
	"github.com/DataDog/opentelemetry-mapping-go/pkg/otlp/attributes/azure"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/exporter/exportertest"
	"go.opentelemetry.io/collector/pdata/pcommon"
	conventions "go.opentelemetry.io/otel/semconv/v1.6.1"
	"go.uber.org/zap"
)

var (
	mockMetadata = payload.HostMetadata{
		InternalHostname: "hostname",
		Flavor:           "otelcontribcol",
		Version:          "1.0",
		Tags:             &payload.HostTags{OTel: []string{"key1:val1"}},
		Meta: &payload.Meta{
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

	mockExporterCreateSettings = exporter.Settings{
		TelemetrySettings: componenttest.NewNopTelemetrySettings(),
		BuildInfo:         mockBuildInfo,
	}
)

func TestFillHostMetadata(t *testing.T) {
	params := exportertest.NewNopSettings(exportertest.NopType)
	params.BuildInfo = mockBuildInfo

	pcfg := PusherConfig{
		ConfigHostname: "hostname",
		ConfigTags:     []string{"key1:tag1", "key2:tag2", "env:prod"},
	}

	hostProvider, err := GetSourceProvider(componenttest.NewNopTelemetrySettings(), "hostname", 31*time.Second)
	require.NoError(t, err)

	metadata := payload.NewEmpty()
	fillHostMetadata(params, pcfg, hostProvider, &metadata)

	assert.Equal(t, "hostname", metadata.InternalHostname)
	assert.Equal(t, "otelcontribcol", metadata.Flavor)
	assert.Equal(t, "1.0", metadata.Version)
	assert.Equal(t, "hostname", metadata.Meta.Hostname)
	assert.ElementsMatch(t, metadata.Tags.OTel, []string{"key1:tag1", "key2:tag2", "env:prod"})

	metadataWithVals := payload.HostMetadata{
		InternalHostname: "my-custom-hostname",
		Meta:             &payload.Meta{Hostname: "my-custom-hostname"},
		Tags:             &payload.HostTags{},
	}

	fillHostMetadata(params, pcfg, hostProvider, &metadataWithVals)
	assert.Equal(t, "my-custom-hostname", metadataWithVals.InternalHostname)
	assert.Equal(t, "otelcontribcol", metadataWithVals.Flavor)
	assert.Equal(t, "1.0", metadataWithVals.Version)
	assert.Equal(t, "my-custom-hostname", metadataWithVals.Meta.Hostname)
	assert.ElementsMatch(t, metadataWithVals.Tags.OTel, []string{"key1:tag1", "key2:tag2", "env:prod"})
}

func TestMetadataFromAttributes(t *testing.T) {
	tests := []struct {
		name     string
		attrs    pcommon.Map
		expected *payload.HostMetadata
	}{
		{
			name: "AWS",
			attrs: testutil.NewAttributeMap(map[string]string{
				string(conventions.CloudProviderKey): conventions.CloudProviderAWS.Value.AsString(),
				string(conventions.HostIDKey):        "host-id",
				string(conventions.HostNameKey):      "ec2amaz-host-name",
				"ec2.tag.tag1":                       "val1",
				"ec2.tag.tag2":                       "val2",
			}),
			expected: &payload.HostMetadata{
				InternalHostname: "host-id",
				Meta: &payload.Meta{
					Hostname:    "host-id",
					InstanceID:  "host-id",
					EC2Hostname: "ec2amaz-host-name",
				},
				Tags: &payload.HostTags{OTel: []string{"tag1:val1", "tag2:val2"}},
			},
		},
		{
			name: "GCP",
			attrs: testutil.NewAttributeMap(map[string]string{
				string(conventions.CloudProviderKey):         conventions.CloudProviderGCP.Value.AsString(),
				string(conventions.HostIDKey):                "host-id",
				string(conventions.CloudAccountIDKey):        "project-id",
				string(conventions.HostNameKey):              "host-name",
				string(conventions.HostTypeKey):              "host-type",
				string(conventions.CloudAvailabilityZoneKey): "cloud-zone",
			}),
			expected: &payload.HostMetadata{
				InternalHostname: "host-name.project-id",
				Meta: &payload.Meta{
					Hostname: "host-name.project-id",
				},
				Tags: &payload.HostTags{
					GCP: []string{"instance-id:host-id", "project:project-id", "zone:cloud-zone", "instance-type:host-type"},
				},
			},
		},
		{
			name: "Azure",
			attrs: testutil.NewAttributeMap(map[string]string{
				string(conventions.CloudProviderKey):  conventions.CloudProviderAzure.Value.AsString(),
				string(conventions.HostNameKey):       "azure-host-name",
				string(conventions.CloudRegionKey):    "location",
				string(conventions.HostIDKey):         "azure-vm-id",
				string(conventions.CloudAccountIDKey): "subscriptionID",
				azure.AttributeResourceGroupName:      "resourceGroup",
			}),
			expected: &payload.HostMetadata{
				InternalHostname: "azure-vm-id",
				Meta: &payload.Meta{
					Hostname: "azure-vm-id",
				},
				Tags: &payload.HostTags{},
			},
		},
		{
			name: "Custom name",
			attrs: testutil.NewAttributeMap(map[string]string{
				attributes.AttributeDatadogHostname: "custom-name",
			}),
			expected: &payload.HostMetadata{
				InternalHostname: "custom-name",
				Meta: &payload.Meta{
					Hostname: "custom-name",
				},
				Tags: &payload.HostTags{},
			},
		},
	}

	for _, testInstance := range tests {
		t.Run(testInstance.name, func(t *testing.T) {
			metadata := metadataFromAttributes(testInstance.attrs, nil)
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
	handler.HandleFunc("/intake", func(_ http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "apikey", r.Header.Get("DD-Api-Key"))
		assert.Equal(t, "otelcontribcol/1.0", r.Header.Get("User-Agent"))
		reader, err := gzip.NewReader(r.Body)
		assert.NoError(t, err)
		body, err := io.ReadAll(reader)
		assert.NoError(t, err)

		var recvMetadata payload.HostMetadata
		err = json.Unmarshal(body, &recvMetadata)
		assert.NoError(t, err)
		assert.Equal(t, mockMetadata, recvMetadata)
	})

	ts := httptest.NewServer(handler)
	defer ts.Close()
	pcfg.MetricsEndpoint = ts.URL

	pusher := NewPusher(mockExporterCreateSettings, pcfg)
	err := pusher.Push(context.Background(), mockMetadata)
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

	pusher := NewPusher(mockExporterCreateSettings, pcfg)
	err := pusher.Push(context.Background(), mockMetadata)
	require.Error(t, err)
}

func TestPusher(t *testing.T) {
	pcfg := PusherConfig{
		APIKey:              "apikey",
		UseResourceMetadata: true,
	}
	params := exportertest.NewNopSettings(exportertest.NopType)
	params.BuildInfo = mockBuildInfo

	hostProvider, err := GetSourceProvider(componenttest.NewNopTelemetrySettings(), "source-hostname", 31*time.Second)
	require.NoError(t, err)

	attrs := testutil.NewAttributeMap(map[string]string{
		attributes.AttributeDatadogHostname: "datadog-hostname",
	})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	server := testutil.DatadogServerMock()
	defer server.Close()
	pcfg.MetricsEndpoint = server.URL

	pusher := NewPusher(mockExporterCreateSettings, pcfg)
	reporter, err := inframetadata.NewReporter(zap.NewNop(), pusher, 1*time.Second)
	require.NoError(t, err)

	go RunPusher(ctx, params, pcfg, hostProvider, attrs, reporter)

	recvMetadata := <-server.MetadataChan
	assert.Equal(t, "datadog-hostname", recvMetadata.InternalHostname)
	assert.Equal(t, recvMetadata.Version, mockBuildInfo.Version)
	assert.Equal(t, recvMetadata.Flavor, mockBuildInfo.Command)
	require.NotNil(t, recvMetadata.Meta)
	hostname, err := os.Hostname()
	require.NoError(t, err)
	assert.Equal(t, recvMetadata.Meta.SocketHostname, hostname)
}
