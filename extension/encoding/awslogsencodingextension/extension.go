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
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding"
	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/awslogsencodingextension/internal/constants"
	awsunmarshaler "github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/awslogsencodingextension/internal/unmarshaler"
	cloudtraillog "github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/awslogsencodingextension/internal/unmarshaler/cloudtraillog"
	elbaccesslogs "github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/awslogsencodingextension/internal/unmarshaler/elb-access-log"
	s3accesslog "github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/awslogsencodingextension/internal/unmarshaler/s3-access-log"
	subscriptionfilter "github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/awslogsencodingextension/internal/unmarshaler/subscription-filter"
	vpcflowlog "github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/awslogsencodingextension/internal/unmarshaler/vpc-flow-log"
	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/awslogsencodingextension/internal/unmarshaler/waf"
)

const (
	gzipEncoding    = "gzip"
	bytesEncoding   = "bytes"
	parquetEncoding = "parquet"
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
	case constants.FormatCloudWatchLogsSubscriptionFilter, constants.FormatCloudWatchLogsSubscriptionFilterV1:
		if cfg.Format == constants.FormatCloudWatchLogsSubscriptionFilterV1 {
			settings.Logger.Warn("using old format value. This format will be removed in version 0.138.0.",
				zap.String("old_format", string(constants.FormatCloudWatchLogsSubscriptionFilterV1)),
				zap.String("new_format", string(constants.FormatCloudWatchLogsSubscriptionFilter)),
			)
		}
		return &encodingExtension{
			unmarshaler: subscriptionfilter.NewSubscriptionFilterUnmarshaler(settings.BuildInfo),
			format:      constants.FormatCloudWatchLogsSubscriptionFilter,
		}, nil
	case constants.FormatVPCFlowLog, constants.FormatVPCFlowLogV1:
		var fileFormat string
		if cfg.Format == constants.FormatVPCFlowLogV1 {
			settings.Logger.Warn("using old format value. This format will be removed in version 0.138.0.",
				zap.String("old_format", string(constants.FormatVPCFlowLogV1)),
				zap.String("new_format", string(constants.FormatVPCFlowLog)),
			)
			fileFormat = cfg.VPCFlowLogConfigV1.FileFormat
		} else {
			fileFormat = cfg.VPCFlowLogConfig.FileFormat
		}
		unmarshaler, err := vpcflowlog.NewVPCFlowLogUnmarshaler(
			fileFormat,
			settings.BuildInfo,
			settings.Logger,
		)
		return &encodingExtension{
			unmarshaler: unmarshaler,
			vpcFormat:   fileFormat,
			format:      constants.FormatVPCFlowLog,
		}, err
	case constants.FormatS3AccessLog, constants.FormatS3AccessLogV1:
		if cfg.Format == constants.FormatS3AccessLogV1 {
			settings.Logger.Warn("using old format value. This format will be removed in version 0.138.0.",
				zap.String("old_format", string(constants.FormatS3AccessLogV1)),
				zap.String("new_format", string(constants.FormatS3AccessLog)),
			)
		}
		return &encodingExtension{
			unmarshaler: s3accesslog.NewS3AccessLogUnmarshaler(settings.BuildInfo),
			format:      constants.FormatS3AccessLog,
		}, nil
	case constants.FormatWAFLog, constants.FormatWAFLogV1:
		if cfg.Format == constants.FormatWAFLogV1 {
			settings.Logger.Warn("using old format value. This format will be removed in version 0.138.0.",
				zap.String("old_format", string(constants.FormatWAFLogV1)),
				zap.String("new_format", string(constants.FormatWAFLog)),
			)
		}
		return &encodingExtension{
			unmarshaler: waf.NewWAFLogUnmarshaler(settings.BuildInfo),
			format:      constants.FormatWAFLog,
		}, nil
	case constants.FormatCloudTrailLog, constants.FormatCloudTrailLogV1:
		if cfg.Format == constants.FormatCloudTrailLogV1 {
			settings.Logger.Warn("using old format value. This format will be removed in version 0.138.0.",
				zap.String("old_format", string(constants.FormatCloudTrailLogV1)),
				zap.String("new_format", string(constants.FormatCloudTrailLog)),
			)
		}
		return &encodingExtension{
			unmarshaler: cloudtraillog.NewCloudTrailLogUnmarshaler(settings.BuildInfo),
			format:      constants.FormatCloudTrailLog,
		}, nil
	case constants.FormatELBAccessLog, constants.FormatELBAccessLogV1:
		if cfg.Format == constants.FormatELBAccessLogV1 {
			settings.Logger.Warn("using old format value. This format will be removed in version 0.138.0.",
				zap.String("old_format", string(constants.FormatELBAccessLogV1)),
				zap.String("new_format", string(constants.FormatELBAccessLog)),
			)
		}
		return &encodingExtension{
			unmarshaler: elbaccesslogs.NewELBAccessLogUnmarshaler(
				settings.BuildInfo,
				settings.Logger,
			),
			format: constants.FormatELBAccessLog,
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
	var err error
	gzipReader, ok := e.gzipPool.Get().(*gzip.Reader)
	if !ok {
		gzipReader, err = gzip.NewReader(bytes.NewReader(buf))
	} else {
		err = gzipReader.Reset(bytes.NewBuffer(buf))
	}

	if err != nil {
		if gzipReader != nil {
			e.gzipPool.Put(gzipReader)
		}
		return nil, fmt.Errorf("failed to decompress content: %w", err)
	}

	return gzipReader, nil
}

// isGzipData checks if the buffer contains gzip-compressed data by examining magic bytes
func isGzipData(buf []byte) bool {
	return len(buf) > 2 && buf[0] == 0x1f && buf[1] == 0x8b
}

// getReaderForData returns the appropriate reader and encoding type based on data format
func (e *encodingExtension) getReaderForData(buf []byte) (string, io.Reader, error) {
	if isGzipData(buf) {
		reader, err := e.getGzipReader(buf)
		return gzipEncoding, reader, err
	}
	return bytesEncoding, bytes.NewReader(buf), nil
}

func (e *encodingExtension) getReaderFromFormat(buf []byte) (string, io.Reader, error) {
	switch e.format {
	case constants.FormatWAFLog, constants.FormatCloudWatchLogsSubscriptionFilter, constants.FormatCloudTrailLog, constants.FormatELBAccessLog:
		return e.getReaderForData(buf)

	case constants.FormatS3AccessLog:
		return bytesEncoding, bytes.NewReader(buf), nil

	case constants.FormatVPCFlowLog:
		switch e.vpcFormat {
		case constants.FileFormatParquet:
			return parquetEncoding, nil, fmt.Errorf("%q still needs to be implemented", e.vpcFormat)
		case constants.FileFormatPlainText:
			return e.getReaderForData(buf)
		default:
			// should not be possible
			return "", nil, fmt.Errorf(
				"unsupported file fileFormat %q for VPC flow log, expected one of %q",
				e.vpcFormat,
				supportedVPCFlowLogFileFormat,
			)
		}

	default:
		// should not be possible
		return "", nil, fmt.Errorf("unimplemented: format %q has no reader", e.format)
	}
}

func (e *encodingExtension) UnmarshalLogs(buf []byte) (plog.Logs, error) {
	encodingReader, reader, err := e.getReaderFromFormat(buf)
	if err != nil {
		return plog.Logs{}, fmt.Errorf("failed to get reader for %q logs: %w", e.format, err)
	}

	defer func() {
		if encodingReader == gzipEncoding {
			r := reader.(*gzip.Reader)
			_ = r.Close()
			e.gzipPool.Put(r)
		}
	}()

	logs, err := e.unmarshaler.UnmarshalAWSLogs(reader)
	if err != nil {
		return plog.Logs{}, fmt.Errorf("failed to unmarshal logs as %q format: %w", e.format, err)
	}

	return logs, nil
}
