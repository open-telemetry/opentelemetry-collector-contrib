// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sematextexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/sematextexporter"

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"sort"
	"sync"
	"time"

	"github.com/influxdata/influxdb-observability/common"
	"github.com/influxdata/influxdb-observability/otel2influx"
	"github.com/influxdata/line-protocol/v2/lineprotocol"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/consumer/consumererror"
)

var _ otel2influx.InfluxWriter = (*sematextHTTPWriter)(nil)

type sematextHTTPWriter struct {
	encoderPool sync.Pool
	httpClient  *http.Client

	httpClientSettings confighttp.ClientConfig
	telemetrySettings  component.TelemetrySettings
	writeURL           string
	payloadMaxLines    int
	payloadMaxBytes    int
	hostname           string
	token              string
	logger             common.Logger
}

func newSematextHTTPWriter(logger common.Logger, config *Config, telemetrySettings component.TelemetrySettings) (*sematextHTTPWriter, error) {
	writeURL, err := composeWriteURL(config)
	if err != nil {
		return nil, err
	}
	hostname, err := os.Hostname()
	if err != nil {
		return nil, fmt.Errorf("could not detect hostname: %w", err)
	}

	return &sematextHTTPWriter{
		encoderPool: sync.Pool{
			New: func() any {
				e := new(lineprotocol.Encoder)
				e.SetLax(false)
				e.SetPrecision(lineprotocol.Nanosecond)
				return e
			},
		},
		telemetrySettings: telemetrySettings,
		writeURL:          writeURL,
		payloadMaxLines:   config.PayloadMaxLines,
		payloadMaxBytes:   config.PayloadMaxBytes,
		logger:            logger,
		hostname:          hostname,
		token:             config.MetricsConfig.AppToken,
	}, nil
}

func composeWriteURL(config *Config) (string, error) {
	writeURL, err := url.Parse(config.MetricsEndpoint)
	if err != nil {
		return "", err
	}
	if writeURL.Path == "" || writeURL.Path == "/" {
		writeURL, err = writeURL.Parse("write?db=metrics")
		if err != nil {
			return "", err
		}
	}
	queryValues := writeURL.Query()

	writeURL.RawQuery = queryValues.Encode()

	return writeURL.String(), nil
}

// Start implements component.StartFunc
func (w *sematextHTTPWriter) Start(ctx context.Context, host component.Host) error {
	httpClient, err := w.httpClientSettings.ToClient(ctx, host, w.telemetrySettings)
	if err != nil {
		return err
	}
	w.httpClient = httpClient
	return nil
}

func (w *sematextHTTPWriter) Shutdown(_ context.Context) error {
	if w.httpClient != nil {
		w.httpClient.CloseIdleConnections() // Closes all idle connections for the HTTP client
	}
	w.logger.Debug("HTTP client connections closed successfully for Sematext HTTP Writer")
	return nil
}

func (w *sematextHTTPWriter) NewBatch() otel2influx.InfluxWriterBatch {
	return newSematextHTTPWriterBatch(w)
}

var _ otel2influx.InfluxWriterBatch = (*sematextHTTPWriterBatch)(nil)

type sematextHTTPWriterBatch struct {
	*sematextHTTPWriter
	encoder      *lineprotocol.Encoder
	payloadLines int
}

func newSematextHTTPWriterBatch(w *sematextHTTPWriter) *sematextHTTPWriterBatch {
	return &sematextHTTPWriterBatch{
		sematextHTTPWriter: w,
	}
}

// EnqueuePoint emits a set of line protocol attributes (metrics, tags, fields, timestamp)
// to the internal line protocol buffer.
// If the buffer is full, it will be flushed by calling WriteBatch.
func (b *sematextHTTPWriterBatch) EnqueuePoint(ctx context.Context, measurement string, tags map[string]string, fields map[string]any, ts time.Time, _ common.InfluxMetricValueType) error {
	if b.encoder == nil {
		b.encoder = b.encoderPool.Get().(*lineprotocol.Encoder)
	}
	// Add token and os.host tags
	if tags == nil {
		tags = make(map[string]string)
	}

	b.encoder.StartLine(measurement)
	for _, tag := range b.optimizeTags(tags) {
		b.encoder.AddTag(tag.k, tag.v)
	}
	for k, v := range b.convertFields(fields) {
		b.encoder.AddField(k, v)
	}
	b.encoder.EndLine(ts)

	if err := b.encoder.Err(); err != nil {
		b.encoder.Reset()
		b.encoder.ClearErr()
		b.encoderPool.Put(b.encoder)
		b.encoder = nil
		return consumererror.NewPermanent(fmt.Errorf("failed to encode point: %w", err))
	}

	b.payloadLines++
	if b.payloadLines >= b.payloadMaxLines || len(b.encoder.Bytes()) >= b.payloadMaxBytes {
		if err := b.WriteBatch(ctx); err != nil {
			return err
		}
	}

	return nil
}

// WriteBatch sends the internal line protocol buffer to Sematext.
func (b *sematextHTTPWriterBatch) WriteBatch(ctx context.Context) error {
	if b.encoder == nil {
		return nil
	}

	defer func() {
		b.encoder.Reset()
		b.encoder.ClearErr()
		b.encoderPool.Put(b.encoder)
		b.encoder = nil
		b.payloadLines = 0
	}()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, b.writeURL, bytes.NewReader(b.encoder.Bytes()))
	if err != nil {
		return consumererror.NewPermanent(err)
	}
	res, err := b.httpClient.Do(req)
	if err != nil {
		return err
	}
	body, err := io.ReadAll(res.Body)
	if err != nil {
		return err
	}

	if err = res.Body.Close(); err != nil {
		return err
	}

	switch res.StatusCode {
	case http.StatusOK, http.StatusNoContent:
		break
	case http.StatusInternalServerError:
		return fmt.Errorf("line protocol write returned %q %q", res.Status, string(body))
	default:
		return consumererror.NewPermanent(fmt.Errorf("line protocol write returned %q %q", res.Status, string(body)))
	}

	return nil
}

type tag struct {
	k, v string
}

// optimizeTags filters for allowed tags and sorts them
func (b *sematextHTTPWriterBatch) optimizeTags(m map[string]string) []tag {
	// Define allowed tags set
	allowedTags := map[string]struct{}{
		"service.name":              {},
		"service.instance.id":       {},
		"process.pid":               {},
		"os.type":                   {},
		"os.host":                   {},
		"http.response.status_code": {},
		"network.protocol.version":  {},
		"jvm.memory.type":           {},
		"http.request.method":       {},
		"jvm.gc.name":               {},
		"token":                     {},
	}

	// Create filtered map with only allowed tags
	filteredMap := make(map[string]string)

	// Always ensure token and os.host are present
	filteredMap["token"] = b.token
	filteredMap["os.host"] = b.hostname

	// Only include allowed tags
	for k, v := range m {
		// Skip empty keys/values
		if k == "" || v == "" {
			b.logger.Debug("skipping empty tag", "key", k, "value", v)
			continue
		}

		// Only include tags from our allowed list
		if _, isAllowed := allowedTags[k]; isAllowed {
			filteredMap[k] = v
		} else {
			b.logger.Debug("dropping non-allowed tag", "key", k)
		}
	}

	// Convert to sorted slice
	tags := make([]tag, 0, len(filteredMap))
	for k, v := range filteredMap {
		tags = append(tags, tag{k, v})
	}

	// Sort tags by key
	sort.Slice(tags, func(i, j int) bool {
		return tags[i].k < tags[j].k
	})

	return tags
}

func (b *sematextHTTPWriterBatch) convertFields(m map[string]any) (fields map[string]lineprotocol.Value) {
	fields = make(map[string]lineprotocol.Value, len(m))
	for k, v := range m {
		if k == "" {
			b.logger.Debug("empty field key")
		} else if lpv, ok := lineprotocol.NewValue(v); !ok {
			b.logger.Debug("invalid field value", "key", k, "value", v)
		} else {
			fields[k] = lpv
		}
	}
	return
}
