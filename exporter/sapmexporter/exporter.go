// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Package sapmexporter exports trace data using Splunk's SAPM protocol.
package sapmexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/sapmexporter"

import (
	"context"
	"errors"

	"github.com/jaegertracing/jaeger/model"
	sapmclient "github.com/signalfx/sapm-proto/client"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/consumer/consumererror"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/splunk"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/batchperresourceattr"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/jaeger"
)

// TODO: Find a place for this to be shared.
type baseTracesExporter struct {
	component.Component
	consumer.Traces
}

// sapmExporter is a wrapper struct of SAPM exporter
type sapmExporter struct {
	client *sapmclient.Client
	logger *zap.Logger
	config *Config
}

func (se *sapmExporter) Shutdown(context.Context) error {
	se.client.Stop()
	return nil
}

func newSAPMExporter(cfg *Config, params exporter.CreateSettings) (sapmExporter, error) {

	client, err := sapmclient.New(cfg.clientOptions()...)
	if err != nil {
		return sapmExporter{}, err
	}

	return sapmExporter{
		client: client,
		logger: params.Logger,
		config: cfg,
	}, err
}

func newSAPMTracesExporter(cfg *Config, set exporter.CreateSettings) (exporter.Traces, error) {
	se, err := newSAPMExporter(cfg, set)
	if err != nil {
		return nil, err
	}

	te, err := exporterhelper.NewTracesExporter(
		context.TODO(),
		set,
		cfg,
		se.pushTraceData,
		exporterhelper.WithShutdown(se.Shutdown),
		exporterhelper.WithQueue(cfg.QueueSettings),
		exporterhelper.WithRetry(cfg.RetrySettings),
		exporterhelper.WithTimeout(cfg.TimeoutSettings),
	)

	if err != nil {
		return nil, err
	}

	// If AccessTokenPassthrough enabled, split the incoming Traces data by splunk.SFxAccessTokenLabel,
	// this ensures that we get batches of data for the same token when pushing to the backend.
	if cfg.AccessTokenPassthrough {
		te = &baseTracesExporter{
			Component: te,
			Traces:    batchperresourceattr.NewBatchPerResourceTraces(splunk.SFxAccessTokenLabel, te),
		}
	}
	return te, nil
}

// pushTraceData exports traces in SAPM proto by associated SFx access token and returns number of dropped spans
// and the last experienced error if any translation or export failed
func (se *sapmExporter) pushTraceData(ctx context.Context, td ptrace.Traces) error {
	rss := td.ResourceSpans()
	if rss.Len() == 0 {
		return nil
	}

	// All metrics in the pmetric.Metrics will have the same access token because of the BatchPerResourceMetrics.
	accessToken := se.retrieveAccessToken(rss.At(0))
	batches, err := jaeger.ProtoFromTraces(td)
	if err != nil {
		return consumererror.NewPermanent(err)
	}

	// Cannot remove the access token from the pdata, because exporters required to not modify incoming pdata,
	// so need to remove that after conversion.
	filterToken(batches)

	ingestResponse, err := se.client.ExportWithAccessTokenAndGetResponse(ctx, batches, accessToken)
	if se.config.LogDetailedResponse && ingestResponse != nil {
		if ingestResponse.Err != nil {
			se.logger.Debug("Failed to get response from trace ingest", zap.Error(ingestResponse.Err))
		} else {
			se.logger.Debug("Detailed response from ingest", zap.ByteString("response", ingestResponse.Body))
		}
	}

	if err != nil {
		sendErr := &sapmclient.ErrSend{}
		if errors.As(err, &sendErr) && sendErr.Permanent {
			return consumererror.NewPermanent(sendErr)
		}
		return err
	}

	return nil
}

func (se *sapmExporter) retrieveAccessToken(md ptrace.ResourceSpans) string {
	if !se.config.AccessTokenPassthrough {
		// Nothing to do if token is pass through not configured or resource is nil.
		return ""
	}

	attrs := md.Resource().Attributes()
	if accessToken, ok := attrs.Get(splunk.SFxAccessTokenLabel); ok {
		return accessToken.Str()
	}
	return ""
}

// filterToken filters the access token from the batch processor to avoid leaking credentials to the backend.
func filterToken(batches []*model.Batch) {
	for _, batch := range batches {
		filterTokenFromProcess(batch.Process)
	}
}

func filterTokenFromProcess(proc *model.Process) {
	if proc == nil {
		return
	}
	for i := 0; i < len(proc.Tags); {
		if proc.Tags[i].Key == splunk.SFxAccessTokenLabel {
			proc.Tags[i] = proc.Tags[len(proc.Tags)-1]
			// We do not need to put proc.Tags[i] at the end, as it will be discarded anyway
			proc.Tags = proc.Tags[:len(proc.Tags)-1]
			continue
		}
		i++
	}
}
