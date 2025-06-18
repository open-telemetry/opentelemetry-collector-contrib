// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awslogsencodingextension // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/awslogsencodingextension"

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"sync"

	"github.com/klauspost/compress/gzip"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/extension"
	"go.opentelemetry.io/collector/pdata/plog"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding"
	awsunmarshaler "github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/awslogsencodingextension/internal/unmarshaler"
	s3accesslog "github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/awslogsencodingextension/internal/unmarshaler/s3-access-log"
	subscriptionfilter "github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/awslogsencodingextension/internal/unmarshaler/subscription-filter"
	vpcflowlog "github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/awslogsencodingextension/internal/unmarshaler/vpc-flow-log"
	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/awslogsencodingextension/internal/unmarshaler/waf"
)

var _ encoding.LogsUnmarshalerExtension = (*encodingExtension)(nil)

type encodingExtension struct {
	unmarshaler awsunmarshaler.AWSUnmarshaler
	format      string
	gzipPool    sync.Pool

	// if format is VPC, then content can be in parquet or
	// gzip encoding
	vpcFormat string
}

func newExtension(cfg *Config, settings extension.Settings) (*encodingExtension, error) {
	switch cfg.Format {
	case formatCloudWatchLogsSubscriptionFilter:
		return &encodingExtension{
			unmarshaler: subscriptionfilter.NewSubscriptionFilterUnmarshaler(settings.BuildInfo),
			format:      formatCloudWatchLogsSubscriptionFilter,
		}, nil
	case formatVPCFlowLog:
		unmarshaler, err := vpcflowlog.NewVPCFlowLogUnmarshaler(
			cfg.VPCFlowLogConfig.FileFormat,
			settings.BuildInfo,
			settings.Logger,
		)
		return &encodingExtension{
			unmarshaler: unmarshaler,
			vpcFormat:   cfg.VPCFlowLogConfig.FileFormat,
			format:      formatVPCFlowLog,
		}, err
	case formatS3AccessLog:
		return &encodingExtension{
			unmarshaler: s3accesslog.NewS3AccessLogUnmarshaler(settings.BuildInfo),
			format:      formatS3AccessLog,
		}, nil
	case formatWAFLog:
		return &encodingExtension{
			unmarshaler: waf.NewWAFLogUnmarshaler(settings.BuildInfo),
			format:      formatWAFLog,
		}, nil
	default:
		// Format will have been validated by Config.Validate,
		// so we'll only get here if we haven't handled a valid
		// format.
		return nil, fmt.Errorf("unimplemented format %q", cfg.Format)
	}
}

func (*encodingExtension) Start(_ context.Context, _ component.Host) error {
	return nil
}

func (*encodingExtension) Shutdown(_ context.Context) error {
	return nil
}

func (e *encodingExtension) getGzipReader(buf []byte) (io.Reader, error) {
	var errGzipReader error
	gzipReader, ok := e.gzipPool.Get().(*gzip.Reader)
	if !ok {
		gzipReader, errGzipReader = gzip.NewReader(bytes.NewReader(buf))
	} else {
		errGzipReader = gzipReader.Reset(bytes.NewReader(buf))
	}
	if errGzipReader != nil {
		if gzipReader != nil {
			e.gzipPool.Put(gzipReader)
		}
		return nil, fmt.Errorf("failed to decompress content: %w", errGzipReader)
	}
	defer func() {
		_ = gzipReader.Close()
		e.gzipPool.Put(gzipReader)
	}()
	return gzipReader, nil
}

func (e *encodingExtension) getReaderFromFormat(buf []byte) (io.Reader, error) {
	switch e.format {
	case formatWAFLog, formatCloudWatchLogsSubscriptionFilter:
		return e.getGzipReader(buf)
	case formatS3AccessLog:
		return bytes.NewReader(buf), nil
	case formatVPCFlowLog:
		switch e.vpcFormat {
		case fileFormatParquet:
			return nil, fmt.Errorf("%q still needs to be implemented", e.vpcFormat)
		case fileFormatPlainText:
			return e.getGzipReader(buf)
		default:
			// should not be possible
			return nil, fmt.Errorf(
				"unsupported file fileFormat %q for VPC flow log, expected one of %q",
				e.vpcFormat,
				supportedVPCFlowLogFileFormat,
			)
		}
	default:
		// should not be possible
		return nil, fmt.Errorf("unimplemented: format %q has no reader", e.format)
	}
}

func (e *encodingExtension) UnmarshalLogs(buf []byte) (plog.Logs, error) {
	reader, err := e.getReaderFromFormat(buf)
	if err != nil {
		return plog.Logs{}, fmt.Errorf("failed to get reader for %q logs: %w", e.format, err)
	}

	logs, err := e.unmarshaler.UnmarshalAWSLogs(reader)
	if err != nil {
		return plog.Logs{}, fmt.Errorf("failed to unmarshal logs as %q format: %w", e.format, err)
	}
	return logs, nil
}
