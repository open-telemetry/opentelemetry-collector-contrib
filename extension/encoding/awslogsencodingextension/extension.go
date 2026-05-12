// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awslogsencodingextension // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/awslogsencodingextension"

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"sync"

	"github.com/klauspost/compress/gzip"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/extension"
	"go.opentelemetry.io/collector/featuregate"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding"
	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/awslogsencodingextension/internal/constants"
	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/awslogsencodingextension/internal/metadata"
	awsunmarshaler "github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/awslogsencodingextension/internal/unmarshaler"
	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/awslogsencodingextension/internal/unmarshaler/cloudtraillog"
	elbaccesslogs "github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/awslogsencodingextension/internal/unmarshaler/elb-access-log"
	networkfirewall "github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/awslogsencodingextension/internal/unmarshaler/network-firewall-log"
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

var (
	_ encoding.LogsUnmarshalerExtension = (*encodingExtension)(nil)
	_ encoding.LogsDecoderExtension     = (*encodingExtension)(nil)
)

var (
	vpcFlowStartISO8601FormatFeatureGate    *featuregate.Gate
	cloudTrailUserIdentityPrefixFeatureGate *featuregate.Gate
)

func init() {
	vpcFlowStartISO8601FormatFeatureGate = featuregate.GlobalRegistry().MustRegister(
		constants.VPCFlowStartISO8601FormatID,
		featuregate.StageAlpha,
		featuregate.WithRegisterDescription("When enabled, aws.vpc.flow.start field will be formatted as ISO-8601 string instead of seconds since epoch integer."),
		featuregate.WithRegisterReferenceURL("https://github.com/open-telemetry/opentelemetry-collector-contrib/pull/43390"),
	)

	cloudTrailUserIdentityPrefixFeatureGate = featuregate.GlobalRegistry().MustRegister(
		constants.CloudTrailEnableUserIdentityPrefixID,
		featuregate.StageAlpha,
		featuregate.WithRegisterDescription("When enabled, CloudTrail log userIdentity attributes will use 'aws.user_identity' prefix. This helps to preserve the attribute origin."),
		featuregate.WithRegisterReferenceURL("https://github.com/open-telemetry/opentelemetry-collector-contrib/pull/45459"))
}

type encodingExtension struct {
	cfg *Config

	unmarshaler             awsunmarshaler.AWSUnmarshaler
	format                  string
	gzipPool                sync.Pool
	logger                  *zap.Logger
	warnGzipDeprecationOnce sync.Once
}

func newExtension(cfg *Config, settings extension.Settings) (*encodingExtension, error) {
	switch cfg.Format {
	case constants.FormatCloudWatchLogsSubscriptionFilter:
		return &encodingExtension{
			unmarshaler: subscriptionfilter.NewSubscriptionFilterUnmarshaler(settings.BuildInfo),
			format:      constants.FormatCloudWatchLogsSubscriptionFilter,
			logger:      settings.Logger,
		}, nil
	case constants.FormatVPCFlowLog:
		unmarshaler, err := vpcflowlog.NewVPCFlowLogUnmarshaler(
			cfg.VPCFlowLogConfig,
			settings.BuildInfo,
			settings.Logger,
			vpcFlowStartISO8601FormatFeatureGate.IsEnabled(),
		)
		return &encodingExtension{
			cfg:         cfg,
			unmarshaler: unmarshaler,
			format:      constants.FormatVPCFlowLog,
			logger:      settings.Logger,
		}, err
	case constants.FormatS3AccessLog:
		return &encodingExtension{
			unmarshaler: s3accesslog.NewS3AccessLogUnmarshaler(settings.BuildInfo),
			format:      constants.FormatS3AccessLog,
			logger:      settings.Logger,
		}, nil
	case constants.FormatWAFLog:
		return &encodingExtension{
			unmarshaler: waf.NewWAFLogUnmarshaler(settings.BuildInfo),
			format:      constants.FormatWAFLog,
			logger:      settings.Logger,
		}, nil
	case constants.FormatCloudTrailLog:
		if metadata.ExtensionEncodingAwslogsencodingDontEmitV0RPCConventionsFeatureGate.IsEnabled() &&
			!metadata.ExtensionEncodingAwslogsencodingEmitV1RPCConventionsFeatureGate.IsEnabled() {
			return nil, errors.New("extension.encoding.awslogsencoding.DontEmitV0RPCConventions requires extension.encoding.awslogsencoding.EmitV1RPCConventions to be enabled")
		}
		return &encodingExtension{
			unmarshaler: cloudtraillog.NewCloudTrailLogUnmarshaler(
				settings.BuildInfo,
				cloudTrailUserIdentityPrefixFeatureGate.IsEnabled()),
			format: constants.FormatCloudTrailLog,
			logger: settings.Logger,
		}, nil
	case constants.FormatELBAccessLog:
		return &encodingExtension{
			unmarshaler: elbaccesslogs.NewELBAccessLogUnmarshaler(
				settings.BuildInfo,
				settings.Logger,
			),
			format: constants.FormatELBAccessLog,
			logger: settings.Logger,
		}, nil
	case constants.FormatNetworkFirewallLog:
		return &encodingExtension{
			unmarshaler: networkfirewall.NewNetworkFirewallLogUnmarshaler(settings.BuildInfo),
			format:      constants.FormatNetworkFirewallLog,
			logger:      settings.Logger,
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

// NewLogsDecoder returns a LogsDecoder if the underlying unmarshaler supports streaming.
// Caller must perform any decompression before passing the reader to the decoder.
// Implementations must utilize derived buffered readers as is.
func (e *encodingExtension) NewLogsDecoder(reader io.Reader, options ...encoding.DecoderOption) (encoding.LogsDecoder, error) {
	if u, ok := e.unmarshaler.(awsunmarshaler.StreamingLogsUnmarshaler); ok {
		return u.NewLogsDecoder(reader, options...)
	}

	return nil, fmt.Errorf("streaming not supported for format %q", e.format)
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
		e.warnGzipDeprecationOnce.Do(func() {
			if e.logger != nil {
				e.logger.Warn("transparent gzip decompression in aws_logs_encoding is deprecated and will be removed in a future release; callers should decompress payloads before invoking the extension")
			}
		})
		reader, err := e.getGzipReader(buf)
		return gzipEncoding, reader, err
	}
	return bytesEncoding, bytes.NewReader(buf), nil
}

func (e *encodingExtension) getReaderFromFormat(buf []byte) (string, io.Reader, error) {
	switch e.format {
	case constants.FormatWAFLog, constants.FormatCloudWatchLogsSubscriptionFilter, constants.FormatCloudTrailLog, constants.FormatELBAccessLog, constants.FormatNetworkFirewallLog:
		return e.getReaderForData(buf)

	case constants.FormatS3AccessLog:
		return bytesEncoding, bytes.NewReader(buf), nil

	case constants.FormatVPCFlowLog:
		switch e.cfg.VPCFlowLogConfig.FileFormat {
		case constants.FileFormatParquet:
			return parquetEncoding, nil, fmt.Errorf("%q still needs to be implemented", constants.FileFormatParquet)
		case constants.FileFormatPlainText:
			return e.getReaderForData(buf)
		default:
			// should not be possible
			return "", nil, fmt.Errorf(
				"unsupported file fileFormat %q for VPC flow log, expected one of %q",
				e.cfg.VPCFlowLogConfig.FileFormat,
				supportedVPCFlowLogFileFormat,
			)
		}

	default:
		// should not be possible
		return "", nil, fmt.Errorf("unimplemented: format %q has no reader", e.format)
	}
}
