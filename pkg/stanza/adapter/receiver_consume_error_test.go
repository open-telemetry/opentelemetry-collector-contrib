// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package adapter

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/consumer/consumererror"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/receiver/receivertest"

	commontestutil "github.com/open-telemetry/opentelemetry-collector-contrib/internal/common/testutil"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/entry"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/helper"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/parser/regex"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/transformer/filter"
)

// newTestInput wires input -> [regex parser ->] emitter, where the emitter is
// the one the factory picked for the current feature gate value.
func newTestInput(t *testing.T, next consumer.Logs, onError string) (*helper.InputOperator, *receiver) {
	t.Helper()
	factory := NewFactory(TestReceiverType{}, component.StabilityLevelDevelopment)
	rcv, err := factory.CreateLogs(t.Context(), receivertest.NewNopSettings(factory.Type()), factory.CreateDefaultConfig(), next)
	require.NoError(t, err)
	stanzaRcv := rcv.(*receiver)
	set := componenttest.NewNopTelemetrySettings()

	var first operator.Operator = stanzaRcv.emitter
	if onError != "" {
		cfg := regex.NewConfig()
		cfg.Regex = `^(?P<word>\w+)$`
		cfg.OnError = onError
		cfg.OutputIDs = []string{stanzaRcv.emitter.ID()}
		parser, buildErr := cfg.Build(set)
		require.NoError(t, buildErr)
		require.NoError(t, parser.SetOutputs([]operator.Operator{stanzaRcv.emitter}))
		first = parser
	}

	input, err := helper.NewInputConfig("test_input", "test_input").Build(set)
	require.NoError(t, err)
	input.OutputIDs = []string{first.ID()}
	require.NoError(t, input.SetOutputs([]operator.Operator{first}))
	return &input, stanzaRcv
}

func newBodyEntry(body string) *entry.Entry {
	e := entry.New()
	e.Body = body
	return e
}

func TestConsumeErrorReachesInputWithSynchronousEmitter(t *testing.T) {
	defer commontestutil.SetFeatureGateForTest(t, metadata.StanzaSynchronousLogEmitterFeatureGate, true)()
	exportErr := errors.New("export failed")

	t.Run("consumer succeeds", func(t *testing.T) {
		sink := &consumertest.LogsSink{}
		input, _ := newTestInput(t, sink, "")
		require.NoError(t, input.Write(t.Context(), newBodyEntry("hello")))
		require.Equal(t, 1, sink.LogRecordCount())
	})

	t.Run("consumer fails", func(t *testing.T) {
		input, _ := newTestInput(t, consumertest.NewErr(exportErr), "")
		err := input.Write(t.Context(), newBodyEntry("hello"))
		var consumeErr *helper.ConsumeError
		require.ErrorAs(t, err, &consumeErr)
		require.ErrorIs(t, err, exportErr)

		err = input.WriteBatch(t.Context(), []*entry.Entry{newBodyEntry("a"), newBodyEntry("b")})
		require.ErrorAs(t, err, &consumeErr)
		require.ErrorIs(t, err, exportErr)
	})

	t.Run("valid line through a parser, consumer fails", func(t *testing.T) {
		input, _ := newTestInput(t, consumertest.NewErr(exportErr), helper.SendOnError)
		var consumeErr *helper.ConsumeError
		require.ErrorAs(t, input.Write(t.Context(), newBodyEntry("hello")), &consumeErr)
		require.ErrorAs(t, input.WriteBatch(t.Context(), []*entry.Entry{newBodyEntry("a"), newBodyEntry("b")}), &consumeErr)
	})

	t.Run("permanent consumer error stays detectable", func(t *testing.T) {
		input, _ := newTestInput(t, consumertest.NewErr(consumererror.NewPermanent(exportErr)), "")
		err := input.Write(t.Context(), newBodyEntry("hello"))
		var consumeErr *helper.ConsumeError
		require.ErrorAs(t, err, &consumeErr)
		require.True(t, consumererror.IsPermanent(err))
	})

	t.Run("parse error with on_error drop is not a consume error", func(t *testing.T) {
		sink := &consumertest.LogsSink{}
		input, _ := newTestInput(t, sink, helper.DropOnError)
		err := input.Write(t.Context(), newBodyEntry("two words"))
		require.Error(t, err)
		var consumeErr *helper.ConsumeError
		require.NotErrorAs(t, err, &consumeErr)
		require.Equal(t, 0, sink.LogRecordCount())
	})

	t.Run("parse error with on_error send and failing consumer is a consume error", func(t *testing.T) {
		input, _ := newTestInput(t, consumertest.NewErr(exportErr), helper.SendOnError)
		err := input.Write(t.Context(), newBodyEntry("two words"))
		var consumeErr *helper.ConsumeError
		require.ErrorAs(t, err, &consumeErr)
		require.ErrorIs(t, err, exportErr)
	})

	t.Run("parse error with on_error send_quiet and failing consumer is a consume error", func(t *testing.T) {
		input, _ := newTestInput(t, consumertest.NewErr(exportErr), helper.SendOnErrorQuiet)
		err := input.Write(t.Context(), newBodyEntry("two words"))
		var consumeErr *helper.ConsumeError
		require.ErrorAs(t, err, &consumeErr)
	})
}

// The batching emitter flushes from its own goroutine, so Write cannot report
// a consumer error. This test pins that behavior.
func TestConsumeErrorNotReturnedWithBatchingEmitter(t *testing.T) {
	defer commontestutil.SetFeatureGateForTest(t, metadata.StanzaSynchronousLogEmitterFeatureGate, false)()
	var calls atomic.Int32
	next, err := consumer.NewLogs(func(context.Context, plog.Logs) error {
		calls.Add(1)
		return errors.New("export failed")
	})
	require.NoError(t, err)
	input, rcv := newTestInput(t, next, "")
	require.NoError(t, rcv.emitter.Start(nil))
	defer func() { require.NoError(t, rcv.emitter.Stop()) }()

	require.NoError(t, input.Write(t.Context(), newBodyEntry("hello")))
	// The flusher delivers the entry later; its error is only logged.
	require.Eventually(t, func() bool { return calls.Load() == 1 }, 5*time.Second, 10*time.Millisecond)
}

// A batch whose entries are all dropped by a parser or filtered out must not
// reach the next consumer, so it cannot produce a ConsumeError.
func TestEmptyBatchDoesNotReachConsumer(t *testing.T) {
	defer commontestutil.SetFeatureGateForTest(t, metadata.StanzaSynchronousLogEmitterFeatureGate, true)()

	var calls atomic.Int32
	next, err := consumer.NewLogs(func(context.Context, plog.Logs) error {
		calls.Add(1)
		return errors.New("export failed")
	})
	require.NoError(t, err)

	t.Run("all entries dropped by parser", func(t *testing.T) {
		calls.Store(0)
		input, _ := newTestInput(t, next, helper.DropOnErrorQuiet)
		err := input.WriteBatch(t.Context(), []*entry.Entry{newBodyEntry("two words"), newBodyEntry("three more words")})
		require.NoError(t, err)
		require.Equal(t, int32(0), calls.Load())
	})

	t.Run("all entries filtered out", func(t *testing.T) {
		calls.Store(0)
		factory := NewFactory(TestReceiverType{}, component.StabilityLevelDevelopment)
		rcv, err := factory.CreateLogs(t.Context(), receivertest.NewNopSettings(factory.Type()), factory.CreateDefaultConfig(), next)
		require.NoError(t, err)
		emitter := rcv.(*receiver).emitter
		set := componenttest.NewNopTelemetrySettings()

		cfg := filter.NewConfig()
		cfg.Expression = `true`
		cfg.OutputIDs = []string{emitter.ID()}
		filterOp, err := cfg.Build(set)
		require.NoError(t, err)
		require.NoError(t, filterOp.SetOutputs([]operator.Operator{emitter}))

		err = filterOp.ProcessBatch(t.Context(), []*entry.Entry{newBodyEntry("a"), newBodyEntry("b")})
		require.NoError(t, err)
		require.Equal(t, int32(0), calls.Load())
	})
}
