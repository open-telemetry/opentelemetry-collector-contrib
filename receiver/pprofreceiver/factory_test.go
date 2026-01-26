// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package pprofreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/pprofreceiver"

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"runtime/pprof"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/receiver/receivertest"
	"go.opentelemetry.io/collector/receiver/xreceiver"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/pprofreceiver/internal/metadata"
)

// TestNewFactory_PprofConversion tests that pprof CPU profiles are converted to OTel profiles
func TestNewFactory_PprofConversion(t *testing.T) {
	tempDir := t.TempDir()
	t.Cleanup(func() {
		os.RemoveAll(tempDir)
	})
	pprofFile := filepath.Join(tempDir, "cpu.pprof")

	f, err := os.Create(pprofFile)
	require.NoError(t, err)
	defer f.Close()

	// Increase sampling frequency to generate more data.
	runtime.SetCPUProfileRate(257)
	defer runtime.SetCPUProfileRate(100)

	err = pprof.StartCPUProfile(f)
	require.NoError(t, err)

	// Generate some CPU activity to create test data
	start := time.Now()
	result := 0
	for time.Since(start) < 200*time.Millisecond {
		for i := range 100000 {
			result += i * i
			result = result % 1000000
		}
	}
	_ = result

	pprof.StopCPUProfile()
	f.Close()

	fileInfo, err := os.Stat(pprofFile)
	require.NoError(t, err)
	require.Greater(t, fileInfo.Size(), int64(0), "pprof file should have content")

	factory := NewFactory()

	cfg := factory.CreateDefaultConfig().(*Config)
	cfg.Include = pprofFile
	cfg.ControllerConfig.CollectionInterval = 100 * time.Millisecond

	sink := new(consumertest.ProfilesSink)
	receiver, err := factory.(xreceiver.Factory).CreateProfiles(
		context.Background(),
		receivertest.NewNopSettings(metadata.Type),
		cfg,
		sink,
	)
	require.NoError(t, err)
	require.NotNil(t, receiver)

	err = receiver.Start(context.Background(), componenttest.NewNopHost())
	require.NoError(t, err)

	assert.Eventually(t, func() bool {
		return len(sink.AllProfiles()) > 0
	}, 5*time.Second, 100*time.Millisecond, "Expected profiles to be received")

	// Verify the profiles contain at least one Profile with at least one Sample
	profiles := sink.AllProfiles()
	require.NotEmpty(t, profiles, "Expected at least one profiles batch")

	foundProfile := false
	foundSample := false
	for _, pd := range profiles {
		for i := 0; i < pd.ResourceProfiles().Len(); i++ {
			rp := pd.ResourceProfiles().At(i)
			for j := 0; j < rp.ScopeProfiles().Len(); j++ {
				sp := rp.ScopeProfiles().At(j)
				for k := 0; k < sp.Profiles().Len(); k++ {
					profile := sp.Profiles().At(k)
					foundProfile = true
					if profile.Samples().Len() > 0 {
						foundSample = true
						break
					}
				}
				if foundSample {
					break
				}
			}
			if foundSample {
				break
			}
		}
		if foundSample {
			break
		}
	}

	assert.True(t, foundProfile, "Expected at least one Profile")
	assert.True(t, foundSample, "Expected at least one Sample in Profile")

	err = receiver.Shutdown(context.Background())
	require.NoError(t, err)
}
