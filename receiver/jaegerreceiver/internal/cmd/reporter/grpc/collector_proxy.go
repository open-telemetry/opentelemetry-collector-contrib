// Copyright The OpenTelemetry Authors
// Copyright (c) 2018 The Jaeger Authors.
// SPDX-License-Identifier: Apache-2.0

package grpc

import (
	"context"
	"errors"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/jaegerreceiver/internal/cmd/reporter"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/jaegerreceiver/internal/pkg/metrics"
	"go.uber.org/zap"
	"google.golang.org/grpc"
)

// ProxyBuilder holds objects communicating with collector
type ProxyBuilder struct {
	reporter *reporter.ClientMetricsReporter
	conn     *grpc.ClientConn
}

// NewCollectorProxy creates ProxyBuilder
func NewCollectorProxy(ctx context.Context, builder *ConnBuilder, agentTags map[string]string, mFactory metrics.Factory, logger *zap.Logger) (*ProxyBuilder, error) {
	conn, err := builder.CreateConnection(ctx, logger, mFactory)
	if err != nil {
		return nil, err
	}
	grpcMetrics := mFactory.Namespace(metrics.NSOptions{Name: "", Tags: map[string]string{"protocol": "grpc"}})
	r1 := NewReporter(conn, agentTags, logger)
	r2 := reporter.WrapWithMetrics(r1, grpcMetrics)
	r3 := reporter.WrapWithClientMetrics(reporter.ClientMetricsReporterParams{
		Reporter:       r2,
		Logger:         logger,
		MetricsFactory: mFactory,
	})
	return &ProxyBuilder{
		conn:     conn,
		reporter: r3,	
	}, nil
}

// GetConn returns grpc conn
func (b ProxyBuilder) GetConn() *grpc.ClientConn {
	return b.conn
}

// GetReporter returns Reporter
func (b ProxyBuilder) GetReporter() reporter.Reporter {
	return b.reporter
}

// Close closes connections used by proxy.
func (b ProxyBuilder) Close() error {
	return errors.Join(
		b.reporter.Close(),
		b.GetConn().Close(),
	)
}
