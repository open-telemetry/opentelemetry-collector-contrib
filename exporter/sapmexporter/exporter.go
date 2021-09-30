// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package sapmexporter exports trace data using Splunk's SAPM protocol.
package sapmexporter

import (
	"context"

	"github.com/jaegertracing/jaeger/model"
	sapmclient "github.com/signalfx/sapm-proto/client"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/consumer/consumererror"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
	"go.opentelemetry.io/collector/model/pdata"
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

func newSAPMExporter(cfg *Config, params component.ExporterCreateSettings) (sapmExporter, error) {
	err := cfg.validate()
	if err != nil {
		return sapmExporter{}, err
	}

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

func newSAPMTracesExporter(cfg *Config, set component.ExporterCreateSettings) (component.TracesExporter, error) {
	se, err := newSAPMExporter(cfg, set)
	if err != nil {
		return nil, err
	}

	te, err := exporterhelper.NewTracesExporter(
		cfg,
		set,
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
func (se *sapmExporter) pushTraceData(ctx context.Context, td pdata.Traces) error {
	rss := td.ResourceSpans()
	if rss.Len() == 0 {
		return nil
	}

	// All metrics in the pdata.Metrics will have the same access token because of the BatchPerResourceMetrics.
	accessToken := se.retrieveAccessToken(rss.At(0))
	batches, err := jaeger.InternalTracesToJaegerProto(td)
	if err != nil {
		return consumererror.NewPermanent(err)
	}

	// Cannot remove the access token from the pdata, because exporters required to not modify incoming pdata,
	// so need to remove that after conversion.
	filterToken(batches)

	err = se.client.ExportWithAccessToken(ctx, batches, accessToken)
	if err != nil {
		if sendErr, ok := err.(*sapmclient.ErrSend); ok && sendErr.Permanent {
			return consumererror.NewPermanent(sendErr)
		}
		return err
	}

	return nil
}

func (se *sapmExporter) retrieveAccessToken(md pdata.ResourceSpans) string {
	if !se.config.AccessTokenPassthrough {
		// Nothing to do if token is pass through not configured or resource is nil.
		return ""
	}

	attrs := md.Resource().Attributes()
	if accessToken, ok := attrs.Get(splunk.SFxAccessTokenLabel); ok {
		return accessToken.StringVal()
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
