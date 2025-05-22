// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package coralogixexporter

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/configgrpc"
	"go.opentelemetry.io/collector/config/configopaque"
	"go.opentelemetry.io/collector/config/configtls"
	"go.opentelemetry.io/collector/exporter/exportertest"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pprofile"
	"go.opentelemetry.io/collector/pdata/pprofile/pprofileotlp"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
	"google.golang.org/grpc"
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
		Profiles: configgrpc.ClientConfig{
			Headers: map[string]configopaque.String{},
		},
	}

	exp, err := newProfilesExporter(cfg, exportertest.NewNopSettings(exportertest.NopType))
	require.NoError(t, err)

	err = exp.start(context.Background(), componenttest.NewNopHost())
	require.NoError(t, err)
	assert.NotNil(t, exp.clientConn)
	assert.NotNil(t, exp.profilesExporter)
	assert.Contains(t, exp.config.Profiles.Headers, "Authorization")

	// Test shutdown
	err = exp.shutdown(context.Background())
	require.NoError(t, err)
}

func TestProfilesExporter_EnhanceContext(t *testing.T) {
	cfg := &Config{
		Domain:     "test.domain.com",
		PrivateKey: "test-key",
		Profiles: configgrpc.ClientConfig{
			Headers: map[string]configopaque.String{
				"test-header": "test-value",
			},
		},
	}

	exp, err := newProfilesExporter(cfg, exportertest.NewNopSettings(exportertest.NopType))
	require.NoError(t, err)

	ctx := context.Background()
	enhancedCtx := exp.enhanceContext(ctx)
	assert.NotEqual(t, ctx, enhancedCtx)
}

func TestProfilesExporter_PushProfiles(t *testing.T) {
	cfg := &Config{
		Domain:     "test.domain.com",
		PrivateKey: "test-key",
		Profiles: configgrpc.ClientConfig{
			Headers: map[string]configopaque.String{},
		},
	}

	exp, err := newProfilesExporter(cfg, exportertest.NewNopSettings(exportertest.NopType))
	require.NoError(t, err)

	// Initialize the exporter by calling start
	err = exp.start(context.Background(), componenttest.NewNopHost())
	require.NoError(t, err)
	defer func() {
		err = exp.shutdown(context.Background())
		require.NoError(t, err)
	}()

	// Create test profiles
	profiles := pprofile.NewProfiles()
	resourceProfiles := profiles.ResourceProfiles()
	rp := resourceProfiles.AppendEmpty()

	resource := rp.Resource()
	resource.Attributes().PutStr("service.name", "test-service")

	err = exp.pushProfiles(context.Background(), profiles)
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
				Profiles: configgrpc.ClientConfig{
					Headers: map[string]configopaque.String{},
				},
				RateLimiter: RateLimiterConfig{
					Enabled:   tt.enabled,
					Threshold: 1,
					Duration:  time.Minute,
				},
			}

			exp, err := newProfilesExporter(cfg, exportertest.NewNopSettings(exportertest.NopType))
			require.NoError(t, err)

			err = exp.start(context.Background(), componenttest.NewNopHost())
			require.NoError(t, err)
			defer func() {
				err = exp.shutdown(context.Background())
				require.NoError(t, err)
			}()

			rateLimitErr := errors.New("rate limit exceeded")
			exp.EnableRateLimit(rateLimitErr)
			exp.EnableRateLimit(rateLimitErr)

			profiles := pprofile.NewProfiles()
			resourceProfiles := profiles.ResourceProfiles()
			rp := resourceProfiles.AppendEmpty()
			resource := rp.Resource()
			resource.Attributes().PutStr("service.name", "test-service")

			err = exp.pushProfiles(context.Background(), profiles)
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
}

func (m *mockProfilesServer) Export(_ context.Context, req pprofileotlp.ExportRequest) (pprofileotlp.ExportResponse, error) {
	m.recvCount += req.Profiles().ResourceProfiles().Len()
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
	srv = &mockProfilesServer{}
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
			TLSSetting: configtls.ClientConfig{
				Insecure: true,
			},
			Headers: map[string]configopaque.String{},
		},
		PrivateKey: "test-key",
	}

	exp, err := newProfilesExporter(cfg, exportertest.NewNopSettings(exportertest.NopType))
	if err != nil {
		b.Fatalf("failed to create profiles exporter: %v", err)
	}
	if err := exp.start(context.Background(), componenttest.NewNopHost()); err != nil {
		b.Fatalf("failed to start profiles exporter: %v", err)
	}
	defer func() {
		_ = exp.shutdown(context.Background())
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
			for i := 0; i < b.N; i++ {
				profiles := pprofile.NewProfiles()
				rp := profiles.ResourceProfiles().AppendEmpty()
				rp.Resource().Attributes().PutStr("service.name", "benchmark-service")
				sp := rp.ScopeProfiles().AppendEmpty()
				for j := 0; j < numProfiles; j++ {
					profile := sp.Profiles().AppendEmpty()
					var id [16]byte
					binary.LittleEndian.PutUint64(id[:8], uint64(j))
					profile.SetProfileID(id)
					profile.SetStartTime(pcommon.NewTimestampFromTime(time.Now()))
					profile.SetDuration(1000000000) // 1 second in nanoseconds
				}
				_ = exp.pushProfiles(context.Background(), profiles)
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
			TLSSetting: configtls.ClientConfig{
				Insecure: true,
			},
			Headers: map[string]configopaque.String{},
		},
		PrivateKey: "test-key",
	}

	exp, err := newProfilesExporter(cfg, exportertest.NewNopSettings(exportertest.NopType))
	require.NoError(t, err)

	err = exp.start(context.Background(), componenttest.NewNopHost())
	require.NoError(t, err)
	defer func() {
		err = exp.shutdown(context.Background())
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
	copy(id1[:], []byte("profile1-unique"))
	profile1.SetProfileID(id1)
	profile2 := scopeProfiles.Profiles().AppendEmpty()
	var id2 [16]byte
	copy(id2[:], []byte("profile2-unique"))
	profile2.SetProfileID(id2)

	partialSuccess := pprofileotlp.NewExportPartialSuccess()
	partialSuccess.SetErrorMessage("some profiles were rejected")
	partialSuccess.SetRejectedProfiles(1)
	mockSrv.partialSuccess = &partialSuccess

	core, observed := observer.New(zapcore.ErrorLevel)
	logger := zap.New(core)
	exp.settings.Logger = logger

	err = exp.pushProfiles(context.Background(), profiles)
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
