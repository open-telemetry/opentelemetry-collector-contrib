// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package adapter // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/adapter"

import (
	"context"
	"fmt"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/extension/xextension/storage"
	rcvr "go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/receiver/receiverhelper"
	"go.uber.org/multierr"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/entry"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/helper"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/pipeline"
)

type receiver struct {
	set component.TelemetrySettings
	id  component.ID

	pipe     pipeline.Pipeline
	emitter  helper.LogEmitter
	consumer consumer.Logs
	obsrecv  *receiverhelper.ObsReport

	storageID     *component.ID
	storageClient storage.Client
}

// Ensure this receiver adheres to required interface
var _ rcvr.Logs = (*receiver)(nil)

// Start tells the receiver to start
func (r *receiver) Start(ctx context.Context, host component.Host) error {
	r.set.Logger.Info("Starting stanza receiver")

	if err := r.setStorageClient(ctx, host); err != nil {
		return fmt.Errorf("storage client: %w", err)
	}

	if err := r.pipe.Start(r.storageClient); err != nil {
		return fmt.Errorf("start stanza: %w", err)
	}

	return nil
}

func (r *receiver) consumeEntries(ctx context.Context, entries []*entry.Entry) {
	obsrecvCtx := r.obsrecv.StartLogsOp(ctx)
	pLogs := ConvertEntries(entries)
	logRecordCount := pLogs.LogRecordCount()

	cErr := r.consumer.ConsumeLogs(ctx, pLogs)
	if cErr != nil {
		r.set.Logger.Error("ConsumeLogs() failed", zap.Error(cErr))
	}
	r.obsrecv.EndLogsOp(obsrecvCtx, "stanza", logRecordCount, cErr)
}

// Shutdown is invoked during service shutdown
func (r *receiver) Shutdown(ctx context.Context) error {
	r.set.Logger.Info("Stopping stanza receiver")
	pipelineErr := r.pipe.Stop()

	if r.storageClient != nil {
		clientErr := r.storageClient.Close(ctx)
		return multierr.Combine(pipelineErr, clientErr)
	}
	return pipelineErr
}
