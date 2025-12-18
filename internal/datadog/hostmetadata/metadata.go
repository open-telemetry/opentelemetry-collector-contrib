// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Package hostmetadata is responsible for collecting host metadata from different providers
// such as EC2, ECS, AWS, etc and pushing it to Datadog.
package hostmetadata // import "github.com/open-telemetry/opentelemetry-collector-contrib/internal/datadog/hostmetadata"

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/DataDog/datadog-agent/pkg/opentelemetry-mapping-go/inframetadata"
	"github.com/DataDog/datadog-agent/pkg/opentelemetry-mapping-go/inframetadata/payload"
	"github.com/DataDog/datadog-agent/pkg/opentelemetry-mapping-go/otlp/attributes"
	ec2Attributes "github.com/DataDog/datadog-agent/pkg/opentelemetry-mapping-go/otlp/attributes/ec2"
	"github.com/DataDog/datadog-agent/pkg/opentelemetry-mapping-go/otlp/attributes/gcp"
	"github.com/DataDog/datadog-agent/pkg/opentelemetry-mapping-go/otlp/attributes/source"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/pdata/pcommon"
	conventions "go.opentelemetry.io/otel/semconv/v1.38.0"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/datadog/clientutil"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/datadog/hostmetadata/internal/ec2"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/datadog/hostmetadata/internal/gohai"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/datadog/hostmetadata/internal/system"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/datadog/scrub"
)

// metadataFromAttributes gets metadata info from attributes following
// OpenTelemetry semantic conventions
func metadataFromAttributes(attrs pcommon.Map, hostFromAttributesHandler attributes.HostFromAttributesHandler) payload.HostMetadata {
	hm := payload.HostMetadata{Meta: &payload.Meta{}, Tags: &payload.HostTags{}}

	if src, ok := attributes.SourceFromAttrs(attrs, hostFromAttributesHandler); ok && src.Kind == source.HostnameKind {
		hm.InternalHostname = src.Identifier
		hm.Meta.Hostname = src.Identifier
	}

	// AWS EC2 resource metadata
	cloudProvider, ok := attrs.Get(string(conventions.CloudProviderKey))
	switch {
	case ok && cloudProvider.Str() == conventions.CloudProviderAWS.Value.AsString():
		ec2HostInfo := ec2Attributes.HostInfoFromAttributes(attrs)
		hm.Meta.InstanceID = ec2HostInfo.InstanceID
		hm.Meta.EC2Hostname = ec2HostInfo.EC2Hostname
		hm.Tags.OTel = append(hm.Tags.OTel, ec2HostInfo.EC2Tags...)
	case ok && cloudProvider.Str() == conventions.CloudProviderGCP.Value.AsString():
		gcpHostInfo := gcp.HostInfoFromAttrs(attrs)
		hm.Tags.GCP = gcpHostInfo.GCPTags
		hm.Meta.HostAliases = append(hm.Meta.HostAliases, gcpHostInfo.HostAliases...)
	}

	return hm
}

func fillHostMetadata(params exporter.Settings, pcfg PusherConfig, p source.Provider, hm *payload.HostMetadata) {
	// Could not get hostname from attributes
	if hm.InternalHostname == "" {
		if src, err := p.Source(context.TODO()); err == nil && src.Kind == source.HostnameKind {
			hm.InternalHostname = src.Identifier
			hm.Meta.Hostname = src.Identifier
		}
	}

	// This information always gets filled in here
	// since it does not come from OTEL conventions
	hm.Flavor = params.BuildInfo.Command
	hm.Version = params.BuildInfo.Version
	hm.Tags.OTel = append(hm.Tags.OTel, pcfg.ConfigTags...)
	hm.Payload = gohai.NewPayload(params.Logger)
	hm.Processes = gohai.NewProcessesPayload(hm.Meta.Hostname, params.Logger)
	// EC2 data was not set from attributes
	if hm.Meta.EC2Hostname == "" {
		ec2HostInfo := ec2.GetHostInfo(context.Background(), params.Logger)
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

type pusher struct {
	params     exporter.Settings
	pcfg       PusherConfig
	retrier    *clientutil.Retrier
	httpClient *http.Client
}

func (p *pusher) pushMetadata(hm payload.HostMetadata) error {
	path := p.pcfg.MetricsEndpoint + "/intake"
	marshaled, err := json.Marshal(hm)
	if err != nil {
		return fmt.Errorf("error marshaling metadata payload: %w", err)
	}

	var buf bytes.Buffer
	g := gzip.NewWriter(&buf)
	if _, err = g.Write(marshaled); err != nil {
		return fmt.Errorf("error compressing metadata payload: %w", err)
	}
	if err = g.Close(); err != nil {
		return fmt.Errorf("error closing gzip writer: %w", err)
	}

	req, err := http.NewRequest(http.MethodPost, path, &buf)
	if err != nil {
		return fmt.Errorf("error creating metadata request: %w", err)
	}

	clientutil.SetDDHeaders(req.Header, p.params.BuildInfo, p.pcfg.APIKey)
	// Set the content type to JSON and the content encoding to gzip
	clientutil.SetExtraHeaders(req.Header, clientutil.JSONHeaders)

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf(
			"%q error when sending metadata payload to %s",
			resp.Status,
			path,
		)
	}

	return nil
}

func (p *pusher) Push(_ context.Context, hm payload.HostMetadata) error {
	if hm.Meta.Hostname == "" {
		// if the hostname is empty, don't send metadata; we don't need it.
		p.params.Logger.Debug("Skipping host metadata since the hostname is empty")
		return nil
	}

	p.params.Logger.Debug("Sending host metadata payload", zap.Any("payload", hm))

	_, err := p.retrier.DoWithRetries(context.Background(), func(context.Context) error {
		return p.pushMetadata(hm)
	})

	return err
}

var _ inframetadata.Pusher = (*pusher)(nil)

// NewPusher creates a new inframetadata.Pusher that pushes metadata payloads
func NewPusher(params exporter.Settings, pcfg PusherConfig) inframetadata.Pusher {
	return &pusher{
		params:     params,
		pcfg:       pcfg,
		retrier:    clientutil.NewRetrier(params.Logger, pcfg.RetrySettings, scrub.NewScrubber()),
		httpClient: clientutil.NewHTTPClient(pcfg.ClientConfig),
	}
}

// RunPusher to push host metadata payloads from the host where the Collector is running periodically to Datadog intake.
// This function is blocking and it is meant to be run on a goroutine.
func RunPusher(ctx context.Context, params exporter.Settings, pcfg PusherConfig, p source.Provider, attrs pcommon.Map, reporter *inframetadata.Reporter) {
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
	hostMetadata := payload.NewEmpty()
	if pcfg.UseResourceMetadata {
		hostMetadata = metadataFromAttributes(attrs, nil)
	}
	fillHostMetadata(params, pcfg, p, &hostMetadata)
	// Consume one first time
	if err := reporter.ConsumeHostMetadata(hostMetadata); err != nil {
		params.Logger.Warn("Failed to consume host metadata", zap.Any("payload", hostMetadata))
	}

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := reporter.ConsumeHostMetadata(hostMetadata); err != nil {
				params.Logger.Warn("Failed to consume host metadata", zap.Any("payload", hostMetadata))
			}
		}
	}
}
