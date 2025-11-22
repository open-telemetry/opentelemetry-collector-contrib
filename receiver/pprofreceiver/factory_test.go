// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package pprofreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/pprofreceiver"
import (
	"net/http"
	_ "net/http/pprof" //nolint:gosec // #nosec G108
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/receiver/receivertest"
	"go.opentelemetry.io/collector/receiver/xreceiver"
	"go.uber.org/zap"
)

func TestStartStop(t *testing.T) {
	f := NewFactory().(xreceiver.Factory)
	profilesConsumer := new(consumertest.ProfilesSink)
	set := receivertest.NewNopSettings(f.Type())
	var err error
	set.Logger, err = zap.NewDevelopment()
	require.NoError(t, err)
	cfg := f.CreateDefaultConfig().(*Config)
	cfg.CollectionInterval = 1 * time.Second
	r, err := f.CreateProfiles(t.Context(), set, cfg, profilesConsumer)
	require.NoError(t, err)
	err = r.Start(t.Context(), componenttest.NewNopHost())
	require.NoError(t, err)
	require.EventuallyWithT(t, func(tt *assert.CollectT) {
		require.NotEmpty(tt, profilesConsumer.AllProfiles())
	}, 5*time.Second, 100*time.Millisecond, "failed to receive data from pprof receiver")
	err = r.Shutdown(t.Context())
	require.NoError(t, err)
}

func TestStartStopRemote(t *testing.T) {
	server := &http.Server{
		ReadTimeout: 1 * time.Second,
		Addr:        "localhost:6060",
		Handler:     http.DefaultServeMux,
	}
	go func() {
		err := server.ListenAndServe()
		assert.ErrorIs(t, err, http.ErrServerClosed, "server closed")
	}()
	defer func() {
		assert.NoError(t, server.Shutdown(t.Context()))
	}()

	f := NewFactory().(xreceiver.Factory)
	profilesConsumer := new(consumertest.ProfilesSink)
	set := receivertest.NewNopSettings(f.Type())
	var err error
	set.Logger, err = zap.NewDevelopment()
	require.NoError(t, err)
	cfg := f.CreateDefaultConfig().(*Config)
	cfg.CollectionInterval = 1 * time.Second
	cfg.Endpoint = "http://localhost:6060/debug/pprof/profile?seconds=1"
	r, err := f.CreateProfiles(t.Context(), set, cfg, profilesConsumer)
	require.NoError(t, err)
	err = r.Start(t.Context(), componenttest.NewNopHost())
	require.NoError(t, err)
	require.EventuallyWithT(t, func(tt *assert.CollectT) {
		require.NotEmpty(tt, profilesConsumer.AllProfiles())
	}, 5*time.Second, 100*time.Millisecond, "failed to receive data from pprof receiver")
	err = r.Shutdown(t.Context())
	require.NoError(t, err)
}
