// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package cloudwatchencodingextension // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/cloudwatchencodingextension"

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/extension"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"
	"io"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/cloudwatch"
)

var (
	_ encoding.LogsUnmarshalerExtension    = (*cloudwatchExtension)(nil)
	_ encoding.MetricsUnmarshalerExtension = (*cloudwatchExtension)(nil)
)

type cloudwatchExtension struct {
	config *Config
	logger *zap.Logger
}

func createExtension(_ context.Context, settings extension.Settings, config component.Config) (extension.Extension, error) {
	return &cloudwatchExtension{
		config: config.(*Config),
		logger: settings.Logger,
	}, nil
}

func (c *cloudwatchExtension) Start(_ context.Context, _ component.Host) error {
	return nil
}

func (c *cloudwatchExtension) Shutdown(_ context.Context) error {
	return nil
}

func decompress(buf []byte, encodings []contentEncoding) ([]byte, error) {
	result := buf
	for _, e := range encodings {
		switch e {
		case noEncoding:
			return buf, nil
		case gzipEncoding:
			reader, err := gzip.NewReader(bytes.NewReader(result))
			if err != nil {
				return nil, fmt.Errorf("failed to create gzip reader: %w", err)
			}
			defer reader.Close()
			if result, err = io.ReadAll(reader); err != nil {
				return nil, err
			}
		case base64Encoding:
			var err error
			if result, err = base64.StdEncoding.DecodeString(string(result)); err != nil {
				return nil, err
			}
		default:
			// not possible, prevented by config.Validate
			return nil, errors.New("invalid content encoding")
		}
	}
	return result, nil
}

func (c *cloudwatchExtension) UnmarshalLogs(buf []byte) (plog.Logs, error) {
	data, err := decompress(buf, c.config.Encoding)
	if err != nil {
		return plog.Logs{}, err
	}
	return cloudwatch.UnmarshalLogs(data)
}

func (c *cloudwatchExtension) UnmarshalMetrics(buf []byte) (pmetric.Metrics, error) {
	data, err := decompress(buf, c.config.Encoding)
	if err != nil {
		return pmetric.Metrics{}, err
	}
	return cloudwatch.UnmarshalMetrics(c.config.Format, data)
}
