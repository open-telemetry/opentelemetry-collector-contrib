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
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/helper"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/pipeline"
)

const DefaultScopeName = "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza"

type receiver struct {
	set component.TelemetrySettings
	id  component.ID

	pipe     pipeline.Pipeline
	emitter  helper.LogEmitter
	consumer consumer.Logs
	obsrecv  *receiverhelper.ObsReport

	scopeName     string
	buildInfo     component.BuildInfo
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

	// Offer the host to operators that opt in via HostSetter (e.g. tcp input).
	// Each operator decides whether it actually needs it (e.g. only when an
	// auth extension is configured).
	for _, op := range r.pipe.Operators() {
		if hs, ok := op.(operator.HostSetter); ok {
			hs.SetHost(host)
		}
	}

	if err := r.pipe.Start(r.storageClient); err != nil {
		return fmt.Errorf("start stanza: %w", err)
	}

	return nil
}

// consumeEntries is the callback of the batching emitter. The batching emitter
// flushes from its own goroutine, so there is no caller to return the error to.
func (r *receiver) consumeEntries(ctx context.Context, entries []*entry.Entry) {
	_ = r.consumeEntriesWithError(ctx, entries)
}

// consumeEntriesWithError converts the entries, sends them to the next
// consumer, and returns the consumer's error.
func (r *receiver) consumeEntriesWithError(ctx context.Context, entries []*entry.Entry) error {
	// Operators call WriteBatch even when every entry was dropped or filtered
	// out. Sending an empty batch could report a ConsumeError for entries that
	// were never meant to be delivered.
	if len(entries) == 0 {
		return nil
	}
	obsrecvCtx := r.obsrecv.StartLogsOp(ctx)
	scope := r.scopeName
	if scope == "" {
		scope = DefaultScopeName
	}
	pLogs := ConvertEntries(entries, scope, r.buildInfo.Version)
	for _, e := range entries {
		entry.Put(e)
	}
	logRecordCount := pLogs.LogRecordCount()

	cErr := r.consumer.ConsumeLogs(ctx, pLogs)
	if cErr != nil {
		r.set.Logger.Error("ConsumeLogs() failed", zap.Error(cErr))
	}
	r.obsrecv.EndLogsOp(obsrecvCtx, "stanza", logRecordCount, cErr)
	return cErr
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
