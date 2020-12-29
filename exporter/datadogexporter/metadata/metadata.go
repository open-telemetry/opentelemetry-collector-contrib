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
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter/config"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter/metadata/ec2"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter/metadata/system"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter/utils"
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

func getHostMetadata(params component.ExporterCreateParams, cfg *config.Config) *HostMetadata {
	hostname := *GetHost(params.Logger, cfg)
	tags := cfg.GetHostTags()

	ec2HostInfo := ec2.GetHostInfo(params.Logger)
	systemHostInfo := system.GetHostInfo(params.Logger)

	return &HostMetadata{
		InternalHostname: hostname,
		Flavor:           params.ApplicationStartInfo.ExeName,
		Version:          params.ApplicationStartInfo.Version,
		Tags:             &HostTags{tags},
		Meta: &Meta{
			InstanceID:     ec2HostInfo.InstanceID,
			EC2Hostname:    ec2HostInfo.EC2Hostname,
			Hostname:       hostname,
			SocketHostname: systemHostInfo.OS,
			SocketFqdn:     systemHostInfo.FQDN,
		},
	}
}

func pushMetadata(cfg *config.Config, startInfo component.ApplicationStartInfo, metadata *HostMetadata) error {
	path := cfg.Metrics.TCPAddr.Endpoint + "/intake"
	buf, _ := json.Marshal(metadata)
	req, _ := http.NewRequest(http.MethodPost, path, bytes.NewBuffer(buf))
	utils.SetDDHeaders(req.Header, startInfo, cfg.API.Key)
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

func getAndPushMetadata(params component.ExporterCreateParams, cfg *config.Config) {
	const maxRetries = 5
	hostMetadata := getHostMetadata(params, cfg)

	params.Logger.Debug("Sending host metadata payload", zap.Any("payload", hostMetadata))

	numRetries, err := utils.DoWithRetries(maxRetries, func() error {
		return pushMetadata(cfg, params.ApplicationStartInfo, hostMetadata)
	})

	if err != nil {
		params.Logger.Warn("Sending host metadata failed", zap.Error(err))
	} else {
		params.Logger.Info("Sent host metadata", zap.Int("retries", numRetries))
	}

}

// Pusher pushes host metadata payloads periodically to Datadog intake
func Pusher(ctx context.Context, params component.ExporterCreateParams, cfg *config.Config) {
	// Push metadata every 30 minutes
	ticker := time.NewTicker(30 * time.Minute)
	defer ticker.Stop()
	defer params.Logger.Debug("Shut down host metadata routine")

	// Run one first time at startup
	getAndPushMetadata(params, cfg)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C: // Send host metadata
			getAndPushMetadata(params, cfg)
		}
	}
}
