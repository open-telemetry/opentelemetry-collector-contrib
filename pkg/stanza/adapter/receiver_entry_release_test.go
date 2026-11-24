// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package adapter

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/receiver/receivertest"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"

	commontestutil "github.com/open-telemetry/opentelemetry-collector-contrib/internal/common/testutil"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/entry"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/transformer/router"
)

// TestFailedConsumeKeepsEntriesForCaller checks that entries are not returned
// to the pool when the next consumer fails. Operators such as router read the
// entry after a failed write to log it; if the entry were already pooled,
// another goroutine could reuse and reset it at the same time.
func TestFailedConsumeKeepsEntriesForCaller(t *testing.T) {
	defer commontestutil.SetFeatureGateForTest(t, metadata.StanzaSynchronousLogEmitterFeatureGate, true)()

	// The consumer takes an entry from the pool while it runs, the way a
	// concurrent connection would, and then fails.
	failing, err := consumer.NewLogs(func(context.Context, plog.Logs) error {
		reused := entry.New()
		reused.AddAttribute("k", "overwritten")
		return errors.New("export failed")
	})
	require.NoError(t, err)

	factory := NewFactory(TestReceiverType{}, component.StabilityLevelDevelopment)
	rcv, err := factory.CreateLogs(t.Context(), receivertest.NewNopSettings(factory.Type()), factory.CreateDefaultConfig(), failing)
	require.NoError(t, err)
	stanzaRcv := rcv.(*receiver)

	core, logs := observer.New(zap.ErrorLevel)
	set := componenttest.NewNopTelemetrySettings()
	set.Logger = zap.New(core)

	cfg := router.NewConfig()
	cfg.Routes = []*router.RouteConfig{{Expression: "true", OutputIDs: []string{stanzaRcv.emitter.ID()}}}
	rt, err := cfg.Build(set)
	require.NoError(t, err)
	require.NoError(t, rt.SetOutputs([]operator.Operator{stanzaRcv.emitter}))

	e := entry.New()
	e.Body = "hello"
	e.AddAttribute("k", "original")
	require.NoError(t, rt.Process(t.Context(), e))

	failures := logs.FilterMessage("Failed to process entry").All()
	require.Len(t, failures, 1)
	require.Equal(t, "original", failures[0].ContextMap()["k"])
}
