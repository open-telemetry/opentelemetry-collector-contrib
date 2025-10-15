package prometheusremotewritereceiver

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gogo/protobuf/proto"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/prometheusremotewritereceiver/internal/metadata"
	promconfig "github.com/prometheus/prometheus/config"
	writev2 "github.com/prometheus/prometheus/prompb/io/prometheus/write/v2"
	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/exporter/debugexporter"
	"go.opentelemetry.io/collector/exporter/exportertest"
	"go.opentelemetry.io/collector/receiver/receiverhelper"
	"go.opentelemetry.io/collector/receiver/receivertest"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest"
	"go.uber.org/zap/zaptest/observer"
)

func TestReceiverWithDebugExporter(t *testing.T) {
	// Create debug exporter
	exporterCfg := debugexporter.NewFactory().CreateDefaultConfig()
	core, obslogs := observer.New(zapcore.InfoLevel)
	set := exportertest.NewNopSettings(component.MustNewType("debug"))
	set.Logger = zap.New(zapcore.NewTee(core, zaptest.NewLogger(t).Core()))
	exporter, err := debugexporter.NewFactory().CreateMetrics(
		context.Background(),
		set,
		exporterCfg,
	)
	if err != nil {
		t.Fatalf("failed to create debug exporter: %v", err)
	}

	// Setup receiver as in receiver_test.go
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()
	prwReceiver, err := factory.CreateMetrics(context.Background(), receivertest.NewNopSettings(metadata.Type), cfg, exporter)
	if err != nil {
		t.Fatalf("failed to create receiver: %v", err)
	}
	obsrecv, err := receiverhelper.NewObsReport(receiverhelper.ObsReportSettings{
		ReceiverID:             component.MustNewID("test"),
		Transport:              "http",
		ReceiverCreateSettings: receivertest.NewNopSettings(metadata.Type),
	})
	if err != nil {
		t.Fatalf("failed to create receiver: %v", err)
	}

	receiver := prwReceiver.(*prometheusRemoteWriteReceiver)
	receiver.nextConsumer = exporter
	receiver.obsrecv = obsrecv

	ts := httptest.NewServer(http.HandlerFunc(receiver.handlePRW))
	defer ts.Close()
	{
		// Prepare a sample writev2.Request
		req := &writev2.Request{
			Symbols: []string{"", "__name__", "test_metric", "job", "test_job", "instance", "test_instance"},
			Timeseries: []writev2.TimeSeries{
				{
					Metadata:         writev2.Metadata{Type: writev2.Metadata_METRIC_TYPE_UNSPECIFIED},
					LabelsRefs:       []uint32{1, 2, 3, 4, 5, 6},
					Samples:          []writev2.Sample{{Value: 1, Timestamp: 1}},
					CreatedTimestamp: 1,
				},
			},
		}
		pBuf := proto.NewBuffer(nil)
		// we don't need to compress the body to use the snappy compression in the unit test
		// because the encoder is just initialized when we initialize the http server.
		// so we can just use the uncompressed body.
		err = pBuf.Marshal(req)
		assert.NoError(t, err)

		resp, err := http.Post(
			ts.URL,
			fmt.Sprintf("application/x-protobuf;proto=%s", promconfig.RemoteWriteProtoMsgV2),
			bytes.NewBuffer(pBuf.Bytes()),
		)
		assert.NoError(t, err)
		defer resp.Body.Close()
		body, err := io.ReadAll(resp.Body)
		assert.NoError(t, err)
		assert.Equal(t, http.StatusNoContent, resp.StatusCode, string(body))

	}

	{
		// Prepare a sample writev2.Request
		req := &writev2.Request{
			Symbols: []string{"", "__name__", "test_metric", "job", "test_job", "instance", "test_instance"},
			Timeseries: []writev2.TimeSeries{
				{
					Metadata:         writev2.Metadata{Type: writev2.Metadata_METRIC_TYPE_UNSPECIFIED},
					LabelsRefs:       []uint32{1, 2, 3, 4, 5, 6},
					Samples:          []writev2.Sample{{Value: 2, Timestamp: 2}},
					CreatedTimestamp: 1,
				},
			},
		}
		pBuf := proto.NewBuffer(nil)
		// we don't need to compress the body to use the snappy compression in the unit test
		// because the encoder is just initialized when we initialize the http server.
		// so we can just use the uncompressed body.
		err = pBuf.Marshal(req)
		assert.NoError(t, err)

		resp, err := http.Post(
			ts.URL,
			fmt.Sprintf("application/x-protobuf;proto=%s", promconfig.RemoteWriteProtoMsgV2),
			bytes.NewBuffer(pBuf.Bytes()),
		)
		assert.NoError(t, err)
		defer resp.Body.Close()
		body, err := io.ReadAll(resp.Body)
		assert.NoError(t, err)
		assert.Equal(t, http.StatusNoContent, resp.StatusCode, string(body))

	}
	fmt.Println(obslogs.All()[0].Entry.Message)
}
