// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package libhoneyreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/libhoneyreceiver"

import (
	"context"
	"fmt"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/config/configoptional"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/sharedcomponent"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/libhoneyreceiver/internal/libhoneyevent"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/libhoneyreceiver/internal/metadata"
)

const (
	httpPort = 8080
)

var defaultTracesURLPaths = []string{"/events", "/event", "/batch"}

// NewFactory creates a new OTLP receiver factory.
func NewFactory() receiver.Factory {
	return receiver.NewFactory(
		metadata.Type,
		createDefaultConfig,
		receiver.WithTraces(createTraces, metadata.TracesStability),
		receiver.WithLogs(createLogs, metadata.LogsStability),
	)
}

// createDefaultConfig creates the default configuration for receiver.
func createDefaultConfig() component.Config {
	durationFieldsArr := []string{"duration_ms"}
	endpointStr := fmt.Sprintf("localhost:%d", httpPort)
	return &Config{
		HTTP: configoptional.Default(HTTPConfig{
			ServerConfig: confighttp.ServerConfig{
				Endpoint:              endpointStr,
				CompressionAlgorithms: []string{"", "zstd", "gzip", "deflate"},
			},
			TracesURLPaths: defaultTracesURLPaths,
		}),
		AuthAPI: "",
		FieldMapConfig: libhoneyevent.FieldMapConfig{
			Resources: libhoneyevent.ResourcesConfig{
				ServiceName: "service.name",
			},
			Scopes: libhoneyevent.ScopesConfig{
				LibraryName:    "library.name",
				LibraryVersion: "library.version",
			},
			Attributes: libhoneyevent.AttributesConfig{
				TraceID:        "trace.trace_id",
				SpanID:         "trace.span_id",
				ParentID:       "trace.parent_id",
				Name:           "name",
				Error:          "error",
				SpanKind:       "span.kind",
				DurationFields: durationFieldsArr,
			},
		},
	}
}

func createLogs(
	_ context.Context,
	set receiver.Settings,
	cfg component.Config,
	nextConsumer consumer.Logs,
) (receiver.Logs, error) {
	oCfg := cfg.(*Config)
	var err error
	r := receivers.GetOrAdd(
		oCfg,
		func() (lh component.Component) {
			lh, err = newLibhoneyReceiver(oCfg, &set)
			return lh
		},
	)

	if err != nil {
		return nil, err
	}

	r.Unwrap().(*libhoneyReceiver).registerLogConsumer(nextConsumer)
	return r, nil
}

// createTraces creates a trace receiver based on provided config.
func createTraces(
	_ context.Context,
	set receiver.Settings,
	cfg component.Config,
	nextConsumer consumer.Traces,
) (receiver.Traces, error) {
	oCfg := cfg.(*Config)
	var err error
	r := receivers.GetOrAdd(
		oCfg,
		func() (lh component.Component) {
			lh, err = newLibhoneyReceiver(oCfg, &set)
			return lh
		},
	)
	if err != nil {
		return nil, err
	}

	r.Unwrap().(*libhoneyReceiver).registerTraceConsumer(nextConsumer)
	return r, nil
}

var receivers = sharedcomponent.NewSharedComponents()
