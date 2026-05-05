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
	"github.com/tidwall/gjson"
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
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/xstreamencoding"
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

	// selfID is the component ID of this extension, used for cycle detection
	// when resolving inner encoding extensions in the CloudWatch route router.
	selfID component.ID
	// router is built at Start time when the format is the CloudWatch
	// subscription-filter and Config.CloudWatch.Streams is non-empty.
	router *subscriptionfilter.Router
}

func newExtension(cfg *Config, settings extension.Settings) (*encodingExtension, error) {
	switch cfg.Format {
	case constants.FormatCloudWatchLogsSubscriptionFilter, constants.FormatCloudWatchLogsSubscriptionFilterV1:
		if cfg.Format == constants.FormatCloudWatchLogsSubscriptionFilterV1 {
			settings.Logger.Warn("using old format value. This format will be removed in version 0.138.0.",
				zap.String("old_format", constants.FormatCloudWatchLogsSubscriptionFilterV1),
				zap.String("new_format", constants.FormatCloudWatchLogsSubscriptionFilter),
			)
		}
		return &encodingExtension{
			cfg:         cfg,
			unmarshaler: subscriptionfilter.NewSubscriptionFilterUnmarshaler(settings.BuildInfo),
			format:      constants.FormatCloudWatchLogsSubscriptionFilter,
			logger:      settings.Logger,
			selfID:      settings.ID,
		}, nil
	case constants.FormatVPCFlowLog, constants.FormatVPCFlowLogV1:
		if cfg.Format == constants.FormatVPCFlowLogV1 {
			settings.Logger.Warn("using old format value. This format will be removed in version 0.138.0.",
				zap.String("old_format", constants.FormatVPCFlowLogV1),
				zap.String("new_format", constants.FormatVPCFlowLog),
			)
		}
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
	case constants.FormatS3AccessLog, constants.FormatS3AccessLogV1:
		if cfg.Format == constants.FormatS3AccessLogV1 {
			settings.Logger.Warn("using old format value. This format will be removed in version 0.138.0.",
				zap.String("old_format", constants.FormatS3AccessLogV1),
				zap.String("new_format", constants.FormatS3AccessLog),
			)
		}
		return &encodingExtension{
			unmarshaler: s3accesslog.NewS3AccessLogUnmarshaler(settings.BuildInfo),
			format:      constants.FormatS3AccessLog,
			logger:      settings.Logger,
		}, nil
	case constants.FormatWAFLog, constants.FormatWAFLogV1:
		if cfg.Format == constants.FormatWAFLogV1 {
			settings.Logger.Warn("using old format value. This format will be removed in version 0.138.0.",
				zap.String("old_format", constants.FormatWAFLogV1),
				zap.String("new_format", constants.FormatWAFLog),
			)
		}
		return &encodingExtension{
			unmarshaler: waf.NewWAFLogUnmarshaler(settings.BuildInfo),
			format:      constants.FormatWAFLog,
			logger:      settings.Logger,
		}, nil
	case constants.FormatCloudTrailLog, constants.FormatCloudTrailLogV1:
		if cfg.Format == constants.FormatCloudTrailLogV1 {
			settings.Logger.Warn("using old format value. This format will be removed in version 0.138.0.",
				zap.String("old_format", constants.FormatCloudTrailLogV1),
				zap.String("new_format", constants.FormatCloudTrailLog),
			)
		}
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
	case constants.FormatELBAccessLog, constants.FormatELBAccessLogV1:
		if cfg.Format == constants.FormatELBAccessLogV1 {
			settings.Logger.Warn("using old format value. This format will be removed in version 0.138.0.",
				zap.String("old_format", constants.FormatELBAccessLogV1),
				zap.String("new_format", constants.FormatELBAccessLog),
			)
		}
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

func (e *encodingExtension) Start(_ context.Context, host component.Host) error {
	// Routing is only meaningful for the CloudWatch subscription-filter format.
	if e.format != constants.FormatCloudWatchLogsSubscriptionFilter ||
		len(e.cfg.CloudWatch.Streams) == 0 {
		return nil
	}

	router, err := subscriptionfilter.NewRouter(e.cfg.CloudWatch.Streams, host, e.selfID)
	if err != nil {
		return fmt.Errorf("failed to build CloudWatch route router: %w", err)
	}
	e.router = router
	return nil
}

func (*encodingExtension) Shutdown(_ context.Context) error {
	return nil
}

func (e *encodingExtension) UnmarshalLogs(buf []byte) (plog.Logs, error) {
	if e.router != nil {
		return e.unmarshalLogsViaRouter(buf)
	}

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
	if e.router != nil {
		return e.newLogsDecoderViaRouter(reader)
	}

	if u, ok := e.unmarshaler.(awsunmarshaler.StreamingLogsUnmarshaler); ok {
		return u.NewLogsDecoder(reader, options...)
	}

	return nil, fmt.Errorf("streaming not supported for format %q", e.format)
}

// unmarshalLogsViaRouter handles the routing path for UnmarshalLogs: it
// decompresses the incoming CloudWatch subscription event, peeks logGroup /
// logStream, picks the matching inner encoding via the router, and delegates
// the (decompressed) envelope bytes to it.
func (e *encodingExtension) unmarshalLogsViaRouter(buf []byte) (plog.Logs, error) {
	envelope, release, err := e.decompressEnvelope(buf)
	if err != nil {
		return plog.Logs{}, fmt.Errorf("failed to decompress CloudWatch event: %w", err)
	}
	defer release()

	logGroup := gjson.GetBytes(envelope, "logGroup").String()
	logStream := gjson.GetBytes(envelope, "logStream").String()

	inner, name, err := e.router.Match(logGroup, logStream)
	if err != nil {
		return plog.Logs{}, err
	}

	logs, err := inner.UnmarshalLogs(envelope)
	if err != nil {
		return plog.Logs{}, fmt.Errorf("route %q: inner encoding failed: %w", name, err)
	}
	return logs, nil
}

// newLogsDecoderViaRouter handles the routing path for NewLogsDecoder. The
// path is one-shot: read the full envelope, peek routing keys, dispatch to
// the inner encoding's UnmarshalLogs, and wrap the resulting logs in an
// adapter that yields them once and then EOF.
func (e *encodingExtension) newLogsDecoderViaRouter(reader io.Reader) (encoding.LogsDecoder, error) {
	envelope, err := io.ReadAll(reader)
	if err != nil {
		return nil, fmt.Errorf("failed to read CloudWatch envelope: %w", err)
	}

	logGroup := gjson.GetBytes(envelope, "logGroup").String()
	logStream := gjson.GetBytes(envelope, "logStream").String()

	inner, name, err := e.router.Match(logGroup, logStream)
	if err != nil {
		return nil, err
	}

	logs, err := inner.UnmarshalLogs(envelope)
	if err != nil {
		return nil, fmt.Errorf("route %q: inner encoding failed: %w", name, err)
	}

	emitted := false
	return xstreamencoding.NewLogsDecoderAdapter(
		func() (plog.Logs, error) {
			if emitted {
				return plog.NewLogs(), io.EOF
			}
			emitted = true
			return logs, nil
		},
		func() int64 { return int64(len(envelope)) },
	), nil
}

// decompressEnvelope returns the gunzipped bytes of buf when buf is gzip
// compressed, otherwise returns buf as-is.
func (e *encodingExtension) decompressEnvelope(buf []byte) ([]byte, func(), error) {
	if !isGzipData(buf) {
		return buf, func() {}, nil
	}
	r, err := e.getGzipReader(buf)
	if err != nil {
		return nil, nil, err
	}
	gz := r.(*gzip.Reader)
	release := func() {
		_ = gz.Close()
		e.gzipPool.Put(gz)
	}
	decompressed, err := io.ReadAll(gz)
	if err != nil {
		release()
		return nil, nil, err
	}
	return decompressed, release, nil
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
