// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package lokiexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/lokiexporter"

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"

	"github.com/gogo/protobuf/proto"
	"github.com/golang/snappy"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer/consumererror"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.uber.org/multierr"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/lokiexporter/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/loki"
)

const (
	maxErrMsgLen          = 1024
	missingLabelsErrorMsg = "error at least one label pair is required per stream"
)

type lokiExporter struct {
	config   *Config
	settings component.TelemetrySettings
	client   *http.Client
	wg       sync.WaitGroup

	telemetryBuilder *metadata.TelemetryBuilder
}

func newExporter(config *Config, settings component.TelemetrySettings) (*lokiExporter, error) {
	settings.Logger.Info("using the new Loki exporter")

	builder, err := metadata.NewTelemetryBuilder(settings)
	if err != nil {
		return nil, err
	}

	return &lokiExporter{
		config:           config,
		settings:         settings,
		telemetryBuilder: builder,
	}, nil
}

func (l *lokiExporter) pushLogData(ctx context.Context, ld plog.Logs) error {
	requests := loki.LogsToLokiRequests(ld, l.config.DefaultLabelsEnabled)

	var errs error
	for tenant, request := range requests {
		err := l.sendPushRequest(ctx, tenant, request, ld)
		if isErrMissingLabels(err) {
			l.telemetryBuilder.LokiexporterSendFailedDueToMissingLabels.Add(ctx, int64(ld.LogRecordCount()))
		}

		errs = multierr.Append(errs, err)
	}

	return errs
}

func (l *lokiExporter) sendPushRequest(ctx context.Context, tenant string, request loki.PushRequest, ld plog.Logs) error {
	pushReq := request.PushRequest
	report := request.Report
	if len(pushReq.Streams) == 0 {
		return consumererror.NewPermanent(fmt.Errorf("failed to transform logs into Loki log streams"))
	}
	if len(report.Errors) > 0 {
		l.settings.Logger.Info(
			"not all log entries were converted to Loki",
			zap.Int("dropped", report.NumDropped),
			zap.Int("submitted", report.NumSubmitted),
		)
	}

	buf, err := encode(pushReq)
	if err != nil {
		return consumererror.NewPermanent(err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, l.config.Endpoint, bytes.NewReader(buf))
	if err != nil {
		return consumererror.NewPermanent(err)
	}

	for k, v := range l.config.Headers {
		req.Header.Set(k, string(v))
	}
	req.Header.Set("Content-Type", "application/x-protobuf")
	if len(tenant) > 0 {
		req.Header.Set("X-Scope-OrgID", tenant)
	}

	resp, err := l.client.Do(req)
	if err != nil {
		return consumererror.NewLogs(err, ld)
	}

	defer func() {
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()
	}()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		scanner := bufio.NewScanner(io.LimitReader(resp.Body, maxErrMsgLen))
		line := ""
		if scanner.Scan() {
			line = scanner.Text()
		}
		err = fmt.Errorf("HTTP %d %q: %s", resp.StatusCode, http.StatusText(resp.StatusCode), line)

		// Errors with 4xx status code (excluding 429) should not be retried
		if resp.StatusCode >= http.StatusBadRequest &&
			resp.StatusCode < http.StatusInternalServerError &&
			resp.StatusCode != http.StatusTooManyRequests {
			return consumererror.NewPermanent(err)
		}

		return consumererror.NewLogs(err, ld)
	}

	return nil
}

func encode(pb proto.Message) ([]byte, error) {
	buf, err := proto.Marshal(pb)
	if err != nil {
		return nil, err
	}
	buf = snappy.Encode(nil, buf)
	return buf, nil
}

func (l *lokiExporter) start(ctx context.Context, host component.Host) (err error) {
	client, err := l.config.ToClient(ctx, host, l.settings)
	if err != nil {
		return err
	}

	l.client = client

	return nil
}

func (l *lokiExporter) stop(context.Context) (err error) {
	l.wg.Wait()
	return nil
}

func isErrMissingLabels(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), missingLabelsErrorMsg)
}
