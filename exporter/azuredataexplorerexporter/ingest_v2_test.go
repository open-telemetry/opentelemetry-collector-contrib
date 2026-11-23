// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package azuredataexplorerexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/azuredataexplorerexporter"

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	kustov2 "github.com/Azure/azure-kusto-go/azkustoingestv2"
	v2options "github.com/Azure/azure-kusto-go/azkustoingestv2/ingestoptions"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

type configurationFunc func(context.Context) (*v2options.ConfigurationResponse, error)

func (f configurationFunc) GetConfigurationDetails(ctx context.Context) (*v2options.ConfigurationResponse, error) {
	return f(ctx)
}

type mockQueuedV2 struct {
	ctx              context.Context
	database, table  string
	payload, mapping string
	format           v2options.DataFormat
	err              error
	calls, closes    int
}

func (m *mockQueuedV2) Ingest(ctx context.Context, database, table string, source v2options.IngestionSource, props *v2options.IngestRequestProperties) (*v2options.ExtendedIngestResponse, error) {
	m.ctx, m.database, m.table, m.mapping, m.format = ctx, database, table, props.IngestionMappingRef, props.Format
	m.calls++
	defer source.Close()
	reader, err := source.(v2options.LocalSource).Data()
	if err != nil {
		return nil, err
	}
	body, err := io.ReadAll(reader)
	m.payload = string(body)
	return nil, errors.Join(err, m.err)
}

func (m *mockQueuedV2) Close() error {
	m.closes++
	return nil
}

func TestIngestV2Selection(t *testing.T) {
	for _, tc := range []struct {
		name, method, upload string
		configErr            error
		wantV2               bool
	}{
		{"rest", "Rest", "Storage", nil, true},
		{"lake", "Queued", "Lake", nil, true},
		{"both", "Rest", "Lake", nil, true},
		{"normalized", " rEsT ", " lAkE ", nil, true},
		{"legacy", "Queued", "Storage", nil, false},
		{"missing", "", "", nil, false},
		{"failed", "", "", errors.New("configuration unavailable"), false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			calls := 0
			legacy := &mockingestor{}
			v2 := &mockQueuedV2{}
			state := &ingestV2State{
				configuration: configurationFunc(func(context.Context) (*v2options.ConfigurationResponse, error) {
					calls++
					return &v2options.ConfigurationResponse{
						IngestionSettings: &v2options.IngestionSettings{PreferredIngestionMethod: tc.method},
						ContainerSettings: &v2options.ContainerSettings{PreferredUploadMethod: tc.upload},
					}, tc.configErr
				}),
				newClient: func() (queuedV2Client, error) { return v2, nil },
				now:       time.Now, database: "db", table: "table", mapping: "mapping", logger: zaptest.NewLogger(t),
			}
			producer := &adxDataProducer{ingestor: legacy, v2: state, logger: zaptest.NewLogger(t)}
			payload := []string{`{"value":1}`, `{"value":2}`}
			require.NoError(t, producer.ingestData(t.Context(), payload))
			require.NoError(t, producer.ingestData(t.Context(), payload))
			expectedCalls := 1
			if tc.configErr != nil {
				expectedCalls = 3
			}
			require.Equal(t, expectedCalls, calls)
			if tc.wantV2 {
				require.Equal(t, 2, v2.calls)
				require.Empty(t, legacy.records)
				require.Equal(t, strings.Join(payload, "\n"), v2.payload)
				require.Equal(t, "db", v2.database)
				require.Equal(t, "table", v2.table)
				require.Equal(t, "mapping", v2.mapping)
				require.Equal(t, v2options.FormatJSON, v2.format)
				require.Equal(t, t.Context(), v2.ctx)
			} else {
				require.Zero(t, v2.calls)
				require.Equal(t, payload, legacy.records)
			}
			require.NoError(t, producer.Close(t.Context()))
			require.NoError(t, state.close())
			if tc.wantV2 {
				require.Equal(t, 1, v2.closes)
			}
		})
	}
}

func TestIngestV2SubmissionDoesNotFallBack(t *testing.T) {
	ingestErr := errors.New("submission outcome unknown")
	v2 := &mockQueuedV2{err: ingestErr}
	legacy := &mockingestor{}
	state := &ingestV2State{
		client: v2, lastConfig: &v2options.ConfigurationResponse{
			IngestionSettings: &v2options.IngestionSettings{PreferredIngestionMethod: "Rest"},
		},
		refreshAt: time.Now().Add(time.Hour), now: time.Now, logger: zaptest.NewLogger(t),
	}
	producer := &adxDataProducer{ingestor: legacy, v2: state, logger: zaptest.NewLogger(t)}
	require.ErrorIs(t, producer.ingestData(t.Context(), []string{`{}`}), ingestErr)
	require.Equal(t, 1, v2.calls)
	require.Empty(t, legacy.records)
	require.NoError(t, producer.Close(t.Context()))
}

func TestIngestV2RefreshAndRecovery(t *testing.T) {
	now := time.Now()
	configErr := errors.New("configuration unavailable")
	config := &v2options.ConfigurationResponse{
		IngestionSettings: &v2options.IngestionSettings{PreferredIngestionMethod: "Rest"},
		ContainerSettings: &v2options.ContainerSettings{RefreshInterval: "00:05:00"},
	}
	state := &ingestV2State{
		configuration: configurationFunc(func(context.Context) (*v2options.ConfigurationResponse, error) { return config, configErr }),
		newClient:     func() (queuedV2Client, error) { return &mockQueuedV2{}, nil },
		now:           func() time.Time { return now }, logger: zaptest.NewLogger(t),
	}
	useV2, err := state.selectIngestion(t.Context())
	require.NoError(t, err)
	require.False(t, useV2)
	configErr = nil
	now = now.Add(11 * time.Second)
	useV2, err = state.selectIngestion(t.Context())
	require.NoError(t, err)
	require.True(t, useV2)
	require.Equal(t, now.Add(5*time.Minute), state.refreshAt)
	configErr = errors.New("refresh unavailable")
	now = now.Add(6 * time.Minute)
	useV2, err = state.selectIngestion(t.Context())
	require.NoError(t, err)
	require.True(t, useV2)
	configErr = nil
	config = &v2options.ConfigurationResponse{}
	now = now.Add(2 * time.Minute)
	useV2, err = state.selectIngestion(t.Context())
	require.NoError(t, err)
	require.False(t, useV2)
	require.NoError(t, state.close())
}

func TestIngestV2CanceledPlanningDoesNotUseLegacy(t *testing.T) {
	legacy := &mockingestor{}
	state := &ingestV2State{logger: zaptest.NewLogger(t)}
	producer := &adxDataProducer{ingestor: legacy, v2: state}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	require.ErrorIs(t, producer.ingestData(ctx, []string{`{}`}), context.Canceled)
	require.Empty(t, legacy.records)
}

func TestIngestV2FlagAndManagedStayLegacy(t *testing.T) {
	for _, tc := range []struct {
		enabled bool
		mode    string
	}{
		{false, queuedIngestTest}, {true, managedIngestType},
	} {
		cfg := &Config{ClusterURI: "https://cluster.kusto.windows.net", ManagedIdentityID: "system",
			IngestionType: tc.mode, EnableIngestV2: tc.enabled}
		producer, err := newExporter(cfg, zaptest.NewLogger(t), logsType, "test")
		require.NoError(t, err)
		require.Nil(t, producer.v2)
		require.NoError(t, producer.Close(t.Context()))
	}
}

func TestIngestV2ConfigurationUsesPublishedSDK(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		require.Equal(t, "/v1/rest/ingestion/configuration", r.URL.Path)
		require.Equal(t, "Bearer test-token", r.Header.Get("Authorization"))
		_, _ = io.WriteString(w, `{"containerSettings":{"preferredUploadMethod":"Lake"}}`)
	}))
	defer server.Close()
	client, err := kustov2.NewConfigurationClient(server.URL, kustov2.WithTokenProvider(func(context.Context) (string, error) {
		return "test-token", nil
	}))
	require.NoError(t, err)
	state := &ingestV2State{configuration: client, newClient: func() (queuedV2Client, error) { return &mockQueuedV2{}, nil },
		now: time.Now, logger: zaptest.NewLogger(t)}
	useV2, err := state.selectIngestion(t.Context())
	require.NoError(t, err)
	require.True(t, useV2)
	require.Equal(t, 1, requests)
	require.NoError(t, state.close())
}
