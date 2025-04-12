// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package testconsumer // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/otelarrowreceiver/internal/testconsumer"

import (
	"context"

	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

type BlockingConsumer struct {
	block chan struct{}
}

func NewBlockingConsumer() *BlockingConsumer {
	return &BlockingConsumer{
		block: make(chan struct{}),
	}
}

func (bc *BlockingConsumer) ConsumeTraces(_ context.Context, _ ptrace.Traces) error {
	<-bc.block
	return nil
}

func (bc *BlockingConsumer) ConsumeMetrics(_ context.Context, _ pmetric.Metrics) error {
	<-bc.block
	return nil
}

func (bc *BlockingConsumer) ConsumeLogs(_ context.Context, _ plog.Logs) error {
	<-bc.block
	return nil
}

func (bc *BlockingConsumer) Unblock() {
	close(bc.block)
}

func (bc *BlockingConsumer) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{MutatesData: false}
}
