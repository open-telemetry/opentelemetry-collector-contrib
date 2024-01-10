// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package datasenders // import "github.com/open-telemetry/opentelemetry-collector-contrib/testbed/datasenders"

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	jaegerproto "github.com/jaegertracing/jaeger/proto-gen/api_v2"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/configgrpc"
	"go.opentelemetry.io/collector/config/configretry"
	"go.opentelemetry.io/collector/config/configtls"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/consumer/consumererror"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
	"go.opentelemetry.io/collector/exporter/exportertest"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/connectivity"
	"google.golang.org/grpc/metadata"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/jaeger"
	"github.com/open-telemetry/opentelemetry-collector-contrib/testbed/testbed"
)

// jaegerGRPCDataSender implements TraceDataSender for Jaeger thrift_http exporter.
type jaegerGRPCDataSender struct {
	testbed.DataSenderBase
	consumer.Traces
}

// Ensure jaegerGRPCDataSender implements TraceDataSender.
var _ testbed.TraceDataSender = (*jaegerGRPCDataSender)(nil)

// NewJaegerGRPCDataSender creates a new Jaeger exporter sender that will send
// to the specified port after Start is called.
func NewJaegerGRPCDataSender(host string, port int) testbed.TraceDataSender {
	return &jaegerGRPCDataSender{
		DataSenderBase: testbed.DataSenderBase{Port: port, Host: host},
	}
}

func (je *jaegerGRPCDataSender) Start() error {
	params := exportertest.NewNopCreateSettings()
	params.Logger = zap.L()

	exp, err := je.newTracesExporter(params)
	if err != nil {
		return err
	}

	je.Traces = exp
	return exp.Start(context.Background(), componenttest.NewNopHost())
}

func (je *jaegerGRPCDataSender) GenConfigYAMLStr() string {
	return fmt.Sprintf(`
  jaeger:
    protocols:
      grpc:
        endpoint: "%s"`, je.GetEndpoint())
}

func (je *jaegerGRPCDataSender) ProtocolName() string {
	return "jaeger"
}

// Config defines configuration for Jaeger gRPC exporter.
type jaegerConfig struct {
	exporterhelper.TimeoutSettings `mapstructure:",squash"` // squash ensures fields are correctly decoded in embedded struct.
	exporterhelper.QueueSettings   `mapstructure:"sending_queue"`
	configretry.BackOffConfig      `mapstructure:"retry_on_failure"`

	configgrpc.GRPCClientSettings `mapstructure:",squash"` // squash ensures fields are correctly decoded in embedded struct.
}

var _ component.Config = (*jaegerConfig)(nil)

// Validate checks if the exporter configuration is valid
func (cfg *jaegerConfig) Validate() error {
	if cfg.Endpoint == "" {
		return errors.New("must have a non-empty \"endpoint\"")
	}
	return nil
}

// newTracesExporter returns a new Jaeger gRPC exporter.
// The exporter name is the name to be used in the observability of the exporter.
// The collectorEndpoint should be of the form "hostname:14250" (a gRPC target).
func (je *jaegerGRPCDataSender) newTracesExporter(set exporter.CreateSettings) (exporter.Traces, error) {
	cfg := jaegerConfig{}
	cfg.Endpoint = je.GetEndpoint().String()
	cfg.TLSSetting = configtls.TLSClientSetting{
		Insecure: true,
	}

	s := &protoGRPCSender{
		name:                      set.ID.String(),
		settings:                  set.TelemetrySettings,
		metadata:                  metadata.New(nil),
		waitForReady:              cfg.WaitForReady,
		connStateReporterInterval: time.Second,
		stopCh:                    make(chan struct{}),
		clientSettings:            &cfg.GRPCClientSettings,
	}

	return exporterhelper.NewTracesExporter(
		context.TODO(), set, cfg, s.pushTraces,
		exporterhelper.WithCapabilities(consumer.Capabilities{MutatesData: false}),
		exporterhelper.WithStart(s.start),
		exporterhelper.WithShutdown(s.shutdown),
	)
}

// protoGRPCSender forwards spans encoded in the jaeger proto
// format, to a grpc server.
type protoGRPCSender struct {
	name         string
	settings     component.TelemetrySettings
	client       jaegerproto.CollectorServiceClient
	metadata     metadata.MD
	waitForReady bool

	conn                      stateReporter
	connStateReporterInterval time.Duration

	stopCh         chan struct{}
	stopped        bool
	stopLock       sync.Mutex
	clientSettings *configgrpc.GRPCClientSettings
}

type stateReporter interface {
	GetState() connectivity.State
}

func (s *protoGRPCSender) pushTraces(
	ctx context.Context,
	td ptrace.Traces,
) error {

	batches, err := jaeger.ProtoFromTraces(td)
	if err != nil {
		return consumererror.NewPermanent(fmt.Errorf("failed to push trace data via Jaeger exporter: %w", err))
	}

	if s.metadata.Len() > 0 {
		ctx = metadata.NewOutgoingContext(ctx, s.metadata)
	}

	for _, batch := range batches {
		_, err = s.client.PostSpans(
			ctx,
			&jaegerproto.PostSpansRequest{Batch: *batch}, grpc.WaitForReady(s.waitForReady))

		if err != nil {
			s.settings.Logger.Debug("failed to push trace data to Jaeger", zap.Error(err))
			return fmt.Errorf("failed to push trace data via Jaeger exporter: %w", err)
		}
	}

	return nil
}

func (s *protoGRPCSender) shutdown(context.Context) error {
	s.stopLock.Lock()
	s.stopped = true
	s.stopLock.Unlock()
	close(s.stopCh)
	return nil
}

func (s *protoGRPCSender) start(ctx context.Context, host component.Host) error {
	if s.clientSettings == nil {
		return fmt.Errorf("client settings not found")
	}
	conn, err := s.clientSettings.ToClientConn(ctx, host, s.settings)
	if err != nil {
		return err
	}

	s.client = jaegerproto.NewCollectorServiceClient(conn)
	s.conn = conn

	return nil
}
