// Copyright The OpenTelemetry Authors
// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package testutils

import (
	"context"

	"github.com/jaegertracing/jaeger-idl/model/v1"
	"github.com/jaegertracing/jaeger-idl/thrift-gen/jaeger"
	"github.com/jaegertracing/jaeger-idl/thrift-gen/zipkincore"
	"go.uber.org/zap"
)

// MockReporter is a test reporter that forwards to a mock collector
type MockReporter struct {
	Collector *MockCollector
	Logger    *zap.Logger
}

// NewMockReporter creates a new MockReporter
func NewMockReporter(collector *MockCollector, logger *zap.Logger) *MockReporter {
	return &MockReporter{
		Collector: collector,
		Logger:    logger,
	}
}

// EmitBatch implements the agent.Agent processing functionality
func (r *MockReporter) EmitBatch(ctx context.Context, batch *jaeger.Batch) error {
	r.Logger.Debug("MockReporter received jaeger batch", zap.Int("spans", len(batch.Spans)))
	
	// Convert to model.Batch
modelBatch := &model.Batch{
    Process: &model.Process{
        ServiceName: batch.Process.ServiceName,
    },
    Spans: make([]*model.Span, len(batch.Spans)),
}

	
	for i, span := range batch.Spans {
		modelBatch.Spans[i] = &model.Span{
			OperationName: span.OperationName,
		}
	}
	
	r.Collector.StoreBatch(modelBatch)
	return nil
}

// EmitZipkinBatch implements the agent.Agent processing functionality for Zipkin
func (r *MockReporter) EmitZipkinBatch(ctx context.Context, spans []*zipkincore.Span) error {
    r.Logger.Debug("MockReporter received zipkin batch", zap.Int("spans", len(spans)))
    
    // Convert to model.Batch using a pointer for Process.
    modelBatch := &model.Batch{
        Process: &model.Process{
            ServiceName: "zipkin", // Default for test
        },
        Spans: make([]*model.Span, len(spans)),
    }
    
    for i, span := range spans {
        serviceName := "unknown"
        if len(span.Annotations) > 0 && span.Annotations[0].Host != nil {
            serviceName = span.Annotations[0].Host.ServiceName
        }
        
        // Use modelBatch, not batch.
        modelBatch.Process.ServiceName = serviceName
        modelBatch.Spans[i] = &model.Span{
            OperationName: span.Name,
        }
    }
    
    r.Collector.StoreBatch(modelBatch)
    return nil
}