// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package coralogixexporter

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"net"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/configgrpc"
	"go.opentelemetry.io/collector/config/configopaque"
	"go.opentelemetry.io/collector/config/configtls"
	"go.opentelemetry.io/collector/exporter/exportertest"
	"go.opentelemetry.io/collector/pdata/pprofile"
	"go.opentelemetry.io/collector/pdata/pprofile/pprofileotlp"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

func TestNewProfilesExporter(t *testing.T) {
	tests := []struct {
		name        string
		cfg         *Config
		shouldError bool
	}{
		{
			name: "Valid domain config",
			cfg: &Config{
				Domain:     "test.domain.com",
				PrivateKey: "test-key",
			},
			shouldError: false,
		},
		{
			name: "Valid profiles endpoint config",
			cfg: &Config{
				Profiles: configgrpc.ClientConfig{
					Endpoint: "localhost:4317",
				},
				PrivateKey: "test-key",
			},
			shouldError: false,
		},
		{
			name: "Missing both domain and endpoint",
			cfg: &Config{
				PrivateKey: "test-key",
			},
			shouldError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exp, err := newProfilesExporter(tt.cfg, exportertest.NewNopSettings(exportertest.NopType))
			if tt.shouldError {
				assert.Error(t, err)
				assert.Nil(t, exp)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, exp)
			}
		})
	}
}

func TestProfilesExporter_Start(t *testing.T) {
	cfg := &Config{
		Domain:     "test.domain.com",
		PrivateKey: "test-key",
	}

	exp, err := newProfilesExporter(cfg, exportertest.NewNopSettings(exportertest.NopType))
	require.NoError(t, err)

	err = exp.start(t.Context(), componenttest.NewNopHost())
	require.NoError(t, err)
	assert.NotNil(t, exp.clientConn)
	assert.NotNil(t, exp.profilesExporter)
	assert.Len(t, exp.metadata.Get("Authorization"), 1)

	// Test shutdown
	err = exp.shutdown(t.Context())
	require.NoError(t, err)
}

func TestProfilesExporter_EnhanceContext(t *testing.T) {
	cfg := &Config{
		Domain:     "test.domain.com",
		PrivateKey: "test-key",
		Profiles: configgrpc.ClientConfig{
			Headers: configopaque.MapList{
				{Name: "test-header", Value: "test-value"},
			},
		},
	}

	exp, err := newProfilesExporter(cfg, exportertest.NewNopSettings(exportertest.NopType))
	require.NoError(t, err)

	ctx := t.Context()
	enhancedCtx := exp.enhanceContext(ctx)
	assert.NotEqual(t, ctx, enhancedCtx)
}

func TestProfilesExporter_PushProfiles(t *testing.T) {
	cfg := &Config{
		Domain:     "test.domain.com",
		PrivateKey: "test-key",
	}

	exp, err := newProfilesExporter(cfg, exportertest.NewNopSettings(exportertest.NopType))
	require.NoError(t, err)

	// Initialize the exporter by calling start
	err = exp.start(t.Context(), componenttest.NewNopHost())
	require.NoError(t, err)
	defer func() {
		err = exp.shutdown(t.Context())
		require.NoError(t, err)
	}()

	// Create test profiles
	profiles := pprofile.NewProfiles()
	resourceProfiles := profiles.ResourceProfiles()
	rp := resourceProfiles.AppendEmpty()

	resource := rp.Resource()
	resource.Attributes().PutStr("service.name", "test-service")

	err = exp.pushProfiles(t.Context(), profiles)
	assert.Error(t, err)
}

func TestProfilesExporter_PushProfiles_WhenCannotSend(t *testing.T) {
	tests := []struct {
		description string
		enabled     bool
	}{
		{
			description: "Rate limit exceeded config enabled",
			enabled:     true,
		},
		{
			description: "Rate limit exceeded config disabled",
			enabled:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.description, func(t *testing.T) {
			cfg := &Config{
				Domain:     "test.domain.com",
				PrivateKey: "test-key",
				RateLimiter: RateLimiterConfig{
					Enabled:   tt.enabled,
					Threshold: 1,
					Duration:  time.Minute,
				},
			}

			exp, err := newProfilesExporter(cfg, exportertest.NewNopSettings(exportertest.NopType))
			require.NoError(t, err)

			err = exp.start(t.Context(), componenttest.NewNopHost())
			require.NoError(t, err)
			defer func() {
				err = exp.shutdown(t.Context())
				require.NoError(t, err)
			}()

			// Add two rate limit errors
			exp.EnableRateLimit()
			exp.EnableRateLimit()

			profiles := pprofile.NewProfiles()
			resourceProfiles := profiles.ResourceProfiles()
			rp := resourceProfiles.AppendEmpty()
			resource := rp.Resource()
			resource.Attributes().PutStr("service.name", "test-service")

			err = exp.pushProfiles(t.Context(), profiles)
			assert.Error(t, err)
			if tt.enabled {
				assert.Contains(t, err.Error(), "rate limit exceeded")
			} else {
				assert.Contains(t, err.Error(), "no such host")
			}
		})
	}
}

type mockProfilesServer struct {
	pprofileotlp.UnimplementedGRPCServer
	recvCount      int
	partialSuccess *pprofileotlp.ExportPartialSuccess
	t              testing.TB
}

func (m *mockProfilesServer) Export(ctx context.Context, req pprofileotlp.ExportRequest) (pprofileotlp.ExportResponse, error) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		m.t.Error("No metadata found in context")
		return pprofileotlp.NewExportResponse(), errors.New("no metadata found")
	}

	authHeader := md.Get("authorization")
	if len(authHeader) == 0 {
		m.t.Error("No Authorization header found")
		return pprofileotlp.NewExportResponse(), errors.New("no authorization header")
	}

	if authHeader[0] != "Bearer test-key" {
		m.t.Errorf("Expected Authorization header 'Bearer test-key', got %s", authHeader[0])
		return pprofileotlp.NewExportResponse(), errors.New("invalid authorization header")
	}

	assertAcceptEncodingGzip(m.t, md)

	// Count individual profiles instead of resource profiles
	for _, rp := range req.Profiles().ResourceProfiles().All() {
		for _, sp := range rp.ScopeProfiles().All() {
			m.recvCount += sp.Profiles().Len()
		}
	}

	resp := pprofileotlp.NewExportResponse()
	if m.partialSuccess != nil {
		resp.PartialSuccess().SetErrorMessage(m.partialSuccess.ErrorMessage())
		resp.PartialSuccess().SetRejectedProfiles(m.partialSuccess.RejectedProfiles())
	}
	return resp, nil
}

func startMockOtlpProfilesServer(tb testing.TB) (endpoint string, stopFn func(), srv *mockProfilesServer) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		tb.Fatalf("failed to listen: %v", err)
	}
	grpcServer := grpc.NewServer()
	srv = &mockProfilesServer{t: tb}
	pprofileotlp.RegisterGRPCServer(grpcServer, srv)
	go func() {
		_ = grpcServer.Serve(ln)
	}()
	return ln.Addr().String(), func() {
		grpcServer.Stop()
		ln.Close()
	}, srv
}

func BenchmarkProfilesExporter_PushProfiles(b *testing.B) {
	endpoint, stopFn, mockSrv := startMockOtlpProfilesServer(b)
	defer stopFn()

	cfg := &Config{
		Profiles: configgrpc.ClientConfig{
			Endpoint: endpoint,
			TLS: configtls.ClientConfig{
				Insecure: true,
			},
		},
		PrivateKey: "test-key",
	}

	exp, err := newProfilesExporter(cfg, exportertest.NewNopSettings(exportertest.NopType))
	if err != nil {
		b.Fatalf("failed to create profiles exporter: %v", err)
	}
	if err := exp.start(b.Context(), componenttest.NewNopHost()); err != nil {
		b.Fatalf("failed to start profiles exporter: %v", err)
	}
	defer func() {
		_ = exp.shutdown(b.Context())
	}()

	testCases := []int{
		100000,
		500000,
		1000000,
		5000000,
		10000000,
		50000000,
	}
	for _, numProfiles := range testCases {
		b.Run("numProfiles="+fmt.Sprint(numProfiles), func(b *testing.B) {
			for b.Loop() {
				profiles := pprofile.NewProfiles()
				rp := profiles.ResourceProfiles().AppendEmpty()
				rp.Resource().Attributes().PutStr("service.name", "benchmark-service")
				sp := rp.ScopeProfiles().AppendEmpty()
				for j := range numProfiles {
					profile := sp.Profiles().AppendEmpty()
					var id [16]byte
					binary.LittleEndian.PutUint64(id[:8], uint64(j))
					profile.SetProfileID(id)
					profile.SetDurationNano(uint64(1 * time.Second.Nanoseconds()))
				}
				_ = exp.pushProfiles(b.Context(), profiles)
			}
		})
	}
	b.Logf("Total profiles received by mock server: %d", mockSrv.recvCount)
}

func TestProfilesExporter_PushProfiles_PartialSuccess(t *testing.T) {
	endpoint, stopFn, mockSrv := startMockOtlpProfilesServer(t)
	defer stopFn()

	cfg := &Config{
		Profiles: configgrpc.ClientConfig{
			Endpoint: endpoint,
			TLS: configtls.ClientConfig{
				Insecure: true,
			},
		},
		PrivateKey: "test-key",
	}

	exp, err := newProfilesExporter(cfg, exportertest.NewNopSettings(exportertest.NopType))
	require.NoError(t, err)

	err = exp.start(t.Context(), componenttest.NewNopHost())
	require.NoError(t, err)
	defer func() {
		err = exp.shutdown(t.Context())
		require.NoError(t, err)
	}()

	profiles := pprofile.NewProfiles()
	resourceProfiles := profiles.ResourceProfiles()
	rm := resourceProfiles.AppendEmpty()
	resource := rm.Resource()
	resource.Attributes().PutStr("service.name", "test-service")

	scopeProfiles := rm.ScopeProfiles().AppendEmpty()
	scopeProfiles.Scope().SetName("test-scope")

	profile1 := scopeProfiles.Profiles().AppendEmpty()
	var id1 [16]byte
	copy(id1[:], "profile1-unique")
	profile1.SetProfileID(id1)
	profile2 := scopeProfiles.Profiles().AppendEmpty()
	var id2 [16]byte
	copy(id2[:], "profile2-unique")
	profile2.SetProfileID(id2)

	partialSuccess := pprofileotlp.NewExportPartialSuccess()
	partialSuccess.SetErrorMessage("some profiles were rejected")
	partialSuccess.SetRejectedProfiles(1)
	mockSrv.partialSuccess = &partialSuccess

	core, observed := observer.New(zapcore.ErrorLevel)
	logger := zap.New(core)
	exp.settings.Logger = logger

	err = exp.pushProfiles(t.Context(), profiles)
	require.NoError(t, err)

	entries := observed.All()
	found := false
	for _, entry := range entries {
		if entry.Message == "Partial success response from Coralogix" &&
			entry.Level == zapcore.ErrorLevel &&
			entry.ContextMap()["message"] == "some profiles were rejected" &&
			entry.ContextMap()["rejected_profiles"] == int64(1) {
			found = true
		}
	}
	assert.True(t, found, "Expected partial success log with correct fields")
}

func TestProfilesExporter_PushProfiles_Performance(t *testing.T) {
	isIntegrationTest := os.Getenv("INTEGRATION_TEST")
	if isIntegrationTest != "true" {
		t.Skip("Skipping E2E test: INTEGRATION_TEST not set")
	}

	endpoint, stopFn, mockSrv := startMockOtlpProfilesServer(t)
	defer stopFn()

	cfg := &Config{
		Profiles: configgrpc.ClientConfig{
			Endpoint: endpoint,
			TLS: configtls.ClientConfig{
				Insecure: true,
			},
		},
		PrivateKey: "test-key",
		RateLimiter: RateLimiterConfig{
			Enabled:   true,
			Threshold: 3,
			Duration:  time.Second,
		},
	}

	exp, err := newProfilesExporter(cfg, exportertest.NewNopSettings(exportertest.NopType))
	require.NoError(t, err)

	err = exp.start(t.Context(), componenttest.NewNopHost())
	require.NoError(t, err)
	defer func() {
		err = exp.shutdown(t.Context())
		require.NoError(t, err)
	}()

	t.Run("Under rate limit", func(t *testing.T) {
		mockSrv.recvCount = 0
		profiles := pprofile.NewProfiles()
		rp := profiles.ResourceProfiles().AppendEmpty()
		rp.Resource().Attributes().PutStr("service.name", "test-service")
		sp := rp.ScopeProfiles().AppendEmpty()

		profileCount := 3000
		for i := range profileCount {
			profile := sp.Profiles().AppendEmpty()
			var id [16]byte
			binary.LittleEndian.PutUint64(id[:8], uint64(i))
			profile.SetProfileID(id)
			profile.SetDurationNano(uint64(1 * time.Second.Nanoseconds()))
		}

		start := time.Now()
		err = exp.pushProfiles(t.Context(), profiles)
		duration := time.Since(start)

		require.NoError(t, err)
		assert.Equal(t, profileCount, mockSrv.recvCount, "Expected to receive exactly %d profiles", profileCount)
		assert.Less(t, duration, time.Millisecond*100, "Operation took longer than 100 milliseconds")
	})

	t.Run("Over rate limit", func(t *testing.T) {
		mockSrv.recvCount = 0

		for range 5 {
			exp.EnableRateLimit()
		}

		profiles := pprofile.NewProfiles()
		rp := profiles.ResourceProfiles().AppendEmpty()
		rp.Resource().Attributes().PutStr("service.name", "test-service")
		sp := rp.ScopeProfiles().AppendEmpty()

		profileCount := 7000
		for i := range profileCount {
			profile := sp.Profiles().AppendEmpty()
			var id [16]byte
			binary.LittleEndian.PutUint64(id[:8], uint64(i))
			profile.SetProfileID(id)
			profile.SetDurationNano(uint64(1 * time.Second.Nanoseconds()))
		}

		start := time.Now()
		err = exp.pushProfiles(t.Context(), profiles)
		duration := time.Since(start)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "rate limit exceeded")
		assert.Less(t, duration, time.Millisecond*100, "Operation took longer than 100 milliseconds")
		assert.Zero(t, mockSrv.recvCount, "Expected no profiles to be received due to rate limiting")
	})

	t.Run("Rate limit reset", func(t *testing.T) {
		mockSrv.recvCount = 0

		require.Eventually(t, func() bool {
			testProfiles := pprofile.NewProfiles()
			testRp := testProfiles.ResourceProfiles().AppendEmpty()
			testRp.Resource().Attributes().PutStr("service.name", "test-service")
			testSp := testRp.ScopeProfiles().AppendEmpty()
			testProfile := testSp.Profiles().AppendEmpty()
			var testID [16]byte
			binary.LittleEndian.PutUint64(testID[:8], uint64(0))
			testProfile.SetProfileID(testID)

			errPush := exp.pushProfiles(t.Context(), testProfiles)
			return errPush == nil
		}, 3*time.Second, 100*time.Millisecond, "Rate limit should reset within 3 seconds")

		// It's 1 because the last push is successful
		require.Equal(t, 1, mockSrv.recvCount)
		mockSrv.recvCount = 0

		profiles := pprofile.NewProfiles()
		rp := profiles.ResourceProfiles().AppendEmpty()
		rp.Resource().Attributes().PutStr("service.name", "test-service")
		sp := rp.ScopeProfiles().AppendEmpty()

		profileCount := 3000
		for i := range profileCount {
			profile := sp.Profiles().AppendEmpty()
			var id [16]byte
			binary.LittleEndian.PutUint64(id[:8], uint64(i))
			profile.SetProfileID(id)
			profile.SetDurationNano(uint64(1 * time.Second.Nanoseconds()))
		}

		start := time.Now()
		err = exp.pushProfiles(t.Context(), profiles)
		duration := time.Since(start)

		require.NoError(t, err)
		assert.Equal(t, profileCount, mockSrv.recvCount, "Expected to receive exactly %d profiles after rate limit reset", profileCount)
		assert.Less(t, duration, time.Millisecond*100, "Operation took longer than 100 milliseconds")
	})
}

func TestProfilesExporter_RateLimitErrorCountReset(t *testing.T) {
	endpoint, stopFn, srv := startMockOtlpProfilesServer(t)
	defer stopFn()

	cfg := &Config{
		Profiles: configgrpc.ClientConfig{
			Endpoint: endpoint,
			TLS: configtls.ClientConfig{
				Insecure: true,
			},
		},
		PrivateKey: "test-key",
		RateLimiter: RateLimiterConfig{
			Enabled:   true,
			Threshold: 5,
			Duration:  time.Second,
		},
	}

	exp, err := newProfilesExporter(cfg, exportertest.NewNopSettings(exportertest.NopType))
	require.NoError(t, err)

	err = exp.start(t.Context(), componenttest.NewNopHost())
	require.NoError(t, err)
	defer func() {
		err = exp.shutdown(t.Context())
		require.NoError(t, err)
	}()

	for range 5 {
		exp.EnableRateLimit()
	}
	assert.Equal(t, int32(5), exp.rateError.errorCount.Load())

	profiles := pprofile.NewProfiles()
	resourceProfiles := profiles.ResourceProfiles().AppendEmpty()
	resource := resourceProfiles.Resource()
	resource.Attributes().PutStr("service.name", "test-service")
	scopeProfiles := resourceProfiles.ScopeProfiles().AppendEmpty()
	profile := scopeProfiles.Profiles().AppendEmpty()
	var id [16]byte
	binary.LittleEndian.PutUint64(id[:8], uint64(1))
	profile.SetProfileID(id)
	profile.SetDurationNano(uint64(1 * time.Second.Nanoseconds()))

	err = exp.pushProfiles(t.Context(), profiles)
	assert.Error(t, err)
	assert.Equal(t, int32(5), exp.rateError.errorCount.Load())
	assert.Equal(t, 0, srv.recvCount)

	require.Eventually(t, func() bool {
		err = exp.pushProfiles(t.Context(), profiles)
		return err == nil &&
			exp.rateError.errorCount.Load() == 0 &&
			srv.recvCount > 0
	}, 3*time.Second, 100*time.Millisecond)
}

func TestProfilesExporter_RateLimitCounterResetOnSuccess(t *testing.T) {
	endpoint, stopFn, srv := startMockOtlpProfilesServer(t)
	defer stopFn()

	cfg := &Config{
		Profiles: configgrpc.ClientConfig{
			Endpoint: endpoint,
			TLS: configtls.ClientConfig{
				Insecure: true,
			},
		},
		PrivateKey: "test-key",
		RateLimiter: RateLimiterConfig{
			Enabled:   true,
			Threshold: 5,
			Duration:  time.Second,
		},
	}

	exp, err := newProfilesExporter(cfg, exportertest.NewNopSettings(exportertest.NopType))
	require.NoError(t, err)

	err = exp.start(t.Context(), componenttest.NewNopHost())
	require.NoError(t, err)
	defer func() {
		err = exp.shutdown(t.Context())
		require.NoError(t, err)
	}()

	createTestProfiles := func() pprofile.Profiles {
		profiles := pprofile.NewProfiles()
		resourceProfiles := profiles.ResourceProfiles().AppendEmpty()
		resource := resourceProfiles.Resource()
		resource.Attributes().PutStr("service.name", "test-service")
		scopeProfiles := resourceProfiles.ScopeProfiles().AppendEmpty()
		profile := scopeProfiles.Profiles().AppendEmpty()
		var id [16]byte
		binary.LittleEndian.PutUint64(id[:8], uint64(1))
		profile.SetProfileID(id)
		profile.SetDurationNano(uint64(1 * time.Second.Nanoseconds()))
		return profiles
	}

	t.Run("Initial successful push", func(t *testing.T) {
		profiles := createTestProfiles()
		err = exp.pushProfiles(t.Context(), profiles)
		require.NoError(t, err)
		assert.Equal(t, int32(0), exp.rateError.errorCount.Load())
		assert.Equal(t, 1, srv.recvCount)
	})

	t.Run("Trigger errors below threshold", func(t *testing.T) {
		for range 4 {
			exp.EnableRateLimit()
		}
		assert.Equal(t, int32(4), exp.rateError.errorCount.Load())
		assert.False(t, exp.rateError.isRateLimited(), "Should not be rate limited yet")
	})

	t.Run("Successful push after errors", func(t *testing.T) {
		profiles := createTestProfiles()
		err = exp.pushProfiles(t.Context(), profiles)
		require.NoError(t, err)
		assert.Equal(t, int32(0), exp.rateError.errorCount.Load())
		assert.Equal(t, 2, srv.recvCount)
	})

	t.Run("Verify error count stays at 0", func(t *testing.T) {
		profiles := createTestProfiles()
		err = exp.pushProfiles(t.Context(), profiles)
		require.NoError(t, err)
		assert.Equal(t, int32(0), exp.rateError.errorCount.Load())
		assert.Equal(t, 3, srv.recvCount)
	})
}
