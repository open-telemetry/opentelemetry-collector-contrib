// Copyright 2021, OpenTelemetry Authors
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

package coralogixexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/coralogixexporter"

import (
	"context"
	"fmt"

	cxpb "github.com/coralogix/opentelemetry-cx-protobuf-api/coralogixpb"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/model/pdata"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/jaeger"
)

// CoralogixClient by Coralogix
type coralogixClient struct {
	cfg               Config
	logger            *zap.Logger
	conn              *grpc.ClientConn
	client            cxpb.CollectorServiceClient
	telemetrySettings component.TelemetrySettings
}

// NewCoralogixClient by Coralogix
func newCoralogixClient(cfg *Config, set component.ExporterCreateSettings) *coralogixClient {
	c := &coralogixClient{
		cfg:               *cfg,
		logger:            set.Logger,
		telemetrySettings: set.TelemetrySettings,
	}
	return c
}
func (c *coralogixClient) startConnection(ctx context.Context, host component.Host) error {
	dialOpts, err := c.cfg.ToDialOptions(host, c.telemetrySettings)
	if err != nil {
		return fmt.Errorf("error with dial connection to Coralogix endpoint %w", err)
	}
	var clientConn *grpc.ClientConn
	if clientConn, err = grpc.DialContext(ctx, c.cfg.GRPCClientSettings.Endpoint, dialOpts...); err != nil {
		return err
	}
	c.conn = clientConn
	c.client = cxpb.NewCollectorServiceClient(c.conn)
	return nil
}

func (c *coralogixClient) newPost(ctx context.Context, td pdata.Traces) error {
	batches, err := jaeger.ProtoFromTraces(td)
	if err != nil {
		return fmt.Errorf("can't translate to jaeger proto: %w", err)
	}

	ctx = metadata.NewOutgoingContext(ctx, metadata.New(c.cfg.GRPCClientSettings.Headers))
	for _, batch := range batches {
		_, err := c.client.PostSpans(ctx, &cxpb.PostSpansRequest{
			Batch:    *batch,
			Metadata: &cxpb.Metadata{ApplicationName: c.cfg.AppName, SubsystemName: batch.GetProcess().GetServiceName()},
		}, grpc.WaitForReady(c.cfg.WaitForReady))
		if err != nil {
			return fmt.Errorf("Failed to push trace data via Coralogix exporter %w", err)
		}
		c.logger.Debug("Trace was sent successfully")
	}
	return nil
}
