// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package influxdbexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/influxdbexporter"

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"sync"
	"time"

	"github.com/influxdata/influxdb-observability/common"
	"github.com/influxdata/influxdb-observability/otel2influx"
	"github.com/influxdata/line-protocol/v2/lineprotocol"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/config/configopaque"
	"go.opentelemetry.io/collector/consumer/consumererror"
)

var _ otel2influx.InfluxWriter = (*influxHTTPWriter)(nil)

type influxHTTPWriter struct {
	encoderPool sync.Pool
	httpClient  *http.Client

	httpClientSettings confighttp.HTTPClientSettings
	telemetrySettings  component.TelemetrySettings
	writeURL           string
	payloadMaxLines    int
	payloadMaxBytes    int

	logger common.Logger
}

func newInfluxHTTPWriter(logger common.Logger, config *Config, telemetrySettings component.TelemetrySettings) (*influxHTTPWriter, error) {
	writeURL, err := composeWriteURL(config)
	if err != nil {
		return nil, err
	}

	return &influxHTTPWriter{
		encoderPool: sync.Pool{
			New: func() any {
				e := new(lineprotocol.Encoder)
				e.SetLax(false)
				e.SetPrecision(lineprotocol.Nanosecond)
				return e
			},
		},
		httpClientSettings: config.HTTPClientSettings,
		telemetrySettings:  telemetrySettings,
		writeURL:           writeURL,
		payloadMaxLines:    config.PayloadMaxLines,
		payloadMaxBytes:    config.PayloadMaxBytes,
		logger:             logger,
	}, nil
}

func composeWriteURL(config *Config) (string, error) {
	writeURL, err := url.Parse(config.HTTPClientSettings.Endpoint)
	if err != nil {
		return "", err
	}
	if writeURL.Path == "" || writeURL.Path == "/" {
		if config.V1Compatibility.Enabled {
			writeURL, err = writeURL.Parse("write")
			if err != nil {
				return "", err
			}
		} else {
			writeURL, err = writeURL.Parse("api/v2/write")
			if err != nil {
				return "", err
			}
		}
	}
	queryValues := writeURL.Query()
	queryValues.Set("precision", "ns")

	if config.V1Compatibility.Enabled {
		queryValues.Set("db", config.V1Compatibility.DB)

		if config.V1Compatibility.Username != "" && config.V1Compatibility.Password != "" {
			basicAuth := base64.StdEncoding.EncodeToString(
				[]byte(config.V1Compatibility.Username + ":" + string(config.V1Compatibility.Password)))
			if config.HTTPClientSettings.Headers == nil {
				config.HTTPClientSettings.Headers = make(map[string]configopaque.String, 1)
			}
			config.HTTPClientSettings.Headers["Authorization"] = configopaque.String("Basic " + basicAuth)
		}
	} else {
		queryValues.Set("org", config.Org)
		queryValues.Set("bucket", config.Bucket)

		if config.Token != "" {
			if config.HTTPClientSettings.Headers == nil {
				config.HTTPClientSettings.Headers = make(map[string]configopaque.String, 1)
			}
			config.HTTPClientSettings.Headers["Authorization"] = "Token " + config.Token
		}
	}

	writeURL.RawQuery = queryValues.Encode()

	return writeURL.String(), nil
}

// Start implements component.StartFunc
func (w *influxHTTPWriter) Start(_ context.Context, host component.Host) error {
	httpClient, err := w.httpClientSettings.ToClient(host, w.telemetrySettings)
	if err != nil {
		return err
	}
	w.httpClient = httpClient
	return nil
}

func (w *influxHTTPWriter) NewBatch() otel2influx.InfluxWriterBatch {
	return newInfluxHTTPWriterBatch(w)
}

var _ otel2influx.InfluxWriterBatch = (*influxHTTPWriterBatch)(nil)

type influxHTTPWriterBatch struct {
	*influxHTTPWriter
	encoder      *lineprotocol.Encoder
	payloadLines int
}

func newInfluxHTTPWriterBatch(w *influxHTTPWriter) *influxHTTPWriterBatch {
	return &influxHTTPWriterBatch{
		influxHTTPWriter: w,
	}
}

// EnqueuePoint emits a set of line protocol attributes (metrics, tags, fields, timestamp)
// to the internal line protocol buffer.
// If the buffer is full, it will be flushed by calling WriteBatch.
func (b *influxHTTPWriterBatch) EnqueuePoint(ctx context.Context, measurement string, tags map[string]string, fields map[string]any, ts time.Time, _ common.InfluxMetricValueType) error {
	if b.encoder == nil {
		b.encoder = b.encoderPool.Get().(*lineprotocol.Encoder)
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

// WriteBatch sends the internal line protocol buffer to InfluxDB.
func (b *influxHTTPWriterBatch) WriteBatch(ctx context.Context) error {
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
	switch res.StatusCode / 100 {
	case 2: // Success
		break
	case 5: // Retryable error
		return fmt.Errorf("line protocol write returned %q %q", res.Status, string(body))
	default: // Terminal error
		return consumererror.NewPermanent(fmt.Errorf("line protocol write returned %q %q", res.Status, string(body)))
	}

	return nil
}

type tag struct {
	k, v string
}

// optimizeTags sorts tags by key and removes tags with empty keys or values
func (b *influxHTTPWriterBatch) optimizeTags(m map[string]string) []tag {
	tags := make([]tag, 0, len(m))
	for k, v := range m {
		switch {
		case k == "":
			b.logger.Debug("empty tag key")
		case v == "":
			b.logger.Debug("empty tag value", "key", k)
		default:
			tags = append(tags, tag{k, v})
		}
	}
	sort.Slice(tags, func(i, j int) bool {
		return tags[i].k < tags[j].k
	})
	return tags
}

func (b *influxHTTPWriterBatch) convertFields(m map[string]any) (fields map[string]lineprotocol.Value) {
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
