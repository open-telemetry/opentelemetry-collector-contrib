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
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/model/pdata"
	conventions "go.opentelemetry.io/collector/model/semconv/v1.5.0"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter/config"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter/internal/attributes"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter/internal/attributes/azure"
	ec2Attributes "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter/internal/attributes/ec2"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter/internal/attributes/gcp"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter/internal/metadata/ec2"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter/internal/metadata/system"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter/internal/utils"
)

// HostMetadata includes metadata about the host tags,
// host aliases and identifies the host as an OpenTelemetry host
type HostMetadata struct {
	// Meta includes metadata about the host.
	Meta *Meta `json:"meta"`

	// InternalHostname is the canonical hostname
	InternalHostname string `json:"internalHostname"`

	// Version is the OpenTelemetry Collector version.
	// This is used for correctly identifying the Collector in the backend,
	// and for telemetry purposes.
	Version string `json:"otel_version"`

	// Flavor is always set to "opentelemetry-collector".
	// It is used for telemetry purposes in the backend.
	Flavor string `json:"agent-flavor"`

	// Tags includes the host tags
	Tags *HostTags `json:"host-tags"`
}

// HostTags are the host tags.
// Currently only system (configuration) tags are considered.
type HostTags struct {
	// OTel are host tags set in the configuration
	OTel []string `json:"otel,omitempty"`

	// GCP are Google Cloud Platform tags
	GCP []string `json:"google cloud platform,omitempty"`
}

// Meta includes metadata about the host aliases
type Meta struct {
	// InstanceID is the EC2 instance id the Collector is running on, if available
	InstanceID string `json:"instance-id,omitempty"`

	// EC2Hostname is the hostname from the EC2 metadata API
	EC2Hostname string `json:"ec2-hostname,omitempty"`

	// Hostname is the canonical hostname
	Hostname string `json:"hostname"`

	// SocketHostname is the OS hostname
	SocketHostname string `json:"socket-hostname,omitempty"`

	// SocketFqdn is the FQDN hostname
	SocketFqdn string `json:"socket-fqdn,omitempty"`

	// HostAliases are other available host names
	HostAliases []string `json:"host-aliases,omitempty"`
}

// metadataFromAttributes gets metadata info from attributes following
// OpenTelemetry semantic conventions
func metadataFromAttributes(attrs pdata.AttributeMap) *HostMetadata {
	hm := &HostMetadata{Meta: &Meta{}, Tags: &HostTags{}}

	if hostname, ok := attributes.HostnameFromAttributes(attrs); ok {
		hm.InternalHostname = hostname
		hm.Meta.Hostname = hostname
	}

	// AWS EC2 resource metadata
	cloudProvider, ok := attrs.Get(conventions.AttributeCloudProvider)
	if ok && cloudProvider.StringVal() == conventions.AttributeCloudProviderAWS {
		ec2HostInfo := ec2Attributes.HostInfoFromAttributes(attrs)
		hm.Meta.InstanceID = ec2HostInfo.InstanceID
		hm.Meta.EC2Hostname = ec2HostInfo.EC2Hostname
		hm.Tags.OTel = append(hm.Tags.OTel, ec2HostInfo.EC2Tags...)
	} else if ok && cloudProvider.StringVal() == conventions.AttributeCloudProviderGCP {
		gcpHostInfo := gcp.HostInfoFromAttributes(attrs)
		hm.Tags.GCP = gcpHostInfo.GCPTags
		hm.Meta.HostAliases = append(hm.Meta.HostAliases, gcpHostInfo.HostAliases...)
	} else if ok && cloudProvider.StringVal() == conventions.AttributeCloudProviderAzure {
		azureHostInfo := azure.HostInfoFromAttributes(attrs)
		hm.Meta.HostAliases = append(hm.Meta.HostAliases, azureHostInfo.HostAliases...)
	}

	return hm
}

func fillHostMetadata(params component.ExporterCreateSettings, cfg *config.Config, hm *HostMetadata) {
	// Could not get hostname from attributes
	if hm.InternalHostname == "" {
		hostname := GetHost(params.Logger, cfg)
		hm.InternalHostname = hostname
		hm.Meta.Hostname = hostname
	}

	// This information always gets filled in here
	// since it does not come from OTEL conventions
	hm.Flavor = params.BuildInfo.Command
	hm.Version = params.BuildInfo.Version
	hm.Tags.OTel = append(hm.Tags.OTel, cfg.GetHostTags()...)

	// EC2 data was not set from attributes
	if hm.Meta.EC2Hostname == "" {
		ec2HostInfo := ec2.GetHostInfo(params.Logger)
		hm.Meta.EC2Hostname = ec2HostInfo.EC2Hostname
		hm.Meta.InstanceID = ec2HostInfo.InstanceID
	}

	// System data was not set from attributes
	if hm.Meta.SocketHostname == "" {
		systemHostInfo := system.GetHostInfo(params.Logger)
		hm.Meta.SocketHostname = systemHostInfo.OS
		hm.Meta.SocketFqdn = systemHostInfo.FQDN
	}
}

func pushMetadata(cfg *config.Config, buildInfo component.BuildInfo, metadata *HostMetadata) error {
	path := cfg.Metrics.TCPAddr.Endpoint + "/intake"
	buf, _ := json.Marshal(metadata)
	req, _ := http.NewRequest(http.MethodPost, path, bytes.NewBuffer(buf))
	utils.SetDDHeaders(req.Header, buildInfo, cfg.API.Key)
	utils.SetExtraHeaders(req.Header, utils.JSONHeaders)
	client := utils.NewHTTPClient(10 * time.Second)
	resp, err := client.Do(req)

	if err != nil {
		return err
	}

	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf(
			"'%s' error when sending metadata payload to %s",
			resp.Status,
			path,
		)
	}

	return nil
}

func pushMetadataWithRetry(params component.ExporterCreateSettings, cfg *config.Config, hostMetadata *HostMetadata) {
	const maxRetries = 5

	params.Logger.Debug("Sending host metadata payload", zap.Any("payload", hostMetadata))

	numRetries, err := utils.DoWithRetries(maxRetries, func() error {
		return pushMetadata(cfg, params.BuildInfo, hostMetadata)
	})

	if err != nil {
		params.Logger.Warn("Sending host metadata failed", zap.Error(err))
	} else {
		params.Logger.Info("Sent host metadata", zap.Int("retries", numRetries))
	}

}

// Pusher pushes host metadata payloads periodically to Datadog intake
func Pusher(ctx context.Context, params component.ExporterCreateSettings, cfg *config.Config, attrs pdata.AttributeMap) {
	// Push metadata every 30 minutes
	ticker := time.NewTicker(30 * time.Minute)
	defer ticker.Stop()
	defer params.Logger.Debug("Shut down host metadata routine")

	// Get host metadata from resources and fill missing info using our exporter.
	// Currently we only retrieve it once but still send the same payload
	// every 30 minutes for consistency with the Datadog Agent behavior.
	//
	// All fields that are being filled in by our exporter
	// do not change over time. If this ever changes `hostMetadata`
	// *must* be deep copied before calling `fillHostMetadata`.
	hostMetadata := &HostMetadata{Meta: &Meta{}, Tags: &HostTags{}}
	if cfg.UseResourceMetadata {
		hostMetadata = metadataFromAttributes(attrs)
	}
	fillHostMetadata(params, cfg, hostMetadata)

	// Run one first time at startup
	pushMetadataWithRetry(params, cfg, hostMetadata)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C: // Send host metadata
			pushMetadataWithRetry(params, cfg, hostMetadata)
		}
	}
}
