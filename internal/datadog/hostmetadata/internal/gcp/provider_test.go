// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package gcp

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"cloud.google.com/go/compute/metadata"
	"github.com/DataDog/datadog-agent/pkg/opentelemetry-mapping-go/otlp/attributes/source"
	"github.com/GoogleCloudPlatform/opentelemetry-operations-go/detectors/gcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	testShortHostname = "hostname"
	testCloudAccount  = "projectID"
	testHostname      = testShortHostname + ".c." + testCloudAccount + ".internal"
	testBadHostname   = "badhostname"
)

var (
	testGCPIntegrationHostname    = fmt.Sprintf("%s.%s", testShortHostname, testCloudAccount)
	testGCPIntegrationBadHostname = fmt.Sprintf("%s.%s", testBadHostname, testCloudAccount)
)

var _ gcpDetector = (*mockDetector)(nil)

type mockDetector struct {
	platform     gcp.Platform
	projectID    string
	instanceName string
}

func (m *mockDetector) CloudPlatform(context.Context) gcp.Platform {
	return m.platform
}

func (m *mockDetector) ProjectID(context.Context) (string, error) {
	return m.projectID, nil
}

func (m *mockDetector) GCEHostName(context.Context) (string, error) {
	return m.instanceName, nil
}

func (*mockDetector) GKEClusterName(context.Context) (string, error) {
	return "", errors.New("not available")
}

func TestProvider(t *testing.T) {
	tests := []struct {
		name         string
		projectID    string
		platform     gcp.Platform
		instanceName string
		hostname     string
	}{
		{
			name:         "good hostname",
			platform:     gcp.GCE,
			projectID:    testCloudAccount,
			instanceName: testHostname,
			hostname:     testGCPIntegrationHostname,
		},
		{
			name:         "bad hostname",
			platform:     gcp.GKE,
			projectID:    testCloudAccount,
			instanceName: testBadHostname,
			hostname:     testGCPIntegrationBadHostname,
		},
	}

	for _, testInstance := range tests {
		t.Run(testInstance.name, func(t *testing.T) {
			provider := &Provider{detector: &mockDetector{
				platform:     testInstance.platform,
				projectID:    testInstance.projectID,
				instanceName: testInstance.instanceName,
			}}

			src, err := provider.Source(t.Context())
			require.NoError(t, err)
			assert.Equal(t, source.HostnameKind, src.Kind)
			assert.Equal(t, testInstance.hostname, src.SourceIdentifier.Primary)
		})
	}
}

var metadataResponses = map[string]string{
	"/computeMetadata/v1/instance/machine-type":                "projects/123/machineTypes/n1-standard-1",
	"/computeMetadata/v1/instance/name":                        testHostname,
	"/computeMetadata/v1/project/project-id":                   testCloudAccount,
	"/computeMetadata/v1/instance/attributes/cluster-name":     "test-cluster",
	"/computeMetadata/v1/instance/attributes/cluster-location": "us-central1-a",
}

// unsetPlatformEnv keeps the host environment out of platform detection.
func unsetPlatformEnv(t *testing.T) {
	for _, key := range []string{
		"BMS_PROJECT_ID", "BMS_REGION", "BMS_INSTANCE_ID", "KUBERNETES_SERVICE_HOST", "FUNCTION_TARGET",
		"K_CONFIGURATION", "CLOUD_RUN_JOB", "CLOUD_RUN_WORKER_POOL", "GAE_ENV", "GAE_SERVICE",
	} {
		t.Setenv(key, "") // restores the original value when the test ends
		require.NoError(t, os.Unsetenv(key))
	}
}

func serveMetadata(t *testing.T, handler http.HandlerFunc) {
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	t.Setenv("GCE_METADATA_HOST", strings.TrimPrefix(srv.URL, "http://"))
}

func writeMetadata(w http.ResponseWriter, r *http.Request) {
	if body, ok := metadataResponses[r.URL.Path]; ok {
		_, _ = w.Write([]byte(body))
		return
	}
	http.NotFound(w, r)
}

// stall holds a request open until the client gives up on it.
func stall(r *http.Request) {
	select {
	case <-r.Context().Done():
	case <-time.After(5 * time.Second):
	}
}

func sourceErr(ctx context.Context, p *Provider) error  { _, err := p.Source(ctx); return err }
func clusterErr(ctx context.Context, p *Provider) error { _, err := p.ClusterName(ctx); return err }

func TestProviderWithMetadataServer(t *testing.T) {
	unsetPlatformEnv(t)
	for _, platform := range []gcp.Platform{gcp.GCE, gcp.GKE} {
		t.Run(platformDescription(platform), func(t *testing.T) {
			if platform == gcp.GKE {
				t.Setenv("KUBERNETES_SERVICE_HOST", "10.0.0.1")
			}
			serveMetadata(t, func(w http.ResponseWriter, r *http.Request) {
				if platform == gcp.GKE && r.URL.Path == "/computeMetadata/v1/instance/machine-type" {
					http.NotFound(w, r) // GKE detection must not depend on the GCE probe
					return
				}
				writeMetadata(w, r)
			})
			p := NewProvider()
			src, err := p.Source(t.Context())
			require.NoError(t, err)
			assert.Equal(t, testGCPIntegrationHostname, src.SourceIdentifier.Primary)
			cluster, err := p.ClusterName(t.Context())
			require.NoError(t, err)
			assert.Equal(t, "test-cluster", cluster)
		})
	}
}

func TestProviderRespectsContext(t *testing.T) {
	unsetPlatformEnv(t)

	t.Run("canceled before the call", func(t *testing.T) {
		var calls atomic.Int32
		serveMetadata(t, func(w http.ResponseWriter, r *http.Request) { calls.Add(1); writeMetadata(w, r) })
		ctx, cancel := context.WithCancel(t.Context())
		cancel()
		require.ErrorIs(t, sourceErr(ctx, NewProvider()), context.Canceled)
		require.ErrorIs(t, clusterErr(ctx, NewProvider()), context.Canceled)
		assert.Zero(t, calls.Load())
	})

	t.Run("deadline while metadata stalls", func(t *testing.T) {
		t.Setenv("KUBERNETES_SERVICE_HOST", "10.0.0.1")
		serveMetadata(t, func(_ http.ResponseWriter, r *http.Request) { stall(r) })
		for _, call := range []func(context.Context, *Provider) error{sourceErr, clusterErr} {
			ctx, cancel := context.WithTimeout(t.Context(), 50*time.Millisecond)
			start := time.Now()
			require.ErrorIs(t, call(ctx, NewProvider()), context.DeadlineExceeded)
			assert.Less(t, time.Since(start), time.Second)
			cancel()
		}
	})

	for _, tc := range []struct {
		path string
		gke  bool
		call func(context.Context, *Provider) error
	}{
		{
			path: "/computeMetadata/v1/instance/machine-type",
			call: sourceErr,
		},
		{
			path: "/computeMetadata/v1/instance/name",
			call: sourceErr,
		},
		{
			path: "/computeMetadata/v1/project/project-id",
			call: sourceErr,
		},
		{
			path: "/computeMetadata/v1/instance/attributes/cluster-location",
			gke:  true,
			call: sourceErr,
		},
		{
			path: "/computeMetadata/v1/instance/attributes/cluster-name",
			call: clusterErr,
		},
	} {
		t.Run("canceled during "+path.Base(tc.path), func(t *testing.T) {
			if tc.gke {
				t.Setenv("KUBERNETES_SERVICE_HOST", "10.0.0.1")
			}
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			serveMetadata(t, func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != tc.path {
					writeMetadata(w, r)
					return
				}
				cancel()
				stall(r)
			})
			start := time.Now()
			require.ErrorIs(t, tc.call(ctx, NewProvider()), context.Canceled)
			assert.Less(t, time.Since(start), time.Second)
		})
	}

	for _, status := range []int{http.StatusInternalServerError, http.StatusTooManyRequests} {
		t.Run(fmt.Sprintf("canceled while retrying %d", status), func(t *testing.T) {
			var calls atomic.Int32
			cancelLater := make(chan context.CancelFunc, 1)
			serveMetadata(t, func(w http.ResponseWriter, _ *http.Request) {
				calls.Add(1)
				select {
				case cancel := <-cancelLater:
					time.AfterFunc(25*time.Millisecond, cancel) // lands in the retry backoff
				default:
				}
				w.WriteHeader(status)
			})
			p := NewProvider()
			require.ErrorContains(t, sourceErr(t.Context(), p), "not on GCE or GKE")
			uncanceledCalls := calls.Load()
			assert.Greater(t, uncanceledCalls, int32(1))

			calls.Store(0)
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			cancelLater <- cancel
			require.ErrorIs(t, sourceErr(ctx, p), context.Canceled)
			assert.Less(t, calls.Load(), uncanceledCalls)
		})
	}

	t.Run("canceled during dial", func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()
		testDone := t.Context().Done()
		client := &http.Client{Transport: &http.Transport{
			DialContext: func(context.Context, string, string) (net.Conn, error) {
				cancel()
				select { // stall like an unreachable metadata server until the test ends
				case <-testDone:
				case <-time.After(5 * time.Second):
				}
				return nil, errors.New("dial abandoned")
			},
		}}
		p := &Provider{detector: &contextDetector{metadata: metadata.NewClient(client)}}
		start := time.Now()
		require.ErrorIs(t, sourceErr(ctx, p), context.Canceled)
		assert.Less(t, time.Since(start), time.Second)
	})
}

func TestCloudPlatformParity(t *testing.T) {
	unsetPlatformEnv(t)
	for _, tc := range []struct {
		name     string
		env      map[string]string
		gke, gce bool // whether the cluster-location and machine-type probes succeed
		want     gcp.Platform
	}{
		{
			name: "unknown",
			want: gcp.UnknownPlatform,
		},
		{
			name: "gce",
			gce:  true,
			want: gcp.GCE,
		},
		{
			name: "bms over gke",
			env: map[string]string{
				"BMS_PROJECT_ID":          "p",
				"BMS_REGION":              "r",
				"BMS_INSTANCE_ID":         "i",
				"KUBERNETES_SERVICE_HOST": "h",
			},
			gke:  true,
			gce:  true,
			want: gcp.BareMetalSolution,
		},
		// BMS needs non-empty values, while the other variables only need to be set.
		{
			name: "empty bms value falls through to gke",
			env: map[string]string{
				"BMS_PROJECT_ID":          "",
				"BMS_REGION":              "r",
				"BMS_INSTANCE_ID":         "i",
				"KUBERNETES_SERVICE_HOST": "",
			},
			gke:  true,
			gce:  true,
			want: gcp.GKE,
		},
		{
			name: "gke over cloud run",
			env: map[string]string{
				"KUBERNETES_SERVICE_HOST": "h",
				"K_CONFIGURATION":         "c",
			},
			gke:  true,
			gce:  true,
			want: gcp.GKE,
		},
		{
			name: "failed gke probe falls through",
			env: map[string]string{
				"KUBERNETES_SERVICE_HOST": "h",
				"K_CONFIGURATION":         "c",
			},
			gce:  true,
			want: gcp.CloudRun,
		},
		{
			name: "functions over cloud run",
			env: map[string]string{
				"FUNCTION_TARGET": "f",
				"K_CONFIGURATION": "c",
			},
			gce:  true,
			want: gcp.CloudFunctions,
		},
		{
			name: "cloud run over job",
			env: map[string]string{
				"K_CONFIGURATION": "c",
				"CLOUD_RUN_JOB":   "j",
			},
			gce:  true,
			want: gcp.CloudRun,
		},
		{
			name: "job over worker pool",
			env: map[string]string{
				"CLOUD_RUN_JOB":         "j",
				"CLOUD_RUN_WORKER_POOL": "w",
			},
			gce:  true,
			want: gcp.CloudRunJob,
		},
		{
			name: "worker pool over app engine",
			env: map[string]string{
				"CLOUD_RUN_WORKER_POOL": "w",
				"GAE_ENV":               "standard",
			},
			gce:  true,
			want: gcp.CloudRunWorkerPool,
		},
		{
			name: "app engine standard over flex",
			env: map[string]string{
				"GAE_ENV":     "standard",
				"GAE_SERVICE": "s",
			},
			gce:  true,
			want: gcp.AppEngineStandard,
		},
		{
			name: "app engine flex over gce",
			env: map[string]string{
				"GAE_ENV":     "flex",
				"GAE_SERVICE": "s",
			},
			gce:  true,
			want: gcp.AppEngineFlex,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for k, v := range tc.env {
				t.Setenv(k, v)
			}
			probes := map[string]bool{
				"/computeMetadata/v1/instance/attributes/cluster-location": tc.gke,
				"/computeMetadata/v1/instance/machine-type":                tc.gce,
			}
			serveMetadata(t, func(w http.ResponseWriter, r *http.Request) {
				if !probes[r.URL.Path] {
					http.NotFound(w, r)
					return
				}
				writeMetadata(w, r)
			})
			got := NewProvider().detector.CloudPlatform(t.Context())
			assert.Equal(t, gcp.NewDetector().CloudPlatform(), got)
			assert.Equal(t, tc.want, got)
		})
	}
}
