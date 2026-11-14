// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package googlecloudstorageexporter

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/klauspost/compress/zstd"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configcompression"
	"go.opentelemetry.io/collector/extension"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"
	"google.golang.org/api/googleapi"
)

type metricsRecordedUpload struct {
	objectName string
	payload    []byte
}

type metricsRecordingEmulator struct {
	server     *httptest.Server
	mu         sync.Mutex
	uploads    []metricsRecordedUpload
	failUpload bool
	failStatus int
}

func (e *metricsRecordingEmulator) getUploads() []metricsRecordedUpload {
	e.mu.Lock()
	defer e.mu.Unlock()
	res := make([]metricsRecordedUpload, len(e.uploads))
	copy(res, e.uploads)
	return res
}

func (e *metricsRecordingEmulator) setFailUpload(fail bool, status ...int) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.failUpload = fail
	if len(status) > 0 {
		e.failStatus = status[0]
	} else {
		e.failStatus = http.StatusInternalServerError
	}
}

func newMetricsRecordingEmulator(t *testing.T, bucketName string) *metricsRecordingEmulator {
	emu := &metricsRecordingEmulator{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/storage/v1/b":
			var body struct {
				Name string `json:"name"`
			}
			_ = json.NewDecoder(r.Body).Decode(&body)
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(body)
		case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/storage/v1/b/"):
			bName := strings.TrimPrefix(r.URL.Path, "/storage/v1/b/")
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(map[string]any{"name": bName})
		case r.Method == http.MethodPost && r.URL.Path == "/upload/storage/v1/b/"+bucketName+"/o":
			emu.mu.Lock()
			fail := emu.failUpload
			statusCode := emu.failStatus
			emu.mu.Unlock()

			if fail {
				w.WriteHeader(statusCode)
				_ = json.NewEncoder(w).Encode(googleapi.Error{
					Code:    statusCode,
					Message: "simulated upload error",
				})
				return
			}

			objectName := r.URL.Query().Get("name")
			payload, err := extractMetricsPayload(r)
			assert.NoError(t, err)

			emu.mu.Lock()
			emu.uploads = append(emu.uploads, metricsRecordedUpload{
				objectName: objectName,
				payload:    payload,
			})
			emu.mu.Unlock()

			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"bucket": bucketName,
				"name":   objectName,
			})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))

	t.Cleanup(func() {
		server.Close()
	})
	t.Setenv("STORAGE_EMULATOR_HOST", server.Listener.Addr().String())
	emu.server = server
	return emu
}

func extractMetricsPayload(r *http.Request) ([]byte, error) {
	mediaType, params, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err == nil && strings.HasPrefix(mediaType, "multipart/") {
		mr := multipart.NewReader(r.Body, params["boundary"])
		var payload []byte
		for {
			p, err := mr.NextPart()
			if errors.Is(err, io.EOF) {
				break
			}
			if err != nil {
				return nil, err
			}
			data, err := io.ReadAll(p)
			if err != nil {
				return nil, err
			}
			partType := p.Header.Get("Content-Type")
			if !strings.HasPrefix(partType, "application/json") {
				payload = data
			}
		}
		return payload, nil
	}
	return io.ReadAll(r.Body)
}

func createSampleMetrics(serviceName, metricName string, val float64) pmetric.Metrics {
	md := pmetric.NewMetrics()
	rm := md.ResourceMetrics().AppendEmpty()
	if serviceName != "" {
		rm.Resource().Attributes().PutStr("service.name", serviceName)
	}
	sm := rm.ScopeMetrics().AppendEmpty()
	sm.Scope().SetName("test-meter")
	metric := sm.Metrics().AppendEmpty()
	metric.SetName(metricName)
	gauge := metric.SetEmptyGauge()
	dp := gauge.DataPoints().AppendEmpty()
	dp.SetDoubleValue(val)
	dp.SetTimestamp(pcommon.NewTimestampFromTime(time.Date(2026, 11, 14, 12, 0, 0, 0, time.UTC)))
	return md
}

func newTestMetricsGCSExporter(t *testing.T, cfg *Config) *storageExporter {
	exp, err := newStorageExporter(
		t.Context(),
		cfg,
		func(_ context.Context) (string, error) { return "us-central1", nil },
		func(_ context.Context) (string, error) { return "test-project", nil },
		zap.NewNop(),
		signalTypeMetrics,
	)
	require.NoError(t, err)
	return exp
}

type protoMetricMarshaler struct {
	extension.Extension
	marshaler pmetric.ProtoMarshaler
}

func (p *protoMetricMarshaler) MarshalMetrics(md pmetric.Metrics) ([]byte, error) {
	return p.marshaler.MarshalMetrics(md)
}

var _ pmetric.Marshaler = (*protoMetricMarshaler)(nil)

func TestConsumeMetrics_Serialization(t *testing.T) {
	bucketName := "metrics-ser-bucket"
	emu := newMetricsRecordingEmulator(t, bucketName)

	t.Run("default_json_serialization", func(t *testing.T) {
		exp := newTestMetricsGCSExporter(t, &Config{
			Bucket: bucketConfig{Name: bucketName},
		})
		err := exp.Start(t.Context(), &mockHost{extensions: make(map[component.ID]component.Component)})
		require.NoError(t, err)

		originalMD := createSampleMetrics("my-service", "system.cpu.load", 95.5)
		err = exp.ConsumeMetrics(t.Context(), originalMD)
		require.NoError(t, err)

		uploads := emu.getUploads()
		require.NotEmpty(t, uploads)
		lastUpload := uploads[len(uploads)-1]

		unmarshaler := &pmetric.JSONUnmarshaler{}
		decodedMD, err := unmarshaler.UnmarshalMetrics(lastUpload.payload)
		require.NoError(t, err)

		require.Equal(t, 1, decodedMD.ResourceMetrics().Len())
		res := decodedMD.ResourceMetrics().At(0).Resource()
		svcVal, ok := res.Attributes().Get("service.name")
		require.True(t, ok)
		require.Equal(t, "my-service", svcVal.AsString())

		metric := decodedMD.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0)
		require.Equal(t, "system.cpu.load", metric.Name())
		require.Equal(t, 95.5, metric.Gauge().DataPoints().At(0).DoubleValue())
	})

	t.Run("custom_encoding_extension_proto", func(t *testing.T) {
		extID := component.MustNewID("proto_encoding")
		protoExt := &protoMetricMarshaler{}
		host := &mockHost{
			extensions: map[component.ID]component.Component{
				extID: protoExt,
			},
		}

		exp := newTestMetricsGCSExporter(t, &Config{
			Bucket:   bucketConfig{Name: bucketName},
			Encoding: &extID,
		})
		err := exp.Start(t.Context(), host)
		require.NoError(t, err)

		originalMD := createSampleMetrics("proto-service", "memory.usage", 1024.0)
		err = exp.ConsumeMetrics(t.Context(), originalMD)
		require.NoError(t, err)

		uploads := emu.getUploads()
		require.NotEmpty(t, uploads)
		lastUpload := uploads[len(uploads)-1]

		protoUnmarshaler := &pmetric.ProtoUnmarshaler{}
		decodedMD, err := protoUnmarshaler.UnmarshalMetrics(lastUpload.payload)
		require.NoError(t, err)

		require.Equal(t, 1, decodedMD.ResourceMetrics().Len())
		metric := decodedMD.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0)
		require.Equal(t, "memory.usage", metric.Name())
		require.Equal(t, 1024.0, metric.Gauge().DataPoints().At(0).DoubleValue())
	})

	t.Run("empty_metrics_batch", func(t *testing.T) {
		exp := newTestMetricsGCSExporter(t, &Config{
			Bucket: bucketConfig{Name: bucketName},
		})
		err := exp.Start(t.Context(), &mockHost{extensions: make(map[component.ID]component.Component)})
		require.NoError(t, err)

		emptyMD := pmetric.NewMetrics()
		err = exp.ConsumeMetrics(t.Context(), emptyMD)
		require.NoError(t, err)

		uploads := emu.getUploads()
		require.NotEmpty(t, uploads)
		lastUpload := uploads[len(uploads)-1]

		unmarshaler := &pmetric.JSONUnmarshaler{}
		decodedMD, err := unmarshaler.UnmarshalMetrics(lastUpload.payload)
		require.NoError(t, err)
		require.Equal(t, 0, decodedMD.ResourceMetrics().Len())
	})

	t.Run("complex_metric_types", func(t *testing.T) {
		exp := newTestMetricsGCSExporter(t, &Config{
			Bucket: bucketConfig{Name: bucketName},
		})
		err := exp.Start(t.Context(), &mockHost{extensions: make(map[component.ID]component.Component)})
		require.NoError(t, err)

		md := pmetric.NewMetrics()
		rm := md.ResourceMetrics().AppendEmpty()
		rm.Resource().Attributes().PutStr("service.name", "complex-svc")
		sm := rm.ScopeMetrics().AppendEmpty()

		// 1. Sum metric
		sumMetric := sm.Metrics().AppendEmpty()
		sumMetric.SetName("http.requests.total")
		sum := sumMetric.SetEmptySum()
		sum.SetIsMonotonic(true)
		sum.SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)
		dpSum := sum.DataPoints().AppendEmpty()
		dpSum.SetIntValue(1250)

		// 2. Histogram metric
		histMetric := sm.Metrics().AppendEmpty()
		histMetric.SetName("http.request.duration")
		hist := histMetric.SetEmptyHistogram()
		hist.SetAggregationTemporality(pmetric.AggregationTemporalityDelta)
		dpHist := hist.DataPoints().AppendEmpty()
		dpHist.SetCount(10)
		dpHist.SetSum(15.2)
		dpHist.ExplicitBounds().FromRaw([]float64{0.5, 1.0, 2.5})
		dpHist.BucketCounts().FromRaw([]uint64{2, 5, 2, 1})

		// 3. Summary metric
		summMini := sm.Metrics().AppendEmpty()
		summMini.SetName("rpc.latency")
		summ := summMini.SetEmptySummary()
		dpSumm := summ.DataPoints().AppendEmpty()
		dpSumm.SetCount(50)
		dpSumm.SetSum(100.0)
		qv := dpSumm.QuantileValues().AppendEmpty()
		qv.SetQuantile(0.99)
		qv.SetValue(5.0)

		err = exp.ConsumeMetrics(t.Context(), md)
		require.NoError(t, err)

		uploads := emu.getUploads()
		require.NotEmpty(t, uploads)
		lastUpload := uploads[len(uploads)-1]

		unmarshaler := &pmetric.JSONUnmarshaler{}
		decodedMD, err := unmarshaler.UnmarshalMetrics(lastUpload.payload)
		require.NoError(t, err)

		require.Equal(t, 1, decodedMD.ResourceMetrics().Len())
		metrics := decodedMD.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics()
		require.Equal(t, 3, metrics.Len())
		require.Equal(t, "http.requests.total", metrics.At(0).Name())
		require.Equal(t, int64(1250), metrics.At(0).Sum().DataPoints().At(0).IntValue())
		require.Equal(t, "http.request.duration", metrics.At(1).Name())
		require.Equal(t, uint64(10), metrics.At(1).Histogram().DataPoints().At(0).Count())
		require.Equal(t, "rpc.latency", metrics.At(2).Name())
		require.Equal(t, uint64(50), metrics.At(2).Summary().DataPoints().At(0).Count())
	})
}

func TestConsumeMetrics_SuccessfulUploads(t *testing.T) {
	bucketName := "metrics-upload-bucket"
	emu := newMetricsRecordingEmulator(t, bucketName)

	t.Run("single_batch_upload", func(t *testing.T) {
		exp := newTestMetricsGCSExporter(t, &Config{
			Bucket: bucketConfig{Name: bucketName, FilePrefix: "metrics"},
		})
		err := exp.Start(t.Context(), &mockHost{extensions: make(map[component.ID]component.Component)})
		require.NoError(t, err)

		md := createSampleMetrics("svc", "metric1", 10.0)
		err = exp.ConsumeMetrics(t.Context(), md)
		require.NoError(t, err)

		uploads := emu.getUploads()
		require.NotEmpty(t, uploads)
		require.True(t, strings.HasPrefix(uploads[len(uploads)-1].objectName, "metrics_"))
	})

	t.Run("consecutive_batches_upload", func(t *testing.T) {
		exp := newTestMetricsGCSExporter(t, &Config{
			Bucket: bucketConfig{Name: bucketName, FilePrefix: "metrics"},
		})
		err := exp.Start(t.Context(), &mockHost{extensions: make(map[component.ID]component.Component)})
		require.NoError(t, err)

		initialCount := len(emu.getUploads())

		for i := 1; i <= 3; i++ {
			md := createSampleMetrics("svc", "batch.metric", float64(i))
			err = exp.ConsumeMetrics(t.Context(), md)
			require.NoError(t, err)
		}

		uploads := emu.getUploads()
		require.Equal(t, initialCount+3, len(uploads))

		// Check each uploaded object has a unique name
		names := make(map[string]bool)
		for _, u := range uploads[initialCount:] {
			require.False(t, names[u.objectName], "Duplicate object name detected: %s", u.objectName)
			names[u.objectName] = true
		}
	})

	t.Run("gzip_compressed_upload", func(t *testing.T) {
		exp := newTestMetricsGCSExporter(t, &Config{
			Bucket: bucketConfig{
				Name:        bucketName,
				FilePrefix:  "metrics",
				Compression: configcompression.TypeGzip,
			},
		})
		err := exp.Start(t.Context(), &mockHost{extensions: make(map[component.ID]component.Component)})
		require.NoError(t, err)

		md := createSampleMetrics("gzip-svc", "compressed.metric", 42.0)
		err = exp.ConsumeMetrics(t.Context(), md)
		require.NoError(t, err)

		uploads := emu.getUploads()
		require.NotEmpty(t, uploads)
		lastUpload := uploads[len(uploads)-1]
		require.True(t, strings.HasSuffix(lastUpload.objectName, ".gz"))

		// Decompress payload
		gzReader, err := gzip.NewReader(bytes.NewReader(lastUpload.payload))
		require.NoError(t, err)
		decompressed, err := io.ReadAll(gzReader)
		require.NoError(t, err)
		require.NoError(t, gzReader.Close())

		unmarshaler := &pmetric.JSONUnmarshaler{}
		decodedMD, err := unmarshaler.UnmarshalMetrics(decompressed)
		require.NoError(t, err)
		require.Equal(t, 1, decodedMD.ResourceMetrics().Len())
		metric := decodedMD.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0)
		require.Equal(t, "compressed.metric", metric.Name())
	})

	t.Run("zstd_compressed_upload", func(t *testing.T) {
		exp := newTestMetricsGCSExporter(t, &Config{
			Bucket: bucketConfig{
				Name:        bucketName,
				FilePrefix:  "metrics",
				Compression: configcompression.TypeZstd,
			},
		})
		err := exp.Start(t.Context(), &mockHost{extensions: make(map[component.ID]component.Component)})
		require.NoError(t, err)

		md := createSampleMetrics("zstd-svc", "zstd.metric", 99.0)
		err = exp.ConsumeMetrics(t.Context(), md)
		require.NoError(t, err)

		uploads := emu.getUploads()
		require.NotEmpty(t, uploads)
		lastUpload := uploads[len(uploads)-1]
		require.True(t, strings.HasSuffix(lastUpload.objectName, ".zst"))

		// Decompress payload
		zstdReader, err := zstd.NewReader(bytes.NewReader(lastUpload.payload))
		require.NoError(t, err)
		defer zstdReader.Close()
		decompressed, err := io.ReadAll(zstdReader)
		require.NoError(t, err)

		unmarshaler := &pmetric.JSONUnmarshaler{}
		decodedMD, err := unmarshaler.UnmarshalMetrics(decompressed)
		require.NoError(t, err)
		require.Equal(t, 1, decodedMD.ResourceMetrics().Len())
		metric := decodedMD.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0)
		require.Equal(t, "zstd.metric", metric.Name())
	})
}

func TestConsumeMetrics_ErrorHandling(t *testing.T) {
	bucketName := "metrics-err-bucket"
	emu := newMetricsRecordingEmulator(t, bucketName)

	t.Run("marshaling_error", func(t *testing.T) {
		exp := newTestMetricsGCSExporter(t, &Config{
			Bucket: bucketConfig{Name: bucketName},
		})
		err := exp.Start(t.Context(), &mockHost{extensions: make(map[component.ID]component.Component)})
		require.NoError(t, err)

		// Inject failing marshaler
		exp.metricsMarshaler = &mockMetricMarshaler{shouldFail: true}

		err = exp.ConsumeMetrics(t.Context(), pmetric.NewMetrics())
		require.Error(t, err)
		require.Contains(t, err.Error(), "failed to marshal metrics")
	})

	t.Run("upload_backend_error_500", func(t *testing.T) {
		exp := newTestMetricsGCSExporter(t, &Config{
			Bucket: bucketConfig{Name: bucketName},
		})
		err := exp.Start(t.Context(), &mockHost{extensions: make(map[component.ID]component.Component)})
		require.NoError(t, err)

		emu.setFailUpload(true, http.StatusInternalServerError)
		defer emu.setFailUpload(false)

		err = exp.ConsumeMetrics(t.Context(), createSampleMetrics("svc", "m", 1.0))
		require.Error(t, err)
		require.Contains(t, err.Error(), "failed to upload metrics")
	})

	t.Run("upload_backend_error_403", func(t *testing.T) {
		exp := newTestMetricsGCSExporter(t, &Config{
			Bucket: bucketConfig{Name: bucketName},
		})
		err := exp.Start(t.Context(), &mockHost{extensions: make(map[component.ID]component.Component)})
		require.NoError(t, err)

		emu.setFailUpload(true, http.StatusForbidden)
		defer emu.setFailUpload(false)

		err = exp.ConsumeMetrics(t.Context(), createSampleMetrics("svc", "m", 1.0))
		require.Error(t, err)
		require.Contains(t, err.Error(), "failed to upload metrics")
	})

	t.Run("canceled_context", func(t *testing.T) {
		exp := newTestMetricsGCSExporter(t, &Config{
			Bucket: bucketConfig{Name: bucketName},
		})
		err := exp.Start(t.Context(), &mockHost{extensions: make(map[component.ID]component.Component)})
		require.NoError(t, err)

		ctx, cancel := context.WithCancel(t.Context())
		cancel() // Cancel before call

		err = exp.ConsumeMetrics(ctx, createSampleMetrics("svc", "m", 1.0))
		require.Error(t, err)
	})
}

func TestConsumeMetrics_ObjectNamingAndPrefixes(t *testing.T) {
	bucketName := "metrics-naming-bucket"
	emu := newMetricsRecordingEmulator(t, bucketName)

	tests := []struct {
		name          string
		cfg           Config
		serviceName   string
		expectedRegex string
	}{
		{
			name: "default_file_prefix_logs",
			cfg: Config{
				Bucket: bucketConfig{
					Name: bucketName,
					// Omitted FilePrefix -> defaults to "logs" via createDefaultConfig
					FilePrefix: "logs",
				},
			},
			expectedRegex: `^logs_[0-9a-f-]{36}$`,
		},
		{
			name: "custom_file_prefix_metrics",
			cfg: Config{
				Bucket: bucketConfig{
					Name:       bucketName,
					FilePrefix: "metrics",
				},
			},
			expectedRegex: `^metrics_[0-9a-f-]{36}$`,
		},
		{
			name: "file_prefix_with_trailing_slash",
			cfg: Config{
				Bucket: bucketConfig{
					Name:       bucketName,
					FilePrefix: "metrics/",
				},
			},
			expectedRegex: `^metrics/[0-9a-f-]{36}$`,
		},
		{
			name: "partition_prefix_only",
			cfg: Config{
				Bucket: bucketConfig{
					Name:       bucketName,
					FilePrefix: "metrics",
					Partition: partitionConfig{
						Prefix: "cluster-alpha",
					},
				},
			},
			expectedRegex: `^cluster-alpha/metrics_[0-9a-f-]{36}$`,
		},
		{
			name: "time_partition_format",
			cfg: Config{
				Bucket: bucketConfig{
					Name:       bucketName,
					FilePrefix: "metrics",
					Partition: partitionConfig{
						Format: "year=%Y/month=%m",
					},
				},
			},
			expectedRegex: `^year=\d{4}/month=\d{2}/metrics_[0-9a-f-]{36}$`,
		},
		{
			name: "resource_attribute_partition",
			cfg: Config{
				Bucket: bucketConfig{
					Name:       bucketName,
					FilePrefix: "metrics",
				},
				ResourceAttrsToGCS: ResourceAttrsToGCS{
					Prefix: "service.name",
				},
			},
			serviceName:   "billing-svc",
			expectedRegex: `^billing-svc/metrics_[0-9a-f-]{36}$`,
		},
		{
			name: "full_prefix_hierarchy",
			cfg: Config{
				Bucket: bucketConfig{
					Name:       bucketName,
					FilePrefix: "metrics",
					Partition: partitionConfig{
						Prefix: "data",
						Format: "year=%Y",
					},
				},
				ResourceAttrsToGCS: ResourceAttrsToGCS{
					Prefix: "service.name",
				},
			},
			serviceName:   "payment-api",
			expectedRegex: `^data/payment-api/year=\d{4}/metrics_[0-9a-f-]{36}$`,
		},
		{
			name: "resource_attribute_missing_graceful_omission",
			cfg: Config{
				Bucket: bucketConfig{
					Name:       bucketName,
					FilePrefix: "metrics",
					Partition: partitionConfig{
						Prefix: "data",
						Format: "year=%Y",
					},
				},
				ResourceAttrsToGCS: ResourceAttrsToGCS{
					Prefix: "service.name",
				},
			},
			serviceName:   "", // missing
			expectedRegex: `^data/year=\d{4}/metrics_[0-9a-f-]{36}$`,
		},
		{
			name: "resource_attribute_sanitization",
			cfg: Config{
				Bucket: bucketConfig{
					Name:       bucketName,
					FilePrefix: "metrics",
				},
				ResourceAttrsToGCS: ResourceAttrsToGCS{
					Prefix: "service.name",
				},
			},
			serviceName:   "../../prod/serviceA/..",
			expectedRegex: `^prod/metrics_[0-9a-f-]{36}$`,
		},
		{
			name: "compression_extension_gzip",
			cfg: Config{
				Bucket: bucketConfig{
					Name:        bucketName,
					FilePrefix:  "metrics",
					Compression: configcompression.TypeGzip,
				},
			},
			expectedRegex: `^metrics_[0-9a-f-]{36}\.gz$`,
		},
		{
			name: "compression_extension_zstd",
			cfg: Config{
				Bucket: bucketConfig{
					Name:        bucketName,
					FilePrefix:  "metrics",
					Compression: configcompression.TypeZstd,
				},
			},
			expectedRegex: `^metrics_[0-9a-f-]{36}\.zst$`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfgCopy := tt.cfg
			exp := newTestMetricsGCSExporter(t, &cfgCopy)
			err := exp.Start(t.Context(), &mockHost{extensions: make(map[component.ID]component.Component)})
			require.NoError(t, err)

			md := createSampleMetrics(tt.serviceName, "test.metric", 123.45)
			err = exp.ConsumeMetrics(t.Context(), md)
			require.NoError(t, err)

			uploads := emu.getUploads()
			require.NotEmpty(t, uploads)
			lastUpload := uploads[len(uploads)-1]

			re := regexp.MustCompile(tt.expectedRegex)
			require.True(t, re.MatchString(lastUpload.objectName),
				"Object name %q did not match expected regex %q", lastUpload.objectName, tt.expectedRegex)
		})
	}
}
